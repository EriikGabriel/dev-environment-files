import { getTauriVersion, getVersion } from "@tauri-apps/api/app";
import { useEffect, useState } from "react";
import { Drawer } from "./Drawer";

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
  onClose: () => void;
}

/** About: the version this machine runs, to tell which release it is. */
export function AboutDrawer({ version, root, system, onClose }: Props) {
  const [tauri, setTauri] = useState<string | null>(null);
  useEffect(() => {
    getTauriVersion()
      .then(setTauri)
      .catch(() => setTauri(null));
  }, []);
  const tag = version ? `v${version}` : "—";

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
          Uma versão nova chega instalando o instalador da release. Os dados que vieram com o app (Documentos\envsetup) só ganham os arquivos
          novos com “Atualizar dados”; um arquivo que já existe na pasta não é trocado.
        </p>
      </div>
    </Drawer>
  );
}
