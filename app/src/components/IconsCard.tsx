import { open } from "@tauri-apps/plugin-dialog";
import { useEffect, useMemo, useState } from "react";
import * as api from "../api";
import { errorMessage } from "../api";
import type { IconEntry, IconPackPage, IconScope, Icons, IconsConfig } from "../types";
import { Help } from "./Help";

interface Props {
  root: string;
  /** False on a machine that is not Windows: the style can be chosen, nothing can be scanned or applied. */
  windows: boolean;
  data: Icons;
  onData: (d: Icons) => void;
}

const SCOPES: [IconScope, string][] = [
  ["desktop", "Área de trabalho"],
  ["start-menu", "Menu Iniciar"],
  ["taskbar", "Barra de tarefas"],
];

/** How many icons of the pack the gallery shows at first, and how many more each time. */
const PAGE = 60;

/** The taskbar modes of the mod Ilha e Dock estilo Mac, as the mod names them. */
const DOCK_MODES: Record<string, string> = {
  dock: "Dock",
  windowsTransparent: "Barra do Windows transparente",
  windowsApps: "Barra do Windows só com os apps",
  windows: "Barra do Windows sem mudanças",
};

const FROM: Record<IconEntry["from"], string> = { style: "estilo", pack: "pack", override: "escolhido", skip: "mantido" };

