import { invoke } from "@tauri-apps/api/core";
import { listen, type UnlistenFn } from "@tauri-apps/api/event";
import type { AddResult, ApplyEvent, DesktopPreview, FileView, IconPackPage, Icons, IconsConfig, Layout, LayoutChoice, Media, NewApp, Plan, Playnite, PlayniteConfig, PlayniteManifest, Policies, Windhawk, Profile, Selection, SystemInfo, VentoyDrive, Wizard, Written } from "./types";

const run = <T>(root: string, args: string[]) => invoke<T>("envsetup", { root, args });

export const getRoot = () => invoke<string | null>("get_root");
export const setRoot = (root: string) => invoke<string>("set_root", { root });

export const info = (root: string) => run<SystemInfo>(root, ["info"]);
export const profiles = (root: string) => run<Profile[]>(root, ["profiles"]);
export const steps = (root: string, profile: string) => run<Wizard>(root, ["steps", "-p", profile]);

/** The typed values that are not empty. */
const filledVars = (sel: Selection) => Object.fromEntries(Object.entries(sel.vars).filter(([, v]) => v.trim() !== ""));

/** CLI flags for a selection: the exact app set, one --choose per step and one --set per value. */
const selectionArgs = (sel: Selection) => [
  ...(sel.apps.length ? ["--apps", sel.apps.join(",")] : ["--no-apps"]),
  ...Object.entries(sel.choices).flatMap(([step, option]) => ["--choose", `${step}=${option}`]),
  ...Object.entries(filledVars(sel)).flatMap(([key, value]) => ["--set", `${key}=${value.trim()}`]),
];

export const plan = (root: string, profile: string, sel: Selection) =>
  run<Plan>(root, ["plan", "-p", profile, ...selectionArgs(sel)]);

/** The desktop as the profile describes it, with real icons and wallpaper. */
export const desktop = (root: string, profile: string) => run<DesktopPreview>(root, ["desktop", "-p", profile]);

/** Saves the dock order into the profile (written to the system on apply). */
export const setDock = (root: string, profile: string, ids: string[]) =>
  run<{ pinned: string[] }>(root, ["set-dock", "-p", profile, ...ids]);

/** Reads a file or lists a directory inside the repo. */
export const show = (root: string, path: string) => run<FileView>(root, ["show", path]);

/** Saves the directories a step option creates into the profile YAML. */
export const setDirs = (root: string, profile: string, step: string, option: string, dirs: string[]) =>
  run<{ dirs: string[] }>(root, ["set-dirs", "-p", profile, "--step", step, ...(option ? ["--option", option] : []), ...dirs]);

/** Puts an app in a step of the profile, creating its catalogue entry when it is new. */
export const addApp = (root: string, profile: string, step: string, app: NewApp) =>
  run<{ id: string; step: string; created: boolean }>(root, [
    "add-app", "-p", profile, "--step", step, "--id", app.id,
    ...(app.name ? ["--name", app.name] : []),
    ...(app.method ? ["--method", app.method] : []),
    ...(app.description ? ["--description", app.description] : []),
    ...(app.optional ? ["--optional"] : []),
  ]);

/** Takes an app out of a step of the profile; its catalogue entry stays. */
export const removeApp = (root: string, profile: string, step: string, id: string) =>
  run<{ id: string; step: string }>(root, ["remove-app", "-p", profile, "--step", step, id]);

export interface AddOptions {
  profile: string;
  group?: string;
  /** Keep the original and deploy by copy instead of replacing it with a symlink. */
  copy?: boolean;
}

export const addFile = (root: string, path: string, o: AddOptions) => {
  const args = ["add", path, "-p", o.profile];
  if (o.group) args.push("--group", o.group);
  if (o.copy) args.push("--copy");
  return run<AddResult>(root, args);
};

export interface ApplyRequest {
  root: string;
  profile: string;
  selection: Selection;
  sudoPassword?: string;
}

export const apply = (r: ApplyRequest) =>
  invoke<void>("apply", {
    root: r.root,
    profile: r.profile,
    choices: r.selection.choices,
    apps: r.selection.apps,
    vars: Object.fromEntries(Object.entries(filledVars(r.selection)).map(([k, v]) => [k, v.trim()])),
    noFiles: false,
    sudoPassword: r.sudoPassword ?? null,
  });

export const cancelApply = () => invoke<void>("cancel_apply");

export const onApplyEvent = (fn: (e: ApplyEvent) => void): Promise<UnlistenFn> =>
  listen<string>("apply-event", (ev) => {
    try {
      fn(JSON.parse(ev.payload) as ApplyEvent);
    } catch {
      // a non-JSON line from the sidecar is not worth crashing the UI over
    }
  });

export const errorMessage = (e: unknown): string => (typeof e === "string" ? e : e instanceof Error ? e.message : String(e));

// ---- Windows installation media and group policies ----

/** The answer file, the Ninite list and the Ventoy drives of the profile. */
export const media = (root: string, profile: string) => run<Media>(root, ["media", "-p", profile]);

/** Saves the Ninite list: app names, or a ninite.com link. */
export const setNinite = (root: string, profile: string, apps: string[]) =>
  run<{ selected: string[]; unknown: string[] }>(root, ["set-ninite", "-p", profile, ...apps]);

/** Replaces the profile's answer file with one downloaded from the generator. */
export const setUnattend = (root: string, profile: string, file: string) =>
  run<{ path: string; has_password: boolean; has_link: boolean }>(root, ["set-unattend", "-p", profile, file]);

/** Writes the answer file, ventoy.json and (optionally) the Ninite installer to a Ventoy drive. */
export const writeMedia = (root: string, profile: string, drive: string, ninite: boolean) =>
  run<{ written: Written[]; drive: VentoyDrive }>(root, ["write-media", "-p", profile, "--drive", drive, ...(ninite ? [] : ["--no-ninite"])]);

