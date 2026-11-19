import { Check } from "lucide-react";
import { useMemo, useState } from "react";
import type { InstallMethod, NewApp, Plan, Selection, Wizard, WizardApp, WizardFile, WizardStep } from "../types";
import { DirTree } from "./DirTree";
import { Help } from "./Help";

interface Props {
  wizard: Wizard;
  selection: Selection;
  plan: Plan | null;
  /** Index of the open step; wizard.steps.length is the review. */
  index: number;
  onIndex: (i: number) => void;
  onChange: (sel: Selection) => void;
  onReset: () => void;
  onApply: () => void;
  onOpenFile: (f: WizardFile) => void;
  onSaveDirs: (step: string, option: string, dirs: string[]) => Promise<void>;
  onAddApp: (step: string, app: NewApp) => Promise<void>;
  onRemoveApp: (step: string, id: string) => Promise<void>;
}

/** Install methods a new app can use, by platform family; the first is the default. */
const METHODS: Record<string, { id: InstallMethod; label: string; hint: string }[]> = {
  windows: [
    { id: "winget", label: "winget", hint: "ID exato do pacote, ex.: Valve.Steam (winget search nome)" },
    { id: "steam", label: "Steam", hint: "AppID da loja, ex.: 431960 (o número na URL da página do jogo)" },
    { id: "scoop", label: "scoop", hint: "nome do pacote no scoop" },
  ],
  arch: [
    { id: "native", label: "pacman", hint: "nome do pacote nos repositórios" },
    { id: "aur", label: "AUR", hint: "nome do pacote no AUR" },
  ],
  debian: [
    { id: "native", label: "apt", hint: "nome do pacote" },
    { id: "flatpak", label: "flatpak", hint: "ID no Flathub, ex.: org.videolan.VLC" },
  ],
};

export function SetupWizard({ wizard, selection, plan, index, onIndex, onChange, onReset, onApply, onOpenFile, onSaveDirs, onAddApp, onRemoveApp }: Props) {
  const steps = wizard.steps;
  const review = index >= steps.length;
  const step = review ? null : steps[index];
  const picked = useMemo(() => new Set(selection.apps), [selection.apps]);
  const missingDirs = useMemo(
    () => new Set((plan?.steps ?? []).filter((l) => l.kind === "dir" && l.pending).map((l) => l.id.replace(/^\/home\/[^/]+/, "~"))),
    [plan],
  );

  const setApps = (ids: string[], on: boolean) => {
    const next = new Set(selection.apps);
    for (const id of ids) on ? next.add(id) : next.delete(id);
    onChange({ ...selection, apps: [...next] });
  };

  return (
    <section className="wizard">
      <ol className="stepper">
        {[...steps.map((s) => s.title), "Revisão"].map((title, i) => (
          <li key={title}>
            <button
              className={i === index ? "pill active" : i < index ? "pill done" : "pill"}
              onClick={() => onIndex(i)}
              aria-current={i === index ? "step" : undefined}
            >
              <span className="pill-n">{i < index ? <Check size={13} strokeWidth={3} /> : i + 1}</span>
              {title}
            </button>
          </li>
        ))}
      </ol>
      <div className="progress" aria-hidden="true">
        <div style={{ width: `${(index / steps.length) * 100}%` }} />
      </div>

      {step ? (
        <StepPage
          key={step.id}
          step={step}
          picked={picked}
          choice={selection.choices[step.id]}
          onToggle={(id) => setApps([id], !picked.has(id))}
          onAll={(ids, on) => setApps(ids, on)}
          onChoice={(c) => onChange({ ...selection, choices: { ...selection.choices, [step.id]: c } })}
          missingDirs={missingDirs}
          onOpenFile={onOpenFile}
          onSaveDirs={(option, dirs) => onSaveDirs(step.id, option, dirs)}
          vars={selection.vars}
          onVar={(id, value) => onChange({ ...selection, vars: { ...selection.vars, [id]: value } })}
          // the gaming profile targets Windows even when it is edited from Linux
          methods={METHODS[wizard.environment === "gaming" ? "windows" : wizard.platform.family] ?? []}
          onAddApp={(app) => onAddApp(step.id, app)}
          onRemoveApp={(id) => onRemoveApp(step.id, id)}
        />
      ) : (
        <Review wizard={wizard} plan={plan} onIndex={onIndex} />
      )}

      <footer className="wizard-nav">
        <button className="ghost" onClick={onReset} title="Voltar às escolhas padrão deste perfil">
          Restaurar padrão
        </button>
        <span className="spacer" />
        <button disabled={index === 0} onClick={() => onIndex(index - 1)}>
          Voltar
        </button>
        {review ? (
          <button className="primary" disabled={!plan || !plan.steps.some((s) => s.pending)} onClick={onApply}>
            Aplicar {plan ? plan.steps.filter((s) => s.pending).length : 0} pendências
          </button>
        ) : (
          <button className="primary" onClick={() => onIndex(index + 1)}>
            {index === steps.length - 1 ? "Revisar" : "Próximo"}
          </button>
        )}
      </footer>
    </section>
  );
}

