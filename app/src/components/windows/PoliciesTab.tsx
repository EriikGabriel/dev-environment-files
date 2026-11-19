import { KeyRound, SquareArrowOutUpRight } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import * as api from "../../api";
import { errorMessage } from "../../api";
import type { Policies, Policy } from "../../types";
import { Help } from "../Help";

interface Props {
  root: string;
  profile: string;
  /** False on a machine that is not Windows: the list is shown, nothing can be applied. */
  windows: boolean;
  /** Called after the registry changed, so the plan is recalculated. */
  onApplied: () => void;
}

type Filter = "all" | "pending" | "changed" | "caution";

const choiceLabel = (p: Policy, choice: string) =>
  choice === "video" ? `${p.video} (Recomendado)` : choice === "off" ? "Não configurar" : (p.options.find((o) => o.id === choice)?.title ?? choice);

/** The group policies of config/policies.yaml: what each does, what it is set to, and applying them. */
export function PoliciesTab({ root, windows, onApplied }: Props) {
  const [data, setData] = useState<Policies | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [busy, setBusy] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const [query, setQuery] = useState("");

  const load = useCallback(async () => {
    try {
      setData(await api.policies(root));
    } catch (e) {
      setError(errorMessage(e));
    }
  }, [root]);
  useEffect(() => void load(), [load]);

  const act = async (what: string, fn: () => Promise<string | void>) => {
    setBusy(what);
    setError(null);
    setNote(null);
    try {
      const msg = await fn();
      if (msg) setNote(msg);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy("");
    }
  };

  const choose = (choices: Record<string, string>, msg?: string) =>
    act("save", async () => {
      await api.setPolicies(root, choices);
      await load();
      return msg;
    });

  const all = data?.policies ?? [];
  const pending = all.filter((p) => !p.applied).length;
  const changed = all.filter((p) => p.choice !== "video");

  const groups = useMemo(() => {
    const q = query.trim().toLowerCase();
    const out: { name: string; items: Policy[] }[] = [];
    for (const p of all) {
      if (filter === "pending" && p.applied) continue;
      if (filter === "changed" && p.choice === "video") continue;
      if (filter === "caution" && !p.caution) continue;
      if (q && !`${p.title} ${p.about} ${p.path}`.toLowerCase().includes(q)) continue;
      const g = out.find((x) => x.name === p.group);
      if (g) g.items.push(p);
      else out.push({ name: p.group, items: [p] });
    }
    return out;
  }, [all, filter, query]);

  if (!data) return error ? <p className="banner bad">{error}</p> : <p className="banner">Lendo as políticas…</p>;

  const filters: [Filter, string][] = [
    ["all", `Todas (${all.length})`],
    ["pending", `Pendentes (${pending})`],
    ["changed", `Diferentes do vídeo (${changed.length})`],
    ["caution", `Com ressalva (${all.filter((p) => p.caution).length})`],
  ];

  return (
    <section className="policies">
      {error && <p className="banner bad">{error}</p>}
      {note && <p className="banner">{note}</p>}

      <div className="card row cta">
        <div className="grow-col">
          <strong>
            Políticas de grupo
            <Help title="Como são aplicadas">
              Cada item é uma configuração do Editor de Política de Grupo (gpedit.msc), em Configuração do Computador ›
              Modelos Administrativos. O envsetup grava o mesmo valor que o gpedit gravaria, no Registry.pol e no registro, então o
              Windows obedece e o gpedit mostra a política como Habilitada ou Desabilitada (reabra o gpedit se ele
              já estava aberto). “Não configurar” apaga o valor e devolve o comportamento padrão. Gravar pede
              elevação (UAC) uma vez; algumas só valem depois de sair da sessão ou reiniciar.
            </Help>
          </strong>
          <span className="muted">
            {data.readable
              ? pending
                ? `${pending} de ${all.length} ainda não estão como escolhido nesta máquina.`
                : `As ${all.length} já estão aplicadas como escolhido.`
              : "O estado só pode ser lido e aplicado no Windows; aqui dá para rever e ajustar as escolhas."}
          </span>
        </div>
        <button disabled={!!busy || changed.length === 0} title="Volta todas as escolhas para o valor recomendado" onClick={() => choose(Object.fromEntries(changed.map((p) => [p.id, "video"])), "Escolhas de volta ao padrão do vídeo.")}>
          Padrão Recomendado
        </button>
        <button
          className="primary"
          disabled={!!busy || !windows || !data.readable || pending === 0}
          onClick={() =>
            act("apply", async () => {
              setData(await api.applyPolicies(root));
              onApplied();
              return "Políticas gravadas no registro. Algumas só passam a valer depois de sair da sessão ou reiniciar.";
            })
          }
        >
          {busy === "apply" ? "Aplicando…" : `Aplicar ${pending || ""} agora`.replace("  ", " ")}
        </button>
      </div>

      <div className="row">
        {filters.map(([id, label]) => (
          <button key={id} className={filter === id ? "filter active" : "filter"} onClick={() => setFilter(id)}>
            {label}
          </button>
        ))}
        <span className="spacer" />
        <input className="search" placeholder="Buscar política" value={query} onChange={(e) => setQuery(e.target.value)} aria-label="Buscar política" />
      </div>

      {groups.length === 0 && <p className="muted">Nenhuma política neste filtro.</p>}
      {groups.map((g) => (
        <div key={g.name} className="policy-group">
          <h3>
            {g.name} <span className="muted">({g.items.length})</span>
          </h3>
          <ul className="list">
            {g.items.map((p) => (
              <li key={p.id} className="policy">
                <div className="grow-col">
                  <div className="row">
                    <strong>{p.title}</strong>
                    {data.readable && <span className={p.applied ? "badge ok" : "badge warn"}>{p.applied ? "aplicada" : "pendente"}</span>}
                    {p.choice !== "video" && <span className="chip">diferente do recomendado</span>}
                  </div>
                  <div className="row policy-path">
                    <span className="muted small-note grow">{p.path}</span>
                    {windows && (
                      <>
                        <button
                          className="link-btn"
                          disabled={!!busy}
                          title="Abre o Editor de Política de Grupo já nesta política (pede permissão de administrador)"
                          onClick={() => act("open-" + p.id, async () => (await api.openPolicy(root, p.id, false)).message || "Política aberta no gpedit.")}
                        >
                          <SquareArrowOutUpRight size={13} /> {busy === "open-" + p.id ? "Abrindo…" : "Abrir no gpedit"}
                        </button>
                        <button
                          className="link-btn"
                          disabled={!!busy}
                          title="Abre o Editor do Registro na chave que esta política grava"
                          onClick={() => act("reg-" + p.id, async () => (await api.openPolicy(root, p.id, true)).message || "Chave aberta no Editor do Registro.")}
                        >
                          <KeyRound size={13} /> {busy === "reg-" + p.id ? "Abrindo…" : "Ver no registro"}
                        </button>
                      </>
                    )}
                  </div>
                  <p>{p.about}</p>
                  {p.caution && (
                    <p className="caution">
                      <strong>Atenção:</strong> {p.caution}
                    </p>
                  )}
                  <details className="script">
                    <summary className="muted small-note">Valores no registro</summary>
                    <pre className="code">
                      {p.set.map((v) => `${v.key}\n    ${v.name} = ${v.value}`).join("\n")}
                    </pre>
                  </details>
                </div>
                <select value={p.choice} disabled={!!busy} aria-label={`Escolha para ${p.title}`} onChange={(e) => choose({ [p.id]: e.target.value })}>
                  {["video", ...p.options.map((o) => o.id), "off"].map((c) => (
                    <option key={c} value={c}>
                      {choiceLabel(p, c)}
                    </option>
                  ))}
                </select>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </section>
  );
}
