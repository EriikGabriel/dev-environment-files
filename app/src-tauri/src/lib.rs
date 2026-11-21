//! Thin Tauri layer over the `envsetup` Go binary (the sidecar). The Go side
//! owns all logic; this file only spawns it, validates arguments, and forwards
//! its JSON output to the React front-end.

use std::{
    io::{BufRead, BufReader, Write},
    path::{Path, PathBuf},
    process::{Child, Command, Stdio},
    sync::{Arc, Mutex},
};

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Emitter, Manager, State};

/// Subcommands the front-end may run through [`envsetup`].
const ALLOWED: &[&str] = &[
    "detect",
    "info",
    "profiles",
    "steps",
    "plan",
    "add",
    "show",
    "set-dirs",
    "desktop",
    "set-dock",
    "add-app",
    "remove-app",
    "detach",
    "windhawk-island",
    "ensure-deno",
    "media",
    "set-ninite",
    "set-unattend",
    "write-media",
    "open-url",
    "policies",
    "set-policy",
    "apply-policies",
    "open-policy",
    "layout",
    "set-layout",
    "apply-layout",
    "icons",
    "set-icons",
    "apply-icons",
    "restore-icons",
    "icon-pack",
    "taskbar-dock",
    "playnite",
    "set-playnite",
    "playnite-save",
    "playnite-restore",
    "playnite-fix",
    "playnite-open",
    "retroarch",
    "set-retroarch",
    "retroarch-save",
    "retroarch-restore",
    "retroarch-fix",
    "retroarch-open",
    "retroarch-move-games",
    "playnite-close",
    "backup",
    "backup-push",
    "backup-pull",
    "backup-clone",
    "role",
    "set-role",
    "windhawk",
    "windhawk-export",
    "windhawk-import",
    "app-update-check",
    "app-update-download",
];

#[derive(Default)]
struct ApplyState(Mutex<Option<Arc<Mutex<Child>>>>);

#[derive(Default, Serialize, Deserialize)]
struct Settings {
    root: Option<String>,
}

fn is_repo(dir: &Path) -> bool {
    dir.join("config").join("apps.yaml").is_file()
}

fn settings_path(app: &AppHandle) -> Result<PathBuf, String> {
    let dir = app.path().app_config_dir().map_err(|e| e.to_string())?;
    std::fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
    Ok(dir.join("settings.json"))
}

fn read_settings(app: &AppHandle) -> Settings {
    settings_path(app)
        .ok()
        .and_then(|p| std::fs::read_to_string(p).ok())
        .and_then(|s| serde_json::from_str(&s).ok())
        .unwrap_or_default()
}

/// Walks up from `start` looking for a directory that holds config/apps.yaml.
fn find_upwards(start: &Path) -> Option<PathBuf> {
    start
        .ancestors()
        .find(|d| is_repo(d))
        .map(Path::to_path_buf)
}

/// Where the data lives (config/, dotfiles/, windows/): the saved choice,
/// then $ENVSETUP_ROOT, then the nearest parent of the working directory or
/// of this executable (a git clone, in development), then the folder the
/// installer's data was copied to. None until the user picks one.
#[tauri::command]
fn get_root(app: AppHandle) -> Option<String> {
    let saved = read_settings(&app)
        .root
        .map(PathBuf::from)
        .filter(|p| is_repo(p));
    let env = std::env::var_os("ENVSETUP_ROOT")
        .map(PathBuf::from)
        .filter(|p| is_repo(p));
    let cwd = std::env::current_dir().ok().and_then(|d| find_upwards(&d));
    let exe = std::env::current_exe()
        .ok()
        .and_then(|e| e.parent().and_then(find_upwards));
    let seeded = default_data_dir(&app).ok().filter(|p| is_repo(p));
    saved
        .or(env)
        .or(cwd)
        .or(exe)
        .or(seeded)
        .map(|p| p.to_string_lossy().into_owned())
}

