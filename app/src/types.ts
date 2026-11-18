// Mirrors the JSON printed by the Go CLI (cmd/envsetup/json.go and
// internal/engine). Keep the two in sync.

export interface Platform {
  os: string;
  arch: string;
  id: string;
  like: string[];
  family: string;
  name: string;
}

/** Output of `info --json`. Unknown values come as empty strings or 0. */
export interface SystemInfo extends Platform {
  supported: boolean;
  hostname: string;
  user: string;
  kernel: string;
  desktop: string;
  session: string;
  shell: string;
  cpu: string;
  cores: number;
  memory_gb: number;
  package_manager: string;
  aur_helper: string;
  flatpak: boolean;
}

export type Environment = "dev" | "gaming" | "";

export interface Profile {
  id: string;
  name: string;
  description: string;
  environment: Environment;
  match: string[] | null;
  /** True for the profile picked automatically for this machine. */
  matches: boolean;
  /** False when the profile is for another operating system; those are not listed. */
  compatible: boolean;
}

export interface WizardApp {
  id: string;
  description: string;
  default: boolean;
  available: boolean;
  installed: boolean;
  detail: string;
}

export interface WizardFile {
  /** Path in the repo, relative to its root. */
  src: string;
  /** Where it is deployed, with ~. */
  dest: string;
  mode: "link" | "copy" | "install";
  /** Deployed only together with this app; "" means always. */
  app: string;
}

export interface FileEntry {
  name: string;
  dir: boolean;
  size: number;
}

/** Output of `show --json`. */
export interface FileView {
  path: string;
  kind: "text" | "dir" | "image" | "binary";
  size: number;
  content?: string;
  truncated: boolean;
  data_url?: string;
  entries: FileEntry[];
}

export interface WizardOption {
  id: string;
  title: string;
  description: string;
  apps: WizardApp[];
  files: WizardFile[];
  /** Directories the option creates. */
  dirs: string[];
}

/** A value the step asks for (git name, e-mail). */
export interface WizardInput {
  /** Key passed to the engine with --set. */
  id: string;
  label: string;
  /** What this machine has today; "" when unknown. */
  value: string;
}

export interface WizardStep {
  id: string;
  title: string;
  description: string;
  apps: WizardApp[];
  files: WizardFile[];
  dirs: string[];
  choice_title: string;
  options: WizardOption[];
  default: string;
  inputs: WizardInput[];
}

/** Output of `steps --json`. */
export interface Wizard {
  profile: string;
  name: string;
  description: string;
  environment: Environment;
  platform: Platform;
  steps: WizardStep[];
}

/** What the user picked: toggled apps, one option per choice step and the typed values. */
export interface Selection {
  apps: string[];
  choices: Record<string, string>;
  /** Step inputs by id. Kept on this computer only, never written to the repo. */
  vars: Record<string, string>;
}

export type InstallMethod = "native" | "aur" | "flatpak" | "winget" | "scoop" | "steam";

/** A new item for a step (`add-app`). */
export interface NewApp {
  id: string;
  /** Package name, winget ID or Steam AppID; "" when the id is already in the catalogue. */
  name: string;
  method: InstallMethod | "";
  description: string;
  optional: boolean;
}

export interface DockItem {
  id: string;
  name: string;
  /** Data URL; empty when no icon was found on this machine. */
  icon: string;
  /** A desktop entry for it exists on this machine. */
  found: boolean;
}

/** Output of `desktop --json`. */
export interface DesktopPreview {
  kind: "niri-noctalia" | "gnome" | "";
  icon_theme: string;
  colors: { accent: string; panel: string; text: string };
  wallpaper: string;
  wallpaper_source: string;
  wallpaper_error: string;
  dock: { position: "bottom" | "top" | "left" | "right"; autohide: boolean; items: DockItem[] };
  bar: { position: "top" | "bottom"; start: string[]; center: string[]; end: string[] };
  /** Installed apps that can be added to the dock. */
  apps: { id: string; name: string }[];
}

export type StepKind = "app" | "file" | "dir" | "dock" | "vscode";

export interface PlanLine {
  kind: StepKind;
  /** Profile step id; "" for extra files. */
  step: string;
  id: string;
  title: string;
  description?: string;
  pending: boolean;
  detail: string;
  src?: string;
  mode?: "link" | "copy";
}

export interface Skipped {
  id: string;
  reason: string;
}

export interface Plan {
  platform: Platform;
  root: string;
  profile: string;
  choices: Record<string, string>;
  steps: PlanLine[];
  skipped: Skipped[];
}

