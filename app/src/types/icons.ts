// Shortcut icons (`envsetup icons --json`, cmd/envsetup/icons.go).

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