fn save_root(app: &AppHandle, dir: &Path) -> Result<String, String> {
    let root = dir.to_string_lossy().into_owned();
    let s = Settings {
        root: Some(root.clone()),
    };
    std::fs::write(
        settings_path(app)?,
        serde_json::to_string_pretty(&s).unwrap(),
    )
    .map_err(|e| e.to_string())?;
    Ok(root)
}

#[tauri::command]
fn set_root(app: AppHandle, root: String) -> Result<String, String> {
    let dir = PathBuf::from(&root);
    if !is_repo(&dir) {
        return Err(format!("{root} has no config/apps.yaml"));
    }
    save_root(&app, &dir)
}

// ---- the data shipped in the installer ----
//
// The installer carries config/, dotfiles/ and windows/ under resources/data
// (see bundle.resources in tauri.conf.json). The install folder is read-only,
// so on first run they are copied to a folder of the user, which becomes the
// root and is edited from then on. The copy never overwrites a file.

/// Marker written in a folder made from the installer's data.
const DATA_MARKER: &str = ".envsetup-data";

/// The data shipped with this build; None in a development build.
fn bundled_data(app: &AppHandle) -> Option<PathBuf> {
    let dir = app.path().resource_dir().ok()?.join("data");
    is_repo(&dir).then_some(dir)
}

/// Where the shipped data is copied: Documents\envsetup on Windows,
/// ~/envsetup elsewhere.
fn default_data_dir(app: &AppHandle) -> Result<PathBuf, String> {
    let base = if cfg!(windows) {
        app.path().document_dir()
    } else {
        app.path().home_dir()
    };
    base.map(|d| d.join("envsetup")).map_err(|e| e.to_string())
}

#[derive(Default, Serialize)]
struct CopyReport {
    /// Files copied because they were missing.
    added: usize,
    /// Files that were already there and were left as they are.
    kept: usize,
}

/// Copies the files of `src` that `dst` does not have. A file that exists is
/// never touched, so the user's edits survive; symlinks are skipped.
fn copy_missing(src: &Path, dst: &Path, report: &mut CopyReport) -> std::io::Result<()> {
    std::fs::create_dir_all(dst)?;
    for entry in std::fs::read_dir(src)? {
        let entry = entry?;
        let kind = entry.file_type()?;
        let to = dst.join(entry.file_name());
        if kind.is_dir() {
            copy_missing(&entry.path(), &to, report)?;
        } else if kind.is_file() {
            match std::fs::OpenOptions::new().write(true).create_new(true).open(&to) {
                Ok(mut out) => {
                    let mut from = std::fs::File::open(entry.path())?;
                    std::io::copy(&mut from, &mut out)?;
                    report.added += 1;
                }
                Err(e) if e.kind() == std::io::ErrorKind::AlreadyExists => report.kept += 1,
                Err(e) => return Err(e),
            }
        }
    }
    Ok(())
}

/// Updates the 8BitDo catalogue entry even in data folders made by an older
/// envsetup release. The entry is maintained by envsetup, unlike arbitrary
/// edits in the rest of apps.yaml.
fn migrate_8bitdo_v1(dst: &Path) -> std::io::Result<bool> {
    let path = dst.join("config").join("apps.yaml");
    let text = match std::fs::read_to_string(&path) {
        Ok(text) => text,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(false),
        Err(e) => return Err(e),
    };
    let start = match text.find("  8bitdo-ultimate-software:\n") {
        Some(start) => start,
        None => return Ok(false),
    };
    let end = text[start..]
        .find("\n  steam:")
        .map(|offset| start + offset)
        .unwrap_or(text.len());
    let new_block = concat!(
        "  8bitdo-ultimate-software:\n",
        "    description: 8BitDo Ultimate Software V1 (buttons, sticks, triggers and profiles of the 8BitDo Ultimate 2.4G first-generation controller)\n",
        "    devices: [VID_2DC8, 8BitDo]\n",
        "    method: { windows: script }\n",
        "    check:\n",
        "      windows: if (-not ((Test-Path -LiteralPath \"$env:LOCALAPPDATA\\Programs\\8BitDo Ultimate Software V1\\8BitDo Ultimate Software.exe\") -and (Test-Path -LiteralPath \"$env:APPDATA\\Microsoft\\Windows\\Start Menu\\Programs\\8BitDo Ultimate Software V1.lnk\"))) { exit 1 }\n",
        "    source:\n",
        "      windows: |\n",
        "        & \"$env:ENVSETUP_ROOT\\windows\\install-8bitdo-v1.ps1\""
    );
    if text[start..end].starts_with(new_block) {
        return Ok(false);
    }
    let mut migrated = String::with_capacity(text.len());
    migrated.push_str(&text[..start]);
    migrated.push_str(new_block);
    migrated.push_str(&text[end..]);
    std::fs::write(path, migrated)?;
    Ok(true)
}

