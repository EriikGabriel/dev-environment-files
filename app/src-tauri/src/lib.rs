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
const ALLOWED: &[&str] = &["detect", "info", "profiles", "steps", "plan", "add", "show", "set-dirs"];

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
    start.ancestors().find(|d| is_repo(d)).map(Path::to_path_buf)
}

/// Where the repo lives: the saved choice, then $ENVSETUP_ROOT, then the
/// nearest parent of the working directory or of this executable.
#[tauri::command]
fn get_root(app: AppHandle) -> Option<String> {
    let saved = read_settings(&app).root.map(PathBuf::from).filter(|p| is_repo(p));
    let env = std::env::var_os("ENVSETUP_ROOT").map(PathBuf::from).filter(|p| is_repo(p));
    let cwd = std::env::current_dir().ok().and_then(|d| find_upwards(&d));
    let exe = std::env::current_exe()
        .ok()
        .and_then(|e| e.parent().and_then(find_upwards));
    saved.or(env).or(cwd).or(exe).map(|p| p.to_string_lossy().into_owned())
}

#[tauri::command]
fn set_root(app: AppHandle, root: String) -> Result<String, String> {
    let dir = PathBuf::from(&root);
    if !is_repo(&dir) {
        return Err(format!("{root} has no config/apps.yaml"));
    }
    let s = Settings { root: Some(root.clone()) };
    std::fs::write(settings_path(&app)?, serde_json::to_string_pretty(&s).unwrap())
        .map_err(|e| e.to_string())?;
    Ok(root)
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

/// Runs a read-style subcommand with --json and returns the parsed output.
#[tauri::command]
async fn envsetup(root: String, args: Vec<String>) -> Result<serde_json::Value, String> {
    let sub = args.first().map(String::as_str).unwrap_or_default();
    if !ALLOWED.contains(&sub) {
        return Err(format!("subcommand {sub:?} is not allowed"));
    }
    let bin = sidecar()?;
    tauri::async_runtime::spawn_blocking(move || {
        let out = Command::new(bin)
            .arg("--root")
            .arg(&root)
            .args(&args)
            .arg("--json")
            .stdin(Stdio::null())
            .output()
            .map_err(|e| e.to_string())?;
        if !out.status.success() {
            let err = String::from_utf8_lossy(&out.stderr);
            // cobra prints "Error: message"
            return Err(err.trim().trim_start_matches("Error: ").lines().next().unwrap_or("failed").to_string());
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
    no_files: bool,
    sudo_password: Option<String>,
) -> Result<(), String> {
    let bin = sidecar()?;
    let mut cmd = Command::new(bin);
    cmd.arg("--root").arg(&root).arg("-p").arg(&profile).arg("apply").arg("--json");
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
    if no_files {
        cmd.arg("--no-files");
    }
    let use_pw = sudo_password.as_deref().is_some_and(|p| !p.is_empty());
    if use_pw {
        cmd.arg("--sudo-password-stdin");
    }
    cmd.stdin(Stdio::piped()).stdout(Stdio::piped()).stderr(Stdio::piped());

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
                BufReader::new(e).lines().map_while(Result::ok).collect::<Vec<_>>().join("\n")
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
            Err(if msg.is_empty() { format!("envsetup exited with {status}") } else { msg })
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

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .manage(ApplyState::default())
        .invoke_handler(tauri::generate_handler![get_root, set_root, envsetup, apply, cancel_apply])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
