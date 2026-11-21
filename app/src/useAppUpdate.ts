import { useCallback, useEffect, useRef, useState } from "react";
import * as api from "./api";
import { errorMessage } from "./api";
import type { AppUpdate } from "./types";

export type UpdatePhase =
  | "idle" // nothing asked yet
  | "checking"
  | "uptodate"
  | "available"
  | "downloading"
  | "installing"
  | "error";

export interface AppUpdater {
  phase: UpdatePhase;
  update: AppUpdate | null;
  error: string | null;
  /** The banner at the top was closed for this session. */
  dismissed: boolean;
  check: () => Promise<void>;
  /** Downloads the installer, checks it and hands over to it; the app closes. */
  install: () => Promise<void>;
  dismiss: () => void;
}

/**
 * Looks for a newer release of the app once, quietly, a moment after it opens
 * (a failure, such as no network, says nothing), and again whenever asked.
 */
export function useAppUpdate(version: string | null): AppUpdater {
  const [phase, setPhase] = useState<UpdatePhase>("idle");
  const [update, setUpdate] = useState<AppUpdate | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [dismissed, setDismissed] = useState(false);
  const busy = useRef(false);

  const check = useCallback(async () => {
    if (!version || busy.current) return;
    busy.current = true;
    setPhase("checking");
    setError(null);
    try {
      const u = await api.appUpdateCheck(version);
      setUpdate(u);
      setPhase(u.available ? "available" : "uptodate");
    } catch (e) {
      setError(errorMessage(e));
      setPhase("error");
    } finally {
      busy.current = false;
    }
  }, [version]);

  // the quiet check: errors are kept for the About page but nothing is shown
  const asked = useRef(false);
  useEffect(() => {
    if (!version || asked.current) return;
    asked.current = true;
    const t = setTimeout(() => void check(), 2500);
    return () => clearTimeout(t);
  }, [version, check]);

  const install = useCallback(async () => {
    if (!version || busy.current) return;
    busy.current = true;
    setError(null);
    try {
      setPhase("downloading");
      const d = await api.appUpdateDownload(version);
      setPhase("installing");
      // give the message a moment on screen: the app closes right after
      await new Promise((r) => setTimeout(r, 1200));
      await api.installUpdate(d.path);
    } catch (e) {
      setError(errorMessage(e));
      setPhase("error");
    } finally {
      busy.current = false;
    }
  }, [version]);

  return { phase, update, error, dismissed, check, install, dismiss: () => setDismissed(true) };
}