export type Site = "unattend" | "generator" | "ninite" | "ninite-home" | "policies-source";

/** Opens one of the sites the setup uses in the default browser. */
export const openSite = (root: string, profile: string, site: Site) => run<{ opened: string }>(root, ["open-url", "-p", profile, site]);

export const policies = (root: string) => run<Policies>(root, ["policies"]);

/** Saves the choice of one or more policies (`{ id: choice }`) in config/policies.yaml. */
export const setPolicies = (root: string, choices: Record<string, string>) =>
  run<{ changed: number }>(root, ["set-policy", ...Object.entries(choices).map(([id, c]) => `${id}=${c}`)]);

/** Writes the chosen policies to the registry; Windows asks for elevation once. */
export const applyPolicies = (root: string) => run<Policies>(root, ["apply-policies"]);

/** The saved desktop layout and the apps that can be pinned; `apps` are the profile's ticked apps. */
export const layout = (root: string, profile: string, apps: string[]) => run<Layout>(root, ["layout", "-p", profile, "--apps", apps.join(",")]);

/** Saves the layout in config/layout.yaml. */
export const setLayout = (root: string, profile: string, c: LayoutChoice) =>
  run<LayoutChoice>(root, ["set-layout", "-p", profile, `--taskbar=${c.taskbar.join(",")}`, `--clear-start=${c.clear_start}`, `--hide-icons=${c.hide_desktop_icons}`]);

/** The icon style; `scan` looks for the shortcuts of this machine (slow), `cached` redraws the last scan. */
export const icons = (root: string, mode: "none" | "cached" | "scan") =>
  run<Icons>(root, ["icons", ...(mode === "scan" ? ["--scan"] : mode === "cached" ? ["--cached"] : [])]);

/** Saves the icon style in config/icons.yaml. */
export const setIcons = (root: string, c: IconsConfig) =>
  run<IconsConfig>(root, [
    "set-icons",
    `--enabled=${c.enabled}`,
    `--style=${c.style}`,
    `--pack=${c.pack}`,
    `--scopes=${c.scopes.join(",")}`,
    `--system-icons=${c.system_icons}`,
    `--skip=${c.skip.join(",")}`,
    "--clear-overrides",
    ...Object.entries(c.overrides).map(([name, file]) => `--override=${name}=${file}`),
  ]);

/** A page of the icons of the pack, all or the ones whose name has `query`. */
export const iconPack = (root: string, query: string, limit: number) =>
  run<IconPackPage>(root, ["icon-pack", `--query=${query}`, `--limit=${limit}`]);

/** Points the shortcuts at the new icons; Windows asks for elevation once. */
export const applyIcons = (root: string) => run<{ applied: boolean }>(root, ["apply-icons"]);

/** Turns on the dock of the Windhawk mod Ilha e Dock estilo Mac (installing it when missing), or puts back the mode it had. */
export const taskbarDock = (root: string, restore: boolean) => run<{ dock: boolean }>(root, ["taskbar-dock", ...(restore ? ["--restore"] : [])]);

/** Puts the Windows icons back: every shortcut, the system icons and the icon cache. */
export const restoreIcons = (root: string) => run<{ restored: boolean }>(root, ["restore-icons"]);

/** Applies the saved layout to this machine; Windows asks for elevation once. */
export const applyLayout = (root: string, profile: string) => run<{ applied: boolean }>(root, ["apply-layout", "-p", profile]);

// ---- Playnite ----

/** The portable Playnite, its snapshot and what it needs outside its folder; `sizes` walks every file. */
export const playnite = (root: string, sizes: boolean) => run<Playnite>(root, ["playnite", ...(sizes ? ["--sizes"] : [])]);

/** Saves config/playnite.yaml; only the fields given change. */
export const setPlaynite = (root: string, c: Partial<PlayniteConfig>) =>
  run<PlayniteConfig>(root, [
    "set-playnite",
    ...(c.path !== undefined ? [`--path=${c.path}`] : []),
    ...(c.store !== undefined ? [`--store=${c.store}`] : []),
    ...(c.exclude !== undefined ? [`--exclude=${c.exclude.join(",")}`] : []),
    ...(c.start_menu !== undefined ? [`--start-menu=${c.start_menu}`] : []),
  ]);

/** Copies the Playnite folder to the store (only what changed). */
export const savePlaynite = (root: string) => run<PlayniteManifest>(root, ["playnite-save"]);

/** Copies the snapshot back; `mirror` also deletes what the snapshot does not have. */
export const restorePlaynite = (root: string, mirror: boolean) =>
  run<{ restored: boolean }>(root, ["playnite-restore", ...(mirror ? ["--mirror"] : [])]);

/** Takes care of one requirement (media-player, start-menu, ffmpeg-normalize). */
export const fixPlaynite = (root: string, id: string) => run<{ fixed: string }>(root, ["playnite-fix", id]);

/** Opens the Playnite folder or the store in Explorer. */
export const openPlaynite = (root: string, what: "folder" | "store") => run<{ opened: string }>(root, ["playnite-open", what]);

// ---- Windhawk ----

/** The Windhawk export of the repo compared with the mods installed here. */
export const windhawk = (root: string) => run<Windhawk>(root, ["windhawk"]);

/** Replaces windows/windhawk/backup.json with what Windhawk has on this machine. */
export const exportWindhawk = (root: string) => run<{ mods: number }>(root, ["windhawk-export"]);

/** Installs the mods of the export with their settings; Windows asks for elevation once. */
export const importWindhawk = (root: string) => run<{ imported: boolean }>(root, ["windhawk-import"]);
