import { useEffect, useMemo, useState } from "react";
import { Help } from "./Help";

interface Node {
  name: string;
  children: Node[];
}

interface Props {
  /** Paths starting with ~/ */
  dirs: string[];
  /** Paths (with ~) that do not exist on this machine yet. */
  missing: Set<string>;
  onSave: (dirs: string[]) => Promise<void>;
}

const toTree = (dirs: string[]): Node[] => {
  const root: Node = { name: "~", children: [] };
  for (const d of dirs) {
    let cur = root;
    for (const part of d.replace(/^~\//, "").split("/").filter(Boolean)) {
      let next = cur.children.find((c) => c.name === part);
      if (!next) cur.children.push((next = { name: part, children: [] }));
      cur = next;
    }
  }
  return root.children;
};

/** Leaves are enough: creating a folder creates its parents too. */
const toDirs = (nodes: Node[], prefix = "~"): string[] =>
  nodes.flatMap((n) => (n.children.length ? toDirs(n.children, `${prefix}/${n.name}`) : [`${prefix}/${n.name}`]));

const toText = (nodes: Node[], depth = 0): string =>
  nodes.map((n) => `${"  ".repeat(depth)}${n.name}\n${toText(n.children, depth + 1)}`).join("");

const badName = (name: string) => !name || name === "." || name === ".." || name.includes("/");

/** Parses an indented outline (two spaces or a tab per level) into a tree. */
function fromText(text: string): Node[] | string {
  const roots: Node[] = [];
  const stack: { depth: number; node: Node }[] = [];
  for (const [i, raw] of text.split("\n").entries()) {
    if (!raw.trim()) continue;
    const indent = raw.match(/^[ \t]*/)![0].replace(/\t/g, "  ").length;
    if (indent % 2) return `Linha ${i + 1}: use dois espaços por nível.`;
    const depth = indent / 2;
    const name = raw.trim().replace(/\/$/, "");
    if (badName(name)) return `Linha ${i + 1}: nome inválido.`;
    while (stack.length && stack[stack.length - 1].depth >= depth) stack.pop();
    if (depth > (stack.length ? stack[stack.length - 1].depth + 1 : 0)) return `Linha ${i + 1}: nível pulado.`;
    const siblings = stack.length ? stack[stack.length - 1].node.children : roots;
    if (siblings.some((s) => s.name === name)) return `Linha ${i + 1}: "${name}" repetido.`;
    const node: Node = { name, children: [] };
    siblings.push(node);
    stack.push({ depth, node });
  }
  return roots;
}

export function DirTree({ dirs, missing, onSave }: Props) {
  const tree = useMemo(() => toTree(dirs), [dirs]);
  const [mode, setMode] = useState<"tree" | "text">("tree");
  const [text, setText] = useState(() => toText(tree));
  const [editing, setEditing] = useState<string | null>(null); // path being renamed, or "<path>/+" for a new child
  const [draft, setDraft] = useState("");
  const [status, setStatus] = useState<{ ok: boolean; text: string } | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => setText(toText(tree)), [tree]);

  const save = async (next: Node[]) => {
    setSaving(true);
    try {
      await onSave(toDirs(next));
      setStatus({ ok: true, text: "Salvo no perfil" });
    } catch (e) {
      setStatus({ ok: false, text: typeof e === "string" ? e : String(e) });
    } finally {
      setSaving(false);
      setEditing(null);
    }
  };

  /** Returns a copy of the tree with `fn` applied to the sibling list that holds `path`. */
  const edit = (path: string[], fn: (siblings: Node[], name: string) => void): Node[] => {
    const copy: Node[] = structuredClone(tree);
    let siblings = copy;
    for (const part of path.slice(0, -1)) siblings = siblings.find((n) => n.name === part)!.children;
    fn(siblings, path[path.length - 1]);
    return copy;
  };

  const commit = (path: string[], isNew: boolean) => {
    const name = draft.trim();
    if (badName(name)) return setStatus({ ok: false, text: "Nome inválido: não pode ser vazio nem conter /" });
    const parentPath = isNew ? [...path, name] : path;
    const next = edit(parentPath, (siblings, last) => {
      if (isNew) {
        if (siblings.some((s) => s.name === name)) return;
        siblings.push({ name, children: [] });
      } else {
        const node = siblings.find((s) => s.name === last)!;
        if (name !== last && siblings.some((s) => s.name === name)) return;
        node.name = name;
      }
    });
    void save(next);
  };

  const input = (path: string[], isNew: boolean) => (
    <form
      className="tree-form"
      onSubmit={(e) => {
        e.preventDefault();
        commit(path, isNew);
      }}
    >
      <input
        autoFocus
        value={draft}
        placeholder="nome da pasta"
        aria-label="Nome da pasta"
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => e.key === "Escape" && setEditing(null)}
      />
      <button className="primary small-btn" disabled={saving}>
        OK
      </button>
      <button type="button" className="ghost small-btn" onClick={() => setEditing(null)}>
        Cancelar
      </button>
    </form>
  );

  const render = (nodes: Node[], path: string[]) => (
    <ul className="tree">
      {nodes.map((n) => {
        const here = [...path, n.name];
        const key = here.join("/");
        const full = `~/${key}`;
        return (
          <li key={key}>
            {editing === key ? (
              input(here, false)
            ) : (
              <div className="tree-row">
                <span aria-hidden="true">📁</span>
                <span className="mono tree-name">{n.name}</span>
                {missing.has(full) && <span className="badge warn">será criada</span>}
                <span className="tree-actions">
                  <button className="ghost small-btn" title="Adicionar subpasta" onClick={() => (setDraft(""), setEditing(`${key}/+`))}>
                    + subpasta
                  </button>
                  <button className="ghost small-btn" title="Renomear" onClick={() => (setDraft(n.name), setEditing(key))}>
                    Renomear
                  </button>
                  <button
                    className="ghost small-btn danger-text"
                    title="Tirar da lista (não apaga nada do disco)"
                    disabled={saving}
                    onClick={() => void save(edit(here, (siblings, last) => void siblings.splice(siblings.findIndex((s) => s.name === last), 1)))}
                  >
                    Remover
                  </button>
                </span>
              </div>
            )}
            {render(n.children, here)}
            {editing === `${key}/+` && <div className="tree-new">{input(here, true)}</div>}
          </li>
        );
      })}
    </ul>
  );

  return (
    <div className="card dirtree">
      <div className="row">
        <strong>Estrutura de pastas</strong>
        <Help title="Como funciona a estrutura">
          Estas são as pastas criadas dentro da sua home ao aplicar. A lista é salva no arquivo do perfil, no repositório,
          então vale para as próximas instalações. Remover daqui só tira da lista: nada é apagado do disco. Pastas que já
          existem são mantidas como estão.
        </Help>
        <span className="spacer" />
        {status && <span className={status.ok ? "badge ok" : "badge bad"}>{status.text}</span>}
        <div className="seg" role="tablist" aria-label="Modo de edição">
          <button role="tab" aria-selected={mode === "tree"} className={mode === "tree" ? "active" : ""} onClick={() => setMode("tree")}>
            Árvore
          </button>
          <button role="tab" aria-selected={mode === "text"} className={mode === "text" ? "active" : ""} onClick={() => setMode("text")}>
            Texto
          </button>
        </div>
      </div>

      {mode === "tree" ? (
        <>
          <div className="tree-row root">
            <span aria-hidden="true">🏠</span>
            <span className="mono tree-name">~</span>
            <span className="muted small">sua pasta pessoal</span>
          </div>
          {render(tree, [])}
          {editing === "/+" ? (
            <div className="tree-new">{input([], true)}</div>
          ) : (
            <button className="ghost small-btn tree-add" onClick={() => (setDraft(""), setEditing("/+"))}>
              + pasta em ~
            </button>
          )}
        </>
      ) : (
        <>
          <p className="muted small-note">Uma pasta por linha, dentro de ~. Use dois espaços para colocar uma pasta dentro da outra.</p>
          <textarea
            className="tree-text mono"
            rows={Math.min(16, Math.max(6, text.split("\n").length + 1))}
            value={text}
            spellCheck={false}
            aria-label="Estrutura de pastas em texto"
            onChange={(e) => setText(e.target.value)}
          />
          <div className="row end">
            <button className="ghost small-btn" onClick={() => setText(toText(tree))}>
              Descartar
            </button>
            <button
              className="primary small-btn"
              disabled={saving || text === toText(tree)}
              onClick={() => {
                const parsed = fromText(text);
                if (typeof parsed === "string") setStatus({ ok: false, text: parsed });
                else void save(parsed);
              }}
            >
              Salvar estrutura
            </button>
          </div>
        </>
      )}
    </div>
  );
}
