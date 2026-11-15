import { open } from "@tauri-apps/plugin-dialog";
import { useCallback, useEffect, useRef, useState } from "react";
import * as api from "./api";
import { errorMessage } from "./api";
import { ApplyView, type StepState } from "./components/ApplyView";
import { DesktopTab } from "./components/DesktopTab";
import { DotfilesTab } from "./components/DotfilesTab";
import { Drawer } from "./components/Drawer";
import { FileDrawer, type FileTarget } from "./components/FileDrawer";
import { ENV_LABEL, HomeTab } from "./components/HomeTab";
import { PasswordDialog } from "./components/PasswordDialog";
import { SetupWizard } from "./components/SetupWizard";
import { defaultSelection, loadSelection, saveSelection } from "./selection";
import type {
	ApplyEvent,
	NewApp,
	Plan,
	Profile,
	Selection,
	SystemInfo,
	Wizard,
} from "./types";
import "./styles.css";

type Tab = "home" | "setup" | "preview" | "dotfiles";
type Summary = Extract<ApplyEvent, { type: "done" }>;

interface ApplyState {
	steps: StepState[];
	total: number;
	summary: Summary | null;
	running: boolean;
	cancelled: boolean;
	startedAt: number;
	endedAt: number | null;
	error: string | null;
}

const TABS: { id: Tab; label: string; icon: string }[] = [
	{ id: "home", label: "Início", icon: "⌂" },
	{ id: "setup", label: "Configurar", icon: "☰" },
	{ id: "preview", label: "Preview", icon: "▣" },
	{ id: "dotfiles", label: "Dotfiles", icon: "❖" },
];

