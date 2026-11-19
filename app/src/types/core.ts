// The JSON printed by the Go CLI for the profiles, the wizard and the plan
// (cmd/envsetup/json.go, apply.go, machine.go and internal/engine).

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
  /** Why a graphics card app starts selected or not ("placa NVIDIA detectada"). */
  hint?: string;
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

/** Where the data folder came from and whether the installer's data can update it (data_info in lib.rs). */
export interface DataInfo {
  /** This build carries config/, dotfiles/ and windows/ (an installed app, not a dev build). */
  bundled: boolean;
  /** Where "use the included data" copies them. */
  default_dir: string | null;
  /** The folder is a git clone: it is updated with git. */
  git: boolean;
  /** App version whose data was copied to the folder; null when it did not come from the installer. */
  seeded_version: string | null;
  app_version: string;
}

/** What "update the data" did: files added, and files that existed and were kept. */
export interface CopyReport {
  added: number;
  kept: number;
}