function StepPage(p: {
  step: WizardStep;
  picked: Set<string>;
  choice: string | undefined;
  onToggle: (id: string) => void;
  onAll: (ids: string[], on: boolean) => void;
  onChoice: (id: string) => void;
  missingDirs: Set<string>;
  onOpenFile: (f: WizardFile) => void;
  onSaveDirs: (option: string, dirs: string[]) => Promise<void>;
  vars: Record<string, string>;
  onVar: (id: string, value: string) => void;
  methods: { id: InstallMethod; label: string; hint: string }[];
  onAddApp: (app: NewApp) => Promise<void>;
  onRemoveApp: (id: string) => Promise<void>;
}) {
  const { step } = p;
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const remove = async (id: string) => {
    if (!window.confirm(`Tirar “${id}” desta etapa? O item sai do perfil (config/profiles), mas nada é desinstalado.`)) return;
    try {
      setError(null);
      await p.onRemoveApp(id);
    } catch (e) {
      setError(typeof e === "string" ? e : String(e));
    }
  };
  const [query, setQuery] = useState("");
  const q = query.trim().toLowerCase();
  const apps = step.apps.filter((a) => !q || a.id.includes(q) || a.description.toLowerCase().includes(q));
  const available = step.apps.filter((a) => a.available).map((a) => a.id);
  const count = available.filter((id) => p.picked.has(id)).length;
  const chosen = step.options.find((o) => o.id === p.choice);

  return (
    <div className="step-page">
      <div>
        <h2 className="big">{step.title}</h2>
        <p className="muted">{step.description}</p>
      </div>

      {step.options.length > 0 && (
        <>
          <h3>
            {step.choice_title || "Escolha uma opção"}
            <Help title="Escolha única">
              Só a opção marcada é instalada e configurada. “Nenhum” pula esta escolha e não mexe no que já existe na
              máquina. Trocar de opção depois não desinstala a anterior.
            </Help>
          </h3>
          <div className="options" role="radiogroup" aria-label={step.choice_title || step.title}>
            {step.options.map((o) => (
              <button
                key={o.id}
                role="radio"
                aria-checked={p.choice === o.id}
                className={p.choice === o.id ? "card option active" : "card option"}
                onClick={() => p.onChoice(o.id)}
              >
                <span className="radio" />
                <strong>{o.title}</strong>
                {o.id === step.default && <span className="chip">padrão</span>}
                <span className="muted full">{o.description}</span>
                <span className="chips full">
                  {o.apps.map((a) => (
                    <span key={a.id} className={a.installed ? "chip ok" : a.available ? "chip" : "chip off"} title={a.detail}>
                      {a.id}
                    </span>
                  ))}
                  {o.dirs.length > 0 && <span className="chip">{o.dirs.length} pastas</span>}
                  {o.files.length > 0 && <span className="chip">{o.files.length} configs</span>}
                </span>
              </button>
            ))}
            <button
              role="radio"
              aria-checked={p.choice === "none"}
              className={p.choice === "none" ? "card option active" : "card option"}
              onClick={() => p.onChoice("none")}
            >
              <span className="radio" />
              <strong>Nenhum</strong>
              <span className="muted full">Pular esta escolha e não mexer no que já existe.</span>
            </button>
          </div>
        </>
      )}

      {chosen && (chosen.files.length > 0 || chosen.dirs.length > 0 || step.id === "workspace") && (
        <div className="detail" key={chosen.id}>
          {chosen.dirs.length > 0 || step.id === "workspace" ? (
            <DirTree dirs={chosen.dirs} missing={p.missingDirs} onSave={(dirs) => p.onSaveDirs(chosen.id, dirs)} />
          ) : null}
          <FileList title={`Configs de ${chosen.title}`} files={chosen.files} onOpen={p.onOpenFile} />
        </div>
      )}

      {step.inputs.length > 0 && (
        <div className="inputs">
          <h3>
            Seus dados
            <Help title="Valores desta etapa">
              O que você digita aqui fica salvo só neste computador e é passado aos itens abaixo na hora de aplicar. Nada
              disso é gravado no repositório. O campo já vem com o valor que a máquina tem hoje.
            </Help>
          </h3>
          {step.inputs.map((i) => (
            <label key={i.id} className="field">
              <span className="muted">{i.label}</span>
              <input type="text" value={p.vars[i.id] ?? ""} placeholder={i.value || i.label} onChange={(e) => p.onVar(i.id, e.target.value)} />
            </label>
          ))}
        </div>
      )}

      {error && <p className="banner bad">{error}</p>}

      {step.apps.length === 0 && step.options.length === 0 && p.methods.length > 0 && (
        <div className="row">
          <span className="muted">Esta etapa ainda não tem itens.</span>
          <span className="spacer" />
          <button className="ghost small-btn" onClick={() => setAdding(true)}>
            + Adicionar app
          </button>
        </div>
      )}

      {step.apps.length > 0 && (
        <>
          <div className="row">
            <h3 className="inline">
              {step.options.length > 0 ? "Complementos" : "Itens"} · {count}/{available.length}
              <Help title="Itens marcáveis">
                Os itens ligados entram no plano. “instalado” indica o que já existe nesta máquina: ele é pulado ao
                aplicar. Desligar um item não o desinstala, só o tira do plano. Passe o mouse para ver como cada um é
                instalado.
              </Help>
            </h3>
            <span className="spacer" />
            {step.apps.length > 8 && (
              <input className="search" placeholder="Filtrar…" value={query} onChange={(e) => setQuery(e.target.value)} aria-label="Filtrar itens" />
            )}
            <button className="ghost small-btn" onClick={() => p.onAll(available, true)}>
              Todos
            </button>
            <button className="ghost small-btn" onClick={() => p.onAll(available, false)}>
              Nenhum
            </button>
            {p.methods.length > 0 && (
              <button className="small-btn" onClick={() => setAdding(true)} title="Adicionar um item a esta etapa do perfil">
                + Adicionar app
              </button>
            )}
          </div>
          <div className="apps">
            {apps.map((a) => (
              <AppCard key={a.id} app={a} on={p.picked.has(a.id)} onToggle={() => p.onToggle(a.id)} onRemove={() => void remove(a.id)} />
            ))}
            {apps.length === 0 && <p className="muted">Nada encontrado para “{query}”.</p>}
          </div>
        </>
      )}

      <FileList title="Configs desta etapa" files={step.files} onOpen={p.onOpenFile} />

      {adding && <AddAppDialog stepTitle={step.title} methods={p.methods} onAdd={p.onAddApp} onClose={() => setAdding(false)} />}
    </div>
  );
}

