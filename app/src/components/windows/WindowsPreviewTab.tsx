import { useCallback, useEffect, useMemo, useState } from "react";
import * as api from "../../api";
import { errorMessage } from "../../api";
import type { IconEntry, Icons, Layout, LayoutItem, LayoutSource } from "../../types";
import { Help } from "../Help";
import { IconsCard } from "./IconsCard";

interface Props {
  root: string;
  profile: string;
  /** Ids of the profile's apps that are ticked in Configurar. */
  apps: string[];
  /** False on a machine that is not Windows: the preview works, nothing can be applied. */
  windows: boolean;
}

const SOURCE: Record<LayoutSource, string> = {
  windows: "Windows",
  ninite: "Ninite",
  unattend: "Instalação automática",
  catalog: "Configurar",
};

const sameList = (a: string[], b: string[]) => a.length === b.length && a.every((x, i) => x === b[i]);

/** How an app's icon is drawn in the preview: the real one when the shortcuts were scanned, else a stand-in. */
interface Look {
  img?: string;
  /** The macOS style is on but there is no scanned icon: draw a plate with the app's colour. */
  plate?: "light" | "dark";
}

function Icon({ item, small, look }: { item: LayoutItem; small?: boolean; look?: Look }) {
  const size = small ? "app-icon small" : "app-icon";
  if (look?.img) return <img className={`${size} img`} src={look.img} alt="" aria-hidden />;
  if (look?.plate) {
    return (
      <span className={`${size} plate ${look.plate}`} style={{ color: look.plate === "light" ? item.color : "#fff" }} aria-hidden>
        {item.glyph}
      </span>
    );
  }
  return (
    <span className={size} style={{ background: item.color }} aria-hidden>
      {item.glyph}
    </span>
  );
}

const squash = (s: string) => s.toLowerCase().replace(/[^a-z0-9]+/g, "");
const ORDER = ["taskbar", "desktop", "start-menu"];

/** The scanned shortcut that stands for an app of the preview, if any. */
function entryFor(item: LayoutItem, entries: IconEntry[]): IconEntry | undefined {
  const key = squash(item.name);
  const first = squash(item.name.split(" ")[0]);
  return entries
    .filter((e) => {
      const n = squash(e.name);
      return n === key || (first.length >= 4 && n.startsWith(first)) || (n.length >= 4 && key.startsWith(n));
    })
    .sort((a, b) => ORDER.indexOf(a.scope) - ORDER.indexOf(b.scope))[0];
}

