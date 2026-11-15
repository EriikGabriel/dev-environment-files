import { useEffect, type ReactNode } from "react";

interface Props {
  title: ReactNode;
  subtitle?: ReactNode;
  onClose: () => void;
  children: ReactNode;
}

/** A panel that slides in from the right, over the page. */
export function Drawer({ title, subtitle, onClose, children }: Props) {
  useEffect(() => {
    const esc = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", esc);
    return () => document.removeEventListener("keydown", esc);
  }, [onClose]);

  return (
    <div className="drawer-wrap" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <aside className="drawer" role="dialog" aria-modal="true">
        <div className="drawer-head">
          <div className="grow-col">
            <strong className="drawer-title">{title}</strong>
            {subtitle && <span className="muted small">{subtitle}</span>}
          </div>
          <button className="ghost small-btn" onClick={onClose} aria-label="Fechar painel">
            ✕
          </button>
        </div>
        <div className="drawer-body">{children}</div>
      </aside>
    </div>
  );
}
