import { invoke } from "@tauri-apps/api/core";
import { listen, type UnlistenFn } from "@tauri-apps/api/event";
import type { AddResult, ApplyEvent, DesktopPreview, FileView, Layout, LayoutChoice, Media, NewApp, Plan, Policies, Profile, Selection, SystemInfo, VentoyDrive, Wizard, Written } from "./types";

const run = <T>(root: string, args: string[]) => invoke<T>("envsetup", { root, args });

export const getRoot = () => invoke<string | null>("get_root");
export const setRoot = (root: string) => invoke<string>("set_root", { root });

export const info = (root: string) => run<SystemInfo>(root, ["info"]);
export const profiles = (root: string) => run<Profile[]>(root, ["profiles"]);
export const steps = (root: string, profile: string) => run<Wizard>(root, ["steps", "-p", profile]);

/** The typed values that are not empty. */
const filledVars = (sel: Selection) => Object.fromEntries(Object.entries(sel.vars).filter(([, v]) => v.trim() !== ""));

/** CLI flags for a selection: the exact app set, one --choose per step and one --set per value. */
const selectionArgs = (sel: Selection) => [
  ...(sel.apps.length ? ["--apps", sel.apps.join(",")] : ["--no-apps"]),
  ...Object.entries(sel.choices).flatMap(([step, option]) => ["--choose", `${step}=${option}`]),
  ...Object.entries(filledVars(sel)).flatMap(([key, value]) => ["--set", `${key}=${value.trim()}`]),
];

export const plan = (root: string, profile: string, sel: Selection) =>
  run<Plan>(root, ["plan", "-p", profile, ...selectionArgs(sel)]);

/** The desktop as the profile describes it, with real icons and wallpaper. */
export const desktop = (root: string, profile: string) => run<DesktopPreview>(root, ["desktop", "-p", profile]);

/** Saves the dock order into the profile (written to the system on apply). */
export const setDock = (root: string, profile: string, ids: string[]) =>
  run<{ pinned: string[] }>(root, ["set-dock", "-p", profile, ...ids]);

/** Reads a file or lists a directory inside the repo. */
export const show = (root: string, path: string) => run<FileView>(root, ["show", path]);

/** Saves the directories a step option creates into the profile YAML. */
export const setDirs = (root: string, profile: string, step: string, option: string, dirs: string[]) =>
  run<{ dirs: string[] }>(root, ["set-dirs", "-p", profile, "--step", step, ...(option ? ["--option", option] : []), ...dirs]);

/** Puts an app in a step of the profile, creating its catalogue entry when it is new. */
export const addApp = (root: string, profile: string, step: string, app: NewApp) =>
  run<{ id: string; step: string; created: boolean }>(root, [
    "add-app", "-p", profile, "--step", step, "--id", app.id,
    ...(app.name ? ["--name", app.name] : []),
    ...(app.method ? ["--method", app.method] : []),
    ...(app.description ? ["--description", app.description] : []),
    ...(app.optional ? ["--optional"] : []),
  ]);

/** Takes an app out of a step of the profile; its catalogue entry stays. */
export const removeApp = (root: string, profile: string, step: string, id: string) =>
  run<{ id: string; step: string }>(root, ["remove-app", "-p", profile, "--step", step, id]);

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
    vars: Object.fromEntries(Object.entries(filledVars(r.selection)).map(([k, v]) => [k, v.trim()])),
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

// ---- Windows installation media and group policies ----

/** The answer file, the Ninite list and the Ventoy drives of the profile. */
export const media = (root: string, profile: string) => run<Media>(root, ["media", "-p", profile]);

/** Saves the Ninite list: app names, or a ninite.com link. */
export const setNinite = (root: string, profile: string, apps: string[]) =>
  run<{ selected: string[]; unknown: string[] }>(root, ["set-ninite", "-p", profile, ...apps]);

/** Replaces the profile's answer file with one downloaded from the generator. */
export const setUnattend = (root: string, profile: string, file: string) =>
  run<{ path: string; has_password: boolean; has_link: boolean }>(root, ["set-unattend", "-p", profile, file]);

/** Writes the answer file, ventoy.json and (optionally) the Ninite installer to a Ventoy drive. */
export const writeMedia = (root: string, profile: string, drive: string, ninite: boolean) =>
  run<{ written: Written[]; drive: VentoyDrive }>(root, ["write-media", "-p", profile, "--drive", drive, ...(ninite ? [] : ["--no-ninite"])]);

export type Site = "unattend" | "generator" | "ninite" | "ninite-home" | "policies-source";

/** Opens one of the sites the setup uses in the default browser. */
export const openSite = (root: string, profile: string, site: Site) => run<{ opened: string }>(root, ["open-url", "-p", profile, site]);

export const policies = (root: string) => run<Policies>(root, ["policies"]);

/** Saves the choice of one or more policies (`{ id: choice }`) in config/policies.yaml. */
export const setPolicies = (root: string, choices: Record<string, string>) =>
  run<{ changed: number }>(root, ["set-policy", ...Object.entries(choices).map(([id, c]) => `${id}=${c}`)]);

/** Writes the chosen policies to the registry; Windows asks for elevation once. */
export const applyPolicies = (root: string) => run<Policies>(root, ["apply-policies"]);

/** The saved desktop layout and the apps that can be pinned; `apps` are the profile's ticked apps. */
export const layout = (root: string, profile: string, apps: string[]) => run<Layout>(root, ["layout", "-p", profile, "--apps", apps.join(",")]);

/** Saves the layout in config/layout.yaml. */
export const setLayout = (root: string, profile: string, c: LayoutChoice) =>
  run<LayoutChoice>(root, ["set-layout", "-p", profile, `--taskbar=${c.taskbar.join(",")}`, `--clear-start=${c.clear_start}`, `--hide-icons=${c.hide_desktop_icons}`]);

/** Applies the saved layout to this machine; Windows asks for elevation once. */
export const applyLayout = (root: string, profile: string) => run<{ applied: boolean }>(root, ["apply-layout", "-p", profile]);
