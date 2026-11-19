// `envsetup retroarch --json` and friends (cmd/envsetup/retroarch.go).

import type { GitPart, PlayniteFolder as SnapshotFolder } from "./playnite";

/** config/retroarch.yaml. */
export interface RetroArchConfig {
  path: string;
  /** Folder of the games, one subfolder per system; "" leaves them out. */
  games: string;
  store: string;
  exclude: string[];
  start_menu: boolean;
  /** RetroArch release a restore downloads (written by a save). */
  version: string;
  /** Cores a restore downloads, file names without _libretro.dll. */
  cores: string[];
  /** libretro asset packs downloaded with the program. */
  bundles: string[];
  /** Files and folders kept in the repo (windows/retroarch). */
  git: string[];
  /** Also keep the whole program in the store. */
  copy_program: boolean;
  /** Also copy the games to the store on each save (off when they already live on the kept drive). */
  copy_games: boolean;
}


export interface RetroArchManifest {
  saved_at: string;
  source: string;
  games: string;
  computer: string;
  bytes: number;
  files: number;
  game_bytes: number;
  game_count: number;
  cores: string[];
  systems: string[];
  version: string;
  /** The store has the whole program (copy_program). */
  program: boolean;
  /** Passwords and tokens left out of git and kept in the store. */
  secrets: number;
}

export interface RetroArchCore {
  file: string;
  name: string;
  system: string;
}

export interface RetroArchSystem {
  name: string;
  games: number;
  bytes: number;
}

export interface RetroArchPlaylist {
  name: string;
  items: number;
  missing: number;
  fixable: number;
}

export interface RetroArchSetting {
  key: string;
  label: string;
  value: string;
}

export interface RetroArchInstall {
  found: boolean;
  cores: RetroArchCore[];
  systems: RetroArchSystem[];
  playlists: RetroArchPlaylist[];
  settings: RetroArchSetting[];
  saves: number;
  states: number;
  bios: number;
  folders: SnapshotFolder[];
  bytes: number;
  files: number;
  game_bytes: number;
  game_count: number;
}

export interface RetroArchRequirement {
  id: "playlists" | "start-menu";
  title: string;
  about: string;
  ok: boolean;
  detail?: string;
  fixable: boolean;
}

export interface RetroArch {
  config: RetroArchConfig;
  saved: boolean;
  windows: boolean;
  install: RetroArchInstall;
  snapshot: { path: string; games: string; manifest: RetroArchManifest | null; error?: string } | null;
  running: boolean;
  requirements: RetroArchRequirement[];
  git: GitPart;
  /** Folders a restore downloads instead of copying. */
  program_dirs: string[];
}
