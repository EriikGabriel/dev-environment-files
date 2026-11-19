import { ask } from "@tauri-apps/plugin-dialog";
import { useCallback, useEffect, useState } from "react";
import * as api from "../../api";
import { errorMessage } from "../../api";
import type { Windhawk, WindhawkMod } from "../../types";
import { Help } from "../Help";

interface Props {
  root: string;
  /** Called after the machine or the export changed, so the plan is recalculated. */
  onChanged: () => void;
}

const STATE: Record<Exclude<WindhawkMod["state"], "">, [string, string]> = {
  same: ["igual ao backup", "badge ok"],
  changed: ["diferente do backup", "badge warn"],
  missing: ["não instalado", "badge warn"],
  extra: ["só nesta máquina", "badge"],
};

const value = (v: unknown) => (typeof v === "string" ? (v === "" ? '""' : v) : JSON.stringify(v));

/** The Windhawk mods kept in the repo (a Windhawk 2 export), compared with this machine. */
export function WindhawkTab({ root, onChanged }: Props) {
  const [data, setData] = useState<Windhawk | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [busy, setBusy] = useState("");

  const load = useCallback(async () => {
    try {
      setData(await api.windhawk(root));
    } catch (e) {
      setError(errorMessage(e));
    }
  }, [root]);
  useEffect(() => void load(), [load]);

  const act = async (what: string, fn: () => Promise<string>) => {
    setBusy(what);
    setError(null);
    setNote(null);
    try {
      const msg = await fn();
      await load();
      onChanged();
      setNote(msg);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy("");
    }
  };

  if (!data) return error ? <p className="banner bad">{error}</p> : <p className="banner">Lendo o Windhawk…</p>;

  const inRepo = data.mods.filter((m) => m.state !== "extra");
  const pending = inRepo.filter((m) => m.state === "missing" || m.state === "changed");
  const extra = data.mods.filter((m) => m.state === "extra");
  const canRun = data.windows && !!data.cli;

  return (
    <section className="policies windhawk">
      {error && <p className="banner bad">{error}</p>}
      {note && <p className="banner">{note}</p>}
      {data.windows && !data.cli && (
        <p className="banner bad">
          O windhawk-cli.exe não foi encontrado: ele só existe no Windhawk 2. Instale o item Windhawk da etapa Aparência (ele baixa
          a versão 2 mais recente do GitHub, pré-release inclusa) e volte aqui.
        </p>
      )}
      {data.error && data.cli && <p className="banner bad">{data.error}</p>}
      {data.warnings.map((w) => (
        <p key={w} className="banner">
          {w}
        </p>
      ))}

      <div className="card row cta">
        <div className="grow-col">
          <strong>
            Mods do Windhawk
            <Help title="Como funciona">
              O Windhawk 2 exporta tudo num arquivo só: as configurações dele, os mods instalados e as configurações de cada mod. O repo guarda
              esse arquivo em {data.path}. Aplicar usa o windhawk-cli para instalar do repositório do Windhawk os mods que faltam e gravar
              as configurações do backup (pede elevação uma vez; o Windhawk pode reiniciar). Mods que só existem nesta máquina ficam como
              estão. “Salvar desta máquina” faz o caminho inverso: troca o arquivo do repo pelo que está instalado agora, e o git mostra o que
              mudou.
            </Help>
          </strong>
          <span className="muted">
            {!data.exists
              ? `Ainda não há backup em ${data.path}.`
              : !data.read
                ? `${inRepo.length} mods no backup. O estado só pode ser comparado no Windows com o Windhawk 2.`
                : pending.length
                  ? `${pending.length} de ${inRepo.length} mods do backup ainda não estão iguais nesta máquina.`
                  : `Os ${inRepo.length} mods do backup estão instalados e configurados como nele.`}
            {data.read && data.app_settings_changed ? " As configurações do próprio Windhawk também mudaram." : ""}
          </span>
        </div>
        <button
          disabled={!!busy || !canRun}
          title="Troca o backup do repo pelos mods e configurações que estão no Windhawk agora"
          onClick={async () => {
            if (data.exists && !(await ask(`Substituir ${data.path} pelo que o Windhawk tem nesta máquina?`, { title: "Salvar do Windhawk", kind: "warning" }))) return;
            await act("export", async () => {
              const r = await api.exportWindhawk(root);
              return `${r.mods} mods salvos em ${data.path}. Faça commit para guardar.`;
            });
          }}
        >
          {busy === "export" ? "Salvando…" : "Salvar desta máquina"}
        </button>
        <button
          className="primary"
          disabled={!!busy || !canRun || !data.exists || (data.read && !data.pending && !data.app_settings_changed)}
          onClick={() =>
            act("import", async () => {
              await api.importWindhawk(root);
              return "Mods e configurações do backup aplicados no Windhawk.";
            })
          }
        >
          {busy === "import" ? "Aplicando… (instala os mods)" : "Aplicar backup"}
        </button>
      </div>

      {inRepo.length > 0 && (
        <ModList title={`No backup (${inRepo.length})`} mods={inRepo} read={data.read} />
      )}
      {extra.length > 0 && (
        <ModList
          title={`Só nesta máquina (${extra.length})`}
          note="Instalados aqui e fora do backup: “Salvar desta máquina” os inclui; aplicar o backup não os remove."
          mods={extra}
          read
        />
      )}

      {Object.keys(data.app_settings).length > 0 && (
        <details className="card script">
          <summary>
            <strong>Configurações do Windhawk</strong> <span className="muted small-note">idioma, tema, processos onde ele injeta</span>
          </summary>
          <pre className="code">{JSON.stringify(data.app_settings, null, 2)}</pre>
        </details>
      )}
    </section>
  );
}

function ModList({ title, note, mods, read }: { title: string; note?: string; mods: WindhawkMod[]; read: boolean }) {
  return (
    <div className="policy-group">
      <h3>{title}</h3>
      {note && <p className="muted small-note">{note}</p>}
      <ul className="list">
        {mods.map((m) => {
          const settings = Object.entries(m.settings);
          return (
            <li key={m.id} className="policy">
              <div className="grow-col">
                <div className="row">
                  <strong>{m.name || m.id}</strong>
                  <span className="muted small-note">{m.version || m.installed}</span>
                  {read && m.state && <span className={STATE[m.state][1]}>{STATE[m.state][0]}</span>}
                  {!m.enabled && <span className="chip off">desativado</span>}
                </div>
                <span className="muted small-note">{m.id}</span>
                {m.changes.map((c) => (
                  <span key={c} className="caution small-note">
                    {c}
                  </span>
                ))}
                {settings.length > 0 && (
                  <details className="script">
                    <summary className="muted small-note">{settings.length} configurações</summary>
                    <pre className="code">{settings.map(([k, v]) => `${k} = ${value(v)}`).join("\n")}</pre>
                  </details>
                )}
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
