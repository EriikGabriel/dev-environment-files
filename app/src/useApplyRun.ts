import { useCallback, useState } from "react";
import * as api from "./api";
import { errorMessage } from "./api";
import type { StepState } from "./components/ApplyView";
import type { ApplyEvent } from "./types";

type Summary = Extract<ApplyEvent, { type: "done" }>;

/** A run of `envsetup apply`, as the Apply screen shows it. */
export interface ApplyState {
  steps: StepState[];
  total: number;
  summary: Summary | null;
  running: boolean;
  cancelled: boolean;
  startedAt: number;
  endedAt: number | null;
  error: string | null;
}

/** Folds one event of the sidecar into the state of the run. */
function onEvent(cur: ApplyState, ev: ApplyEvent): ApplyState {
  const now = Date.now();
  if (ev.type === "done") return { ...cur, summary: ev };
  const i = cur.steps.findIndex((s) => s.index === ev.index);
  if (ev.type === "log") {
    // the line belongs to the step that printed it
    if (i < 0) return cur;
    const steps = [...cur.steps];
    steps[i] = { ...cur.steps[i], log: [...cur.steps[i].log.slice(-400), ev.line], lastOutputAt: now };
    return { ...cur, steps };
  }
  const prev = i >= 0 ? cur.steps[i] : undefined;
  const next: StepState = {
    index: ev.index,
    title: ev.title,
    group: ev.group ?? "",
    status: ev.status,
    detail: ev.detail ?? prev?.detail,
    error: ev.error,
    durationMs: ev.duration_ms,
    log: prev?.log ?? [],
    startedAt: prev?.startedAt ?? now,
    lastOutputAt: prev?.lastOutputAt ?? now,
  };
  const steps = prev ? cur.steps.map((s, k) => (k === i ? next : s)) : [...cur.steps, next];
  return { ...cur, steps, total: ev.total };
}

/** Runs `envsetup apply` and keeps its progress; `run` is null when no run is shown. */
export function useApplyRun() {
  const [run, setRun] = useState<ApplyState | null>(null);

  /** Starts a run; `onEnd` is called when it finishes, failed or not. */
  const start = useCallback(async (req: api.ApplyRequest, onEnd: () => void) => {
    setRun({ steps: [], total: 0, summary: null, running: true, cancelled: false, startedAt: Date.now(), endedAt: null, error: null });
    const off = await api.onApplyEvent((ev) => setRun((cur) => cur && onEvent(cur, ev)));
    try {
      await api.apply(req);
      setRun((cur) => cur && { ...cur, running: false, endedAt: Date.now() });
    } catch (e) {
      setRun((cur) => cur && { ...cur, running: false, endedAt: Date.now(), error: errorMessage(e) });
    } finally {
      off();
      onEnd();
    }
  }, []);

  const cancel = useCallback(() => {
    setRun((cur) => cur && { ...cur, cancelled: true });
    void api.cancelApply();
  }, []);

  /** Leaves the Apply screen. */
  const close = useCallback(() => setRun(null), []);

  return { run, start, cancel, close };
}
