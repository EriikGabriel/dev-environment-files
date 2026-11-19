import { CloudDownload, GitBranch, HardDrive } from "lucide-react";
import { motion } from "motion/react";
import type { ReactNode } from "react";
import type { GitPart } from "../../types";

const size = (b: number) =>
  b >= 1 << 30 ? `${(b / (1 << 30)).toFixed(1)} GB` : b >= 1 << 20 ? `${(b / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`;

export interface Place {
  title: string;
  icon: ReactNode;
  tone: "git" | "net" | "disk";
  /** The big line: a size, a version. */
  head: string;
  lines: string[];
  badge?: { text: string; warn?: boolean };
}

/** The git tile of a program, from the JSON of the CLI. */
export function gitPlace(git: GitPart, what: string): Place {
  return {
    title: "GitHub",
    icon: <GitBranch size={18} />,
    tone: "git",
    head: git.files ? `${git.files} arquivos · ${size(git.bytes)}` : "ainda vazio",
    lines: [what, `pasta ${git.dir} do repo; senhas, chaves de API e tokens ficam de fora`],
    badge: git.pending
      ? { text: `${git.pending} ${git.pending === 1 ? "mudança" : "mudanças"} para salvar e commitar`, warn: true }
      : git.files
        ? { text: "em dia" }
        : undefined,
  };
}

export const netIcon = <CloudDownload size={18} />;
export const diskIcon = <HardDrive size={18} />;

/** Where each part of a portable program lives: git, the internet, the store. */
export function WhereItLives({ places }: { places: Place[] }) {
  return (
    <div className="places">
      {places.map((p, i) => (
        <motion.div
          key={p.title}
          className={`card place place-${p.tone}`}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.05 * i, type: "spring", stiffness: 300, damping: 26 }}
        >
          <div className="row">
            <span className="place-icon" aria-hidden="true">
              {p.icon}
            </span>
            <strong className="grow">{p.title}</strong>
            {p.badge && <span className={p.badge.warn ? "badge warn" : "badge ok"}>{p.badge.text}</span>}
          </div>
          <span className="place-head">{p.head}</span>
          {p.lines.map((l) => (
            <span key={l} className="muted small-note">
              {l}
            </span>
          ))}
        </motion.div>
      ))}
    </div>
  );
}
