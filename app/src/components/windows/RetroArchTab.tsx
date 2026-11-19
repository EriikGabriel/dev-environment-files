import { ask, open } from "@tauri-apps/plugin-dialog";
import { Cpu, FolderOpen, Gamepad2, HardDrive, ListMusic, Play, Save, ShieldCheck, Sparkles, Wrench } from "lucide-react";
import { motion } from "motion/react";
import { useCallback, useEffect, useState } from "react";
import * as api from "../../api";
import { errorMessage } from "../../api";
import type { RetroArch, RetroArchConfig } from "../../types";
import { Help } from "../Help";
import { WhereItLives, diskIcon, gitPlace, netIcon } from "./WhereItLives";
import { CountUp } from "../ui/CountUp";
import { Skeleton } from "../ui/Skeleton";

interface Props {
  root: string;
  /** Called after the folders changed, so the plan is recalculated. */
  onChanged: () => void;
}

const size = (b: number) =>
  b >= 1 << 30 ? `${(b / (1 << 30)).toFixed(1)} GB` : b >= 1 << 20 ? `${(b / (1 << 20)).toFixed(0)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`;

const when = (iso: string) => new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });

/** A color per system folder, from its name, so each console keeps its card color. */
const PALETTE = ["#f87171", "#fbbf24", "#4ade80", "#38bdf8", "#a78bfa", "#f472b6", "#2dd4bf", "#fb923c"];
const tint = (name: string) => PALETTE[[...name].reduce((h, c) => (h * 31 + c.charCodeAt(0)) >>> 0, 7) % PALETTE.length];

const yes = (v: string) => (v === "true" ? "ligado" : v === "false" ? "desligado" : v);

