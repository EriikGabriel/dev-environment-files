// Windhawk (`envsetup windhawk --json`, cmd/envsetup/windhawk.go).

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
  /** The options the mod Ilha e Dock estilo Mac keeps outside Windhawk's export. */
  island: WindhawkIsland;
}

export interface WindhawkIsland {
  /** The mod's own backup, relative to the repo. */
  path: string;
  exists: boolean;
  /** How many options an import writes. */
  values: number;
  /** Options this machine has otherwise. */
  differ: number;
  /** Options this machine has not stored (the mod uses its default for them). */
  missing: number;
  read: boolean;
  error?: string;
  /** Options that stay on each machine (the weather's place, notes, tasks...). */
  personal: string[];
}
