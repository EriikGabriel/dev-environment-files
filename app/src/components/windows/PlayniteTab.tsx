import { ask, open } from "@tauri-apps/plugin-dialog";
import { useCallback, useEffect, useState } from "react";
import * as api from "../../api";
import { errorMessage } from "../../api";
import type { Playnite, PlayniteConfig, PlayniteTheme } from "../../types";
import { Help } from "../Help";

interface Props {
  root: string;
  /** Called after the folder changed, so the plan is recalculated. */
  onChanged: () => void;
}

const size = (b: number) =>
  b >= 1 << 30 ? `${(b / (1 << 30)).toFixed(1)} GB` : b >= 1 << 20 ? `${(b / (1 << 20)).toFixed(0)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`;

const when = (iso: string) => new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });

/** The portable Playnite: a snapshot of its folder outside the repo, and what the folder does not carry. */
export function PlayniteTab({ root, onChanged }: Props) {
  const [data, setData] = useState<Playnite | null>(null);
  const [sized, setSized] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [busy, setBusy] = useState("");
  const [mirror, setMirror] = useState(false);
  const [exclude, setExclude] = useState("");

  const load = useCallback(
    async (sizes: boolean) => {
      try {
        const d = await api.playnite(root, sizes);
        setData(d);
        setExclude(d.config.exclude.join(", "));
        if (sizes) setSized(true);
      } catch (e) {
        setError(errorMessage(e));
      }
    },
    [root],
  );
  // the quick read first, then the sizes (a second or two on a big library)
  useEffect(() => {
    void load(false).then(() => load(true));
  }, [load]);

  const act = async (what: string, fn: () => Promise<string | void>, sizes = false) => {
    setBusy(what);
    setError(null);
    setNote(null);
    try {
      const msg = await fn();
      await load(sizes || sized);
      if (msg) setNote(msg);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy("");
    }
  };

  const save = (c: Partial<PlayniteConfig>, msg?: string) => act("config", async () => (await api.setPlaynite(root, c), msg), "exclude" in c);

  const pickFolder = async (title: string, current: string) => {
    const dir = await open({ directory: true, multiple: false, title, defaultPath: current || undefined });
    return typeof dir === "string" ? dir : null;
  };

  if (!data) return error ? <p className="banner bad">{error}</p> : <p className="banner">Lendo o Playnite…</p>;

  const { config: cfg, install: inst, snapshot: snap } = data;
  const m = snap?.manifest ?? null;
  const running = data.running.length > 0;
  const pending = data.requirements.filter((r) => !r.ok);
  const themes = (mode: PlayniteTheme["mode"]) => inst.themes.filter((t) => t.mode === mode);
  const excluded = new Set(cfg.exclude.map((x) => x.toLowerCase()));

  const toggleFolder = (name: string) =>
    save({ exclude: excluded.has(name.toLowerCase()) ? cfg.exclude.filter((x) => x.toLowerCase() !== name.toLowerCase()) : [...cfg.exclude, name] });

  return (
    <section className="playnite">
      {error && <p className="banner bad">{error}</p>}
      {note && <p className="banner">{note}</p>}
      {running && (
        <p className="banner bad">
          {data.running.join(" e ")} está aberto. Feche o Playnite antes de salvar ou restaurar: a biblioteca dele não pode ser copiada com ele
          rodando.
        </p>
      )}

      <div className="card row cta">
        <div className="grow-col">
          <strong>
            Playnite portable
            <Help title="Como funciona">
              No modo portable o Playnite guarda tudo na própria pasta: biblioteca, capas, trailers, músicas de tema, temas, extensões e as
              configurações de cada uma. Uma cópia da pasta é o setup inteiro, sem refazer downloads nem configurar plugins de novo. A pasta tem
              dezenas de GB, então ela não vai para o git: o snapshot fica numa pasta que você escolhe (HD externo, NAS, pasta sincronizada), e
              o repo só guarda onde ele está em config/playnite.yaml. Na aplicação do perfil, o item Playnite copia o snapshot de volta.
            </Help>
          </strong>
          <span className="muted">
            {inst.found
              ? `${cfg.path} · ${inst.extensions.length} extensões · ${inst.themes.filter((t) => t.id !== "Default" && !t.id.startsWith("Playnite_builtin")).length} temas${sized ? ` · ${size(inst.bytes)} no snapshot` : ""}`
              : `Nenhum Playnite em ${cfg.path}. Restaure um snapshot ou aponte para a pasta certa abaixo.`}
          </span>
        </div>
        <button className="ghost" disabled={!inst.found} onClick={() => act("open", async () => void (await api.openPlaynite(root, "folder")))}>
          Abrir pasta
        </button>
      </div>

      <div className="card">
        <div className="row">
          <div className="grow-col">
            <strong>Snapshot</strong>
            <span className="muted small-note">
              {snap ? `${snap.path}` : "Escolha onde guardar a cópia da pasta, fora do repo."}
            </span>
          </div>
          {snap && (
            <button className="ghost" disabled={!!busy} onClick={() => act("open", async () => void (await api.openPlaynite(root, "store")))}>
              Abrir
            </button>
          )}
          <button
            disabled={!!busy}
            onClick={async () => {
              const dir = await pickFolder("Pasta para o snapshot do Playnite", cfg.store);
              if (dir) await save({ store: dir }, "Pasta do snapshot salva em config/playnite.yaml.");
            }}
          >
            {snap ? "Trocar pasta" : "Escolher pasta"}
          </button>
        </div>

        {snap?.error && <p className="banner bad">O manifesto do snapshot não pôde ser lido: {snap.error}</p>}
        {m ? (
          <dl className="facts snapshot-facts">
            <div>
              <dt>Salvo em</dt>
              <dd>{when(m.saved_at)}</dd>
            </div>
            <div>
              <dt>Máquina</dt>
              <dd>{m.computer}</dd>
            </div>
            <div>
              <dt>Tamanho</dt>
              <dd>
                {size(m.bytes)} · {m.files.toLocaleString("pt-BR")} arquivos
              </dd>
            </div>
            <div>
              <dt>Temas</dt>
              <dd title={`${m.desktop_theme} / ${m.fullscreen_theme}`}>
                {m.desktop_theme || "Default"} / {m.fullscreen_theme || "Default"}
              </dd>
            </div>
          </dl>
        ) : (
          snap && <p className="muted">Ainda não há snapshot nessa pasta.</p>
        )}

        <div className="row">
          <button
            className="primary"
            disabled={!!busy || !snap || !inst.found || running || !data.windows}
            title="Copia só o que mudou desde o último snapshot"
            onClick={() =>
              act("save", async () => {
                const r = await api.savePlaynite(root);
                return `Snapshot salvo: ${size(r.bytes)} em ${r.files.toLocaleString("pt-BR")} arquivos.`;
              })
            }
          >
            {busy === "save" ? "Copiando… (pode levar alguns minutos)" : m ? "Atualizar snapshot" : "Salvar snapshot"}
          </button>
          <button
            disabled={!!busy || !m || running || !data.windows}
            onClick={async () => {
              const ok = await ask(
                `Copiar o snapshot de ${when(m!.saved_at)} para ${cfg.path}?` +
                  (mirror ? "\n\nO que estiver na pasta e não estiver no snapshot será apagado." : "\n\nArquivos que já estão na pasta são substituídos pelos do snapshot."),
                { title: "Restaurar o Playnite", kind: "warning" },
              );
              if (ok)
                await act("restore", async () => {
                  await api.restorePlaynite(root, mirror);
                  onChanged();
                  return "Playnite restaurado.";
                });
            }}
          >
            {busy === "restore" ? "Restaurando…" : "Restaurar nesta máquina"}
          </button>
          <label className="check muted small-note" title="Também apaga da pasta o que o snapshot não tem (extensões e temas removidos)">
            <input type="checkbox" checked={mirror} onChange={(e) => setMirror(e.target.checked)} /> espelhar exatamente
          </label>
        </div>
      </div>

      <h3>
        Fora da pasta <span className="muted">({pending.length ? `${pending.length} pendente${pending.length > 1 ? "s" : ""}` : "tudo certo"})</span>
        <Help title="O que a pasta não leva">
          Coisas que o Playnite usa e que ficam no Windows, não na pasta dele: o Windows Media Player (sem ele os trailers não tocam), o atalho no
          menu Iniciar e programas que alguma extensão aponta por caminho completo. Na aplicação do perfil de jogos, o item Windows Media Player
          e a restauração do Playnite já cuidam dos dois primeiros.
        </Help>
      </h3>
      {!data.windows && <p className="muted">Os requisitos só podem ser verificados no Windows.</p>}
      <ul className="list">
        {data.requirements.map((r) => (
          <li key={r.id}>
            <div className="grow-col">
              <div className="row">
                <strong>{r.title}</strong>
                <span className={r.ok ? "badge ok" : "badge warn"}>{r.ok ? "ok" : "pendente"}</span>
              </div>
              <span className="muted small-note">{r.about}</span>
              {r.detail && <span className="muted small-note code-line">{r.detail}</span>}
            </div>
            {!r.ok && r.fixable && (
              <button
                disabled={!!busy || running}
                onClick={() => act("fix-" + r.id, async () => (await api.fixPlaynite(root, r.id), `${r.title}: resolvido.`))}
              >
                {busy === "fix-" + r.id ? "Corrigindo…" : r.elevated ? "Corrigir (UAC)" : "Corrigir"}
              </button>
            )}
          </li>
        ))}
      </ul>

      <h3>Configuração</h3>
      <div className="card playnite-config">
        <label className="field">
          <span className="muted">Pasta do Playnite</span>
          <div className="row">
            <input type="text" className="grow" value={cfg.path} readOnly />
            <button
              disabled={!!busy}
              onClick={async () => {
                const dir = await pickFolder("Pasta do Playnite portable", cfg.path);
                if (dir) await save({ path: dir });
              }}
            >
              Trocar
            </button>
          </div>
        </label>
        <label className="field">
          <span className="muted">Fora do snapshot (pastas relativas à do Playnite e padrões de arquivo, separados por vírgula)</span>
          <div className="row">
            <input type="text" className="grow" value={exclude} onChange={(e) => setExclude(e.target.value)} />
            <button
              disabled={!!busy || exclude === cfg.exclude.join(", ")}
              onClick={() => save({ exclude: exclude.split(",").map((x) => x.trim()).filter(Boolean) })}
            >
              Salvar
            </button>
          </div>
        </label>
        <label className="check">
          <input type="checkbox" checked={cfg.start_menu} disabled={!!busy} onChange={(e) => save({ start_menu: e.target.checked })} /> Depois de
          restaurar, pôr o Playnite no menu Iniciar e em Configurações › Apps
        </label>
      </div>

      {inst.found && (
        <>
          <h3>
            O que vai no snapshot <span className="muted">{sized ? `(${size(inst.bytes)})` : "(medindo…)"}</span>
          </h3>
          {sized && (
            <ul className="list folder-sizes">
              {inst.folders.map((f) => {
                const top = !f.name.startsWith("(");
                return (
                  <li key={f.name} className={f.excluded ? "off" : ""}>
                    <span className="grow">{f.name}</span>
                    <span className="muted small-note">{f.files.toLocaleString("pt-BR")} arquivos</span>
                    <span className="size">{size(f.bytes)}</span>
                    {top ? (
                      <button className="small-btn ghost" disabled={!!busy} onClick={() => toggleFolder(f.name)} title={f.excluded ? "Incluir no snapshot" : "Deixar fora do snapshot"}>
                        {f.excluded ? "fora" : "incluída"}
                      </button>
                    ) : (
                      <span className="small-btn" />
                    )}
                  </li>
                );
              })}
            </ul>
          )}

          <h3>Temas</h3>
          <div className="playnite-themes">
            {(["desktop", "fullscreen"] as const).map((mode) => (
              <div key={mode} className="card">
                <strong>{mode === "desktop" ? "Desktop" : "Fullscreen"}</strong>
                <ul className="plain">
                  {themes(mode).map((t) => (
                    <li key={t.id} className="row">
                      <span className="grow">
                        {t.name} <span className="muted small-note">{t.version}{t.author ? ` · ${t.author}` : ""}</span>
                      </span>
                      {t.active && <span className="badge ok">em uso</span>}
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>

          <h3>
            Extensões <span className="muted">({inst.extensions.length})</span>
          </h3>
          <ul className="list">
            {inst.extensions.map((x) => (
              <li key={x.id || x.name}>
                <div className="grow-col">
                  <div className="row">
                    <strong>{x.name}</strong>
                    <span className="muted small-note">{x.version}</span>
                    {!x.enabled && <span className="chip off">desativada</span>}
                  </div>
                  <span className="muted small-note">
                    {[x.author, x.type].filter(Boolean).join(" · ")}
                    {x.data ? "" : " · sem pasta de configurações"}
                  </span>
                </div>
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}
