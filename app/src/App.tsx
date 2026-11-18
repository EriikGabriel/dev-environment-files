import { open } from "@tauri-apps/plugin-dialog";
import {
	Command as CommandIcon,
	FileCode,
	FolderOpen,
	Gamepad2,
	HardDriveDownload,
	House,
	Info,
	type LucideIcon,
	MonitorSmartphone,
	Rocket,
	ShieldCheck,
	SlidersHorizontal,
	Sparkles,
	UserRound,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useCallback, useEffect, useRef, useState } from "react";
import * as api from "./api";
import { errorMessage } from "./api";
import { ApplyView } from "./components/ApplyView";
import { DotfilesTab } from "./components/DotfilesTab";
import { DataFolder } from "./components/DataFolder";
import { Drawer } from "./components/Drawer";
import { FileDrawer, type FileTarget } from "./components/FileDrawer";
import { ENV_LABEL, HomeTab } from "./components/HomeTab";
import { LinuxPreviewTab } from "./components/linux/LinuxPreviewTab";
import { PasswordDialog } from "./components/PasswordDialog";
import { RootPicker } from "./components/RootPicker";
import { InstallTab } from "./components/windows/InstallTab";
import { PlayniteTab } from "./components/windows/PlayniteTab";
import { PoliciesTab } from "./components/windows/PoliciesTab";
import { WindhawkTab } from "./components/windows/WindhawkTab";
import { WindowsPreviewTab } from "./components/windows/WindowsPreviewTab";
import { SetupWizard } from "./components/SetupWizard";
import { Aurora } from "./components/ui/Aurora";
import { type Command, CommandPalette } from "./components/ui/CommandPalette";
import { Skeleton } from "./components/ui/Skeleton";
import { defaultSelection, loadSelection, saveSelection } from "./selection";
import type {
	InstallMedia,
	NewApp,
	Plan,
	Profile,
	Selection,
	SystemInfo,
	Wizard,
} from "./types";
import "./styles.css";
import { useApplyRun } from "./useApplyRun";

type Tab = "home" | "setup" | "preview" | "dotfiles" | "install" | "policies" | "windhawk" | "playnite";

// Tabs that only exist for a profile with a Windows installation (`media:`
// in the profile).
const WINDOWS_TABS: Tab[] = ["install", "policies", "windhawk", "playnite"];

