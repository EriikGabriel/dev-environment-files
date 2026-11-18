import { open } from "@tauri-apps/plugin-dialog";
import { useEffect, useMemo, useState } from "react";
import * as api from "../../api";
import { errorMessage } from "../../api";
import type { DriveFile, InstallMedia, VentoyDrive, Written } from "../../types";
import type { FileTarget } from "../FileDrawer";
import { Help } from "../Help";

interface Props {
  root: string;
  profile: string;
  media: InstallMedia;
  /** Reloads the media description after something changed. */
  onChanged: () => Promise<void> | void;
  onOpenFile: (f: FileTarget) => void;
}

const FILE_STATE: Record<DriveFile, [string, string]> = {
  current: ["igual ao repo", "badge ok"],
  outdated: ["diferente do repo", "badge warn"],
  missing: ["ausente", "badge warn"],
};

const ACTION: Record<Written["action"], string> = { written: "gravado", unchanged: "já estava igual", downloaded: "baixado" };

/** The unattended Windows installation: answer file, Ninite and the Ventoy drive. */
export function InstallTab({ root, profile, media, onChanged, onOpenFile }: Props) {
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [busy, setBusy] = useState("");

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

  const u = media.unattend;
  const importXml = async () => {
    const file = await open({ multiple: false, title: "Escolha o autounattend.xml baixado do gerador", filters: [{ name: "Arquivo de resposta", extensions: ["xml"] }] });
    if (typeof file !== "string") return;
    await act("import", async () => {
      const r = await api.setUnattend(root, profile, file);
      await onChanged();
      return `${r.path} substituído.${r.has_link ? "" : " Este arquivo não traz o link do gerador, então o resumo fica vazio."}`;
    });
  };

  return (
    <section className="media">
      {error && <p className="banner bad">{error}</p>}
      {note && <p className="banner">{note}</p>}

      <div className="card media-card">
        <div className="row">
          <div className="grow-col">
            <strong>
              Instalação automática do Windows
              <Help title="O que é o autounattend.xml">
                É o arquivo de resposta que o instalador do Windows lê para não perguntar nada: idioma, conta, ajustes, apps
                removidos e scripts. Ele é feito no site schneegans.de e guarda, num comentário, o link que reabre o
                formulário com tudo o que foi marcado.
              </Help>
            </strong>
            <span className="muted mono">{u?.path}</span>
          </div>
          <button disabled={!!busy || !u?.url} title={u?.url ? "Abre o gerador com as opções deste arquivo já marcadas" : "Este arquivo não tem o link do gerador"} onClick={() => act("site", () => api.openSite(root, profile, "unattend").then(() => "Gerador aberto no navegador com as opções deste arquivo. Depois de mudar, baixe o XML e use “Importar XML”."))}>
            Customizar no site
          </button>
          <button disabled={!!busy} onClick={importXml}>
            Importar XML…
          </button>
          {u && !u.error && (
            <button className="ghost" onClick={() => onOpenFile({ src: u.path })}>
              Ver arquivo
            </button>
          )}
        </div>

        {u?.error && <p className="banner bad">Não foi possível ler {u.path}: {u.error}</p>}
        {u?.has_password && (
          <p className="banner bad">
            Este arquivo tem a senha de uma conta, legível por qualquer um (no XML e no link). Não faça commit dele num
            repositório público.
          </p>
        )}
        {u?.warnings?.map((w) => (
          <p key={w} className="banner bad">
            {w}
          </p>
        ))}
        {u && !u.error && !u.url && (
          <p className="muted small-note">
            O arquivo é válido, mas não traz o link do gerador: não dá para resumir o que ele faz nem reabri-lo no site.
          </p>
        )}

        {u && u.sections.length > 0 && (
          <div className="media-sections">
            {u.sections.map((s) => {
              const plain = s.items.every((i) => !i.value);
              return (
                <div key={s.title} className={plain ? "full" : undefined}>
                  <h3>
                    {s.title} {plain && <span className="muted">({s.items.length})</span>}
                  </h3>
                  {plain ? (
                    <div className="chips">
                      {s.items.map((i) => (
                        <span className="chip" key={i.label}>
                          {i.label}
                        </span>
                      ))}
                    </div>
                  ) : (
                    <dl className="kv">
                      {s.items.map((i) => (
                        <div key={i.label}>
                          <dt>{i.label}</dt>
                          <dd>{i.value}</dd>
                        </div>
                      ))}
                    </dl>
                  )}
                </div>
              );
            })}
            {u.scripts.length > 0 && (
              <div className="full">
                <h3>Scripts ({u.scripts.length})</h3>
                {u.scripts.map((s, i) => (
                  <details key={i} className="script">
                    <summary>
                      <span className="chip">{s.phase}</span> <span className="mono">{firstLine(s.body)}</span>
                    </summary>
                    <pre className="code">{s.body}</pre>
                  </details>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {media.ninite && <NiniteCard root={root} profile={profile} media={media} busy={busy} act={act} onChanged={onChanged} />}

      <DrivesCard root={root} profile={profile} media={media} busy={busy} act={act} onChanged={onChanged} />
    </section>
  );
}

const firstLine = (body: string) => body.trim().split("\n")[0].slice(0, 110);

interface CardProps {
  root: string;
  profile: string;
  media: InstallMedia;
  busy: string;
  act: (what: string, fn: () => Promise<string | void>) => Promise<void>;
  onChanged: () => Promise<void> | void;
}

function NiniteCard({ root, profile, media, busy, act, onChanged }: CardProps) {
  const n = media.ninite!;
  const [picked, setPicked] = useState<string[]>(n.selected);
  const [filter, setFilter] = useState("");
  const [paste, setPaste] = useState("");
  useEffect(() => setPicked(n.selected), [n.selected]);

  const dirty = picked.join(" ") !== n.selected.join(" ");
  const groups = useMemo(() => {
    const q = filter.trim().toLowerCase();
    const out: { category: string; apps: typeof n.catalog }[] = [];
    for (const a of n.catalog) {
      if (q && !a.name.toLowerCase().includes(q) && !a.slug.toLowerCase().includes(q)) continue;
      const g = out.find((x) => x.category === a.category);
      if (g) g.apps.push(a);
      else out.push({ category: a.category, apps: [a] });
    }
    return out;
  }, [n.catalog, filter]);
  const nameOf = (slug: string) => n.catalog.find((a) => a.slug === slug)?.name ?? slug;
  const toggle = (slug: string) => setPicked((cur) => (cur.includes(slug) ? cur.filter((s) => s !== slug) : [...cur, slug]));

  const save = (apps: string[]) =>
    act("ninite", async () => {
      const r = await api.setNinite(root, profile, apps);
      await onChanged();
      setPaste("");
      return `${r.selected.length} apps na lista do Ninite.${r.unknown.length ? ` Não estão no site: ${r.unknown.join(", ")}.` : ""}`;
    });

  return (
    <div className="card media-card">
      <div className="row">
        <div className="grow-col">
          <strong>
            Ninite
            <Help title="Como o Ninite entra">
              O Ninite gera um instalador único para os programas marcados, que instala e atualiza tudo sem perguntas. A
              lista fica no repo (item <code>{n.app}</code> do catálogo). O instalador é baixado na hora, tanto para o
              pendrive quanto pelo item “ninite” da etapa Essenciais, em Configurar.
            </Help>
          </strong>
          <span className="muted">
            {n.selected.length} programas na lista salva{dirty ? ` · ${picked.length} marcados agora` : ""}
          </span>
        </div>
        <button disabled={!!busy} title="Abre ninite.com com a lista salva" onClick={() => act("site", () => api.openSite(root, profile, "ninite").then(() => "ninite.com aberto com a lista salva. Para trazer de volta o que mudar lá, cole o link da página abaixo."))}>
          Abrir no Ninite
        </button>
        <button className="primary" disabled={!!busy || !dirty} onClick={() => save(picked)}>
          Salvar lista
        </button>
      </div>

      <div className="chips">
        {picked.length === 0 && <span className="muted">Nenhum programa marcado.</span>}
        {picked.map((s) => (
          <button key={s} className={n.catalog.some((a) => a.slug === s) ? "chip pick" : "chip pick unknown"} title="Tirar da lista" onClick={() => toggle(s)}>
            {nameOf(s)} ✕
          </button>
        ))}
      </div>
      {n.unknown.length > 0 && (
        <p className="muted small-note">
          <span className="bad-text">{n.unknown.join(", ")}</span> não aparece em ninite.com: o instalador é gerado sem ele.
        </p>
      )}

      <div className="row">
        <input className="search" placeholder="Filtrar programas" value={filter} onChange={(e) => setFilter(e.target.value)} aria-label="Filtrar programas do Ninite" />
        <span className="spacer" />
        <input className="paste" placeholder="Colar link do Ninite (https://ninite.com/…)" value={paste} onChange={(e) => setPaste(e.target.value)} aria-label="Link do Ninite" />
        <button disabled={!!busy || !paste.trim()} onClick={() => save([paste.trim()])}>
          Usar este link
        </button>
      </div>

      <div className="ninite-grid">
        {groups.map((g) => (
          <div key={g.category}>
            <h3>{g.category}</h3>
            {g.apps.map((a) => (
              <label className="check" key={a.slug}>
                <input type="checkbox" checked={picked.includes(a.slug)} onChange={() => toggle(a.slug)} /> {a.name}
              </label>
            ))}
          </div>
        ))}
        {groups.length === 0 && <span className="muted">Nada encontrado.</span>}
      </div>
    </div>
  );
}

function DrivesCard({ root, profile, media, busy, act, onChanged }: CardProps) {
  const [withNinite, setWithNinite] = useState(true);
  const [results, setResults] = useState<Record<string, Written[]>>({});
  const u = media.unattend;
  const canNinite = !!media.ninite && media.ninite.selected.length > 0;

  const write = (d: VentoyDrive) =>
    act(`drive:${d.path}`, async () => {
      const r = await api.writeMedia(root, profile, d.path, withNinite && canNinite);
      setResults((cur) => ({ ...cur, [d.path]: r.written }));
      await onChanged();
      return `Pendrive ${d.path} atualizado.`;
    });

  return (
    <div className="card media-card">
      <div className="row">
        <div className="grow-col">
          <strong>
            Pendrive Ventoy
            <Help title="O que é gravado">
              O arquivo de resposta vai para o caminho que o <code>ventoy.json</code> indica, o <code>ventoy.json</code> do
              repo é mesclado ao do pendrive (o que só existe no pendrive é mantido) e o instalador do Ninite é baixado
              para a raiz. Um arquivo que mudaria é guardado antes com final <code>.bak</code>. A imagem do Windows não é
              copiada.
            </Help>
          </strong>
          <span className="muted">
            {media.ventoy?.auto_install.map((a) => `${a.image} usa ${a.template}`).join(" · ") || media.ventoy?.error || "Sem ventoy.json no perfil."}
          </span>
        </div>
        <label className="check" title={canNinite ? "" : "A lista do Ninite está vazia"}>
          <input type="checkbox" checked={withNinite && canNinite} disabled={!canNinite} onChange={(e) => setWithNinite(e.target.checked)} /> incluir o Ninite
        </label>
        <button className="ghost" disabled={!!busy} onClick={() => act("refresh", async () => void (await onChanged()))}>
          Procurar de novo
        </button>
      </div>

      {u?.ninite_hook === false && canNinite && (
        <p className="muted small-note">
          O arquivo de resposta atual não tem o script que procura o Ninite no primeiro login; o instalador vai para o
          pendrive, mas terá de ser aberto à mão.
        </p>
      )}
      {media.drives.length === 0 && <p className="muted">Nenhum pendrive Ventoy conectado (procura-se uma unidade com a pasta <code>ventoy</code>).</p>}

      <ul className="list">
        {media.drives.map((d) => (
          <li key={d.path} className="drive">
            <div className="row full">
              <strong className="mono">{d.path}</strong>
              <span className={FILE_STATE[d.unattend][1]}>resposta: {FILE_STATE[d.unattend][0]}</span>
              <span className={FILE_STATE[d.config][1]}>ventoy.json: {FILE_STATE[d.config][0]}</span>
              <span className={d.ninite ? "badge ok" : "badge warn"}>Ninite: {d.ninite ? "na raiz" : "ausente"}</span>
              {d.images.map((im) => (
                <span key={im.path} className={im.present ? "badge ok" : "badge bad"}>
                  {im.path}: {im.present ? "presente" : "não está no pendrive"}
                </span>
              ))}
              <span className="spacer" />
              <button className="primary" disabled={!!busy || !!u?.error} onClick={() => write(d)}>
                {busy === `drive:${d.path}` ? "Gravando…" : "Gravar neste pendrive"}
              </button>
            </div>
            {results[d.path] && (
              <ul className="written">
                {results[d.path].map((w) => (
                  <li key={w.path}>
                    <span className="chip">{ACTION[w.action]}</span> <span className="mono">{w.path}</span>
                    {w.backup && <span className="muted"> (anterior em {w.backup})</span>}
                  </li>
                ))}
              </ul>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
