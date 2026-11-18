// The unattended Windows installation (`envsetup media --json`, cmd/envsetup/install.go).

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

export interface InstallMedia {
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