/** The macOS-style icons: how they are made, what they would change, and applying them. */
export function IconsCard({ root, windows, data, onData }: Props) {
  const [cfg, setCfg] = useState<IconsConfig>(data.config);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [scope, setScope] = useState<IconScope | "all">("all");
  const [query, setQuery] = useState("");
  const [pack, setPack] = useState(data.config.pack);
  useEffect(() => {
    setCfg(data.config);
    setPack(data.config.pack);
  }, [data.config]);

  // The gallery of the pack: a page of its icons, all or the ones a name finds.
  const [gallery, setGallery] = useState(false);
  const [packQuery, setPackQuery] = useState("");
  const [limit, setLimit] = useState(PAGE);
  const [page, setPage] = useState<IconPackPage | null>(null);
  const [galleryError, setGalleryError] = useState<string | null>(null);
  const packPath = data.pack.error ? "" : data.pack.path;
  useEffect(() => {
    if (!gallery || !packPath) return;
    let stale = false;
    const t = setTimeout(() => {
      api
        .iconPack(root, packQuery.trim(), limit)
        .then((p) => {
          if (stale) return;
          setPage(p);
          setGalleryError(null);
        })
        .catch((e) => !stale && setGalleryError(errorMessage(e)));
    }, 200);
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [gallery, packPath, packQuery, limit, root]);
  const builtin = cfg.pack.split("\\").join("/").replace(/\/+$/, "") === data.builtin_pack;

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

  // Saves a change and redraws the preview from the last scan, which is quick.
  const commit = (next: IconsConfig) => {
    setCfg(next);
    return act("save", async () => {
      await api.setIcons(root, next);
      onData(await api.icons(root, "cached"));
    });
  };

  const toggleScope = (s: IconScope) => commit({ ...cfg, scopes: cfg.scopes.includes(s) ? cfg.scopes.filter((x) => x !== s) : [...cfg.scopes, s] });
  const toggleSkip = (name: string) => commit({ ...cfg, skip: cfg.skip.includes(name) ? cfg.skip.filter((x) => x !== name) : [...cfg.skip, name] });

  const choosePack = async () => {
    const dir = await open({ directory: true, multiple: false, title: "Pasta do icon pack (arquivos .ico ou .png com o nome de cada app)" });
    if (typeof dir === "string") await commit({ ...cfg, pack: dir });
  };
  const chooseImage = async (name: string) => {
    const file = await open({ multiple: false, title: `Imagem para ${name}`, defaultPath: packPath || undefined, filters: [{ name: "Ícone ou imagem", extensions: ["ico", "png"] }] });
    if (typeof file === "string") await commit({ ...cfg, overrides: { ...cfg.overrides, [name]: file } });
  };
  const clearImage = (name: string) => {
    const overrides = { ...cfg.overrides };
    delete overrides[name];
    return commit({ ...cfg, overrides });
  };

  const scan = () =>
    act("scan", async () => {
      const d = await api.icons(root, "scan");
      onData(d);
      return `${d.entries.length} atalhos encontrados.`;
    });
  const apply = () =>
    act("apply", async () => {
      const next = { ...cfg, enabled: true };
      await api.setIcons(root, next);
      await api.applyIcons(root);
      onData(await api.icons(root, "scan"));
      return "Ícones aplicados. O Explorer reiniciou para mostrá-los; os que o Windows mantém em cache podem pedir sair da conta e entrar de novo.";
    });
  const restore = () =>
    act("restore", async () => {
      await api.restoreIcons(root);
      await api.setIcons(root, { ...cfg, enabled: false });
      onData(await api.icons(root, "scan"));
      return "Ícones do Windows de volta: todos os atalhos e ícones do sistema voltaram ao original e o cache de ícones foi refeito. O Explorer reiniciou.";
    });

  const dock = (undo: boolean) =>
    act("dock", async () => {
      await api.taskbarDock(root, undo);
      onData(await api.icons(root, "cached"));
      return undo
        ? "A barra de tarefas voltou ao modo que o mod tinha antes da dock."
        : "Dock ligada: a barra de tarefas virou a dock flutuante do Ilha e Dock estilo Mac.";
    });

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return data.entries.filter((e) => (scope === "all" || e.scope === scope) && (!q || e.name.toLowerCase().includes(q)));
  }, [data.entries, scope, query]);
  const count = (from: IconEntry["from"]) => data.entries.filter((e) => e.from === from).length;
  const dirtyPack = pack !== cfg.pack;

  return (
    <div className="card icons-card">
      <div className="row">
        <div className="grow-col">
          <strong>
            Ícones no estilo macOS
            <Help title="Como funciona e o que não dá para trocar">
              O Windows não tem tema de ícones: o ícone de um executável fica dentro do arquivo. O que dá para trocar são
              os atalhos (.lnk) da área de trabalho, do menu Iniciar e fixados na barra de tarefas, e alguns ícones do
              sistema (Este Computador, Lixeira...). O envsetup acha esses atalhos, pega o ícone que cada um mostra e
              desenha outro: o original numa placa de cantos arredondados, como o macOS faz com apps que não têm ícone
              próprio. O repositório traz um icon pack, o MacTahoe (os ícones de apps do tema MacTahoe, de vinceliuice,
              GPL-3.0, em windows/icons/mactahoe): o app que tiver ícone nele usa o do pack, sem alteração. Dá para usar
              outro: uma pasta de .ico ou .png com o nome de cada app, como “discord.png”. O ícone original de cada atalho fica guardado e
              “Restaurar” desfaz tudo. Não muda: o ícone de um .exe no Explorer, o de janelas abertas e o de apps que o
              Windows recria ao atualizar (aplique de novo depois).
            </Help>
          </strong>
          <span className="muted">
            {data.applied ? "Há ícones trocados nesta máquina." : "Nenhum ícone foi trocado ainda."}
            {data.scanned && ` ${data.entries.length} atalhos: ${count("style")} no estilo, ${count("pack")} do pack, ${count("override")} escolhidos, ${count("skip")} mantidos.`}
          </span>
        </div>
        <button disabled={!!busy || !windows} onClick={scan} title={windows ? "Procura os atalhos desta máquina (leva alguns segundos)" : "Só dá para escanear no Windows"}>
          {busy === "scan" ? "Escaneando…" : data.scanned ? "Escanear de novo" : "Escanear atalhos"}
        </button>
        <button disabled={!!busy || !windows} onClick={restore} title="Desfaz tudo: devolve o ícone original de todos os atalhos (área de trabalho, menu Iniciar, fixados) e dos ícones do sistema, apaga os ícones criados e refaz o cache de ícones do Windows">
          {busy === "restore" ? "Restaurando…" : "Voltar aos ícones do Windows"}
        </button>
        <button className="primary" disabled={!!busy || !windows} onClick={apply} title={windows ? "" : "Só dá para aplicar no Windows"}>
          {busy === "apply" ? "Aplicando…" : "Aplicar ícones"}
        </button>
      </div>

      {error && <p className="banner bad">{error}</p>}
      {note && <p className="banner">{note}</p>}

      <div className="icons-options">
        <label className="field">
          Placa
          <select value={cfg.style} disabled={!!busy} onChange={(e) => void commit({ ...cfg, style: e.target.value as IconsConfig["style"] })}>
            <option value="light">Clara</option>
            <option value="dark">Escura</option>
          </select>
        </label>
        <label className="field grow-col">
          Icon pack (pasta com .ico ou .png)
          <span className="row">
            <input
              className="paste"
              placeholder="Opcional: C:\icones\meu-pack"
              value={pack}
              disabled={!!busy}
              onChange={(e) => setPack(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && dirtyPack && void commit({ ...cfg, pack })}
              aria-label="Pasta do icon pack"
            />
            <button disabled={!!busy || !dirtyPack} onClick={() => void commit({ ...cfg, pack })}>
              Usar
            </button>
            <button disabled={!!busy} onClick={() => void choosePack()}>
              Escolher…
            </button>
            {!builtin && (
              <button disabled={!!busy} onClick={() => void commit({ ...cfg, pack: data.builtin_pack })} title="O pack que vem no repositório: ícones do macOS Tahoe para mais de mil apps">
                Usar MacTahoe
              </button>
            )}
            {cfg.pack && (
              <button className="ghost" disabled={!!busy} onClick={() => void commit({ ...cfg, pack: "" })}>
                Sem pack
              </button>
            )}
          </span>
        </label>
      </div>
      {data.pack.error && <p className="banner bad">Não foi possível ler o pack: {data.pack.error}</p>}
      {cfg.pack && !data.pack.error && (
        <p className="muted small-note">
          {builtin && "MacTahoe, os ícones do macOS Tahoe (tema de vinceliuice, GPL-3.0). "}
          {data.pack.count} ícones no pack.
          {data.pack.system.length > 0 && ` Também tem ícones do sistema: ${data.pack.system.join(", ")}.`}{" "}
          <button className="small-btn ghost" onClick={() => setGallery(!gallery)} aria-expanded={gallery}>
            {gallery ? "Esconder os ícones" : "Ver os ícones do pack"}
          </button>
        </p>
      )}
      {gallery && packPath && (
        <div className="pack-gallery">
          <div className="row">
            <input
              className="search"
              placeholder="Buscar no pack (discord, steam, terminal...)"
              value={packQuery}
              onChange={(e) => {
                setPackQuery(e.target.value);
                setLimit(PAGE);
              }}
              aria-label="Buscar ícone no pack"
            />
            {page && (
              <span className="muted small-note">
                {page.icons.length} de {page.total}
              </span>
            )}
          </div>
          {galleryError && <p className="banner bad">{galleryError}</p>}
          {page ? (
            page.icons.length > 0 ? (
              <ul className="pack-grid">
                {page.icons.map((ic) => (
                  <li key={ic.file} title={ic.file}>
                    {ic.image ? <img src={ic.image} alt="" loading="lazy" /> : <span className="icon-missing" />}
                    <span>{ic.name}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="muted small-note">Nenhum ícone do pack tem esse nome.</p>
            )
          ) : (
            !galleryError && <p className="muted small-note">Carregando os ícones…</p>
          )}
          {page && page.icons.length < page.total && (
            <button className="small-btn" onClick={() => setLimit(limit + PAGE * 2)}>
              Mostrar mais
            </button>
          )}
        </div>
      )}

      <div className="row">
        {SCOPES.map(([id, label]) => (
          <label className="check" key={id}>
            <input type="checkbox" checked={cfg.scopes.includes(id)} disabled={!!busy} onChange={() => void toggleScope(id)} /> {label}
          </label>
        ))}
        <label className="check" title="Este Computador, Lixeira, Arquivos do usuário e Rede, se o pack tiver this-pc, recycle-bin, user-files, network">
          <input type="checkbox" checked={cfg.system_icons} disabled={!!busy} onChange={(e) => void commit({ ...cfg, system_icons: e.target.checked })} /> Ícones do sistema (do pack)
        </label>
      </div>
      <p className="muted small-note">Marcar ou desmarcar um lugar vale na próxima vez que você escanear.</p>

      <div className="row">
        <div className="grow-col">
          <strong>
            Dock na barra de tarefas
            <Help title="O que a dock muda">
              Usa o mod “Ilha e Dock estilo Mac” do Windhawk: a barra de tarefas vira a dock compacta e flutuante dele.
              Ligar põe a opção “Barra de tarefas” do mod em “Dock” e instala o mod se ele ainda não estiver no Windhawk
              (pede elevação uma vez). Tamanho, ampliação, animações e o resto da dock são as configurações do mod, que
              ficam no backup do Windhawk (aba Windhawk). “Desligar dock” volta a opção ao que era antes.
            </Help>
          </strong>
          <span className="muted">
            {!data.dock_mode
              ? "O mod Ilha e Dock estilo Mac não está instalado (ou o Windhawk 2 não foi encontrado)."
              : data.dock
                ? "A barra de tarefas é a dock do Ilha e Dock estilo Mac."
                : `O mod está instalado, com a barra no modo “${DOCK_MODES[data.dock_mode] ?? data.dock_mode}”.`}
          </span>
        </div>
        <button disabled={!!busy || !windows || !data.dock} onClick={() => void dock(true)}>
          {busy === "dock" && data.dock ? "Desligando…" : "Desligar dock"}
        </button>
        <button disabled={!!busy || !windows || data.dock} onClick={() => void dock(false)}>
          {busy === "dock" && !data.dock ? "Ligando…" : data.dock_mode ? "Ligar dock" : "Instalar e ligar dock"}
        </button>
      </div>

      {data.scanned ? (
        <>
          <div className="row">
            <select value={scope} onChange={(e) => setScope(e.target.value as IconScope | "all")} aria-label="Filtrar por lugar">
              <option value="all">Todos os lugares</option>
              {SCOPES.map(([id, label]) => (
                <option key={id} value={id}>
                  {label}
                </option>
              ))}
            </select>
            <input className="search" placeholder="Buscar atalho" value={query} onChange={(e) => setQuery(e.target.value)} aria-label="Buscar atalho" />
            <span className="muted small-note">{shown.length} de {data.entries.length}</span>
          </div>
          <ul className="icon-grid">
            {shown.map((e) => {
              const skipped = e.from === "skip";
              const overridden = e.name in cfg.overrides || Object.keys(cfg.overrides).some((k) => k.toLowerCase() === e.name.toLowerCase());
              return (
                <li key={e.id} className={skipped ? "icon-entry skipped" : "icon-entry"} title={e.error ?? e.path}>
                  <span className="icon-pair">
                    {e.before ? <img src={e.before} alt="" /> : <span className="icon-missing" />}
                    <span aria-hidden>→</span>
                    {e.after ? <img src={e.after} alt={`Novo ícone de ${e.name}`} /> : <span className="icon-missing" />}
                  </span>
                  <span className="grow-col">
                    <strong className="grow">{e.name}</strong>
                    <span className="muted small-note">
                      {SCOPES.find(([id]) => id === e.scope)?.[1]} · {FROM[e.from]}
                      {e.themed && " · já trocado"}
                    </span>
                    {e.error && <span className="bad-text small-note">{e.error}</span>}
                  </span>
                  <span className="icon-actions">
                    <button className="small-btn ghost" disabled={!!busy} onClick={() => void toggleSkip(e.name)}>
                      {skipped ? "Trocar" : "Manter"}
                    </button>
                    <button className="small-btn ghost" disabled={!!busy} onClick={() => void (overridden ? clearImage(e.name) : chooseImage(e.name))}>
                      {overridden ? "Tirar imagem" : "Imagem…"}
                    </button>
                  </span>
                </li>
              );
            })}
          </ul>
        </>
      ) : (
        <p className="muted small-note">Escaneie os atalhos para ver, um a um, o ícone de hoje e o que ele passaria a ser.</p>
      )}
    </div>
  );
}