export default function App() {
	const [root, setRoot] = useState<string | null>(null);
	const [rootLoaded, setRootLoaded] = useState(false);
	const [profiles, setProfiles] = useState<Profile[]>([]);
	const [profileId, setProfileId] = useState<string | null>(null);
	const [info, setInfo] = useState<SystemInfo | null>(null);
	const [wizard, setWizard] = useState<Wizard | null>(null);
	const [selection, setSelection] = useState<Selection | null>(null);
	const [plan, setPlan] = useState<Plan | null>(null);
	const [tab, setTab] = useState<Tab>("home");
	const [stepIndex, setStepIndex] = useState(0);
	const [error, setError] = useState<string | null>(null);
	const [askPassword, setAskPassword] = useState(false);
	const [run, setRun] = useState<ApplyState | null>(null);
	const planSeq = useRef(0);
	const [openFile, setOpenFile] = useState<FileTarget | null>(null);
	const [helpOpen, setHelpOpen] = useState(false);

	const profile = profiles.find((p) => p.id === profileId);
	const env = profile?.environment ?? (info?.os === "windows" ? "gaming" : "");

	useEffect(() => {
		api.getRoot().then((r) => {
			setRoot(r);
			setRootLoaded(true);
		});
	}, []);

	// machine details and the list of profiles; the matching one is preselected
	useEffect(() => {
		if (!root) return;
		api
			.info(root)
			.then(setInfo)
			.catch(() => setInfo(null));
		api
			.profiles(root)
			.then((all) => {
				// profiles for another operating system are not offered at all
				const list = all.filter((p) => p.compatible);
				setProfiles(list);
				setProfileId((cur) => cur ?? list.find((p) => p.matches)?.id ?? null);
				setError(null);
			})
			.catch((e) => setError(errorMessage(e)));
	}, [root]);

	const loadWizard = useCallback(
		async (keepSelection: boolean) => {
			if (!root || !profileId) return;
			try {
				const w = await api.steps(root, profileId);
				setWizard(w);
				if (!keepSelection) setSelection(loadSelection(w));
			} catch (e) {
				setError(errorMessage(e));
			}
		},
		[root, profileId],
	);

	useEffect(() => {
		setWizard(null);
		setSelection(null);
		setPlan(null);
		setStepIndex(0);
		void loadWizard(false);
	}, [loadWizard]);

	// the plan follows the selection; a sequence number drops stale answers
	const loadPlan = useCallback(async () => {
		if (!root || !wizard || !selection || wizard.steps.length === 0) return;
		const seq = ++planSeq.current;
		try {
			const p = await api.plan(root, wizard.profile, selection);
			if (seq === planSeq.current) {
				setPlan(p);
				setError(null);
			}
		} catch (e) {
			if (seq === planSeq.current) setError(errorMessage(e));
		}
	}, [root, wizard, selection]);

	useEffect(() => {
		const t = setTimeout(() => void loadPlan(), 150);
		return () => clearTimeout(t);
	}, [loadPlan]);

	const changeSelection = (sel: Selection) => {
		setSelection(sel);
		if (wizard) saveSelection(wizard.profile, sel);
	};

	const chooseRoot = async () => {
		const dir = await open({
			directory: true,
			multiple: false,
			title: "Selecione a pasta dev-environment-files",
		});
		if (typeof dir !== "string") return;
		try {
			setRoot(await api.setRoot(dir));
			setProfileId(null);
			setError(null);
		} catch (e) {
			setError(errorMessage(e));
		}
	};

	const startApply = async (password: string) => {
		if (!root || !wizard || !selection) return;
		setAskPassword(false);
		setRun({
			steps: [],
			total: 0,
			summary: null,
			running: true,
			cancelled: false,
			startedAt: Date.now(),
			endedAt: null,
			error: null,
		});

		const off = await api.onApplyEvent((ev) => {
			setRun((cur) => {
				if (!cur) return cur;
				const now = Date.now();
				if (ev.type === "done") return { ...cur, summary: ev };
				if (ev.type === "log") {
					// the line belongs to the step that printed it
					const i = cur.steps.findIndex((s) => s.index === ev.index);
					if (i < 0) return cur;
					const s = cur.steps[i];
					const steps = [...cur.steps];
					steps[i] = {
						...s,
						log: [...s.log.slice(-400), ev.line],
						lastOutputAt: now,
					};
					return { ...cur, steps };
				}
				const i = cur.steps.findIndex((s) => s.index === ev.index);
				const prev = i >= 0 ? cur.steps[i] : undefined;
				const next: StepState = {
					index: ev.index,
					title: ev.title,
					group: ev.group ?? "",
					status: ev.status,
					detail: ev.detail ?? prev?.detail,
					error: ev.error,
					durationMs: ev.duration_ms,
					log: prev?.log ?? [],
					startedAt: prev?.startedAt ?? now,
					lastOutputAt: prev?.lastOutputAt ?? now,
				};
				const steps = prev
					? cur.steps.map((s, k) => (k === i ? next : s))
					: [...cur.steps, next];
				return { ...cur, steps, total: ev.total };
			});
		});

		try {
			await api.apply({
				root,
				profile: wizard.profile,
				selection,
				sudoPassword: password,
			});
			setRun((cur) => cur && { ...cur, running: false, endedAt: Date.now() });
		} catch (e) {
			setRun(
				(cur) =>
					cur && {
						...cur,
						running: false,
						endedAt: Date.now(),
						error: errorMessage(e),
					},
			);
		} finally {
			off();
			void loadWizard(true); // refresh the installed state, keep the choices
		}
	};

	const saveDirs = async (step: string, option: string, dirs: string[]) => {
		if (!root || !wizard) return;
		await api.setDirs(root, wizard.profile, step, option, dirs);
		await loadWizard(true);
	};

	const addApp = async (step: string, app: NewApp) => {
		if (!root || !wizard || !selection) return;
		await api.addApp(root, wizard.profile, step, app);
		// a new item is there to be installed: it enters the plan right away
		changeSelection({
			...selection,
			apps: [...new Set([...selection.apps, app.id])],
		});
		await loadWizard(true);
	};

	const removeApp = async (step: string, id: string) => {
		if (!root || !wizard || !selection) return;
		await api.removeApp(root, wizard.profile, step, id);
		const still = wizard.steps.some(
			(s) => s.id !== step && s.apps.some((a) => a.id === id),
		);
		if (!still)
			changeSelection({
				...selection,
				apps: selection.apps.filter((a) => a !== id),
			});
		await loadWizard(true);
	};

	// Windows has no sudo: installers raise their own elevation prompt.
	const requestApply = () => {
		if (info?.os === "windows") void startApply("");
		else setAskPassword(true);
	};

	const goStep = (i: number) => {
		setRun(null);
		setStepIndex(i);
		setTab("setup");
	};

	if (!rootLoaded) return null;

	if (!root) {
		return (
			<main className="center">
				<img src="/icon.svg" alt="" className="brand-icon big-icon" />
        <h1 className="brand">envsetup</h1>
				<p className="muted">
					Selecione a pasta que contém <code>config/</code> e{" "}
					<code>dotfiles/</code>.
				</p>
				{error && <p className="banner bad">{error}</p>}
				<button className="primary" onClick={chooseRoot}>
					Escolher pasta…
				</button>
			</main>
		);
	}

	const busy = !!run?.running;
	const ready = wizard && selection && wizard.steps.length > 0;

	return (
		<div className="shell" data-env={env || "none"}>
			<aside>
				<h1 className="brand">
          <img src="/icon.svg" alt="" className="brand-icon" />
          envsetup
        </h1>
				<div className="env-badge">
					<span className="dot" />
					<span>
						<strong>
							{env === "dev" ? "DEV" : env === "gaming" ? "GAMING" : "—"}
						</strong>
						<small>
							{info?.os === "windows" ? "Windows" : info?.name || "Linux"}
						</small>
					</span>
				</div>

				<nav>
					{TABS.map((t) => (
						<button
							key={t.id}
							className={tab === t.id && !run ? "side active" : "side"}
							disabled={busy}
							onClick={() => {
								setRun(null);
								setTab(t.id);
							}}
						>
							<span className="icon" aria-hidden="true">
								{t.icon}
							</span>
							{t.label}
						</button>
					))}
				</nav>

				<span className="spacer" />
				<button className="side" onClick={() => setHelpOpen(true)}>
					<span className="icon" aria-hidden="true">
						?
					</span>
					Como funciona
				</button>
				<label className="side-label">
					Perfil
					<select
						value={profileId ?? ""}
						disabled={busy}
						onChange={(e) => setProfileId(e.target.value)}
					>
						{!profileId && <option value="">(nenhum)</option>}
						{profiles.map((p) => (
							<option key={p.id} value={p.id}>
								{p.name}
								{p.matches ? " ✓" : ""}
							</option>
						))}
					</select>
				</label>
				<button
					className="ghost small-btn"
					onClick={chooseRoot}
					title={root}
					disabled={busy}
				>
					Trocar repositório…
				</button>
			</aside>

			<main>
				<header>
					<h2>{run ? "Aplicando" : TABS.find((t) => t.id === tab)?.label}</h2>
					<span className="muted">{ENV_LABEL[env]}</span>
				</header>

				{error && <p className="banner bad">{error}</p>}

				{run ? (
					<ApplyView
						{...run}
						stepTitles={Object.fromEntries(
							(wizard?.steps ?? []).map((s) => [s.id, s.title]),
						)}
						onCancel={() => {
							setRun((cur) => cur && { ...cur, cancelled: true });
							void api.cancelApply();
						}}
						onClose={() => setRun(null)}
					/>
				) : tab === "home" ? (
					<HomeTab
						key="home"
						info={info}
						profile={profile}
						wizard={ready ? wizard : null}
						plan={plan}
						root={root}
						onStep={goStep}
						onApplyPending={requestApply}
					/>
				) : !ready ? (
					<p className="banner">
						{wizard && wizard.steps.length === 0
							? "Este perfil ainda não tem etapas."
							: "Carregando as etapas deste perfil…"}
					</p>
				) : tab === "setup" ? (
					<SetupWizard
						wizard={wizard}
						selection={selection}
						plan={plan}
						index={stepIndex}
						onIndex={setStepIndex}
						onChange={changeSelection}
						onReset={() => changeSelection(defaultSelection(wizard))}
						onApply={requestApply}
						onOpenFile={setOpenFile}
						onSaveDirs={saveDirs}
						onAddApp={addApp}
						onRemoveApp={removeApp}
					/>
				) : tab === "preview" ? (
					<DesktopTab root={root} profile={wizard.profile} info={info} plan={plan} onSaved={() => void loadPlan()} />
					) : plan ? (
					<DotfilesTab
						root={root}
						profile={wizard.profile}
						plan={plan}
						busy={busy}
						onChanged={() => void loadWizard(true)}
						onOpenFile={setOpenFile}
					/>
				) : (
					<p className="banner">Calculando o plano…</p>
				)}

				{openFile && (
					<FileDrawer
						root={root}
						file={openFile}
						onClose={() => setOpenFile(null)}
					/>
				)}
				{helpOpen && (
					<Drawer
						title="Como funciona"
						subtitle="Um resumo do que o envsetup faz"
						onClose={() => setHelpOpen(false)}
					>
						<div className="guide">
							<h3>A ideia</h3>
							<p>
								O repositório guarda a descrição do seu ambiente: quais apps
								instalar, quais arquivos de configuração usar e quais pastas
								criar. O envsetup lê isso, compara com a máquina e aplica só o
								que falta.
							</p>
							<h3>Perfis</h3>
							<p>
								Cada perfil leva a um ambiente final. O de CachyOS reproduz o
								ambiente atual (niri + Noctalia, fish, kitty, Zed). O de Ubuntu
								reproduz o dos scripts antigos (GNOME + WhiteSur, zsh, WezTerm,
								VS Code). O de jogos, no Windows, instala drivers, lojas e os
								apps do dia a dia pelo winget. O perfil é escolhido pelo
								sistema da máquina.
							</p>
							<h3>Etapas e escolhas</h3>
							<p>
								Em <strong>Configurar</strong>, cada etapa tem itens que você
								liga ou desliga e, em algumas, uma escolha única (shell,
								terminal, editor, estrutura de pastas). As escolhas ficam salvas
								neste computador. A última tela mostra exatamente o que será
								feito. Em cada etapa dá para adicionar um item novo (nome do
								pacote, ID do winget ou AppID da Steam) ou remover um
								existente: isso altera os arquivos em <code>config/</code>.
							</p>
							<h3>Arquivos de configuração</h3>
							<p>
								Os dotfiles ficam em <code>dotfiles/</code>. Ao aplicar, o
								destino na sua home vira um symlink para o arquivo do repo, e o
								que existia antes é guardado com final <code>.bak</code>. Assim,
								editar a config no lugar de sempre já altera o repo. Clique em
								qualquer arquivo para ler o conteúdo.
							</p>
							<h3>Aplicar</h3>
							<p>
								No Linux, instalar pacotes pede a senha do sudo, usada só
								naquela execução; no Windows cada instalador pede a elevação
								dele. O log mostra item a item o que mudou, o que já estava
								em dia e o que falhou, e uma cópia em texto fica em{" "}
								<code>~/.local/state/envsetup/</code>. Aplicar de novo é seguro:
								o que está em dia é pulado.
							</p>
							<h3>Segredos</h3>
							<p>
								Chaves e senhas não devem ir para o git. Nas cópias do repo, a
								chave do Figma no Zed e as senhas das collections do Bruno são
								placeholders. Confira arquivos novos antes de fazer commit.
							</p>
						</div>
					</Drawer>
				)}
				{askPassword && (
					<PasswordDialog
						onSubmit={startApply}
						onCancel={() => setAskPassword(false)}
					/>
				)}
			</main>
		</div>
	);
}
