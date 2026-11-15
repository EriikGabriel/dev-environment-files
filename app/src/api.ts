import { invoke } from "@tauri-apps/api/core";
import { listen, type UnlistenFn } from "@tauri-apps/api/event";
import type { AddResult, ApplyEvent, FileView, Plan, Profile, Selection, SystemInfo, Wizard } from "./types";

const run = <T>(root: string, args: string[]) => invoke<T>("envsetup", { root, args });

export const getRoot = () => invoke<string | null>("get_root");
export const setRoot = (root: string) => invoke<string>("set_root", { root });

export const info = (root: string) => run<SystemInfo>(root, ["info"]);
export const profiles = (root: string) => run<Profile[]>(root, ["profiles"]);
export const steps = (root: string, profile: string) => run<Wizard>(root, ["steps", "-p", profile]);

/** CLI flags for a selection: the exact app set plus one --choose per step. */
const selectionArgs = (sel: Selection) => [
  ...(sel.apps.length ? ["--apps", sel.apps.join(",")] : ["--no-apps"]),
  ...Object.entries(sel.choices).flatMap(([step, option]) => ["--choose", `${step}=${option}`]),
];

export const plan = (root: string, profile: string, sel: Selection) =>
  run<Plan>(root, ["plan", "-p", profile, ...selectionArgs(sel)]);

/** Reads a file or lists a directory inside the repo. */
export const show = (root: string, path: string) => run<FileView>(root, ["show", path]);

/** Saves the directories a step option creates into the profile YAML. */
export const setDirs = (root: string, profile: string, step: string, option: string, dirs: string[]) =>
  run<{ dirs: string[] }>(root, ["set-dirs", "-p", profile, "--step", step, ...(option ? ["--option", option] : []), ...dirs]);

export interface AddOptions {
  profile: string;
  group?: string;
  /** Keep the original and deploy by copy instead of replacing it with a symlink. */
  copy?: boolean;
}

export const addFile = (root: string, path: string, o: AddOptions) => {
  const args = ["add", path, "-p", o.profile];
  if (o.group) args.push("--group", o.group);
  if (o.copy) args.push("--copy");
  return run<AddResult>(root, args);
};

export interface ApplyRequest {
  root: string;
  profile: string;
  selection: Selection;
  sudoPassword?: string;
}

export const apply = (r: ApplyRequest) =>
  invoke<void>("apply", {
    root: r.root,
    profile: r.profile,
    choices: r.selection.choices,
    apps: r.selection.apps,
    noFiles: false,
    sudoPassword: r.sudoPassword ?? null,
  });

export const cancelApply = () => invoke<void>("cancel_apply");

export const onApplyEvent = (fn: (e: ApplyEvent) => void): Promise<UnlistenFn> =>
  listen<string>("apply-event", (ev) => {
    try {
      fn(JSON.parse(ev.payload) as ApplyEvent);
    } catch {
      // a non-JSON line from the sidecar is not worth crashing the UI over
    }
  });

export const errorMessage = (e: unknown): string => (typeof e === "string" ? e : e instanceof Error ? e.message : String(e));
