// The Windows desktop layout (`envsetup layout --json`, cmd/envsetup/layout.go).

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
  /** "Automatically hide the taskbar" (the Windhawk dock hides with it). */
  taskbar_autohide: boolean;
  /** config/layout.yaml exists. */
  saved: boolean;
  shell: Shell;
  installs: { id: string; name: string; kind: "ninite" | "github" | "runtime" }[];
}

export interface LayoutChoice {
  taskbar: string[];
  clear_start: boolean;
  hide_desktop_icons: boolean;
  /** "Automatically hide the taskbar" (the Windhawk dock hides with it). */
  taskbar_autohide: boolean;
}
