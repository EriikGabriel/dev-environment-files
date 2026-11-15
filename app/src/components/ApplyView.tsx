import { useEffect, useRef, useState } from "react";
import type { ApplyEvent } from "../types";
import { Help } from "./Help";

export interface StepState {
  index: number;
  title: string;
  /** Profile step id; "" for extras. */
  group: string;
  status: "running" | "done" | "uptodate" | "failed";
  detail?: string;
  error?: string;
  durationMs?: number;
  log: string[];
  startedAt: number;
  lastOutputAt: number;
}

interface Props {
  steps: StepState[];
  total: number;
  summary: Extract<ApplyEvent, { type: "done" }> | null;
  running: boolean;
  cancelled: boolean;
  startedAt: number;
  endedAt: number | null;
  error: string | null;
  stepTitles: Record<string, string>;
  onCancel: () => void;
  onClose: () => void;
}

type Filter = "all" | "done" | "uptodate" | "failed";

const MARK = { running: "", done: "✓", uptodate: "=", failed: "✗" } as const;
const LABEL = { running: "executando", done: "alterado", uptodate: "já em dia", failed: "falhou" } as const;
/** A running item with no output for this long is flagged as possibly stuck. */
const QUIET_MS = 30_000;

const clock = (ms: number) => {
  const s = Math.max(0, Math.round(ms / 1000));
  return s < 60 ? `${s}s` : `${Math.floor(s / 60)}m ${String(s % 60).padStart(2, "0")}s`;
};
const short = (t: string) => t.replace(/^\/home\/[^/]+/, "~");

