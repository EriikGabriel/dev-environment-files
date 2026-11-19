import { FolderGit2, PackageOpen } from "lucide-react";
import { motion } from "motion/react";
import { useEffect, useState } from "react";
import * as api from "../api";
import { errorMessage } from "../api";
import type { DataInfo } from "../types";
import { Aurora } from "./ui/Aurora";

interface Props {
  /** Called with the data folder once there is one. */
  onRoot: (root: string) => void;
  /** Opens the folder picker (a git clone, or a folder copied by hand). */
  onChoose: () => void;
  error: string | null;
}

/**
 * The first screen when there is no data folder yet. An installed app offers
 * to copy the data it carries; a development build only has the folder picker.
 */
export function RootPicker({ onRoot, onChoose, error }: Props) {
  const [info, setInfo] = useState<DataInfo | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState<string | null>(null);

  useEffect(() => {
    api.dataInfo(null).then(setInfo, () => setInfo(null));
  }, []);

  const seed = async () => {
    setBusy(true);
    setFailed(null);
    try {
      onRoot(await api.seedData());
    } catch (e) {
      setFailed(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const rise = (i: number) => ({
    initial: { opacity: 0, y: 18 },
    animate: { opacity: 1, y: 0 },
    transition: { delay: 0.08 * i, type: "spring" as const, stiffness: 260, damping: 24 },
  });

  return (
    <div className="shell welcome-shell" data-env="none">
      <Aurora />
      <main className="center welcome">
        <motion.img
          src="/icon.svg"
          alt=""
          className="brand-icon big-icon float"
          initial={{ scale: 0.4, opacity: 0, rotate: -20 }}
          animate={{ scale: 1, opacity: 1, rotate: 0 }}
          transition={{ type: "spring", stiffness: 200, damping: 14 }}
        />
        <motion.h1 className="welcome-title" {...rise(1)}>
          Bem-vindo ao <span className="gradient-text">envsetup</span>
        </motion.h1>
        <motion.p className="muted welcome-lead" {...rise(2)}>
          {info?.bundled ? (
            <>
              Os perfis, o catálogo de apps e os dotfiles ficam numa pasta sua, onde você pode editar. Use os dados que vêm com o app
              (copiados para <code>{info.default_dir}</code>) ou escolha um clone do repositório.
            </>
          ) : (
            <>
              Selecione a pasta que contém <code>config/</code> e <code>dotfiles/</code> (o clone do repositório).
            </>
          )}
        </motion.p>
        {(failed || error) && <p className="banner bad">{failed || error}</p>}
        <div className="welcome-choices">
          {info?.bundled && (
            <motion.button className="card choice spot" disabled={busy} onClick={seed} {...rise(3)} whileHover={{ y: -4 }} whileTap={{ scale: 0.98 }}>
              <span className="choice-icon a">
                <PackageOpen size={26} />
              </span>
              <strong>{busy ? "Copiando…" : "Usar os dados incluídos"}</strong>
              <span className="muted">Uma cópia editável, pronta para usar. Recomendado.</span>
            </motion.button>
          )}
          <motion.button className="card choice spot" disabled={busy} onClick={onChoose} {...rise(4)} whileHover={{ y: -4 }} whileTap={{ scale: 0.98 }}>
            <span className="choice-icon b">
              <FolderGit2 size={26} />
            </span>
            <strong>Escolher pasta…</strong>
            <span className="muted">Um clone git: as mudanças ficam versionadas.</span>
          </motion.button>
        </div>
      </main>
    </div>
  );
}
