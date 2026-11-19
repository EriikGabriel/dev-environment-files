// Group policies (`envsetup policies --json`, cmd/envsetup/policies.go).

export interface PolicyValue {
  key: string;
  name: string;
  value: number;
}

export interface Policy {
  id: string;
  group: string;
  /** Folder in gpedit.msc, under Computer Configuration > Administrative Templates. */
  path: string;
  title: string;
  /** The state the video sets, in gpedit's words. */
  video: string;
  about: string;
  caution?: string;
  /** "video", "off" (not configured) or the id of an option. */
  choice: string;
  set: PolicyValue[];
  options: { id: string; title: string; set: PolicyValue[] }[];
  /** The registry already matches the choice. */
  applied: boolean;
}

export interface Policies {
  source: string;
  /** False when the registry could not be read (not on Windows). */
  readable: boolean;
  policies: Policy[];
}
