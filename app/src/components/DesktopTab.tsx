import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import * as api from "../api";
import { errorMessage } from "../api";
import type { DesktopPreview, DockItem, Plan, SystemInfo } from "../types";
import { Help } from "./Help";

interface Props {
  root: string;
  profile: string;
  info: SystemInfo | null;
  plan: Plan | null;
  /** Called after the dock order was saved, so the plan shows it as pending. */
  onSaved: () => void;
}

/** What a panel widget looks like in the preview. */
const WIDGET: Record<string, { glyph: string; text?: string }> = {
  launcher: { glyph: "⌕" },
  activities: { glyph: "", text: "Atividades" },
  clock: { glyph: "", text: "" }, // replaced by the live clock
  sysmon: { glyph: "▮▯", text: "12%" },
  active_window: { glyph: "▭", text: "kitty" },
  media: { glyph: "♪", text: "Nada tocando" },
  workspaces: { glyph: "" },
  tray: { glyph: "◌ ◌ ◌" },
  notifications: { glyph: "🔔" },
  screen_tools: { glyph: "⛶" },
  screen_recorder: { glyph: "⏺" },
  arch_updater: { glyph: "⟳", text: "0" },
  battery: { glyph: "▰", text: "86%" },
  volume: { glyph: "🔊" },
  brightness: { glyph: "☀" },
  clipboard: { glyph: "⧉" },
  keybinds: { glyph: "⌨" },
  "control-center": { glyph: "⚙" },
  network: { glyph: "⌁" },
  power: { glyph: "⏻" },
};

const tint = (name: string) => `hsl(${[...name].reduce((h, c) => (h * 31 + c.charCodeAt(0)) % 360, 7)} 55% 45%)`;

function useClock() {
  const [now, setNow] = useState(new Date());
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 15_000);
    return () => clearInterval(t);
  }, []);
  return now;
}

