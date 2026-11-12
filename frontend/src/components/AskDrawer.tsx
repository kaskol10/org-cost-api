import { useEffect, useRef, type ReactNode } from "react";

interface Props {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
}

/**
 * Right-hand drawer for the Ask chat. It stays mounted while closed (hidden
 * with visibility, so it's out of the tab order and accessibility tree) so a
 * conversation survives closing and reopening.
 */
export default function AskDrawer({ open, onClose, children }: Props) {
  const panelRef = useRef<HTMLElement>(null);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    const t = window.setTimeout(() => {
      panelRef.current?.querySelector<HTMLElement>("input, textarea")?.focus();
    }, 50);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.clearTimeout(t);
    };
  }, [open, onClose]);

  return (
    <div className={`ask-drawer-root${open ? " is-open" : ""}`}>
      <div className="ask-drawer-scrim" onClick={onClose} aria-hidden="true" />
      <aside ref={panelRef} className="ask-drawer" role="dialog" aria-modal="false" aria-label="Ask about your AWS costs">
        <div className="ask-drawer-head">
          <h2>Ask about your costs</h2>
          <button type="button" className="btn btn-ghost ask-drawer-close" onClick={onClose} aria-label="Close Ask panel">
            ✕
          </button>
        </div>
        <div className="ask-drawer-body">{children}</div>
      </aside>
    </div>
  );
}