fn write_marker(app: &AppHandle, dir: &Path) -> Result<(), String> {
    let marker = serde_json::json!({ "version": app.package_info().version.to_string() });
    std::fs::write(dir.join(DATA_MARKER), marker.to_string()).map_err(|e| e.to_string())
}

#[derive(Serialize)]
struct DataInfo {
    /// This build carries the data (an installed app, not a dev build).
    bundled: bool,
    /// Where "use the included data" copies it.
    default_dir: Option<String>,
    /// The root is a git clone: it is updated with git, not from the app.
    git: bool,
    /// Version of the app whose data was copied to the root (None when the
    /// root was not made from the installer's data).
    seeded_version: Option<String>,
    app_version: String,
}

/// What the data of `root` is and whether the installer's data can update it.
#[tauri::command]
fn data_info(app: AppHandle, root: Option<String>) -> DataInfo {
    let root = root.map(PathBuf::from);
    let seeded_version = root
        .as_ref()
        .and_then(|r| std::fs::read_to_string(r.join(DATA_MARKER)).ok())
        .and_then(|s| serde_json::from_str::<serde_json::Value>(&s).ok())
        .and_then(|v| v["version"].as_str().map(String::from));
    DataInfo {
        bundled: bundled_data(&app).is_some(),
        default_dir: default_data_dir(&app)
            .ok()
            .map(|p| p.to_string_lossy().into_owned()),
        git: root.as_ref().is_some_and(|r| r.join(".git").exists()),
        seeded_version,
        app_version: app.package_info().version.to_string(),
    }
}

/// Copies the installer's data to the default folder (only what is missing
/// there) and makes it the root.
#[tauri::command]
fn seed_data(app: AppHandle) -> Result<String, String> {
    let src = bundled_data(&app).ok_or("this build carries no data: choose a folder with the repo")?;
    let dst = default_data_dir(&app)?;
    let mut report = CopyReport::default();
    copy_missing(&src, &dst, &mut report).map_err(|e| format!("copy the data to {}: {e}", dst.display()))?;
    migrate_8bitdo_v1(&dst).map_err(|e| format!("migrate 8BitDo data: {e}"))?;
    write_marker(&app, &dst)?;
    save_root(&app, &dst)
}

/// Adds to `root` the files of the installer's data it does not have (new
/// profiles, icons, scripts of a newer version). Nothing that exists changes.
#[tauri::command]
fn update_data(app: AppHandle, root: String) -> Result<CopyReport, String> {
    let dst = PathBuf::from(&root);
    if dst.join(".git").exists() {
        return Err(format!("{root} is a git clone: update it with git pull"));
    }
    let src = bundled_data(&app).ok_or("this build carries no data")?;
    let mut report = CopyReport::default();
    copy_missing(&src, &dst, &mut report).map_err(|e| e.to_string())?;
    migrate_8bitdo_v1(&dst).map_err(|e| e.to_string())?;
    write_marker(&app, &dst)?;
    Ok(report)
}