export function DesktopTab({ root, profile, info, plan, onSaved }: Props) {
  const [data, setData] = useState<DesktopPreview | null>(null);
  const [items, setItems] = useState<DockItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [edit, setEdit] = useState(false);
  const [adding, setAdding] = useState(false);
  const [query, setQuery] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [dragFrom, setDragFrom] = useState<number | null>(null);
  const [dragOver, setDragOver] = useState<number | null>(null);
  const now = useClock();
  const frame = useRef<HTMLDivElement>(null);

  const load = useCallback(async () => {
    try {
      setError(null);
      const d = await api.desktop(root, profile);
      setData(d);
      setItems(d.dock.items);
    } catch (e) {
      setError(errorMessage(e));
    }
  }, [root, profile]);

  useEffect(() => {
    setData(null);
    void load();
  }, [load]);

  const dirty = useMemo(
    () => !!data && items.map((i) => i.id).join("\n") !== data.dock.items.map((i) => i.id).join("\n"),
    [data, items],
  );
  const dockPending = plan?.steps.some((s) => s.kind === "dock" && s.pending) ?? false;

  if (error) return <p className="banner bad">{error}</p>;
  if (!data) return <p className="muted">Montando o preview…</p>;
  if (!data.kind) return <p className="banner">Este perfil não descreve um desktop para visualizar.</p>;

  const pos = data.dock.position;
  const vertical = pos === "left" || pos === "right";
  const gnome = data.kind === "gnome";
  const clock = now.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
  const date = now.toLocaleDateString("pt-BR", { weekday: "short", day: "2-digit", month: "short" });
  const style = {
    "--p-accent": data.colors.accent || "#5eead4",
    "--p-panel": data.colors.panel || "#101010",
    "--p-text": data.colors.text || "#eee",
  } as CSSProperties;

  const move = (from: number, to: number) => {
    if (to < 0 || to >= items.length || from === to) return;
    const next = [...items];
    next.splice(to, 0, next.splice(from, 1)[0]);
    setItems(next);
    setSaved(false);
  };
  const remove = (i: number) => {
    setItems(items.filter((_, k) => k !== i));
    setSaved(false);
  };
  const add = (id: string, name: string) => {
    if (items.some((i) => i.id === id)) return;
    setItems([...items, { id, name, icon: "", found: true }]);
    setAdding(false);
    setQuery("");
    setSaved(false);
  };
  const save = async () => {
    setSaving(true);
    try {
      await api.setDock(root, profile, items.map((i) => i.id));
      await load(); // re-read: names and icons of newly added apps
      setSaved(true);
      onSaved();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setSaving(false);
    }
  };

  const widget = (name: string) => {
    const w = WIDGET[name] ?? { glyph: "", text: name };
    if (name === "clock") return <span className="pw clock">{gnome ? `${date}  ${clock}` : clock}</span>;
    if (name === "workspaces")
      return (
        <span className="pw ws" aria-label="Áreas de trabalho">
          {[1, 2, 3, 4].map((n) => (
            <i key={n} className={n === 1 ? "on" : ""} />
          ))}
        </span>
      );
    return (
      <span className="pw" title={name}>
        {w.glyph}
        {w.text && <span className="lbl"> {w.text}</span>}
      </span>
    );
  };

  const matches = data.apps.filter(
    (a) => !items.some((i) => i.id === a.id) && (!query || a.name.toLowerCase().includes(query.toLowerCase()) || a.id.toLowerCase().includes(query.toLowerCase())),
  );

  return (
    <section className="dpreview">
      <div className="row">
        <h3 className="inline">
          Preview do desktop
          <Help title="O que é este preview">
            Uma simulação de como o desktop deste perfil fica: wallpaper, painel, dock e cores vêm do perfil, e os
            ícones e nomes dos apps vêm do que está instalado nesta máquina. Apps ou temas que ainda não estão
            instalados aparecem como um quadrado colorido. Não mostra a posição exata de janelas.
          </Help>
        </h3>
        <span className="spacer" />
        <span className="chip">{gnome ? "GNOME" : "niri + Noctalia"}</span>
        {data.icon_theme && <span className="chip">ícones {data.icon_theme}</span>}
      </div>

      <div ref={frame} className={`screen kind-${data.kind} dock-${pos}`} style={style}>
        <div className="wall" style={data.wallpaper ? { backgroundImage: `url(${data.wallpaper})` } : undefined} />

        <div className={`panel bar-${data.bar.position} ${gnome ? "gnome" : "capsules"}`}>
          <div className="pa start">{data.bar.start.map((w) => <span key={w}>{widget(w)}</span>)}</div>
          <div className="pa center">{data.bar.center.map((w) => <span key={w}>{widget(w)}</span>)}</div>
          <div className="pa end">{data.bar.end.map((w) => <span key={w}>{widget(w)}</span>)}</div>
        </div>

        <div className="win term" aria-hidden="true">
          <div className="win-bar">
            <i /><i /><i />
            <span>{gnome ? "Terminal" : "kitty"}</span>
          </div>
          <pre>
            <b>{info?.user ?? "user"}@{info?.hostname ?? "host"}</b> <em>~</em>
            {"\n"}❯ fastfetch
            {"\n"}<b>OS</b>      {info?.name || "Linux"}
            {"\n"}<b>Kernel</b>  {info?.kernel || "—"}
            {"\n"}<b>Shell</b>   {info?.shell || "—"}
            {"\n"}<b>WM</b>      {gnome ? "Mutter" : "niri"}
            {"\n"}❯ <span className="caret" />
          </pre>
        </div>

        <div className={`dock ${data.dock.autohide ? "autohide" : ""} ${edit ? "editing" : ""}`} role="list" aria-label="Dock">
          {items.map((it, i) => (
            <div
              key={it.id}
              role="listitem"
              className={`dock-item ${dragOver === i ? "over" : ""} ${dragFrom === i ? "dragging" : ""}`}
              draggable={edit}
              onDragStart={(e) => {
                setDragFrom(i);
                e.dataTransfer.effectAllowed = "move";
                e.dataTransfer.setData("text/plain", it.id);
              }}
              onDragOver={(e) => {
                if (dragFrom === null) return;
                e.preventDefault();
                setDragOver(i);
              }}
              onDrop={(e) => {
                e.preventDefault();
                if (dragFrom !== null) move(dragFrom, i);
                setDragFrom(null);
                setDragOver(null);
              }}
              onDragEnd={() => {
                setDragFrom(null);
                setDragOver(null);
              }}
            >
              <span className="tip">{it.name}{!it.found && " (não instalado)"}</span>
              {it.icon ? (
                <img src={it.icon} alt={it.name} draggable={false} />
              ) : (
                <span className="ph" style={{ background: tint(it.id) }} aria-label={it.name}>
                  {it.name.slice(0, 1).toUpperCase()}
                </span>
              )}
              {edit && (
                <>
                  <button className="rm" aria-label={`Remover ${it.name}`} onClick={() => remove(i)}>
                    ✕
                  </button>
                  <span className="mv">
                    <button aria-label={`Mover ${it.name} para antes`} disabled={i === 0} onClick={() => move(i, i - 1)}>
                      {vertical ? "▲" : "◀"}
                    </button>
                    <button aria-label={`Mover ${it.name} para depois`} disabled={i === items.length - 1} onClick={() => move(i, i + 1)}>
                      {vertical ? "▼" : "▶"}
                    </button>
                  </span>
                </>
              )}
            </div>
          ))}
          {items.length === 0 && <span className="dock-empty">dock vazia</span>}
        </div>
      </div>

      {data.wallpaper_error && <p className="banner">Wallpaper indisponível: {data.wallpaper_error}</p>}

      <div className="card dock-editor">
        <div className="row">
          <strong>Dock</strong>
          <Help title="Ordem da dock">
            A ordem dos apps é guardada no perfil e escrita no desktop de verdade ao aplicar: no Noctalia ela vai para o{" "}
            <code>config.toml</code> (o arquivo do repo, por causa do symlink) e no GNOME para os favoritos. Apps que não
            existem na máquina são ignorados no GNOME. Arraste os ícones ou use as setas para reordenar.
          </Help>
          {dirty && <span className="badge warn">alterações não salvas</span>}
          {!dirty && saved && <span className="badge ok">salvo no perfil</span>}
          {!dirty && !saved && dockPending && <span className="badge warn">pendente: aplique para escrever no desktop</span>}
          {!dirty && !saved && !dockPending && plan && <span className="badge ok">dock em dia</span>}
          <span className="spacer" />
          <button className={edit ? "primary small-btn" : "small-btn"} aria-pressed={edit} onClick={() => setEdit((e) => !e)}>
            {edit ? "Concluir edição" : "Editar dock"}
          </button>
        </div>

        {edit && (
          <div className="dock-actions">
            <button className="small-btn" onClick={() => setAdding((a) => !a)} aria-expanded={adding}>
              + Adicionar app
            </button>
            <button className="small-btn ghost" disabled={!dirty} onClick={() => (setItems(data.dock.items), setSaved(false))}>
              Desfazer
            </button>
            <span className="spacer" />
            <button className="primary small-btn" disabled={!dirty || saving} onClick={() => void save()}>
              {saving ? "Salvando…" : "Salvar ordem"}
            </button>
          </div>
        )}

        {edit && adding && (
          <div className="picker">
            <input autoFocus placeholder="Buscar app instalado…" value={query} onChange={(e) => setQuery(e.target.value)} aria-label="Buscar app" />
            <ul className="picker-list">
              {matches.slice(0, 40).map((a) => (
                <li key={a.id}>
                  <button className="entry" onClick={() => add(a.id, a.name)}>
                    <span className="grow">{a.name}</span>
                    <span className="muted mono small">{a.id}</span>
                  </button>
                </li>
              ))}
              {matches.length === 0 && <li className="muted pad">Nenhum app encontrado.</li>}
            </ul>
          </div>
        )}
      </div>
    </section>
  );
}