export function ApplyView(p: Props) {
  const { steps, running } = p;
  const [now, setNow] = useState(Date.now());
  const [filter, setFilter] = useState<Filter>("all");
  const [open, setOpen] = useState<Record<number, boolean>>({});
  const [follow, setFollow] = useState(true);
  const activeRef = useRef<HTMLLIElement>(null);

  useEffect(() => {
    if (!running) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [running]);

  const current = steps.find((s) => s.status === "running");
  useEffect(() => {
    if (follow && running) activeRef.current?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [current?.index, current?.log.length, follow, running]);

  const count = (st: StepState["status"]) => steps.filter((s) => s.status === st).length;
  const finished = steps.filter((s) => s.status !== "running").length;
  const pct = p.total ? Math.round((finished / p.total) * 100) : 0;
  const failed = steps.filter((s) => s.status === "failed");
  const visible = steps.filter((s) => filter === "all" || s.status === filter || s.status === "running");
  // a run that ended while an item was still running stopped there
  const stoppedAt = !running ? current : undefined;

  const headline = running
    ? current
      ? `Executando ${short(current.title)}`
      : "Preparando…"
    : p.cancelled
      ? "Cancelado"
      : p.error || failed.length
        ? "Concluído com falhas"
        : "Concluído";

  return (
    <section className="apply">
      <div className="card apply-head">
        <div className="row">
          {running && <span className="spinner" aria-hidden="true" />}
          <h2 className="big grow">{headline}</h2>
          <span className="muted mono">{clock((p.endedAt ?? now) - p.startedAt)}</span>
          {running ? (
            <button className="danger" onClick={p.onCancel}>
              Cancelar
            </button>
          ) : (
            <button className="primary" onClick={p.onClose}>
              Voltar
            </button>
          )}
        </div>
        <div className={`progress tall ${failed.length ? "has-fail" : ""}`} role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}>
          <div style={{ width: `${pct}%` }} />
        </div>
        <div className="row">
          <span className="muted">
            {finished} de {p.total || "?"} itens
            <Help title="Como ler o log">
              Cada linha é um item, na ordem de execução. “alterado” mudou algo no sistema; “já em dia” não precisou
              fazer nada; “falhou” mostra o erro e a saída do comando. Clique numa linha para abrir ou fechar a saída.
              Um aviso “sem saída” aparece quando o item em execução fica 30 segundos calado, o que pode indicar
              travamento ou só um download longo.
            </Help>
          </span>
          <span className="spacer" />
          {(
            [
              ["all", `Todos ${steps.length}`, ""],
              ["done", `Alterados ${count("done")}`, "ok"],
              ["uptodate", `Já em dia ${count("uptodate")}`, ""],
              ["failed", `Falhas ${count("failed")}`, "bad"],
            ] as [Filter, string, string][]
          ).map(([id, label, tone]) => (
            <button key={id} className={`filter ${tone} ${filter === id ? "active" : ""}`} aria-pressed={filter === id} onClick={() => setFilter(id)}>
              {label}
            </button>
          ))}
          <label className="check muted">
            <input type="checkbox" checked={follow} onChange={(e) => setFollow(e.target.checked)} /> seguir
          </label>
        </div>
      </div>

      {p.error && <p className="banner bad">{p.error}</p>}
      {stoppedAt && (
        <p className="banner bad">
          Parou em <strong>{short(stoppedAt.title)}</strong> ({stoppedAt.detail}). Os itens seguintes não foram executados.
        </p>
      )}
      {!running && failed.length > 0 && (
        <p className="banner bad">
          Falharam: {failed.map((f) => short(f.title)).join(", ")}. Os demais itens continuaram; abra cada um para ver a saída.
        </p>
      )}

      <ul className="timeline">
        {visible.map((s) => {
          const isRunning = s.status === "running" && running;
          const expanded = open[s.index] ?? (isRunning || s.status === "failed");
          const quiet = isRunning && now - s.lastOutputAt > QUIET_MS;
          const time = s.status === "running" ? (running ? clock(now - s.startedAt) : "") : s.durationMs ? clock(s.durationMs) : "";
          return (
            <li key={s.index} className={`tl ${s.status}`} ref={isRunning ? activeRef : undefined}>
              <button className="tl-row" aria-expanded={expanded} onClick={() => setOpen((o) => ({ ...o, [s.index]: !expanded }))}>
                <span className={`mark ${s.status}`}>{isRunning ? <span className="spinner small" /> : MARK[s.status] || "•"}</span>
                <span className="tl-n muted mono">{s.index}</span>
                <span className="tl-title">{short(s.title)}</span>
                {s.group && p.stepTitles[s.group] && <span className="chip">{p.stepTitles[s.group]}</span>}
                <span className="grow muted">{s.error ? "" : s.detail}</span>
                {quiet && <span className="badge warn">sem saída há {clock(now - s.lastOutputAt)}</span>}
                <span className="muted mono">{time}</span>
                <span className={`badge ${s.status === "done" ? "ok" : s.status === "failed" ? "bad" : s.status === "running" ? "warn" : ""}`}>
                  {s.status === "running" && !running ? "interrompido" : LABEL[s.status]}
                </span>
              </button>
              {expanded && (s.error || s.log.length > 0) && (
                <div className="tl-body">
                  {s.error && <p className="bad-text">{s.error}</p>}
                  {s.log.length > 0 && <StepLog lines={s.log} live={isRunning && follow} />}
                </div>
              )}
            </li>
          );
        })}
        {steps.length === 0 && <li className="muted tl-empty">Aguardando o primeiro item…</li>}
      </ul>

      {p.summary && (
        <p className="banner">
          {p.summary.done} alterados · {p.summary.up_to_date} já em dia · {p.summary.failed} com falha
          {p.summary.log_file && (
            <>
              {" · log completo em "}
              <span className="mono">{short(p.summary.log_file)}</span>
            </>
          )}
        </p>
      )}
    </section>
  );
}

function StepLog({ lines, live }: { lines: string[]; live: boolean }) {
  const ref = useRef<HTMLPreElement>(null);
  useEffect(() => {
    if (live) ref.current?.scrollTo({ top: ref.current.scrollHeight });
  }, [lines.length, live]);
  return (
    <pre className="log" ref={ref}>
      {lines.join("\n")}
    </pre>
  );
}
