import { ask, message } from "@tauri-apps/plugin-dialog";
import { useEffect, useState } from "react";
import * as api from "../api";
import { errorMessage } from "../api";
import type { DataInfo } from "../types";

interface Props {
  root: string;
  disabled: boolean;
  /** Opens the folder picker to use another data folder. */
  onChoose: () => void;
}

/**
 * The data folder in the sidebar: which one is in use, switching to another
 * and, when it was copied from the installer, adding what a newer version
 * of the app brings.
 */
export function DataFolder({ root, disabled, onChoose }: Props) {
  const [info, setInfo] = useState<DataInfo | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.dataInfo(root).then(setInfo, () => setInfo(null));
  }, [root]);

  // a git clone is updated with git; a folder made by hand has nothing to update from
  const canUpdate = !!info?.bundled && !info.git;
  const newer = canUpdate && info.seeded_version !== info.app_version;

  const update = async () => {
    const ok = await ask(
      `Copiar para ${root} os arquivos que vêm com o envsetup ${info?.app_version} e ainda não estão lá?\n\n` +
        "Nada que já existe é alterado: suas edições ficam como estão.",
      { title: "Atualizar os dados", kind: "info" },
    );
    if (!ok) return;
    setBusy(true);
    try {
      const r = await api.updateData(root);
      setInfo(await api.dataInfo(root));
      await message(
        r.added
          ? `${r.added} arquivos novos copiados; ${r.kept} que já existiam foram mantidos.`
          : "Nada a copiar: a pasta já tem todos os arquivos desta versão.",
        { title: "Atualizar os dados" },
      );
    } catch (e) {
      await message(errorMessage(e), { title: "Atualizar os dados", kind: "error" });
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <button className="ghost small-btn" onClick={onChoose} title={root} disabled={disabled || busy}>
        Trocar pasta de dados…
      </button>
      {canUpdate && (
        <button
          className="ghost small-btn"
          onClick={update}
          disabled={disabled || busy}
          title="Copia os arquivos novos desta versão do app; não altera os que você editou"
        >
          {busy ? "Atualizando…" : newer ? "Atualizar dados (versão nova)" : "Atualizar dados"}
        </button>
      )}
    </>
  );
}
