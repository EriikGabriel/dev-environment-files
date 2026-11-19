// `envsetup backup --json` (cmd/envsetup/backup.go): what a format needs.

import type { Playnite } from "./playnite";
import type { RetroArch } from "./retroarch";

export interface GitState {
  /** False when the data folder is not a git clone (the copy the installer seeds). */
  repo: boolean;
  branch: string;
  /** "origin/main"; "" when the branch has no upstream. */
  upstream: string;
  /** Changed files under config/ and windows/. */
  dirty: string[];
  /** Commits not pushed / not pulled yet, as of the last fetch. */
  ahead: number;
  behind: number;
  error?: string;
}

/** Where the last backup was pushed, kept in the store for a formatted machine. */
export interface StoreGit {
  url: string;
  branch: string;
  pushed_at: string;
}

export interface Backup {
  root: string;
  windows: boolean;
  git: GitState;
  store_git: StoreGit | null;
  playnite: Playnite;
  retroarch: RetroArch;
  /** False when the configuration is someone else's: no GitHub steps, only their own HD. */
  owner: boolean;
}
