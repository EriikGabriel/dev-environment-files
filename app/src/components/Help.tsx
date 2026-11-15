import { useEffect, useId, useRef, useState, type ReactNode } from "react";

/** A small "?" button that opens an explanation next to it. */
export function Help({ title, children }: { title: string; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLSpanElement>(null);
  const id = useId();

  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    const esc = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    document.addEventListener("mousedown", away);
    document.addEventListener("keydown", esc);
    return () => {
      document.removeEventListener("mousedown", away);
      document.removeEventListener("keydown", esc);
    };
  }, [open]);

  return (
    <span className="help" ref={ref}>
      <button
        type="button"
        className="help-btn"
        aria-label={`Ajuda: ${title}`}
        aria-expanded={open}
        aria-controls={id}
        onClick={(e) => {
          e.stopPropagation();
          setOpen((o) => !o);
        }}
      >
        ?
      </button>
      {open && (
        <span className="help-pop" id={id} role="note">
          <strong>{title}</strong>
          <span>{children}</span>
        </span>
      )}
    </span>
  );
}
