import type { Plan, Profile, SystemInfo, Wizard } from "../types";
import { Help } from "./Help";

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
          ["Kernel", info.kernel],
          ["Desktop", [info.desktop, info.session].filter(Boolean).join(" · ")],
          ["Shell", info.shell],
          ["Pacotes", [info.package_manager, info.aur_helper, info.flatpak ? "flatpak" : ""].filter(Boolean).join(" · ")],
          ["CPU", info.cpu ? `${info.cpu}${info.cores ? ` (${info.cores} threads)` : ""}` : ""],
          ["Memória", info.memory_gb ? `${info.memory_gb.toFixed(1)} GB` : ""],
          ["Máquina", [info.user, info.hostname].filter(Boolean).join("@")],
        ] as [string, string][]
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
        <div className="logo" style={{ background: color }} aria-hidden="true">
          {name.slice(0, 1).toUpperCase()}
        </div>
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
              O app lê a distro desta máquina e procura o perfil feito para ela (CachyOS e Arch, ou Ubuntu e Debian).
              Cada perfil é um arquivo em <code>config/profiles/</code> e leva a um ambiente final diferente. Dá para
              trocar na barra lateral.
            </Help>
          </span>
        </div>
        {wizard && (
          <div className="ring" style={{ "--pct": pct } as React.CSSProperties} title={`${done} de ${total} itens aplicados`}>
            <span>{pct}%</span>
          </div>
        )}
      </div>

      {env === "gaming" && (
        <p className="banner">O ambiente de jogos no Windows ainda não foi implementado. Por enquanto o foco é o ambiente de desenvolvimento no Linux.</p>
      )}

      <div className="card">
        <dl className="facts">
          {facts.map(([k, v]) => (
            <div key={k}>
              <dt>{k}</dt>
              <dd title={v}>{v}</dd>
            </div>
          ))}
        </dl>
      </div>

      {wizard && plan && (
        <>
          <div className="stats">
            <div className="card stat">
              <span className="num">{apps.length - appsPending}<small>/{apps.length}</small></span>
              <span className="muted">apps e ajustes aplicados</span>
              <span className={appsPending ? "badge warn" : "badge ok"}>{appsPending ? `${appsPending} pendentes` : "em dia"}</span>
            </div>
            <div className="card stat">
              <span className="num">{files.length - filesPending}<small>/{files.length}</small></span>
              <span className="muted">configs em sincronia</span>
              <span className={filesPending ? "badge warn" : "badge ok"}>{filesPending ? `${filesPending} pendentes` : "em dia"}</span>
            </div>
            <div className="card stat">
              <span className="num">{plan.skipped.length}</span>
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
              const choice = plan.choices[s.id];
              const opt = s.options.find((o) => o.id === choice);
              return (
                <button className="card step-card" key={s.id} onClick={() => onStep(i)}>
                  <span className="step-n">{i + 1}</span>
                  <span className="grow-col">
                    <strong>{s.title}</strong>
                    <span className="muted">{opt ? opt.title : choice === "none" ? "nenhum" : `${mine.filter((l) => l.kind === "app").length} itens`}</span>
                  </span>
                  <span className={todo ? "badge warn" : "badge ok"}>{todo ? `${todo} pendentes` : "em dia"}</span>
                </button>
              );
            })}
          </div>

          <div className="card row cta">
            <div className="grow-col">
              <strong>Repositório</strong>
              <span className="muted mono">{root}</span>
            </div>
            <button onClick={() => onStep(0)}>Personalizar</button>
            <button className="primary" disabled={appsPending + filesPending === 0} onClick={onApplyPending}>
              Aplicar pendências
            </button>
          </div>
        </>
      )}
    </section>
  );
}