export interface AddResult {
  src: string;
  dest: string;
  mode: "link" | "copy";
}

/** One line of `apply --json`. */
export type ApplyEvent =
  | {
      type: "step";
      index: number;
      total: number;
      title: string;
      status: "running" | "done" | "uptodate" | "failed";
      detail?: string;
      error?: string;
      /** Profile step id the item belongs to. */
      group?: string;
      /** Set when the item finishes. */
      duration_ms?: number;
    }
  | { type: "log"; index?: number; line: string }
  | { type: "done"; done: number; up_to_date: number; failed: number; log_file?: string };

// ---- Windows installation media (`media --json`) ----

export interface UnattendItem {
  label: string;
  value?: string;
}

export interface UnattendScript {
  /** When it runs. */
  phase: string;
  type: string;
  body: string;
}

export interface Unattend {
  /** Path in the repo. */
  path: string;
  /** The site that builds answer files. */
  generator: string;
  error?: string;
  /** Link that reopens the generator with this file's settings; "" when the file has none. */
  url: string;
  sections: { title: string; items: UnattendItem[] }[];
  scripts: UnattendScript[];
  /** A script runs a Ninite installer found on a drive root at first logon. */
  ninite_hook: boolean;
  /** An account has a password, readable in the file. */
  has_password: boolean;
  /** Choices of the file that break something else of the setup. */
  warnings: string[];
}

export interface NiniteApp {
  slug: string;
  name: string;
  category: string;
}

export interface Ninite {
  /** Catalogue item that holds the list. */
  app: string;
  selected: string[];
  /** Selected names that ninite.com does not list. */
  unknown: string[];
  page: string;
  installer: string;
  /** File name of the installer at the root of the drive. */
  on_drive: string;
  catalog: NiniteApp[];
}

export type DriveFile = "missing" | "current" | "outdated";

export interface VentoyDrive {
  path: string;
  images: { path: string; present: boolean }[];
  unattend: DriveFile;
  config: DriveFile;
  ninite: boolean;
}

export interface Media {
  /** False when the profile has no installation media (the Linux ones). */
  available: boolean;
  profile: string;
  unattend?: Unattend;
  ventoy?: { path: string; error?: string; auto_install: { image: string; template: string }[] };
  ninite?: Ninite;
  drives: VentoyDrive[];
}

export interface Written {
  path: string;
  action: "written" | "unchanged" | "downloaded";
  backup?: string;
}

// ---- group policies (`policies --json`) ----

export interface PolicyValue {
  key: string;
  name: string;
  value: number;
}

export interface Policy {
  id: string;
  group: string;
  /** Folder in gpedit.msc, under Computer Configuration > Administrative Templates. */
  path: string;
  title: string;
  /** The state the video sets, in gpedit's words. */
  video: string;
  about: string;
  caution?: string;
  /** "video", "off" (not configured) or the id of an option. */
  choice: string;
  set: PolicyValue[];
  options: { id: string; title: string; set: PolicyValue[] }[];
  /** The registry already matches the choice. */
  applied: boolean;
}

export interface Policies {
  source: string;
  /** False when the registry could not be read (not on Windows). */
  readable: boolean;
  policies: Policy[];
}

// ---- desktop layout (`layout --json`) ----

export type LayoutSource = "windows" | "ninite" | "unattend" | "catalog";

export interface LayoutItem {
  id: string;
  name: string;
  sources: LayoutSource[];
  /** False for what has no window to pin (Java, Node, runtimes). */
  pinnable: boolean;
  /** The installer usually leaves a shortcut on the desktop. */
  desktop: boolean;
  glyph: string;
  color: string;
}

/** How the answer file sets up the desktop. */
export interface Shell {
  taskbar_left: boolean;
  hide_search: boolean;
  hide_task_view: boolean;
  no_widgets: boolean;
  start_pins_empty: boolean;
  desktop_icons: string;
  desktop_system_icons: string[];
  no_edge_desktop_icon: boolean;
}

export interface Layout {
  available: boolean;
  items: LayoutItem[];
  /** Saved taskbar order, by item id. */
  taskbar: string[];
  clear_start: boolean;
  hide_desktop_icons: boolean;
  /** config/layout.yaml exists. */
  saved: boolean;
  shell: Shell;
  installs: { id: string; name: string; kind: "ninite" | "github" | "runtime" }[];
}

// ---- shortcut icons (`icons --json`) ----

