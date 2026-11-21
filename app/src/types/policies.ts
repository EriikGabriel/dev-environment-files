// Group policies (`envsetup policies --json`, cmd/envsetup/policies.go).

export interface PolicyValue {
  key: string;
  name: string;
  value: number;
  /** gpedit's order to delete the value ("**del.<name>" in the Registry.pol), not a value. */
  delete?: boolean;
}

export interface Policy {
  id: string;
  group: string;
  /** Folder in gpedit.msc, under Computer Configuration > Administrative Templates. */
  path: string;
  title: string;
  /** Other names of the policy in gpedit, in other Windows builds. */
  aliases?: string[];
  /** The state the video sets, in gpedit's words. */
  video: string;
  /** What Windows 11 does with the policy not configured (its out-of-the-box state). */
  default: string;
  about: string;
  caution?: string;
  /** "video", "off" (not configured) or the id of an option. */
  choice: string;
  set: PolicyValue[];
  options: { id: string; title: string; set: PolicyValue[] }[];
  /** The registry and the Registry.pol (what gpedit shows) already match the choice. */
  applied: boolean;
}

export interface Policies {
  source: string;
  /** False when the registry could not be read (not on Windows). */
  readable: boolean;
  policies: Policy[];
}
