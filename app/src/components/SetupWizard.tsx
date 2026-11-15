import { useMemo, useState } from "react";
import type { Plan, Selection, Wizard, WizardApp, WizardFile, WizardStep } from "../types";
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
}

export function SetupWizard({ wizard, selection, plan, index, onIndex, onChange, onReset, onApply, onOpenFile, onSaveDirs }: Props) {
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
              <span className="pill-n">{i < index ? "✓" : i + 1}</span>
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
}) {
  const { step } = p;
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
          </div>
          <div className="apps">
            {apps.map((a) => (
              <AppCard key={a.id} app={a} on={p.picked.has(a.id)} onToggle={() => p.onToggle(a.id)} />
            ))}
            {apps.length === 0 && <p className="muted">Nada encontrado para “{query}”.</p>}
          </div>
        </>
      )}

      <FileList title="Configs desta etapa" files={step.files} onOpen={p.onOpenFile} />
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

function AppCard({ app, on, onToggle }: { app: WizardApp; on: boolean; onToggle: () => void }) {
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
