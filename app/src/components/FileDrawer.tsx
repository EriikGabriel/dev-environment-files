import { useEffect, useState } from "react";
import { errorMessage, show } from "../api";
import type { FileView } from "../types";
import { Drawer } from "./Drawer";
import { Help } from "./Help";

export interface FileTarget {
  /** Path in the repo, relative to its root. */
  src: string;
  dest?: string;
  mode?: string;
}

const size = (n: number) => (n < 1024 ? `${n} B` : n < 1 << 20 ? `${(n / 1024).toFixed(1)} KB` : `${(n / (1 << 20)).toFixed(1)} MB`);

/** Shows a repo file (or browses a repo folder) in a side panel. */
export function FileDrawer({ root, file, onClose }: { root: string; file: FileTarget; onClose: () => void }) {
  const [path, setPath] = useState(file.src);
  const [view, setView] = useState<FileView | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => setPath(file.src), [file.src]);
  useEffect(() => {
    let stale = false;
    setView(null);
    setError(null);
    show(root, path)
      .then((v) => !stale && setView(v))
      .catch((e) => !stale && setError(errorMessage(e)));
    return () => {
      stale = true;
    };
  }, [root, path]);

  // breadcrumb below the entry the drawer was opened on
  const inner = path.startsWith(file.src) ? path.slice(file.src.length).split("/").filter(Boolean) : [];
  const lines = view?.kind === "text" ? (view.content ?? "").replace(/\n$/, "").split("\n") : [];

  return (
    <Drawer
      title={<span className="mono">{file.src.split("/").pop()}</span>}
      subtitle={
        <>
          {file.dest ? `${file.dest} ${file.mode === "copy" ? "⇐ cópia de" : "→"} ` : ""}
          <span className="mono">{file.src}</span>
        </>
      }
      onClose={onClose}
    >
      <p className="muted small-note">
        Este é o arquivo guardado no repositório.
        <Help title="Por que o arquivo está no repo?">
          O arquivo de verdade fica em <code>dotfiles/</code>. Na sua home o destino vira um symlink para ele, então editar
          em qualquer um dos dois lugares altera o mesmo arquivo, e um <code>git commit</code> guarda a mudança.
        </Help>
      </p>

      {inner.length > 0 && (
        <div className="crumbs">
          <button className="ghost small-btn" onClick={() => setPath(file.src)}>
            {file.src.split("/").pop()}
          </button>
          {inner.map((part, i) => (
            <span key={i} className="row">
              <span className="muted">/</span>
              <button className="ghost small-btn" onClick={() => setPath([file.src, ...inner.slice(0, i + 1)].join("/"))}>
                {part}
              </button>
            </span>
          ))}
        </div>
      )}

      {error && <p className="banner bad">{error}</p>}
      {!view && !error && <p className="muted">Lendo…</p>}

      {view?.kind === "dir" && (
        <ul className="list entries">
          {view.entries.length === 0 && <li className="muted">Pasta vazia.</li>}
          {view.entries.map((e) => (
            <li key={e.name}>
              <button className="entry" onClick={() => setPath(`${path}/${e.name}`)}>
                <span aria-hidden="true">{e.dir ? "📁" : "📄"}</span>
                <span className="grow mono">{e.name}</span>
                {!e.dir && <span className="muted small">{size(e.size)}</span>}
              </button>
            </li>
          ))}
        </ul>
      )}
      {view?.kind === "image" && <img className="preview" src={view.data_url} alt={view.path} />}
      {view?.kind === "binary" && <p className="banner">Arquivo binário ({size(view.size)}); não dá para mostrar como texto.</p>}
      {view?.kind === "text" && (
        <>
          <div className="row muted small">
            <span>{lines.length} linhas</span>
            <span>·</span>
            <span>{size(view.size)}</span>
            {view.truncated && <span className="badge warn">mostrando só o início</span>}
            <span className="spacer" />
            <button className="ghost small-btn" onClick={() => void navigator.clipboard.writeText(view.content ?? "")}>
              Copiar
            </button>
          </div>
          <pre className="code">
            {lines.map((l, i) => (
              <span className="code-line" key={i}>
                {l || " "}
              </span>
            ))}
          </pre>
        </>
      )}
    </Drawer>
  );
}