/// Finds the bundled `envsetup` binary: $ENVSETUP_BIN, else next to this
/// executable (where Tauri places externalBin, in dev and in releases).
fn sidecar() -> Result<PathBuf, String> {
    if let Some(p) = std::env::var_os("ENVSETUP_BIN") {
        return Ok(PathBuf::from(p));
    }
    let exe = std::env::current_exe().map_err(|e| e.to_string())?;
    let dir = exe.parent().ok_or("executable has no parent directory")?;
    ["envsetup", "envsetup.exe"]
        .iter()
        .map(|n| dir.join(n))
        .find(|p| p.is_file())
        .ok_or_else(|| format!("envsetup binary not found next to {}", exe.display()))
}

/// A command for the sidecar. On Windows it is created without a console
/// window, which would otherwise flash on every call.
fn sidecar_command(bin: PathBuf) -> Command {
    #[allow(unused_mut)]
    let mut cmd = Command::new(bin);
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        cmd.creation_flags(0x0800_0000); // CREATE_NO_WINDOW
    }
    cmd
}

/// Runs a read-style subcommand with --json and returns the parsed output.
#[tauri::command]
async fn envsetup(root: String, args: Vec<String>) -> Result<serde_json::Value, String> {
    let sub = args.first().map(String::as_str).unwrap_or_default();
    if !ALLOWED.contains(&sub) {
        return Err(format!("subcommand {sub:?} is not allowed"));
    }
    let bin = sidecar()?;
    tauri::async_runtime::spawn_blocking(move || {
        let out = sidecar_command(bin)
            .arg("--root")
            .arg(&root)
            .args(&args)
            .arg("--json")
            .stdin(Stdio::null())
            .output()
            .map_err(|e| e.to_string())?;
        if !out.status.success() {
            let err = String::from_utf8_lossy(&out.stderr);
            // cobra prints "Error: message" last; the commands that do work
            // print their progress to stderr before it
            let line = err
                .lines()
                .rev()
                .find_map(|l| l.trim().strip_prefix("Error: "))
                .or_else(|| err.trim().lines().next())
                .unwrap_or("failed");
            return Err(line.to_string());
        }
        serde_json::from_slice(&out.stdout).map_err(|e| format!("invalid JSON from envsetup: {e}"))
    })
    .await
    .map_err(|e| e.to_string())?
}

