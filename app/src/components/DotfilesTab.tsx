import { useCallback, useEffect, useState } from "react";
import { open } from "@tauri-apps/plugin-dialog";
import { homeDir } from "@tauri-apps/api/path";
import { getCurrentWebview } from "@tauri-apps/api/webview";
import { addFile, errorMessage } from "../api";
import type { Plan } from "../types";
import type { FileTarget } from "./FileDrawer";
import { Help } from "./Help";

interface Props {
  root: string;
  profile: string;
  plan: Plan;
  busy: boolean;
  onChanged: () => void;
  onOpenFile: (f: FileTarget) => void;
}

type Outcome = { path: string; ok: boolean; text: string };

export function DotfilesTab({ root, profile, plan, busy, onChanged, onOpenFile }: Props) {
  const [hover, setHover] = useState(false);
  const [group, setGroup] = useState("");
  const [copy, setCopy] = useState(false);
  const [working, setWorking] = useState(false);
  const [outcomes, setOutcomes] = useState<Outcome[]>([]);

  const files = plan.steps.filter((s) => s.kind === "file");
  const rel = (p?: string) => (p && p.startsWith(root) ? p.slice(root.length + 1) : (p ?? ""));

  const capture = useCallback(
    async (paths: string[]) => {
      if (paths.length === 0) return;
      setWorking(true);
      const results: Outcome[] = [];
      for (const path of paths) {
        try {
          // A shared group makes sense for one item; with several, derive each.
          const r = await addFile(root, path, { profile, copy, group: paths.length === 1 ? group || undefined : undefined });
          results.push({ path, ok: true, text: `${r.dest} → ${r.src} (${r.mode})` });
        } catch (e) {
          results.push({ path, ok: false, text: errorMessage(e) });
        }
      }
      setOutcomes(results);
      setWorking(false);
      onChanged();
    },
    [root, profile, copy, group, onChanged],
  );

  // Native drag and drop: unlike the browser it gives real file paths.
  useEffect(() => {
    if (busy) return;
    let off: (() => void) | undefined;
    let cancelled = false;
    getCurrentWebview()
      .onDragDropEvent((ev) => {
        const p = ev.payload;
        if (p.type === "enter" || p.type === "over") setHover(true);
        else if (p.type === "leave") setHover(false);
        else if (p.type === "drop") {
          setHover(false);
          void capture(p.paths);
        }
      })
      .then((fn) => {
        if (cancelled) fn();
        else off = fn;
      });
    return () => {
      cancelled = true;
      off?.();
    };
  }, [busy, capture]);

  const browse = async (directory: boolean) => {
    const picked = await open({ multiple: true, directory, defaultPath: await homeDir() });
    if (picked) void capture(Array.isArray(picked) ? picked : [picked]);
  };

  return (
    <section>
      <div className={`drop ${hover ? "hover" : ""}`}>
        <strong>
          Solte aqui arquivos ou pastas de configuração
          <Help title="Guardar uma config no repo">
            Use para configs que já existem na máquina e ainda não estão no repositório. O arquivo é copiado para{" "}
            <code>dotfiles/</code>, entra na lista do perfil e o original é trocado por um symlink. Com “manter o
            original” nada muda na sua home. Confira se o arquivo não tem senhas ou chaves antes de fazer commit.
          </Help>
        </strong>
        <span className="muted">
          Eles são copiados para <code>dotfiles/</code>, registrados no perfil <code>{profile}</code>, e o original vira
          um symlink. Vale qualquer coisa dentro da sua home, inclusive imagens.
        </span>
        <div className="row">
          <button disabled={busy || working} onClick={() => browse(false)}>
            Adicionar arquivos…
          </button>
          <button disabled={busy || working} onClick={() => browse(true)}>
            Adicionar pasta…
          </button>
          <input
            className="group"
            placeholder="nome da pasta (opcional)"
            value={group}
            onChange={(e) => setGroup(e.target.value)}
            aria-label="Pasta dentro de dotfiles"
          />
          <label className="check">
            <input type="checkbox" checked={copy} onChange={(e) => setCopy(e.target.checked)} /> manter o original, só copiar
          </label>
        </div>
      </div>

      {outcomes.length > 0 && (
        <ul className="list outcomes">
          {outcomes.map((o) => (
            <li key={o.path}>
              <span className={o.ok ? "badge ok" : "badge bad"}>{o.ok ? "adicionado" : "falhou"}</span>
              <span className="grow mono">{o.ok ? o.text : `${o.path}: ${o.text}`}</span>
            </li>
          ))}
        </ul>
      )}

      <h3>
        Arquivos gerenciados ({files.length})
        <Help title="Pendente ou em dia">
          “em dia” quer dizer que o destino na sua home já é o symlink (ou a cópia idêntica) do arquivo do repo.
          “pendente” quer dizer que ainda não é: ao aplicar, o que existir lá vira <code>.bak</code> e o symlink é criado.
        </Help>
      </h3>
      <ul className="list">
        {files.map((f) => (
          <li key={f.id}>
            <button
              className="entry"
              title="Ver o conteúdo"
              onClick={() => onOpenFile({ src: rel(f.src), dest: f.id.replace(/^\/home\/[^/]+/, "~"), mode: f.mode })}
            >
              <span aria-hidden="true">🔗</span>
              <span className="mono file-dest">{f.id.replace(/^\/home\/[^/]+/, "~")}</span>
              <span className="muted mono grow">{rel(f.src)}</span>
              <span className="muted">{f.mode === "copy" ? "cópia" : "symlink"}</span>
              <span className={f.pending ? "badge warn" : "badge ok"}>{f.pending ? "pendente" : "em dia"}</span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