const TABS: { id: Tab; label: string; icon: LucideIcon; hint: string }[] = [
	{ id: "home", label: "Início", icon: House, hint: "resumo da máquina e do perfil" },
	{ id: "setup", label: "Configurar", icon: SlidersHorizontal, hint: "etapas, apps e escolhas" },
	{ id: "preview", label: "Preview", icon: MonitorSmartphone, hint: "como o desktop vai ficar" },
	{ id: "dotfiles", label: "Dotfiles", icon: FileCode, hint: "arquivos de configuração" },
	{ id: "install", label: "Instalação", icon: HardDriveDownload, hint: "autounattend, Ninite e pendrive" },
	{ id: "policies", label: "Políticas", icon: ShieldCheck, hint: "políticas de grupo do Windows" },
	{ id: "windhawk", label: "Windhawk", icon: Sparkles, hint: "mods e a dock" },
	{ id: "playnite", label: "Playnite", icon: Gamepad2, hint: "snapshot do Playnite portable" },
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
	const { run, start: startRun, cancel: cancelRun, close: closeRun } = useApplyRun();
	const planSeq = useRef(0);
	const [openFile, setOpenFile] = useState<FileTarget | null>(null);
	const [helpOpen, setHelpOpen] = useState(false);
	const [installMedia, setInstallMedia] = useState<InstallMedia | null>(null);
	const [paletteOpen, setPaletteOpen] = useState(false);

	// Ctrl+K (⌘K on macOS) opens the command palette from anywhere
	useEffect(() => {
		const onKey = (e: KeyboardEvent) => {
			if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
				e.preventDefault();
				setPaletteOpen((o) => !o);
			}
		};
		window.addEventListener("keydown", onKey);
		return () => window.removeEventListener("keydown", onKey);
	}, []);

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

	// the unattended Windows installation of the profile, when it has one
	const loadInstallMedia = useCallback(async () => {
		if (!root || !profileId) return;
		try {
			setInstallMedia(await api.installMedia(root, profileId));
		} catch {
			setInstallMedia(null);
		}
	}, [root, profileId]);

	useEffect(() => {
		setInstallMedia(null);
		void loadInstallMedia();
	}, [loadInstallMedia]);

	const hasWindows = !!installMedia?.available;
	const tabs = TABS.filter((t) => hasWindows || !WINDOWS_TABS.includes(t.id));
	useEffect(() => {
		// a profile without those tabs cannot stay on one of them
		if (installMedia && !installMedia.available) setTab((t) => (WINDOWS_TABS.includes(t) ? "home" : t));
	}, [installMedia]);

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
			title: "Selecione a pasta de dados (com config/ e dotfiles/)",
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
		await startRun(
			{ root, profile: wizard.profile, selection, sudoPassword: password },
			() => void loadWizard(true), // refresh the installed state, keep the choices
		);
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
		closeRun();
		setStepIndex(i);
		setTab("setup");
	};

	if (!rootLoaded) return null;

	if (!root) {
		return (
			<RootPicker
				error={error}
				onChoose={chooseRoot}
				onRoot={(r) => {
					setRoot(r);
					setProfileId(null);
					setError(null);
				}}
			/>
		);
	}

	const busy = !!run?.running;
	const ready = wizard && selection && wizard.steps.length > 0;
	const current = TABS.find((t) => t.id === tab);
	const pending = (plan?.steps ?? []).filter((s) => s.pending).length;

	const go = (t: Tab) => {
		closeRun();
		setTab(t);
	};
	const commands: Command[] = busy
		? []
		: [
				...tabs.map((t) => ({
					id: `tab-${t.id}`,
					label: t.label,
					group: "Abas",
					hint: t.hint,
					icon: <t.icon size={16} />,
					run: () => go(t.id),
				})),
				...(ready
					? (wizard?.steps ?? []).map((s, i) => ({
							id: `step-${s.id}`,
							label: `Etapa ${i + 1}: ${s.title}`,
							group: "Etapas",
							icon: <span className="palette-n">{i + 1}</span>,
							run: () => goStep(i),
						}))
					: []),
				...(pending
					? [{ id: "apply", label: "Aplicar pendências", group: "Ações", hint: `${pending} itens`, icon: <Rocket size={16} />, run: requestApply }]
					: []),
				{ id: "root", label: "Trocar pasta de dados…", group: "Ações", icon: <FolderOpen size={16} />, run: () => void chooseRoot() },
				{ id: "help", label: "Como funciona", group: "Ações", icon: <Info size={16} />, run: () => setHelpOpen(true) },
				...profiles
					.filter((p) => p.id !== profileId)
					.map((p) => ({
						id: `profile-${p.id}`,
						label: `Usar o perfil ${p.name}`,
						group: "Perfis",
						hint: p.matches ? "desta máquina" : undefined,
						icon: <UserRound size={16} />,
						run: () => setProfileId(p.id),
					})),
			];

	return (
		<div className="shell" data-env={env || "none"}>
			<Aurora />
			<aside>
				<h1 className="brand">
					<img src="/icon.svg" alt="" className="brand-icon" />
					<span className="brand-text">envsetup</span>
				</h1>
				<div className="env-badge">
					<span className="dot" />
					<span>
						<strong>{env === "dev" ? "DEV" : env === "gaming" ? "GAMING" : "—"}</strong>
						<small>{info?.os === "windows" ? "Windows" : info?.name || "Linux"}</small>
					</span>
				</div>

				<button className="palette-trigger" onClick={() => setPaletteOpen(true)} disabled={busy}>
					<CommandIcon size={14} aria-hidden="true" />
					<span className="grow">Buscar…</span>
					<kbd>Ctrl K</kbd>
				</button>

				<nav>
					{tabs.map((t, i) => {
						const active = tab === t.id && !run;
						return (
							<motion.button
								key={t.id}
								className={active ? "side active" : "side"}
								disabled={busy}
								onClick={() => go(t.id)}
								initial={{ opacity: 0, x: -10 }}
								animate={{ opacity: 1, x: 0 }}
								transition={{ delay: 0.03 * i, type: "spring", stiffness: 380, damping: 30 }}
								whileTap={{ scale: 0.97 }}
							>
								{active && (
									<motion.span
										layoutId="nav-pill"
										className="side-pill"
										transition={{ type: "spring", stiffness: 500, damping: 38 }}
									/>
								)}
								<span className="icon" aria-hidden="true">
									<t.icon size={17} strokeWidth={active ? 2.4 : 2} />
								</span>
								<span className="side-text">{t.label}</span>
								{t.id === "home" && pending > 0 && <span className="side-count">{pending}</span>}
							</motion.button>
						);
					})}
				</nav>

				<span className="spacer" />
				<button className="side" onClick={() => setHelpOpen(true)}>
					<span className="icon" aria-hidden="true">
						<Info size={17} />
					</span>
					<span className="side-text">Como funciona</span>
				</button>
				<label className="side-label">
					Perfil
					<select value={profileId ?? ""} disabled={busy} onChange={(e) => setProfileId(e.target.value)}>
						{!profileId && <option value="">(nenhum)</option>}
						{profiles.map((p) => (
							<option key={p.id} value={p.id}>
								{p.name}
								{p.matches ? " ✓" : ""}
							</option>
						))}
					</select>
				</label>
				<DataFolder root={root} disabled={busy} onChoose={chooseRoot} />
			</aside>

			<main>
				<header className="page-head">
					<AnimatePresence mode="popLayout" initial={false}>
						<motion.h2
							key={run ? "run" : tab}
							className="page-title"
							initial={{ opacity: 0, y: 12 }}
							animate={{ opacity: 1, y: 0 }}
							exit={{ opacity: 0, y: -12 }}
							transition={{ type: "spring", stiffness: 420, damping: 34 }}
						>
							{current && !run && <current.icon size={20} aria-hidden="true" />}
							{run ? "Aplicando" : current?.label}
						</motion.h2>
					</AnimatePresence>
					<span className="muted">{ENV_LABEL[env]}</span>
				</header>

				<AnimatePresence>
					{error && (
						<motion.p
							className="banner bad"
							initial={{ opacity: 0, height: 0 }}
							animate={{ opacity: 1, height: "auto" }}
							exit={{ opacity: 0, height: 0 }}
						>
							{error}
						</motion.p>
					)}
				</AnimatePresence>

				<AnimatePresence mode="wait" initial={false}>
				<motion.div
					key={run ? "run" : tab}
					className="page"
					initial={{ opacity: 0, y: 14 }}
					animate={{ opacity: 1, y: 0 }}
					exit={{ opacity: 0, y: -8 }}
					transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
				>
				{run ? (
					<ApplyView
						{...run}
						stepTitles={Object.fromEntries(
							(wizard?.steps ?? []).map((s) => [s.id, s.title]),
						)}
						onCancel={cancelRun}
						onClose={closeRun}
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
				) : tab === "install" && installMedia?.available ? (
					<InstallTab
						root={root}
						profile={installMedia.profile}
						media={installMedia}
						onChanged={async () => {
							await loadInstallMedia();
							void loadWizard(true); // the Ninite item follows the list
						}}
						onOpenFile={setOpenFile}
					/>
				) : tab === "policies" && installMedia?.available ? (
					<PoliciesTab
						root={root}
						profile={installMedia.profile}
						windows={info?.os === "windows"}
						onApplied={() => void loadWizard(true)}
					/>
				) : tab === "windhawk" && installMedia?.available ? (
					<WindhawkTab root={root} onChanged={() => void loadWizard(true)} />
				) : tab === "playnite" && installMedia?.available ? (
					<PlayniteTab root={root} onChanged={() => void loadWizard(true)} />
				) : !ready ? (
					wizard && wizard.steps.length === 0 ? (
						<p className="banner">Este perfil ainda não tem etapas.</p>
					) : (
						<Skeleton label="Carregando as etapas deste perfil…" />
					)
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
				) : tab === "preview" && installMedia?.available ? (
					<WindowsPreviewTab root={root} profile={installMedia.profile} apps={selection?.apps ?? []} windows={info?.os === "windows"} />
				) : tab === "preview" ? (
					<LinuxPreviewTab root={root} profile={wizard.profile} info={info} plan={plan} onSaved={() => void loadPlan()} />
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
					<Skeleton label="Calculando o plano…" rows={4} />
				)}

				</motion.div>
				</AnimatePresence>

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
								A pasta de dados guarda a descrição do seu ambiente: quais apps
								instalar, quais arquivos de configuração usar e quais pastas
								criar. O envsetup lê isso, compara com a máquina e aplica só o
								que falta.
							</p>
							<p>
								Ela pode ser um clone do repositório git (as mudanças vão para o
								git) ou a cópia dos dados que vêm com o app, em Documentos\envsetup
								(Windows) ou ~/envsetup (Linux). Os arquivos dela são editados aqui
								ou à mão; docs/configuracao.md explica cada um.
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
							<h3>Instalação do Windows</h3>
							<p>
								No perfil de jogos, a aba <strong>Instalação</strong> mostra o
								que o <code>autounattend.xml</code> faz (idioma, conta, ajustes,
								apps removidos, scripts), abre o gerador com essas opções para
								mudar, guarda a lista de programas do Ninite e grava tudo num
								pendrive Ventoy. A aba <strong>Políticas</strong> explica cada
								política de grupo, deixa escolher o valor (o padrão é o do
								vídeo de onde vieram) e aplica no registro.
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
				<CommandPalette open={paletteOpen} commands={commands} onClose={() => setPaletteOpen(false)} />
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