/// Runs `apply --json`, emitting each NDJSON line as an `apply-event`.
#[tauri::command]
async fn apply(
    app: AppHandle,
    state: State<'_, ApplyState>,
    root: String,
    profile: String,
    choices: Option<std::collections::BTreeMap<String, String>>,
    apps: Option<Vec<String>>,
    vars: Option<std::collections::BTreeMap<String, String>>,
    no_files: bool,
    sudo_password: Option<String>,
) -> Result<(), String> {
    let bin = sidecar()?;
    let mut cmd = sidecar_command(bin);
    cmd.arg("--root")
        .arg(&root)
        .arg("-p")
        .arg(&profile)
        .arg("apply")
        .arg("--json");
    if let Some(list) = apps {
        // An empty selection must select nothing, not everything.
        if list.is_empty() && no_files {
            return Err("nothing selected".into());
        }
        if list.is_empty() {
            cmd.arg("--no-apps");
        } else {
            cmd.arg("--apps").arg(list.join(","));
        }
    }
    for (step, option) in choices.unwrap_or_default() {
        cmd.arg("--choose").arg(format!("{step}={option}"));
    }
    for (key, value) in vars.unwrap_or_default() {
        cmd.arg("--set").arg(format!("{key}={value}"));
    }
    if no_files {
        cmd.arg("--no-files");
    }
    let use_pw = sudo_password.as_deref().is_some_and(|p| !p.is_empty());
    if use_pw {
        cmd.arg("--sudo-password-stdin");
    }
    cmd.stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    let child = {
        let mut slot = state.0.lock().unwrap();
        if slot.is_some() {
            return Err("an apply is already running".into());
        }
        let child = cmd.spawn().map_err(|e| e.to_string())?;
        let shared = Arc::new(Mutex::new(child));
        *slot = Some(shared.clone());
        shared
    };

    let (stdin, stdout, stderr) = {
        let mut c = child.lock().unwrap();
        (c.stdin.take(), c.stdout.take(), c.stderr.take())
    };
    if let Some(mut stdin) = stdin {
        if use_pw {
            let _ = writeln!(stdin, "{}", sudo_password.unwrap());
        }
        // dropping stdin closes it, so nothing can block waiting for input
    }

    let handle = app.clone();
    let result = tauri::async_runtime::spawn_blocking(move || {
        let err_thread = stderr.map(|e| {
            std::thread::spawn(move || {
                BufReader::new(e)
                    .lines()
                    .map_while(Result::ok)
                    .collect::<Vec<_>>()
                    .join("\n")
            })
        });
        if let Some(out) = stdout {
            for line in BufReader::new(out).lines().map_while(Result::ok) {
                let _ = handle.emit("apply-event", line);
            }
        }
        let stderr_text = err_thread.and_then(|t| t.join().ok()).unwrap_or_default();
        let status = child.lock().unwrap().wait().map_err(|e| e.to_string())?;
        if status.success() {
            Ok(())
        } else {
            let msg = stderr_text.trim().trim_start_matches("Error: ").to_string();
            Err(if msg.is_empty() {
                format!("envsetup exited with {status}")
            } else {
                msg
            })
        }
    })
    .await
    .map_err(|e| e.to_string())?;

    *state.0.lock().unwrap() = None;
    result
}

#[tauri::command]
fn cancel_apply(state: State<'_, ApplyState>) {
    if let Some(child) = state.0.lock().unwrap().as_ref() {
        let _ = child.lock().unwrap().kill();
    }
}

/// Where `envsetup app-update-download` puts the installer.
#[cfg_attr(not(windows), allow(dead_code))]
fn update_dir() -> PathBuf {
    std::env::temp_dir().join("envsetup-update")
}

/// Checks that `path` is the installer the sidecar downloaded: an envsetup
/// `-setup.exe` directly inside `dir`. The front-end hands the path over, so it
/// is not trusted to name any other file.
#[cfg_attr(not(windows), allow(dead_code))]
fn check_installer(path: &Path, dir: &Path) -> Result<PathBuf, String> {
    let file = path.canonicalize().map_err(|e| format!("the installer is gone: {e}"))?;
    let dir = dir.canonicalize().map_err(|e| format!("no download folder: {e}"))?;
    if file.parent() != Some(dir.as_path()) {
        return Err("the installer is not in the download folder".into());
    }
    let name = file.file_name().and_then(|n| n.to_str()).unwrap_or_default();
    if !(name.starts_with("envsetup") && name.ends_with("-setup.exe")) {
        return Err(format!("{name:?} is not an envsetup installer"));
    }
    Ok(file)
}

/// Quotes a path for a single-quoted PowerShell string.
#[cfg_attr(not(windows), allow(dead_code))]
fn ps_quote(p: &Path) -> String {
    p.to_string_lossy().replace('\'', "''")
}

/// The PowerShell that runs the installer silently (NSIS `/S`) and, when it is
/// done, opens the app: the new version if the install worked, the old one if it
/// did not. It runs after the app has exited, which is what lets the installer
/// replace the app's files. Each step goes to `update.log` next to the
/// installer, to see where an update stopped.
#[cfg_attr(not(windows), allow(dead_code))]
fn update_script(installer: &Path, exe: &Path) -> String {
    let log = installer.with_file_name("update.log");
    format!(
        "$log = '{log}'; function Log($s) {{ \"$(Get-Date -Format s) $s\" | Out-File -LiteralPath $log -Append -Encoding UTF8 }}; \
         try {{ Log 'running the installer'; $p = Start-Process -FilePath '{inst}' -ArgumentList '/S' -Wait -PassThru; Log \"installer exited with $($p.ExitCode)\" }} \
         catch {{ Log \"installer failed: $($_.Exception.Message)\" }}; \
         Start-Sleep -Seconds 1; \
         try {{ Start-Process -FilePath '{exe}'; Log 'app reopened' }} catch {{ Log \"could not reopen the app: $($_.Exception.Message)\" }}",
        log = ps_quote(&log),
        inst = ps_quote(installer),
        exe = ps_quote(exe)
    )
}