/** The portable RetroArch and its games: settings and saves in git, the program downloaded, BIOS and games in the store. */
export function RetroArchTab({ root, onChanged }: Props) {
  const [data, setData] = useState<RetroArch | null>(null);
  const [sized, setSized] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [busy, setBusy] = useState("");
  const [mirror, setMirror] = useState(false);
  const [exclude, setExclude] = useState("");

  const load = useCallback(
    async (sizes: boolean) => {
      try {
        const d = await api.retroarch(root, sizes);
        setData(d);
        setExclude(d.config.exclude.join(", "));
        if (sizes) setSized(true);
      } catch (e) {
        setError(errorMessage(e));
      }
    },
    [root],
  );
  useEffect(() => {
    void load(false).then(() => load(true));
  }, [load]);

  const act = async (what: string, fn: () => Promise<string | void>) => {
    setBusy(what);
    setError(null);
    setNote(null);
    try {
      const msg = await fn();
      await load(sized);
      if (msg) setNote(msg);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy("");
    }
  };

  const save = (c: Partial<RetroArchConfig>, msg?: string) => act("config", async () => (await api.setRetroArch(root, c), msg));
  const pickFolder = async (title: string, current: string) => {
    const dir = await open({ directory: true, multiple: false, title, defaultPath: current || undefined });
    return typeof dir === "string" ? dir : null;
  };

  if (!data) return error ? <p className="banner bad">{error}</p> : <Skeleton label="Lendo o RetroArch…" />;

  const { config: cfg, install: inst, snapshot: snap } = data;
  const m = snap?.manifest ?? null;
  const pending = data.requirements.filter((r) => !r.ok);
  const lists = inst.playlists.filter((p) => p.items > 0);
  const excluded = new Set(cfg.exclude.map((x) => x.toLowerCase()));
  const inGit = new Set(cfg.git.filter((g) => !/[*?/\\]/.test(g)).map((x) => x.toLowerCase()));
  const downloaded = new Set(data.program_dirs.map((x) => x.toLowerCase()));
  /** Where a top-level entry of the RetroArch folder goes. */
  const where = (name: string): "fora" | "GitHub" | "download" | "HD" => {
    const n = name.toLowerCase();
    if (excluded.has(n)) return "fora";
    if (inGit.has(n)) return "GitHub";
    if (!cfg.copy_program && (downloaded.has(n) || name.startsWith("("))) return "download";
    return "HD";
  };
  const restorable = !!m || data.git.files > 0;
  const toggleFolder = (name: string) =>
    save({ exclude: excluded.has(name.toLowerCase()) ? cfg.exclude.filter((x) => x.toLowerCase() !== name.toLowerCase()) : [...cfg.exclude, name] });

  return (
    <section className="retro">
      {error && <p className="banner bad">{error}</p>}
      {note && <p className="banner">{note}</p>}
      {data.running && <p className="banner bad">O RetroArch está aberto. Feche antes de salvar, restaurar ou corrigir as playlists: ele grava saves, estados e o retroarch.cfg enquanto roda.</p>}

      <div className="retro-hero card">
        <motion.span className="retro-logo" aria-hidden="true" initial={{ rotate: -20, scale: 0.6 }} animate={{ rotate: 0, scale: 1 }} transition={{ type: "spring", stiffness: 260, damping: 14 }}>
          <Gamepad2 size={34} />
        </motion.span>
        <div className="grow-col">
          <strong className="big">
            RetroArch portable
            <Help title="Como funciona">
              Cada parte do RetroArch fica onde pode ficar. A parte pessoal (retroarch.cfg, opções dos cores, playlists, saves, estados e capas) vai
              para o git, em windows/retroarch, sem senhas nem tokens. O programa, os assets e os cores são baixados do site oficial
              (buildbot.libretro.com), na versão gravada em config/retroarch.yaml. Os jogos e a BIOS não podem ir para o git (direitos autorais e o
              limite de tamanho do GitHub): ficam no HD que você escolhe, junto com as senhas tiradas do git.
            </Help>
          </strong>
          <span className="muted mono small">{inst.found ? cfg.path : `Nenhum RetroArch em ${cfg.path}`}</span>
        </div>
        <button className="ghost" disabled={!inst.found} onClick={() => act("open", async () => void (await api.openRetroArch(root, "folder")))}>
          <FolderOpen size={15} /> Pasta
        </button>
        <button className="primary" disabled={!inst.found || !data.windows || data.running} onClick={() => act("open", async () => void (await api.openRetroArch(root, "app")))}>
          <Play size={15} /> Abrir RetroArch
        </button>
      </div>

      <div className="stats">
        {(
          [
            ["Jogos", inst.game_count, <Gamepad2 size={18} />, `${size(inst.game_bytes)} em ${inst.systems.length} sistemas`],
            ["Cores", inst.cores.length, <Cpu size={18} />, "emuladores instalados"],
            ["Saves e estados", inst.saves + inst.states, <Save size={18} />, `${inst.saves} saves · ${inst.states} estados`],
            ["BIOS e sistema", inst.bios, <ShieldCheck size={18} />, "arquivos na pasta system"],
          ] as [string, number, React.ReactNode, string][]
        ).map(([label, n, icon, sub], i) => (
          <div key={label} className={`card stat tone-${"abca"[i]}`}>
            <span className="stat-icon" aria-hidden="true">
              {icon}
            </span>
            <span className="num">
              <CountUp value={n} />
            </span>
            <strong>{label}</strong>
            <span className="muted small">{sub}</span>
          </div>
        ))}
      </div>

      {inst.systems.length > 0 && (
        <>
          <h3>
            Sistemas <span className="muted">({cfg.games})</span>
          </h3>
          <div className="systems">
            {inst.systems.map((s, i) => (
              <motion.div
                key={s.name}
                className="card system spot"
                style={{ "--sys": tint(s.name) } as React.CSSProperties}
                initial={{ opacity: 0, y: 14, scale: 0.96 }}
                animate={{ opacity: 1, y: 0, scale: 1 }}
                transition={{ delay: 0.04 * i, type: "spring", stiffness: 300, damping: 24 }}
              >
                <span className="system-cart" aria-hidden="true">
                  {s.name.slice(0, 3).toUpperCase()}
                </span>
                <strong>{s.name}</strong>
                <span className="muted small">
                  {s.games} {s.games === 1 ? "jogo" : "jogos"} · {size(s.bytes)}
                </span>
              </motion.div>
            ))}
          </div>
        </>
      )}

      <h3>
        Precisa de atenção <span className="muted">({pending.length ? `${pending.length} pendente${pending.length > 1 ? "s" : ""}` : "tudo certo"})</span>
      </h3>
      {!data.windows && <p className="muted">Só dá para verificar no Windows.</p>}
      <ul className="list">
        {data.requirements.map((r) => (
          <li key={r.id}>
            <span className={r.ok ? "req-dot ok" : "req-dot warn"} aria-hidden="true" />
            <div className="grow-col">
              <div className="row">
                <strong>{r.title}</strong>
                <span className={r.ok ? "badge ok" : "badge warn"}>{r.ok ? "ok" : "pendente"}</span>
              </div>
              <span className="muted small-note">{r.about}</span>
              {r.detail && <span className="muted small-note">{r.detail}</span>}
            </div>
            {!r.ok && r.fixable && (
              <button disabled={!!busy || data.running} onClick={() => act("fix-" + r.id, async () => (await api.fixRetroArch(root, r.id), `${r.title}: corrigido.`))}>
                <Wrench size={14} /> {busy === "fix-" + r.id ? "Corrigindo…" : "Corrigir"}
              </button>
            )}
          </li>
        ))}
      </ul>

      {lists.length > 0 && (
        <>
          <h3>
            Playlists <span className="muted">({lists.length})</span>
          </h3>
          <ul className="list">
            {lists.map((p) => (
              <li key={p.name}>
                <ListMusic size={16} className="muted" aria-hidden="true" />
                <span className="grow">{p.name.replace(/^builtin\//, "histórico: ").replace("content_", "")}</span>
                <span className="muted small-note">{p.items} jogos</span>
                {p.missing ? (
                  <span className="badge warn" title={`${p.fixable} podem ser corrigidos`}>
                    {p.missing} não encontrados
                  </span>
                ) : (
                  <span className="badge ok">ok</span>
                )}
              </li>
            ))}
          </ul>
        </>
      )}

      <div className="retro-cols">
        <div className="card">
          <strong>Cores</strong>
          <ul className="plain core-list">
            {inst.cores.map((c) => (
              <li key={c.file} title={c.file}>
                <Cpu size={14} className="muted" aria-hidden="true" />
                <span className="grow">{c.name}</span>
              </li>
            ))}
            {inst.cores.length === 0 && <li className="muted">nenhum core</li>}
          </ul>
        </div>
        <div className="card">
          <strong>
            Configurações principais
            <Help title="retroarch.cfg">
              Um resumo do retroarch.cfg. Para mudar, use o menu do próprio RetroArch (ele salva ao sair) e depois atualize o snapshot.
            </Help>
          </strong>
          <dl className="kv settings-kv">
            {inst.settings.map((s) => (
              <div key={s.key} title={s.key}>
                <dt>{s.label}</dt>
                <dd>{yes(s.value)}</dd>
              </div>
            ))}
          </dl>
        </div>
      </div>

      <h3>
        Onde cada parte fica
        <Help title="Formatar o PC">
          Antes de formatar: feche o RetroArch, clique em Salvar e faça commit e push do repo. Depois: aplique o perfil de jogos (etapa
          Emuladores) ou clique em Restaurar. O app baixa o RetroArch e os cores, copia a BIOS e os jogos do HD e põe as configurações e saves
          do git, com as senhas de volta. Sem o HD, tudo menos os jogos e a BIOS volta.
        </Help>
      </h3>
      <WhereItLives
        places={[
          gitPlace(data.git, "configurações, opções dos cores, playlists, saves, estados e capas"),
          {
            title: "Download",
            icon: netIcon,
            tone: "net",
            head: cfg.copy_program ? "não (programa no HD)" : cfg.version ? `RetroArch ${cfg.version}` : "versão ainda não gravada",
            lines: [
              `${cfg.cores.length} cores: ${cfg.cores.join(", ") || "salve para gravar a lista"}`,
              `assets: ${cfg.bundles.join(", ")}`,
            ],
          },
          {
            title: "HD",
            icon: diskIcon,
            tone: "disk",
            head: snap ? (m ? `${size(m.bytes)} + ${size(m.game_bytes)} de jogos` : "nada salvo ainda") : "nenhum escolhido",
            lines: [
              snap ? `${cfg.store}` : "escolha uma pasta fora do disco do Windows",
              cfg.copy_program ? "BIOS, jogos, o programa e o que não vai pro git" : "BIOS (pasta system), jogos e as senhas tiradas do git",
            ],
            badge: m ? { text: `salvo ${when(m.saved_at)}` } : undefined,
          },
        ]}
      />

      <div className="card">
        <div className="row">
          <span className="cta-icon" aria-hidden="true">
            <HardDrive size={20} />
          </span>
          <div className="grow-col">
            <strong>HD (store)</strong>
            <span className="muted small-note">
              {snap ? `${snap.path} e ${snap.games}` : "Escolha onde guardar a BIOS e os jogos, num disco que não vai ser formatado (pode ser a mesma pasta do Playnite)."}
            </span>
          </div>
          {snap && (
            <button className="ghost" disabled={!!busy} onClick={() => act("open", async () => void (await api.openRetroArch(root, "store")))}>
              Abrir
            </button>
          )}
          <button
            disabled={!!busy}
            onClick={async () => {
              const dir = await pickFolder("Pasta para a BIOS e os jogos do RetroArch", cfg.store);
              if (dir) await save({ store: dir }, "Pasta salva em config/retroarch.yaml.");
            }}
          >
            {snap ? "Trocar pasta" : "Escolher pasta"}
          </button>
        </div>
        {snap?.error && <p className="banner bad">O manifesto do HD não pôde ser lido: {snap.error}</p>}
        {m && (
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
              <dt>No HD</dt>
              <dd>
                {size(m.bytes)} · {m.files.toLocaleString("pt-BR")} arquivos
              </dd>
            </div>
            <div>
              <dt>Jogos</dt>
              <dd title={m.systems.join(", ")}>
                {m.game_count} · {size(m.game_bytes)}
              </dd>
            </div>
            <div>
              <dt>Senhas guardadas</dt>
              <dd>{m.secrets}</dd>
            </div>
          </dl>
        )}
        <label className="check muted small-note">
          <input type="checkbox" checked={cfg.copy_program} disabled={!!busy} onChange={(e) => save({ copy_program: e.target.checked })} /> Guardar também o
          programa inteiro no HD (restaura sem internet; ~1 GB a mais)
        </label>
        <div className="row">
          <button
            className="primary"
            disabled={!!busy || !inst.found || data.running || !data.windows}
            title="Atualiza o git e copia para o HD só o que mudou"
            onClick={() =>
              act("save", async () => {
                const r = await api.saveRetroArch(root);
                return snap
                  ? `Salvo: RetroArch ${r.version}, ${size(r.bytes)} + ${size(r.game_bytes)} de jogos no HD. Faça commit de windows/retroarch e config/retroarch.yaml.`
                  : "Configurações e saves copiados para windows/retroarch. Sem HD escolhido, a BIOS e os jogos não foram guardados.";
              })
            }
          >
            <Sparkles size={15} /> {busy === "save" ? "Salvando…" : "Salvar"}
          </button>
          <button
            disabled={!!busy || !restorable || data.running || !data.windows}
            onClick={async () => {
              const from = [
                data.git.files ? "configurações e saves do git" : "",
                m ? `BIOS e jogos do HD (${when(m.saved_at)})` : "",
                !cfg.copy_program || !m ? "o programa e os cores baixados se faltarem" : "",
              ].filter(Boolean);
              const ok = await ask(
                `Restaurar em ${cfg.path}${cfg.games ? ` e ${cfg.games}` : ""}: ${from.join(", ")}?` +
                  (mirror ? "\n\nO que estiver nas pastas e não estiver no HD será apagado." : "\n\nArquivos que já estão nas pastas são substituídos."),
                { title: "Restaurar o RetroArch", kind: "warning" },
              );
              if (ok)
                await act("restore", async () => {
                  await api.restoreRetroArch(root, mirror);
                  onChanged();
                  return "RetroArch restaurado; playlists apontadas para os jogos.";
                });
            }}
          >
            {busy === "restore" ? "Restaurando… (pode baixar ~500 MB)" : "Restaurar nesta máquina"}
          </button>
          <label className="check muted small-note" title="Também apaga das pastas o que o HD não tem">
            <input type="checkbox" checked={mirror} onChange={(e) => setMirror(e.target.checked)} /> espelhar exatamente
          </label>
        </div>
      </div>

      <h3>Configuração</h3>
      <div className="card playnite-config">
        {(
          [
            ["path", "Pasta do RetroArch", cfg.path],
            ["games", "Pasta dos jogos (uma subpasta por sistema)", cfg.games],
          ] as const
        ).map(([key, label, value]) => (
          <label key={key} className="field">
            <span className="muted">{label}</span>
            <div className="row">
              <input type="text" className="grow" value={value} readOnly />
              <button
                disabled={!!busy}
                onClick={async () => {
                  const dir = await pickFolder(label, value);
                  if (dir) await save({ [key]: dir });
                }}
              >
                Trocar
              </button>
              {key === "games" && value && (
                <button className="ghost" disabled={!!busy} onClick={() => act("open", async () => void (await api.openRetroArch(root, "games")))}>
                  Abrir
                </button>
              )}
            </div>
          </label>
        ))}
        <label className="field">
          <span className="muted">Nunca copiados (pastas relativas à do RetroArch e padrões de arquivo, separados por vírgula)</span>
          <div className="row">
            <input type="text" className="grow" value={exclude} onChange={(e) => setExclude(e.target.value)} />
            <button disabled={!!busy || exclude === cfg.exclude.join(", ")} onClick={() => save({ exclude: exclude.split(",").map((x) => x.trim()).filter(Boolean) })}>
              Salvar
            </button>
          </div>
        </label>
        <label className="check">
          <input type="checkbox" checked={cfg.start_menu} disabled={!!busy} onChange={(e) => save({ start_menu: e.target.checked })} /> Depois de restaurar,
          pôr o RetroArch no menu Iniciar e em Configurações › Apps
        </label>
      </div>

      {inst.found && (
        <>
          <h3>
            Pastas do RetroArch <span className="muted">{sized ? `(${size(inst.bytes)} vão para o HD, mais ${size(inst.game_bytes)} de jogos)` : "(medindo…)"}</span>
          </h3>
          {sized && (
            <ul className="list folder-sizes">
              {inst.folders.map((f) => {
                const w = where(f.name);
                const toggles = w === "HD" || w === "fora";
                return (
                  <li key={f.name} className={w === "fora" ? "off" : ""}>
                    <span className="grow">{f.name}</span>
                    <span className="muted small-note">{f.files.toLocaleString("pt-BR")} arquivos</span>
                    <span className="size">{size(f.bytes)}</span>
                    {toggles && !f.name.startsWith("(") ? (
                      <button className={`small-btn ghost dest dest-${w}`} disabled={!!busy} onClick={() => toggleFolder(f.name)} title={w === "fora" ? "Copiar para o HD" : "Nunca copiar"}>
                        {w}
                      </button>
                    ) : (
                      <span className={`small-btn dest dest-${w}`}>{w}</span>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </>
      )}
    </section>
  );
}
