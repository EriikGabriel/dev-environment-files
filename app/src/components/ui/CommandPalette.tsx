import { CornerDownLeft, Search } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

export interface Command {
  id: string;
  label: string;
  group: string;
  icon: ReactNode;
  hint?: string;
  run: () => void;
}

interface Props {
  open: boolean;
  commands: Command[];
  onClose: () => void;
}

/** Folds accents and case so "politicas" finds "Políticas". */
const fold = (s: string) => s.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();

/** Ctrl+K: jump to a tab or run an action by typing part of its name. */
export function CommandPalette({ open, commands, onClose }: Props) {
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const input = useRef<HTMLInputElement>(null);
  const list = useRef<HTMLUListElement>(null);

  const found = useMemo(() => {
    const words = fold(query).split(/\s+/).filter(Boolean);
    return commands.filter((c) => {
      const text = fold(`${c.label} ${c.group} ${c.hint ?? ""}`);
      return words.every((w) => text.includes(w));
    });
  }, [commands, query]);

  useEffect(() => {
    if (!open) return;
    setQuery("");
    setActive(0);
    setTimeout(() => input.current?.focus(), 0);
  }, [open]);

  useEffect(() => setActive(0), [query]);

  useEffect(() => {
    list.current?.querySelector<HTMLElement>(`[data-i="${active}"]`)?.scrollIntoView({ block: "nearest" });
  }, [active]);

  const run = (c: Command | undefined) => {
    if (!c) return;
    onClose();
    c.run();
  };

  const onKey = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((i) => Math.min(found.length - 1, i + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => Math.max(0, i - 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      run(found[active]);
    } else if (e.key === "Escape") {
      onClose();
    }
  };

  let lastGroup = "";
  return (
    <AnimatePresence>
      {open && (
        <motion.div
          className="palette-wrap"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.14 }}
          onMouseDown={(e) => e.target === e.currentTarget && onClose()}
        >
          <motion.div
            className="palette"
            role="dialog"
            aria-modal="true"
            aria-label="Paleta de comandos"
            initial={{ opacity: 0, y: -14, scale: 0.97 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -8, scale: 0.98 }}
            transition={{ type: "spring", stiffness: 420, damping: 32 }}
          >
            <label className="palette-input">
              <Search size={17} aria-hidden="true" />
              <input
                ref={input}
                value={query}
                placeholder="Ir para uma aba ou executar uma ação…"
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={onKey}
                aria-activedescendant={found[active] ? `cmd-${found[active].id}` : undefined}
                aria-controls="palette-list"
              />
              <kbd>Esc</kbd>
            </label>
            <ul className="palette-list" id="palette-list" role="listbox" ref={list}>
              {found.length === 0 && <li className="palette-empty muted">Nada encontrado para “{query}”.</li>}
              {found.map((c, i) => {
                const head = c.group !== lastGroup ? c.group : null;
                lastGroup = c.group;
                return (
                  <li key={c.id} role="presentation">
                    {head && <span className="palette-group">{head}</span>}
                    <button
                      id={`cmd-${c.id}`}
                      role="option"
                      aria-selected={i === active}
                      data-i={i}
                      className={i === active ? "palette-item active" : "palette-item"}
                      onMouseMove={() => setActive(i)}
                      onClick={() => run(c)}
                    >
                      {i === active && <motion.span layoutId="palette-hl" className="palette-hl" transition={{ type: "spring", stiffness: 600, damping: 40 }} />}
                      <span className="palette-icon">{c.icon}</span>
                      <span className="grow">{c.label}</span>
                      {c.hint && <span className="muted small">{c.hint}</span>}
                      {i === active && <CornerDownLeft size={14} className="muted" aria-hidden="true" />}
                    </button>
                  </li>
                );
              })}
            </ul>
            <div className="palette-foot muted">
              <span>
                <kbd>↑</kbd>
                <kbd>↓</kbd> navegar
              </span>
              <span>
                <kbd>Enter</kbd> abrir
              </span>
              <span className="spacer" />
              <span>
                <kbd>Ctrl</kbd>
                <kbd>K</kbd>
              </span>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