/// How the update script is started: CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP.
#[cfg_attr(not(windows), allow(dead_code))]
const UPDATE_FLAGS: u32 = 0x0800_0000 | 0x0000_0200;
/// DETACHED_PROCESS, which the update script must not get.
#[cfg(test)]
const DETACHED_PROCESS: u32 = 0x0000_0008;

/// Installs the update the sidecar downloaded and reopens the app. The app
/// closes itself here so its files are free for the installer.
#[tauri::command]
fn install_update(app: AppHandle, path: String) -> Result<(), String> {
    #[cfg(not(windows))]
    {
        let _ = (app, path);
        Err("the app updates itself on Windows only: install the new version from its package".into())
    }
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        let installer = check_installer(Path::new(&path), &update_dir())?;
        let exe = std::env::current_exe().map_err(|e| e.to_string())?;
        // a build run from the source tree is not an install: the installer would not replace it
        if exe.to_string_lossy().to_lowercase().contains("\\target\\") {
            return Err("this is a development build: install a release to use the update".into());
        }
        Command::new("powershell")
            .args(["-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-Command"])
            .arg(update_script(&installer, &exe))
            // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP: a hidden console of its
            // own, so it outlives the app. Not DETACHED_PROCESS: Windows
            // PowerShell started without a console exits without running the
            // script, which closed the app and never ran the installer.
            .creation_flags(UPDATE_FLAGS)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .map_err(|e| format!("could not start the installer: {e}"))?;
        app.exit(0);
        Ok(())
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .manage(ApplyState::default())
        .invoke_handler(tauri::generate_handler![
            get_root,
            set_root,
            data_info,
            seed_data,
            update_data,
            envsetup,
            apply,
            cancel_apply,
            install_update
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn copy_missing_never_overwrites() {
        let base = std::env::temp_dir().join(format!("envsetup-copy-{}", std::process::id()));
        let (src, dst) = (base.join("src"), base.join("dst"));
        std::fs::create_dir_all(src.join("config")).unwrap();
        std::fs::write(src.join("config").join("apps.yaml"), "new").unwrap();
        std::fs::write(src.join("config").join("added.yaml"), "added").unwrap();
        std::fs::create_dir_all(dst.join("config")).unwrap();
        std::fs::write(dst.join("config").join("apps.yaml"), "edited by the user").unwrap();

        let mut report = CopyReport::default();
        copy_missing(&src, &dst, &mut report).unwrap();
        assert_eq!((report.added, report.kept), (1, 1));
        let read = |p: &str| std::fs::read_to_string(dst.join("config").join(p)).unwrap();
        assert_eq!(read("apps.yaml"), "edited by the user");
        assert_eq!(read("added.yaml"), "added");
        std::fs::remove_dir_all(&base).unwrap();
    }

    #[test]
    fn migrates_the_old_8bitdo_catalog_entry() {
        let base = std::env::temp_dir().join(format!("envsetup-8bitdo-migrate-{}", std::process::id()));
        let dst = base.join("dst");
        std::fs::create_dir_all(dst.join("config")).unwrap();
        std::fs::write(
            dst.join("config").join("apps.yaml"),
            concat!(
                "apps:\n",
                "  8bitdo-ultimate-software:\n",
                "    description: old V2 entry\n",
                "    names: { windows: 8BitDo.UltimateSoftwareV2 }\n",
                "  steam: { description: Steam }\n",
            ),
        )
        .unwrap();

        assert!(migrate_8bitdo_v1(&dst).unwrap());
        let text = std::fs::read_to_string(dst.join("config").join("apps.yaml")).unwrap();
        assert!(text.contains("Ultimate Software V1"));
        assert!(text.contains("install-8bitdo-v1.ps1"));
        assert!(!text.contains("UltimateSoftwareV2"));
        assert!(text.contains("  steam:"));
        assert!(!migrate_8bitdo_v1(&dst).unwrap());
        std::fs::remove_dir_all(&base).unwrap();
    }

    #[test]
    fn only_the_downloaded_installer_can_be_run() {
        let base = std::env::temp_dir().join(format!("envsetup-upd-{}", std::process::id()));
        let dir = base.join("envsetup-update");
        std::fs::create_dir_all(&dir).unwrap();
        let ok = dir.join("envsetup_0.2.1_x64-setup.exe");
        std::fs::write(&ok, "x").unwrap();
        // the right kind of file, in the wrong folder
        let elsewhere = base.join("envsetup_0.2.1_x64-setup.exe");
        std::fs::write(&elsewhere, "x").unwrap();
        // the right folder, the wrong kind of file
        let other = dir.join("calc.exe");
        std::fs::write(&other, "x").unwrap();
        let msi = dir.join("envsetup_0.2.1_x64_en-US.msi");
        std::fs::write(&msi, "x").unwrap();
        // a path that climbs out of the folder lands outside it
        let climbing = dir.join("..").join("envsetup_0.2.1_x64-setup.exe");

        assert!(check_installer(&ok, &dir).is_ok());
        for bad in [&elsewhere, &other, &msi, &climbing, &dir.join("missing-setup.exe")] {
            assert!(check_installer(bad, &dir).is_err(), "{} must be refused", bad.display());
        }
        std::fs::remove_dir_all(&base).unwrap();
    }

    #[test]
    fn the_update_script_runs_silently_then_reopens_the_app() {
        let s = update_script(Path::new(r"C:\Temp\it's\envsetup_0.2.1_x64-setup.exe"), Path::new(r"C:\Apps\envsetup.exe"));
        assert!(s.contains("-ArgumentList '/S' -Wait"), "{s}");
        // the installer is awaited before the app is started again
        assert!(s.find("-Wait").unwrap() < s.rfind("envsetup.exe'").unwrap());
        // a quote in the path must not end the PowerShell string
        assert!(s.contains("it''s"), "{s}");
        // each step is logged next to the installer
        assert!(s.contains(r"C:\Temp\it''s\update.log"), "{s}");
    }

    // Windows PowerShell started with DETACHED_PROCESS exits without running
    // anything: the app closed and the installer never ran. With the flags the
    // update uses, the script runs, waits for the installer and reopens the app.
    #[cfg(windows)]
    #[test]
    fn the_update_script_is_started_so_that_it_runs() {
        use std::os::windows::process::CommandExt;
        assert_eq!(UPDATE_FLAGS & DETACHED_PROCESS, 0);
        // a harmless program as installer and app
        let dir = std::env::temp_dir().join(format!("envsetup-upd-script-{}", std::process::id()));
        std::fs::create_dir_all(&dir).unwrap();
        let whoami = Path::new(r"C:\Windows\System32\whoami.exe");
        let installer = dir.join("envsetup_0.0.0_x64-setup.exe");
        std::fs::copy(whoami, &installer).unwrap();
        let status = Command::new("powershell")
            .args(["-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-Command"])
            .arg(update_script(&installer, whoami))
            .creation_flags(UPDATE_FLAGS)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .unwrap();
        assert!(status.success());
        let log = std::fs::read_to_string(dir.join("update.log")).unwrap();
        assert!(log.contains("installer exited with") && log.contains("app reopened"), "{log}");
        std::fs::remove_dir_all(&dir).unwrap();
    }
}
