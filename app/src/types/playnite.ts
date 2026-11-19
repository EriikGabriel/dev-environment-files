// Playnite (`envsetup playnite --json`, cmd/envsetup/playnite.go).

export interface PlayniteConfig {
  /** The Playnite folder on this machine. */
  path: string;
  /** Folder of the snapshot ("" until chosen): the copy is <store>\Playnite. */
  store: string;
  /** Left out of the snapshot: names with * or a dot are file patterns, the rest folders. */
  exclude: string[];
  /** Register Playnite in the Start menu after a restore. */
  start_menu: boolean;
  /** Settings files kept in the repo (windows/playnite); * matches one folder. */
  git: string[];
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
  /** API keys and tokens left out of git and kept in the store. */
  secrets: number;
}

/** The personal part of a program kept in the repo. */
export interface GitPart {
  dir: string;
  files: number;
  bytes: number;
  /** Files of the program folder that differ from git: a save copies them, then they need a commit. */
  pending: number;
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
  /** The settings kept in the repo. */
  git: GitPart;
}
