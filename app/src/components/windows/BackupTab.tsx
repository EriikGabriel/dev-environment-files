import { ask, open } from "@tauri-apps/plugin-dialog";
import { AlertTriangle, CheckCircle2, CircleDashed, CloudUpload, Download, HardDrive, LoaderCircle, Power, RotateCcw, Save, ShieldCheck, Usb } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { type ReactNode, useCallback, useEffect, useState } from "react";
import * as api from "../../api";
import { errorMessage } from "../../api";
import type { Backup } from "../../types";
import { Help } from "../Help";
import { Skeleton } from "../ui/Skeleton";

interface Props {
  root: string;
  /** Opens another tab of the app. */
  onNavigate: (tab: "setup" | "install" | "playnite" | "retroarch") => void;
  /** Called after something changed what the profile would do. */
  onChanged: () => void;
}

type Mode = "before" | "after";
type State = "ok" | "todo" | "warn" | "optional";

interface Step {
  id: string;
  title: string;
  /** What the step is about, always shown. */
  about: string;
  state: State;
  /** The current situation, in a line. */
  detail: string;
  action?: ReactNode;
}

const when = (iso: string) => new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
const ago = (iso: string) => {
  const h = (Date.now() - new Date(iso).getTime()) / 36e5;
  return h < 1 ? "há menos de uma hora" : h < 48 ? `há ${Math.round(h)} h` : `há ${Math.round(h / 24)} dias`;
};

const ICON: Record<State, ReactNode> = {
  ok: <CheckCircle2 size={22} />,
  todo: <CircleDashed size={22} />,
  warn: <AlertTriangle size={22} />,
  optional: <CircleDashed size={22} />,
};

