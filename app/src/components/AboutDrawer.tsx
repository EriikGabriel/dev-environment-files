import { getTauriVersion, getVersion } from "@tauri-apps/api/app";
import { useEffect, useState } from "react";
import type { AppUpdater } from "../useAppUpdate";
import { Drawer } from "./Drawer";
import { INSTALL_SH, updateStatus } from "./UpdateBanner";

/** Where the installers of each version are published. */
export const RELEASES = "https://github.com/EriikGabriel/dev-environment-files/releases";

/** The app's version, read once: the one of tauri.conf.json, which is the installer's. */
export function useAppVersion() {
  const [version, setVersion] = useState<string | null>(null);
  useEffect(() => {
    getVersion()
      .then(setVersion)
      .catch(() => setVersion(null));
  }, []);
  return version;
}

interface Props {
  version: string | null;
  root: string | null;
  system: string;
  updater: AppUpdater;
  onClose: () => void;
}

/** About: the version this machine runs, and whether a newer one is out. */
export function AboutDrawer({ version, root, system, updater, onClose }: Props) {
  const [tauri, setTauri] = useState<string | null>(null);
  useEffect(() => {
    getTauriVersion()
      .then(setTauri)
      .catch(() => setTauri(null));
  }, []);
  const tag = version ? `v${version}` : "—";
  const { phase, update, error } = updater;
  const working = phase === "downloading" || phase === "installing";
  const canInstall = !!update?.installer;

  return (
    <Drawer title="Sobre o envsetup" subtitle="A versão que está rodando nesta máquina" onClose={onClose}>
      <div className="guide">
        <p className="about-version">
          <img src="/icon.svg" alt="" className="brand-icon" />
          <span>
            <strong>envsetup {tag}</strong>
            <span className="muted small">a release de mesmo nome no GitHub tem o instalador desta versão</span>
          </span>
        </p>

        <h3>Atualizações</h3>
        <div className="about-update">
          {phase === "checking" && <p className="muted">Procurando uma versão nova…</p>}
          {phase === "uptodate" && <p>Você está na versão mais nova (v{update?.latest}).</p>}
          {phase === "idle" && <p className="muted">Ainda não procurei uma versão nova.</p>}
          {(phase === "available" || working || (phase === "error" && update?.available)) && update && (
            <>
              <p>
                <strong>Versão v{update.latest} disponível</strong> (esta é a v{update.current}).
              </p>
              {update.notes && <pre className="about-notes">{update.notes}</pre>}
              {updateStatus(updater) && <p className="muted">{updateStatus(updater)}</p>}
            </>
          )}
          {phase === "error" && <p className="bad-text">{error}</p>}
          <div className="update-actions">
            {canInstall && update?.available && (
              <button className="primary" disabled={working} onClick={() => void updater.install()}>
                {working ? "Atualizando…" : "Atualizar agora"}
              </button>
            )}
            <button disabled={working || phase === "checking"} onClick={() => void updater.check()}>
              Procurar atualização
            </button>
          </div>
          {update?.available && !canInstall && (
            <p className="muted small">
              Este sistema instala a versão nova pelo terminal:
              <br />
              <code className="mono">{INSTALL_SH}</code>
            </p>
          )}
        </div>

        <dl className="about-list">
          <dt>Versão</dt>
          <dd className="mono">{tag}</dd>
          <dt>Sistema</dt>
          <dd>{system}</dd>
          <dt>Pasta de dados</dt>
          <dd className="mono">{root ?? "(nenhuma)"}</dd>
          <dt>Tauri</dt>
          <dd className="mono">{tauri ?? "—"}</dd>
          <dt>Releases</dt>
          <dd className="mono">{RELEASES}</dd>
        </dl>
        <p className="muted small">
          No Windows a atualização baixa o instalador da release, confere o arquivo (SHA-256) e instala sozinha; o app fecha e abre de
          novo. Os dados que vieram com o app (Documentos\envsetup) só ganham os arquivos novos com “Atualizar dados”; um arquivo que já
          existe na pasta não é trocado.
        </p>
      </div>
    </Drawer>
  );
}
