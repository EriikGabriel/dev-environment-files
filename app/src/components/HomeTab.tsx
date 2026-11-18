import { ArrowRight, Box, CircleCheck, Cpu, FileCode, FolderOpen, HardDrive, MemoryStick, Monitor, Package, Rocket, Terminal, UserRound, CircleSlash } from "lucide-react";
import { motion } from "motion/react";
import type { Plan, Profile, SystemInfo, Wizard } from "../types";
import { Help } from "./Help";
import { CountUp } from "./ui/CountUp";

interface Props {
  info: SystemInfo | null;
  profile: Profile | undefined;
  wizard: Wizard | null;
  plan: Plan | null;
  root: string;
  onStep: (index: number) => void;
  onApplyPending: () => void;
}

const DISTRO_COLOR: Record<string, string> = {
  arch: "#1793d1",
  cachyos: "#00ccaa",
  ubuntu: "#e95420",
  debian: "#d70a53",
  windows: "#0078d4",
};

export const ENV_LABEL = { dev: "Ambiente de desenvolvimento", gaming: "Ambiente de jogos", "": "Ambiente não identificado" } as const;

export function HomeTab({ info, profile, wizard, plan, root, onStep, onApplyPending }: Props) {
  const env = profile?.environment ?? (info?.os === "windows" ? "gaming" : "");
  const name = info?.name || info?.id || "Sistema";
  const color = DISTRO_COLOR[info?.id ?? ""] ?? DISTRO_COLOR[info?.family ?? ""] ?? "#6ea8fe";

  const facts = info
    ? (
        [
          ["Kernel", info.kernel, HardDrive],
          ["Desktop", [info.desktop, info.session].filter(Boolean).join(" · "), Monitor],
          ["Shell", info.shell, Terminal],
          ["Pacotes", [info.package_manager, info.aur_helper, info.flatpak ? "flatpak" : ""].filter(Boolean).join(" · "), Package],
          ["CPU", info.cpu ? `${info.cpu}${info.cores ? ` (${info.cores} threads)` : ""}` : "", Cpu],
          ["Memória", info.memory_gb ? `${info.memory_gb.toFixed(1)} GB` : "", MemoryStick],
          ["Máquina", [info.user, info.hostname].filter(Boolean).join("@"), UserRound],
        ] as [string, string, typeof Cpu][]
      ).filter(([, v]) => v)
    : [];

  const lines = plan?.steps ?? [];
  const apps = lines.filter((s) => s.kind === "app");
  const files = lines.filter((s) => s.kind !== "app");
  const appsPending = apps.filter((s) => s.pending).length;
  const filesPending = files.filter((s) => s.pending).length;
  const total = lines.length;
  const done = total - appsPending - filesPending;
  const pct = total ? Math.round((done / total) * 100) : 0;

  return (
    <section className="home">
      <div className={`hero ${env || "none"}`}>
        <motion.div
          className="logo"
          style={{ background: color, "--logo": color } as React.CSSProperties}
          aria-hidden="true"
          initial={{ scale: 0.6, rotate: -12, opacity: 0 }}
          animate={{ scale: 1, rotate: 0, opacity: 1 }}
          transition={{ type: "spring", stiffness: 260, damping: 16 }}
        >
          {name.slice(0, 1).toUpperCase()}
        </motion.div>
        <div className="grow-col">
          <span className="env-tag">{ENV_LABEL[env]}</span>
          <div className="row">
            <h2 className="huge">{name}</h2>
            {info && <span className="chip">{info.arch}</span>}
            {info && <span className={info.supported ? "badge ok" : "badge bad"}>{info.supported ? "suportado" : "ainda não suportado"}</span>}
          </div>
          <span className="muted">
            {profile ? `Perfil: ${profile.name}` : "Nenhum perfil corresponde a esta máquina."}
            <Help title="Como o perfil é escolhido">
              O app lê o sistema desta máquina e procura o perfil feito para ele (CachyOS e Arch, Ubuntu e Debian, ou
              Windows para jogos).
              Cada perfil é um arquivo em <code>config/profiles/</code> e leva a um ambiente final diferente. Dá para
              trocar na barra lateral.
            </Help>
          </span>
        </div>
        {wizard && (
          <div className={pct === 100 ? "ring full" : "ring"} style={{ "--pct": pct } as React.CSSProperties} title={`${done} de ${total} itens aplicados`}>
            <span>
              <CountUp value={pct} suffix="%" />
            </span>
          </div>
        )}
      </div>

      <div className="card">
        <dl className="facts">
          {facts.map(([k, v, Icon]) => (
            <div key={k} className="fact">
              <span className="fact-icon" aria-hidden="true">
                <Icon size={16} />
              </span>
              <span className="grow-col">
                <dt>{k}</dt>
                <dd title={v}>{v}</dd>
              </span>
            </div>
          ))}
        </dl>
      </div>

      {wizard && plan && (
        <>
          <div className="stats">
            <Stat icon={<Box size={18} />} tone="a" done={apps.length - appsPending} total={apps.length} label="apps e ajustes aplicados" pending={appsPending} />
            <Stat icon={<FileCode size={18} />} tone="b" done={files.length - filesPending} total={files.length} label="configs em sincronia" pending={filesPending} />
            <div className="card stat tone-c">
              <span className="stat-icon" aria-hidden="true">
                <CircleSlash size={18} />
              </span>
              <span className="num">
                <CountUp value={plan.skipped.length} />
              </span>
              <span className="muted">indisponíveis nesta distro</span>
              <span className="muted small" title={plan.skipped.map((s) => s.id).join(", ")}>
                {plan.skipped.map((s) => s.id).join(", ") || "nenhum"}
              </span>
            </div>
          </div>

          <h3>
            Etapas da configuração
            <Help title="Etapas">
              A configuração é dividida em etapas, na ordem em que são aplicadas. Clique em uma para escolher o que entra.
              “pendentes” é o que falta aplicar nesta máquina com as escolhas atuais.
            </Help>
          </h3>
          <div className="step-cards">
            {wizard.steps.map((s, i) => {
              const mine = lines.filter((l) => l.step === s.id);
              const todo = mine.filter((l) => l.pending).length;
              const stepPct = mine.length ? ((mine.length - todo) / mine.length) * 100 : 100;
              const choice = plan.choices[s.id];
              const opt = s.options.find((o) => o.id === choice);
              return (
                <button className="card step-card" key={s.id} onClick={() => onStep(i)}>
                  <span className="step-n">{i + 1}</span>
                  <span className="grow-col">
                    <strong>{s.title}</strong>
                    <span className="muted">{opt ? opt.title : choice === "none" ? "nenhum" : `${mine.filter((l) => l.kind === "app").length} itens`}</span>
                  </span>
                  <span className={todo ? "badge warn" : "badge ok"}>{todo ? `${todo} pendentes` : <><CircleCheck size={12} /> em dia</>}</span>
                  <ArrowRight size={16} className="step-go" aria-hidden="true" />
                  <span className="step-bar" aria-hidden="true">
                    <motion.span initial={{ width: 0 }} animate={{ width: `${stepPct}%` }} transition={{ duration: 0.9, delay: 0.15 + i * 0.05, ease: [0.16, 1, 0.3, 1] }} />
                  </span>
                </button>
              );
            })}
          </div>

          <div className="card row cta">
            <span className="cta-icon" aria-hidden="true">
              <FolderOpen size={20} />
            </span>
            <div className="grow-col">
              <strong>Pasta de dados</strong>
              <span className="muted mono">{root}</span>
            </div>
            <button onClick={() => onStep(0)}>Personalizar</button>
            <button className="primary shine" disabled={appsPending + filesPending === 0} onClick={onApplyPending}>
              <Rocket size={16} aria-hidden="true" />
              {appsPending + filesPending ? `Aplicar ${appsPending + filesPending} pendências` : "Tudo em dia"}
            </button>
          </div>
        </>
      )}
    </section>
  );
}

function Stat({ icon, tone, done, total, label, pending }: { icon: React.ReactNode; tone: string; done: number; total: number; label: string; pending: number }) {
  return (
    <div className={`card stat tone-${tone}`}>
      <span className="stat-icon" aria-hidden="true">
        {icon}
      </span>
      <span className="num">
        <CountUp value={done} />
        <small>/{total}</small>
      </span>
      <span className="muted">{label}</span>
      <span className={pending ? "badge warn" : "badge ok"}>{pending ? `${pending} pendentes` : "em dia"}</span>
      <span className="stat-bar" aria-hidden="true">
        <motion.span initial={{ width: 0 }} animate={{ width: `${total ? (done / total) * 100 : 100}%` }} transition={{ duration: 1, ease: [0.16, 1, 0.3, 1] }} />
      </span>
    </div>
  );
}
