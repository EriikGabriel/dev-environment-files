// The app's own updates (`envsetup app-update-check --json`, cmd/envsetup/appupdate.go).

export interface AppUpdate {
  /** The version that is running. */
  current: string;
  /** The latest published version, without the v. */
  latest: string;
  tag: string;
  /** The latest release is newer than the one that is running. */
  available: boolean;
  /** The release notes, cut short. */
  notes: string;
  /** The release page on GitHub. */
  page: string;
  /** The file the app can install itself from; null where it cannot (Linux). */
  installer: { name: string; size: number } | null;
}

/** `envsetup app-update-download --json`: where the checked installer was saved. */
export interface UpdateDownload {
  path: string;
  tag: string;
  version: string;
}
