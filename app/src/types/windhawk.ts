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
}