/** A guided checklist for formatting the PC: what to do before, and how to get everything back after. */
export function BackupTab({ root, onNavigate, onChanged }: Props) {
  const [data, setData] = useState<Backup | null>(null);
  const [mode, setMode] = useState<Mode | null>(null);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);

  const load = useCallback(
    async (fetch = false) => {
      try {
        const d = await api.backup(root, fetch);
        setData(d);
        // a machine without Playnite and RetroArch was just formatted
        setMode((m) => m ?? (!d.playnite.install.found && !d.retroarch.install.found ? "after" : "before"));
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
      await load(false);
      onChanged();
      if (msg) setNote(msg);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy("");
    }
  };

  if (!data) return error ? <p className="banner bad">{error}</p> : <Skeleton label="Conferindo o backup…" />;

  const { git, playnite: pn, retroarch: ra } = data;
  const pnSaved = pn.snapshot?.manifest ?? null;
  const raSaved = ra.snapshot?.manifest ?? null;
  const store = ra.config.store || pn.config.store;
  const running = [...pn.running, ...(ra.running ? ["RetroArch"] : [])];
  const button = (id: string, label: string, icon: ReactNode, fn: () => Promise<string | void>, opts: { primary?: boolean; disabled?: boolean; busyLabel?: string } = {}) => (
    <button className={opts.primary ? "primary" : ""} disabled={!!busy || opts.disabled} onClick={() => act(id, fn)}>
      {busy === id ? <LoaderCircle size={15} className="spin" /> : icon} {busy === id ? (opts.busyLabel ?? "Aguarde…") : label}
    </button>
  );
  const pickStore = async () => {
    const dir = await open({ directory: true, multiple: false, title: "Pasta do HD para o backup (num disco que não será formatado)", defaultPath: store || undefined });
    if (typeof dir !== "string") return;
    await act("store", async () => {
      await api.setPlaynite(root, { store: dir });
      await api.setRetroArch(root, { store: dir });
      return `Playnite e RetroArch vão guardar em ${dir}.`;
    });
  };

  const savedState = (saved: { saved_at: string } | null, pending: number, found: boolean): State => (!found ? "warn" : !saved || pending > 0 ? "todo" : "ok");
  const savedDetail = (saved: { saved_at: string } | null, pending: number, found: boolean, what: string) =>
    !found
      ? `${what} não está nesta máquina.`
      : !saved
        ? "Ainda não foi salvo no HD."
        : `Salvo em ${when(saved.saved_at)} (${ago(saved.saved_at)})${pending ? ` · ${pending} ${pending === 1 ? "arquivo mudou" : "arquivos mudaram"} desde então` : " · nada mudou desde então"}.`;

  const gitOk = git.repo && git.dirty.length === 0 && git.ahead === 0 && !!git.upstream;
  const sg = data.store_git;
  // next to the data folder the installer made (Documents\envsetup -> Documents\dev-environment-files)
  const cloneDir = data.root.replace(/[\\/][^\\/]+[\\/]?$/, "") + "\\dev-environment-files";
  const cloneRepo = () =>
    act("clone", async () => {
      if (!sg) return;
      const ok = await ask(
        `Baixar o backup de ${sg.url} (branch ${sg.branch}, enviado em ${when(sg.pushed_at)}) para ${cloneDir}?\n\n` +
          "Se o Git não estiver instalado, ele é instalado antes pelo winget. Depois, o app passa a usar essa pasta.",
        { title: "Baixar do GitHub", kind: "info", okLabel: "Baixar", cancelLabel: "Cancelar" },
      );
      if (!ok) return;
      const r = await api.backupClone(root, cloneDir, sg.url, sg.branch);
      await api.setRoot(r.path);
      window.location.reload();
    });

  const before: Step[] = [
    {
      id: "store",
      title: "Escolher o HD",
      about: "Uma pasta num disco que não será formatado. Lá ficam a BIOS, a biblioteca e as mídias do Playnite e as senhas que não vão para o GitHub.",
      state: store ? "ok" : "todo",
      detail: store ? `Playnite e RetroArch guardam em ${store}.` : "Nenhuma pasta escolhida.",
      action: (
        <button disabled={!!busy} onClick={pickStore}>
          <HardDrive size={15} /> {store ? "Trocar" : "Escolher pasta"}
        </button>
      ),
    },
    {
      id: "closed",
      title: "Fechar o Playnite e o RetroArch",
      about: "Com eles abertos, a biblioteca e os saves não podem ser copiados.",
      state: running.length ? "todo" : "ok",
      detail: running.length ? `Aberto: ${running.join(", ")}.` : "Os dois estão fechados.",
      action: pn.running.length ? button("close", "Fechar o Playnite", <Power size={15} />, async () => (await api.closePlaynite(root), "Playnite fechado.")) : undefined,
    },
    {
      id: "playnite",
      title: "Salvar o Playnite",
      about: "As configurações vão para o GitHub (sem chaves de API); a biblioteca, capas, trailers, temas e extensões vão para o HD.",
      state: savedState(pnSaved, pn.git.pending, pn.install.found),
      detail: savedDetail(pnSaved, pn.git.pending, pn.install.found, "O Playnite"),
      action: button("save-pn", "Salvar", <Save size={15} />, async () => {
        const m = await api.savePlaynite(root);
        return `Playnite salvo: ${(m.bytes / (1 << 30)).toFixed(1)} GB no HD.`;
      }, { disabled: !pn.install.found || !store || pn.running.length > 0, busyLabel: "Salvando… (a primeira vez leva minutos)" }),
    },
    {
      id: "retroarch",
      title: "Salvar o RetroArch",
      about: "Configurações, saves, estados e playlists vão para o GitHub; a BIOS vai para o HD. Os jogos ficam onde estão, no HD.",
      state: savedState(raSaved, ra.git.pending, ra.install.found),
      detail: savedDetail(raSaved, ra.git.pending, ra.install.found, "O RetroArch"),
      action: button("save-ra", "Salvar", <Save size={15} />, async () => {
        const m = await api.saveRetroArch(root);
        return `RetroArch ${m.version} salvo.`;
      }, { disabled: !ra.install.found || !store || ra.running, busyLabel: "Salvando…" }),
    },
    {
      id: "git",
      title: "Enviar para o GitHub",
      about: "Leva as configurações e os saves para o repositório: é de lá que a máquina nova os puxa.",
      state: !git.repo ? "warn" : gitOk ? "ok" : "todo",
      detail: !git.repo
        ? "A pasta de dados não é um clone do Git: escolha um clone do repositório (botão da pasta no topo) para poder enviar."
        : gitOk
          ? `Tudo enviado para ${git.upstream}.`
          : `${git.dirty.length} ${git.dirty.length === 1 ? "arquivo mudado" : "arquivos mudados"}${git.ahead ? `, ${git.ahead} commit(s) para enviar` : ""}${git.upstream ? "" : " (o branch ainda não existe no GitHub)"}.`,
      action: git.repo
        ? button("push", "Enviar", <CloudUpload size={15} />, async () => {
            const r = await api.backupPush(root);
            return r.committed ? `${r.files} arquivos enviados para ${r.upstream}.` : `Nada novo para commitar; ${r.upstream} está em dia.`;
          }, { primary: true, disabled: gitOk || running.length > 0, busyLabel: "Enviando…" })
        : undefined,
    },
    {
      id: "usb",
      title: "Pendrive do Windows (opcional)",
      about: "Instala o Windows sem perguntas, com o Ninite, se for reinstalar por pendrive.",
      state: "optional",
      detail: "Feito na aba Instalação.",
      action: (
        <button className="ghost" onClick={() => onNavigate("install")}>
          <Usb size={15} /> Abrir Instalação
        </button>
      ),
    },
  ];

  const after: Step[] = [
    {
      id: "clone",
      title: "Puxar o backup do GitHub",
      about: "A pasta de dados precisa ser um clone do repositório, atualizado: ele traz as configurações e os saves mais recentes.",
      state: !git.repo ? "warn" : git.behind > 0 ? "todo" : "ok",
      detail: !git.repo
        ? sg
          ? `A pasta de dados é a cópia do instalador. O HD diz que o último backup foi para ${sg.url} (${sg.branch}) em ${when(sg.pushed_at)}.`
          : "A pasta de dados é a cópia do instalador, que pode estar desatualizada. Conecte o HD (ele diz de onde baixar) ou clone o repositório e escolha a pasta (botão da pasta no topo)."
        : git.behind > 0
          ? `${git.behind} commit(s) novos em ${git.upstream}.`
          : `Em dia com ${git.upstream || git.branch}.`,
      action: git.repo ? (
        button("pull", "Atualizar", <Download size={15} />, async () => (await api.backupPull(root), "Pasta de dados atualizada."), { disabled: git.behind === 0 && !!git.upstream, busyLabel: "Atualizando…" })
      ) : sg ? (
        <button className="primary" disabled={!!busy} onClick={cloneRepo}>
          {busy === "clone" ? <LoaderCircle size={15} className="spin" /> : <Download size={15} />} {busy === "clone" ? "Baixando…" : "Baixar do GitHub"}
        </button>
      ) : undefined,
    },
    {
      id: "hd",
      title: "Conectar o HD",
      about: "O disco com a pasta do backup. Se a letra mudou (E: em vez de D:), a pasta é encontrada sozinha.",
      state: pnSaved && raSaved ? "ok" : store ? "warn" : "todo",
      detail:
        pnSaved && raSaved
          ? `Backup encontrado em ${store}: Playnite de ${when(pnSaved.saved_at)}, RetroArch de ${when(raSaved.saved_at)}.`
          : store
            ? `Não achei o backup em ${store}${pnSaved ? " (falta o RetroArch)" : raSaved ? " (falta o Playnite)" : ""}. Conecte o disco ou escolha a pasta.`
            : "Nenhuma pasta escolhida.",
      action: (
        <button disabled={!!busy} onClick={pickStore}>
          <HardDrive size={15} /> Escolher pasta
        </button>
      ),
    },
    {
      id: "games",
      title: "Pasta de jogos",
      about: "Os jogos ficam no HD (não são copiados): o RetroArch e o Playnite apontam para lá.",
      state: !ra.config.games ? "optional" : ra.install.game_count > 0 ? "ok" : "warn",
      detail: !ra.config.games
        ? "Sem pasta de jogos."
        : ra.install.game_count > 0
          ? `${ra.install.game_count} jogos em ${ra.config.games}.`
          : `${ra.config.games} não foi encontrada: conecte o HD.`,
    },
    {
      id: "restore",
      title: "Restaurar tudo",
      about: "Aplique o perfil de jogos: ele instala os programas, os drivers e os periféricos, e restaura o Playnite e o RetroArch (etapas Playnite e Emuladores).",
      state: pn.install.found && ra.install.found ? "ok" : "todo",
      detail:
        pn.install.found && ra.install.found
          ? "Playnite e RetroArch estão nesta máquina."
          : `Falta: ${[!pn.install.found && "Playnite", !ra.install.found && "RetroArch"].filter(Boolean).join(" e ")}.`,
      action: (
        <div className="row">
          <button className="primary" disabled={!!busy} onClick={() => onNavigate("setup")}>
            <RotateCcw size={15} /> Aplicar o perfil
          </button>
          {!ra.install.found &&
            button("restore-ra", "Só o RetroArch", <RotateCcw size={15} />, async () => (await api.restoreRetroArch(root, false), "RetroArch restaurado."), {
              disabled: !raSaved && ra.git.files === 0,
              busyLabel: "Baixando e restaurando…",
            })}
          {!pn.install.found &&
            button("restore-pn", "Só o Playnite", <RotateCcw size={15} />, async () => (await api.restorePlaynite(root, false), "Playnite restaurado."), {
              disabled: !pnSaved,
              busyLabel: "Restaurando… (alguns minutos)",
            })}
        </div>
      ),
    },
    {
      id: "check",
      title: "Conferir",
      about: "O que as abas Playnite e RetroArch ainda apontam como pendente (playlists, menu Iniciar, Windows Media Player).",
      state: [...pn.requirements, ...ra.requirements].some((r) => !r.ok) ? "todo" : "ok",
      detail:
        [...pn.requirements, ...ra.requirements]
          .filter((r) => !r.ok)
          .map((r) => r.title)
          .join(", ") || "Nada pendente.",
      action: (
        <div className="row">
          <button className="ghost" onClick={() => onNavigate("playnite")}>
            Playnite
          </button>
          <button className="ghost" onClick={() => onNavigate("retroarch")}>
            RetroArch
          </button>
        </div>
      ),
    },
  ];

  // someone else's configuration: their backup is their HD, the GitHub steps are the owner's
  const ownerOnly = new Set(["git", "clone"]);
  const steps = (mode === "after" ? after : before).filter((s) => data.owner || !ownerOnly.has(s.id));
  const required = steps.filter((s) => s.state !== "optional");
  const done = required.filter((s) => s.state === "ok").length;
  const next = steps.find((s) => s.state === "todo" || s.state === "warn");

  return (
    <section className="backup">
      {error && <p className="banner bad">{error}</p>}
      {!data.owner && (
        <p className="banner">
          Você está usando a configuração de outra pessoa (veja em Início). O seu backup é o seu HD: salve o Playnite e o RetroArch nele. O
          GitHub fica com o dono.
        </p>
      )}
      {note && <p className="banner">{note}</p>}

      <div className="card backup-hero">
        <span className="backup-shield" aria-hidden="true">
          <ShieldCheck size={30} />
        </span>
        <div className="grow-col">
          <strong className="big">
            Formatar sem perder nada
            <Help title="Onde fica cada coisa">
              GitHub: configurações e saves do Playnite e do RetroArch, sem senhas. Internet: o RetroArch, os cores e os programas do perfil. HD:
              a BIOS, a biblioteca e as mídias do Playnite, as senhas e os jogos (D:\Games). Só o C: é formatado.
            </Help>
          </strong>
          <span className="muted">
            {done} de {required.length} passos prontos
          </span>
          <div className="backup-progress">
            <motion.span initial={{ width: 0 }} animate={{ width: `${(100 * done) / Math.max(1, required.length)}%` }} transition={{ type: "spring", stiffness: 120, damping: 20 }} />
          </div>
        </div>
        <div className="segmented" role="tablist">
          {(["before", "after"] as Mode[]).map((m) => (
            <button key={m} role="tab" aria-selected={mode === m} className={mode === m ? "active" : ""} onClick={() => setMode(m)}>
              {m === "before" ? "Antes de formatar" : "Depois de formatar"}
            </button>
          ))}
        </div>
      </div>

      <AnimatePresence mode="wait">
        <motion.ol key={mode} className="backup-steps" initial={{ opacity: 0, x: 12 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -12 }} transition={{ duration: 0.18 }}>
          {steps.map((s, i) => (
            <li key={s.id} className={`card backup-step st-${s.state}${next?.id === s.id ? " current" : ""}`}>
              <span className="backup-icon" aria-hidden="true">
                {ICON[s.state]}
              </span>
              <div className="grow-col">
                <div className="row">
                  <span className="backup-num">{i + 1}</span>
                  <strong>{s.title}</strong>
                  {s.state === "optional" && <span className="badge">opcional</span>}
                  {next?.id === s.id && <span className="badge warn">próximo</span>}
                </div>
                <span className="muted small-note">{s.about}</span>
                <span className="backup-detail">{s.detail}</span>
              </div>
              {s.action && <div className="backup-action">{s.action}</div>}
            </li>
          ))}
        </motion.ol>
      </AnimatePresence>

      {mode === "before" && done === required.length && (
        <motion.p className="banner ok-banner" initial={{ scale: 0.96, opacity: 0 }} animate={{ scale: 1, opacity: 1 }}>
          Pronto para formatar. Formate só o C: (o SSD) e deixe o HD como está. Depois, abra o app numa pasta clonada do repositório e use
          “Depois de formatar”.
        </motion.p>
      )}
      {mode === "before" && done < required.length && (
        <p className="muted small-note">Resolva os passos pendentes antes de formatar: o que não estiver no GitHub ou no HD se perde junto com o C:.</p>
      )}
    </section>
  );
}
