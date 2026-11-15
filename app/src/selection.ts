import type { Selection, Wizard } from "./types";

const key = (profile: string) => `envsetup.selection.${profile}`;

/** The profile's own defaults: `apps` on, `optional` off, default options. */
export function defaultSelection(w: Wizard): Selection {
  const sel: Selection = { apps: [], choices: {} };
  for (const s of w.steps) {
    for (const a of s.apps) if (a.default && a.available) sel.apps.push(a.id);
    if (s.options.length) sel.choices[s.id] = s.default || "none";
  }
  return sel;
}

/** The saved selection, cleaned of anything the profile no longer has. */
export function loadSelection(w: Wizard): Selection {
  const def = defaultSelection(w);
  try {
    const raw = localStorage.getItem(key(w.profile));
    if (!raw) return def;
    const saved = JSON.parse(raw) as Selection;
    const known = new Set(w.steps.flatMap((s) => s.apps.filter((a) => a.available).map((a) => a.id)));
    const choices = { ...def.choices };
    for (const s of w.steps) {
      const c = saved.choices?.[s.id];
      if (c && (c === "none" || s.options.some((o) => o.id === c))) choices[s.id] = c;
    }
    return { apps: (saved.apps ?? []).filter((id) => known.has(id)), choices };
  } catch {
    return def;
  }
}

export const saveSelection = (profile: string, sel: Selection) => localStorage.setItem(key(profile), JSON.stringify(sel));