const slug = (s: string) =>
  s
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9+._]+/g, "-")
    .replace(/^[^a-z0-9]+|-+$/g, "");

/** Form for a new item of a step. It writes to config/apps.yaml and to the profile. */
function AddAppDialog(p: {
  stepTitle: string;
  methods: { id: InstallMethod; label: string; hint: string }[];
  onAdd: (app: NewApp) => Promise<void>;
  onClose: () => void;
}) {
  const [title, setTitle] = useState("");
  const [name, setName] = useState("");
  const [method, setMethod] = useState<InstallMethod>(p.methods[0].id);
  const [selected, setSelected] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const id = slug(title);
  const hint = p.methods.find((m) => m.id === method)?.hint ?? "";

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      // the first method is the platform's native one, which the catalogue leaves implicit
      await p.onAdd({ id, name: name.trim(), method: method === p.methods[0].id ? "" : method, description: title.trim() === id ? "" : title.trim(), optional: !selected });
      p.onClose();
    } catch (e) {
      setError(typeof e === "string" ? e : String(e));
      setBusy(false);
    }
  };

  return (
    <div className="overlay" role="dialog" aria-modal="true" aria-label="Adicionar app">
      <form
        className="dialog"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <h2>Adicionar app</h2>
        <p className="muted">
          O item entra na etapa “{p.stepTitle}” deste perfil. Se ele ainda não existe no catálogo, uma entrada é criada em{" "}
          <code>config/apps.yaml</code>.
        </p>
        <label className="field">
          <span className="muted">Nome</span>
          <input type="text" autoFocus value={title} onChange={(e) => setTitle(e.target.value)} placeholder="ex.: Heroic Launcher" />
        </label>
        {id && <span className="muted small-note">id no catálogo: <code>{id}</code></span>}
        <label className="field">
          <span className="muted">Instalar com</span>
          <select value={method} onChange={(e) => setMethod(e.target.value as InstallMethod)}>
            {p.methods.map((m) => (
              <option key={m.id} value={m.id}>
                {m.label}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span className="muted">Pacote</span>
          <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder={hint} />
        </label>
        <span className="muted small-note">{hint}. Deixe em branco se o id já existe no catálogo.</span>
        <label className="check">
          <input type="checkbox" checked={selected} onChange={(e) => setSelected(e.target.checked)} />
          Começa marcado
        </label>
        {error && <p className="banner bad">{error}</p>}
        <div className="row end">
          <button type="button" className="ghost" onClick={p.onClose} disabled={busy}>
            Cancelar
          </button>
          <button type="submit" className="primary" disabled={busy || !id}>
            Adicionar
          </button>
        </div>
      </form>
    </div>
  );
}

function FileList({ title, files, onOpen }: { title: string; files: WizardFile[]; onOpen: (f: WizardFile) => void }) {
  if (files.length === 0) return null;
  return (
    <div className="filelist">
      <h3>
        {title}
        <Help title="Arquivos de configuração">
          Cada linha é um arquivo guardado no repositório e o lugar onde ele vai parar. Ao aplicar, o destino vira um
          symlink para o arquivo do repo (o que existia antes é guardado com final <code>.bak</code>). Clique para ler o
          conteúdo. Um arquivo ligado a um app só é implantado se o app estiver marcado.
        </Help>
      </h3>
      <ul className="list files">
        {files.map((f) => (
          <li key={f.src + f.dest}>
            <button className="entry" onClick={() => onOpen(f)} title="Ver o conteúdo">
              <span aria-hidden="true">{f.mode === "install" ? "🧩" : "🔗"}</span>
              <span className="mono file-dest">{f.dest}</span>
              <span className="muted mono grow">{f.src}</span>
              {f.app && <span className="chip">com {f.app}</span>}
              <span className="muted">ver ›</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

function AppCard({ app, on, onToggle, onRemove }: { app: WizardApp; on: boolean; onToggle: () => void; onRemove: () => void }) {
  return (
    <div className="app-wrap">
      <button className="app-remove" onClick={onRemove} title="Tirar desta etapa do perfil" aria-label={`Tirar ${app.id} desta etapa`}>
        ×
      </button>
      <AppToggle app={app} on={on} onToggle={onToggle} />
    </div>
  );
}

function AppToggle({ app, on, onToggle }: { app: WizardApp; on: boolean; onToggle: () => void }) {
  return (
    <button
      role="switch"
      aria-checked={on}
      disabled={!app.available}
      className={!app.available ? "card app off" : on ? "card app active" : "card app"}
      onClick={onToggle}
      title={app.detail}
    >
      <span className="toggle" />
      <span className="app-text">
        <strong>{app.id}</strong>
        <span className="muted">{app.available ? app.description || app.detail : `indisponível: ${app.detail}`}</span>
        {app.hint && <span className={app.default ? "app-hint match" : "app-hint"}>{app.hint}</span>}
      </span>
      {app.installed && <span className="badge ok">instalado</span>}
    </button>
  );
}

function Review({ wizard, plan, onIndex }: { wizard: Wizard; plan: Plan | null; onIndex: (i: number) => void }) {
  if (!plan) return <p className="muted">Calculando o plano…</p>;
  const groups = [...wizard.steps.map((s, i) => ({ id: s.id, title: s.title, i })), { id: "", title: "Arquivos extras", i: -1 }];
  const pending = plan.steps.filter((s) => s.pending).length;

  return (
    <div className="step-page">
      <div>
        <h2 className="big">
          Revisão
          <Help title="O que acontece ao aplicar">
            Só as ações pendentes são executadas, na ordem das etapas: primeiro apps, depois pastas e por fim os arquivos
            de config. Dá para aplicar várias vezes sem problema: o que já está em dia é pulado. Se um item falhar, os
            outros continuam e a falha aparece no log.
          </Help>
        </h2>
        <p className="muted">
          {pending === 0
            ? "Tudo o que você escolheu já está aplicado nesta máquina."
            : `${pending} ações serão executadas. O que já está instalado ou em dia é pulado.`}
        </p>
      </div>
      {groups.map((g) => {
        const lines = plan.steps.filter((l) => l.step === g.id);
        if (lines.length === 0) return null;
        const todo = lines.filter((l) => l.pending);
        const choice = plan.choices[g.id];
        return (
          <div className="card review" key={g.id || "extras"}>
            <div className="row">
              <strong>{g.title}</strong>
              {choice && <span className="chip">{choice === "none" ? "nenhum" : choice}</span>}
              <span className="spacer" />
              <span className="muted">{lines.length - todo.length} em dia</span>
              <span className={todo.length ? "badge warn" : "badge ok"}>{todo.length ? `${todo.length} pendentes` : "ok"}</span>
              {g.i >= 0 && (
                <button className="ghost small-btn" onClick={() => onIndex(g.i)}>
                  Editar
                </button>
              )}
            </div>
            {todo.length > 0 && (
              <ul className="review-list">
                {todo.map((l) => (
                  <li key={l.kind + l.id}>
                    <span className="mono">{l.kind === "file" || l.kind === "dir" ? l.id.replace(/^\/home\/[^/]+/, "~") : l.title}</span>
                    <span className="muted">{l.kind === "file" ? (l.mode === "copy" ? "copiar" : "symlink") : l.kind === "dir" ? "criar pasta" : l.detail}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        );
      })}
      {plan.skipped.length > 0 && (
        <p className="muted small-note">Indisponíveis nesta distro: {plan.skipped.map((s) => s.id).join(", ")}</p>
      )}
    </div>
  );
}
