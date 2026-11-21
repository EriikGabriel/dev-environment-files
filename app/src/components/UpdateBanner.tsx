import type { AppUpdater } from "../useAppUpdate";

/** The command that installs the latest version on a system the app cannot update itself on. */
export const INSTALL_SH = "curl -fsSL https://github.com/EriikGabriel/dev-environment-files/releases/latest/download/install.sh | bash";

const mb = (n: number) => `${Math.max(1, Math.round(n / 1048576))} MB`;

/** What the update is doing, in a sentence; null when there is nothing to say. */
export function updateStatus(u: AppUpdater): string | null {
  const v = u.update ? `v${u.update.latest}` : "";
  switch (u.phase) {
    case "downloading":
      return `Baixando a ${v} e conferindo o arquivo…`;
    case "installing":
      return `Instalando a ${v}: o envsetup fecha e abre de novo sozinho.`;
    default:
      return null;
  }
}

/** A newer release is out: says so at the top of the page, and updates in one click on Windows. */
export function UpdateBanner({ updater, onDetails }: { updater: AppUpdater; onDetails: () => void }) {
  const { phase, update, error, dismissed } = updater;
  const working = phase === "downloading" || phase === "installing";
  const failed = phase === "error" && update?.available;
  if (!update?.available || (dismissed && !working && !failed)) return null;
  const canInstall = !!update.installer;
  const status = updateStatus(updater);

  return (
    <div className={failed ? "banner bad update-banner" : "banner update-banner"} role="status">
      <div className="update-text">
        <strong>
          {failed ? `Não deu para instalar a v${update.latest}` : `Versão v${update.latest} disponível`}
        </strong>
        <span className="muted small">
          {failed ? error : (status ?? `Esta máquina tem a v${update.current}.${canInstall ? ` O instalador tem ${mb(update.installer?.size ?? 0)}.` : ""}`)}
        </span>
      </div>
      <div className="update-actions">
        {canInstall ? (
          <button className="primary" disabled={working} onClick={() => void updater.install()}>
            {working ? "Atualizando…" : failed ? "Tentar de novo" : "Atualizar agora"}
          </button>
        ) : (
          <button onClick={onDetails}>Como atualizar</button>
        )}
        {!working && (
          <button className="ghost" onClick={onDetails}>
            Detalhes
          </button>
        )}
        {!working && (
          <button className="ghost" onClick={updater.dismiss}>
            Depois
          </button>
        )}
      </div>
    </div>
  );
}