export type IconScope = "desktop" | "start-menu" | "taskbar";

export interface IconsConfig {
  enabled: boolean;
  style: "light" | "dark";
  pack: string;
  scopes: IconScope[];
  system_icons: boolean;
  skip: string[];
  overrides: Record<string, string>;
}

export interface IconEntry {
  id: string;
  name: string;
  scope: IconScope;
  path: string;
  target: string;
  /** The shortcut already points at an icon made here. */
  themed: boolean;
  from: "style" | "pack" | "override" | "skip";
  file?: string;
  /** Data URLs of the icon it shows and of the one it would get. */
  before: string;
  after: string;
  error?: string;
}

/** A page of the icons of the pack (`icon-pack --json`). */
export interface IconPackPage {
  path: string;
  count: number;
  /** How many icons the query found; `icons` holds a page of them. */
  total: number;
  icons: { name: string; file: string; image: string }[];
}

export interface Icons {
  config: IconsConfig;
  saved: boolean;
  pack: { path: string; count: number; error?: string; system: string[] };
  /** The pack the repo ships (repo-relative), to offer it. */
  builtin_pack: string;
  /** envsetup changed icons on this machine and can undo it. */
  applied: boolean;
  /** The taskbar is the dock of the Windhawk mod Ilha e Dock estilo Mac. */
  dock: boolean;
  /** That mod's taskbarMode (dock, windowsTransparent, windowsApps, windows); "" when it is not installed. */
  dock_mode: string;
  /** `entries` holds the shortcuts found by a scan. */
  scanned: boolean;
  entries: IconEntry[];
}

export interface LayoutChoice {
  taskbar: string[];
  clear_start: boolean;
  hide_desktop_icons: boolean;
}

// ---- Playnite (envsetup playnite --json) ----

export interface PlayniteConfig {
  /** The Playnite folder on this machine. */
  path: string;
  /** Folder of the snapshot ("" until chosen): the copy is <store>\Playnite. */
  store: string;
  /** Left out of the snapshot: names with * or a dot are file patterns, the rest folders. */
  exclude: string[];
  /** Register Playnite in the Start menu after a restore. */
  start_menu: boolean;
}

export interface PlayniteTheme {
  mode: "desktop" | "fullscreen";
  id: string;
  name: string;
  author: string;
  version: string;
  active: boolean;
}

export interface PlayniteExtension {
  id: string;
  name: string;
  author: string;
  version: string;
  type: string;
  enabled: boolean;
  /** Its settings folder in ExtensionsData, when found. */
  data?: string;
}

export interface PlayniteFolder {
  name: string;
  bytes: number;
  files: number;
  excluded: boolean;
}

export interface PlayniteManifest {
  saved_at: string;
  source: string;
  computer: string;
  bytes: number;
  files: number;
  desktop_theme: string;
  fullscreen_theme: string;
  extensions: string[];
}

export interface PlayniteRequirement {
  id: string;
  title: string;
  about: string;
  ok: boolean;
  detail?: string;
  fixable: boolean;
  elevated: boolean;
}

export interface Playnite {
  config: PlayniteConfig;
  saved: boolean;
  windows: boolean;
  install: {
    found: boolean;
    themes: PlayniteTheme[];
    extensions: PlayniteExtension[];
    /** Empty unless asked with sizes. */
    folders: PlayniteFolder[];
    bytes: number;
    files: number;
  };
  /** Null until a store is chosen. */
  snapshot: { path: string; manifest: PlayniteManifest | null; error?: string } | null;
  /** Playnite programs that are open. */
  running: string[];
  requirements: PlayniteRequirement[];
}

// ---- Windhawk (envsetup windhawk --json) ----

export interface WindhawkMod {
  id: string;
  name: string;
  /** Version in the repo ("" for a mod only on the machine). */
  version: string;
  enabled: boolean;
  settings: Record<string, unknown>;
  /** "" when the machine was not read. */
  state: "" | "same" | "changed" | "missing" | "extra";
  /** Version on the machine ("" when missing). */
  installed: string;
  changes: string[];
}

export interface Windhawk {
  path: string;
  exists: boolean;
  windows: boolean;
  /** windhawk-cli.exe; "" when Windhawk 2 is not installed. */
  cli: string;
  read: boolean;
  error?: string;
  app_settings: Record<string, unknown>;
  app_settings_changed: boolean;
  mods: WindhawkMod[];
  pending: boolean;
  warnings: string[];
}