/** The desktop as it will look: taskbar pins you can rearrange, with their macOS icons, and the desktop icons. */
export function WindowsPreviewTab({ root, profile, apps, windows }: Props) {
  const [data, setData] = useState<Layout | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [busy, setBusy] = useState("");
  const [taskbar, setTaskbar] = useState<string[]>([]);
  const [clearStart, setClearStart] = useState(false);
  const [hideIcons, setHideIcons] = useState(false);
  const [autoHide, setAutoHide] = useState(false);
  const [dragId, setDragId] = useState<string | null>(null);
  const [overIndex, setOverIndex] = useState<number | null>(null);
  const [icons, setIcons] = useState<Icons | null>(null);
  const appsKey = apps.join(",");

  // the icon style and the last scan of the shortcuts, which the preview draws
  useEffect(() => {
    api
      .icons(root, "cached")
      .then(setIcons)
      .catch(() => setIcons(null));
  }, [root]);

  const load = useCallback(async () => {
    try {
      const d = await api.layout(root, profile, appsKey ? appsKey.split(",") : []);
      setData(d);
      setTaskbar(d.taskbar);
      setClearStart(d.clear_start);
      setHideIcons(d.hide_desktop_icons);
      setAutoHide(d.taskbar_autohide);
    } catch (e) {
      setError(errorMessage(e));
    }
  }, [root, profile, appsKey]);
  useEffect(() => void load(), [load]);

  const byId = useMemo(() => new Map((data?.items ?? []).map((i) => [i.id, i])), [data]);
  const pinned = taskbar.map((id) => byId.get(id)).filter((i): i is LayoutItem => !!i);
  const pool = (data?.items ?? []).filter((i) => !taskbar.includes(i.id));

  if (!data) return error ? <p className="banner bad">{error}</p> : <p className="banner">Lendo o layout…</p>;

  const shell = data.shell;
  const dirty = !sameList(taskbar, data.taskbar) || clearStart !== data.clear_start || hideIcons !== data.hide_desktop_icons || autoHide !== data.taskbar_autohide || !data.saved;

  // Moves an app to a position of the taskbar (or appends it); an app that is already there is moved.
  const place = (id: string, index?: number) =>
    setTaskbar((cur) => {
      const item = byId.get(id);
      if (!item?.pinnable) return cur;
      const at = index === undefined ? cur.length : index;
      const before = cur.slice(0, at).filter((x) => x !== id);
      const after = cur.slice(at).filter((x) => x !== id);
      return [...before, id, ...after];
    });
  const unpin = (id: string) => setTaskbar((cur) => cur.filter((x) => x !== id));
  const move = (id: string, by: -1 | 1) =>
    setTaskbar((cur) => {
      const i = cur.indexOf(id);
      const j = i + by;
      if (i < 0 || j < 0 || j >= cur.length) return cur;
      const next = [...cur];
      [next[i], next[j]] = [next[j], next[i]];
      return next;
    });
  const suggest = () => setTaskbar(data.items.filter((i) => i.pinnable).map((i) => i.id).slice(0, 10));

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
  const choice = { taskbar, clear_start: clearStart, hide_desktop_icons: hideIcons, taskbar_autohide: autoHide };
  const save = () => act("save", async () => {
    await api.setLayout(root, profile, choice);
    await load();
    return "Layout salvo em config/layout.yaml.";
  });
  const saveAndApply = () => act("apply", async () => {
    await api.setLayout(root, profile, choice);
    await api.applyLayout(root, profile);
    await load();
    return "Layout aplicado. A barra de tarefas reiniciou com os apps fixados na ordem escolhida; eles continuam lá depois de sair e entrar de novo.";
  });

  // The pack's icon (or the override) the icons step gives the app's shortcut; then the last scan, for the
  // apps drawn on a plate from their own icon; then a stand-in plate.
  const look = (item: LayoutItem): Look | undefined => {
    const packed = data.icons?.[item.id];
    if (packed) return { img: packed };
    if (!icons?.config.enabled && !icons?.applied && !icons?.scanned) return undefined;
    const style = icons?.config.style ?? "light";
    const entry = icons?.scanned ? entryFor(item, icons.entries) : undefined;
    if (entry && entry.from !== "skip" && entry.after) return { img: entry.after };
    if (entry?.from === "skip" && entry.before) return { img: entry.before };
    return icons?.config.enabled ? { plate: style } : undefined;
  };
  const systemIcons = shell.desktop_icons === "Custom" ? shell.desktop_system_icons : ["Lixeira"];
  const shortcuts = data.items.filter((i) => i.desktop && i.pinnable);

  const dropOnBar = (index: number) => {
    if (dragId) place(dragId, index);
    setDragId(null);
    setOverIndex(null);
  };

  return (
    <section className="layout">
      {error && <p className="banner bad">{error}</p>}
      {note && <p className="banner">{note}</p>}

      <div className="card row cta">
        <div className="grow-col">
          <strong>
            Preview do desktop
            <Help title="Como funciona">
              Uma simulação de como a área de trabalho fica: a barra de tarefas com os apps fixados e os ícones, já no
              estilo macOS que o cartão Ícones aplica. Arraste os apps da bandeja para a barra, e reordene arrastando os ícones dela. A barra segue as
              opções do arquivo de resposta (ela alinhada à esquerda, sem pesquisa etc.). “Salvar e aplicar” grava no
              Windows: os apps são fixados na barra como se você os fixasse à mão, substituindo os que estavam lá (sem
              atalhos duplicados); o menu Iniciar é limpo por política (pede elevação uma vez) e a barra de tarefas
              reinicia. Apps ainda não instalados ficam de fora da barra até você
              aplicar de novo. Com a barra vazia, a barra atual não é alterada.
            </Help>
          </strong>
          <span className="muted">
            {data.saved ? (dirty ? "Há mudanças ainda não salvas." : "Layout salvo.") : "Ainda não salvo: o preview parte do arquivo de resposta."}
          </span>
        </div>
        <button disabled={!!busy || !dirty} className="ghost" onClick={() => void load()}>
          Desfazer
        </button>
        <button disabled={!!busy || !dirty} onClick={save}>
          Salvar
        </button>
        <button className="primary" disabled={!!busy || !windows} title={windows ? "" : "Só dá para aplicar no Windows"} onClick={saveAndApply}>
          {busy === "apply" ? "Aplicando…" : "Salvar e aplicar"}
        </button>
      </div>

      <div className="card layout-options">
        <label className="check">
          <input type="checkbox" checked={clearStart} onChange={(e) => setClearStart(e.target.checked)} /> Limpar o menu Iniciar (tirar os apps fixados)
          {shell.start_pins_empty && <span className="chip">o arquivo de resposta já faz isso na instalação</span>}
        </label>
        <label className="check">
          <input type="checkbox" checked={hideIcons} onChange={(e) => setHideIcons(e.target.checked)} /> Ocultar os ícones da área de trabalho
          <span className="chip">clique direito › Exibir mostra de novo</span>
        </label>
        <label className="check">
          <input type="checkbox" checked={autoHide} onChange={(e) => setAutoHide(e.target.checked)} /> Esconder a barra de tarefas automaticamente
          <span className="chip">a dock do Windhawk se esconde junto</span>
        </label>
      </div>

      <div className="wscreen" aria-label="Preview da área de trabalho">
        <div className="desk-icons">
          {hideIcons ? (
            <span className="desk-hidden">ícones ocultos</span>
          ) : (
            <>
              {systemIcons.map((name) => (
                <span className="desk-icon" key={name}>
                  <span className="app-icon" style={{ background: "#3b82f6" }}>
                    {name.slice(0, 1)}
                  </span>
                  {name}
                </span>
              ))}
              {shortcuts.map((i) => (
                <span className="desk-icon" key={i.id} title="Atalho que o instalador costuma deixar">
                  <Icon item={i} look={look(i)} />
                  {i.name}
                </span>
              ))}
            </>
          )}
        </div>

        <div className={shell.taskbar_left ? "taskbar left" : "taskbar"} onDragOver={(e) => dragId && e.preventDefault()} onDrop={() => dropOnBar(taskbar.length)}>
          <span className="tb-side">{!shell.no_widgets && <span className="tb-btn" title="Widgets">☁</span>}</span>
          <div className="tb-group">
            {!shell.hide_search && <span className="tb-search">Pesquisar</span>}
            {!shell.hide_task_view && <span className="tb-btn" title="Visão de Tarefas">▭</span>}
            {pinned.map((i, index) => (
              <button
                key={i.id}
                className={dragId === i.id ? "tb-app dragging" : overIndex === index ? "tb-app over" : "tb-app"}
                draggable
                title={`${i.name} (arraste para mudar, Alt+setas move, Delete tira)`}
                onDragStart={(e) => {
                  setDragId(i.id);
                  e.dataTransfer.setData("text/plain", i.id);
                  e.dataTransfer.effectAllowed = "move";
                }}
                onDragEnd={() => {
                  setDragId(null);
                  setOverIndex(null);
                }}
                onDragOver={(e) => {
                  if (!dragId) return;
                  e.preventDefault();
                  e.stopPropagation();
                  setOverIndex(index);
                }}
                onDrop={(e) => {
                  e.stopPropagation();
                  dropOnBar(index);
                }}
                onKeyDown={(e) => {
                  if (e.altKey && e.key === "ArrowLeft") {
                    e.preventDefault();
                    move(i.id, -1);
                  } else if (e.altKey && e.key === "ArrowRight") {
                    e.preventDefault();
                    move(i.id, 1);
                  } else if (e.key === "Delete" || e.key === "Backspace") {
                    e.preventDefault();
                    unpin(i.id);
                  }
                }}
              >
                <Icon item={i} small look={look(i)} />
              </button>
            ))}
            {pinned.length === 0 && <span className="tb-empty">arraste apps para cá</span>}
          </div>
          <span className="tb-tray">PT &nbsp; 12:00</span>
        </div>
      </div>
      <p className="muted small-note">
        {shortcuts.length > 0 && !hideIcons ? "Os atalhos na área de trabalho são os que os instaladores costumam deixar; o Windows não os cria sempre. " : ""}
        {pinned.length === 0 ? "Com a barra vazia, o app não altera a barra de tarefas atual." : `${pinned.length} apps na barra.`}
      </p>

      <div
        className={dragId && taskbar.includes(dragId) ? "card tray drop-remove" : "card tray"}
        onDragOver={(e) => dragId && taskbar.includes(dragId) && e.preventDefault()}
        onDrop={() => {
          if (dragId) unpin(dragId);
          setDragId(null);
          setOverIndex(null);
        }}
      >
        <div className="row">
          <div className="grow-col">
            <strong>
              Bandeja de apps
              <Help title="De onde vêm os apps">
                São os apps que a máquina terá: os da lista do Ninite, os que o arquivo de resposta instala por script e
                os que estão marcados em Configurar. Java, Node e runtimes não têm janela, então não dá para fixá-los.
                Clique em um app ou arraste para a barra; arraste de volta para tirá-lo.
              </Help>
            </strong>
            <span className="muted">{pool.length ? "Clique ou arraste para a barra de tarefas." : "Todos os apps já estão na barra."}</span>
          </div>
          <button className="ghost" disabled={!!busy} onClick={suggest} title="Explorador, navegadores e os primeiros apps da lista">
            Sugerir
          </button>
          <button className="ghost" disabled={!!busy || taskbar.length === 0} onClick={() => setTaskbar([])}>
            Limpar a barra
          </button>
        </div>
        <div className="tray-items">
          {pool.map((i) => (
            <button
              key={i.id}
              className={i.pinnable ? "tray-item" : "tray-item off"}
              disabled={!i.pinnable}
              draggable={i.pinnable}
              title={i.pinnable ? `Fixar ${i.name}` : `${i.name} não tem janela para fixar`}
              onClick={() => place(i.id)}
              onDragStart={(e) => {
                setDragId(i.id);
                e.dataTransfer.setData("text/plain", i.id);
                e.dataTransfer.effectAllowed = "move";
              }}
              onDragEnd={() => {
                setDragId(null);
                setOverIndex(null);
              }}
            >
              <Icon item={i} small look={look(i)} />
              <span className="grow">{i.name}</span>
              {i.sources.map((s) => (
                <span key={s} className="chip">
                  {SOURCE[s]}
                </span>
              ))}
            </button>
          ))}
        </div>
      </div>

      {icons && <IconsCard root={root} windows={windows} data={icons} onData={setIcons} />}
    </section>
  );
}
