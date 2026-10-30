import { useEffect, useId, useRef, type FormEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { Button } from './primitives';

type OpenModalEntry = { close: () => void };
const openModalStack: OpenModalEntry[] = [];

export function Modal({ open, title, description, onClose, onSubmit, submitLabel = 'Save', submitVariant = 'primary', busy, children, wide = false, fullscreen = false, jobLayout = false }: {
  open: boolean; title: string; description?: string; onClose: () => void; onSubmit?: (event: FormEvent<HTMLFormElement>) => void;
  submitLabel?: string; submitVariant?: 'primary' | 'secondary' | 'ghost' | 'danger'; busy?: boolean; children: ReactNode; wide?: boolean; fullscreen?: boolean; jobLayout?: boolean;
}) {
  const titleId = useId();
  const onCloseRef = useRef(onClose);
  useEffect(() => { onCloseRef.current = onClose; }, [onClose]);
  useEffect(() => {
    if (!open) return;
    const previousOverflow = document.body.style.overflow;
    const entry: OpenModalEntry = { close: () => onCloseRef.current() };
    openModalStack.push(entry);
    document.body.style.overflow = 'hidden';
    const escape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || openModalStack[openModalStack.length - 1] !== entry) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      entry.close();
    };
    window.addEventListener('keydown', escape);
    return () => {
      window.removeEventListener('keydown', escape);
      const index = openModalStack.lastIndexOf(entry);
      if (index >= 0) openModalStack.splice(index, 1);
      document.body.style.overflow = previousOverflow;
    };
  }, [open]);
  if (!open) return null;
  return createPortal(<div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.currentTarget === event.target) onClose(); }}>
    <section className={`modal ${wide ? 'modal--wide' : ''} ${fullscreen ? 'modal--fullscreen' : ''} ${jobLayout ? 'modal--job-layout' : ''}`} role="dialog" aria-modal="true" aria-labelledby={titleId}>
      <header className="modal__header"><div><h2 id={titleId}>{title}</h2>{description && <p>{description}</p>}</div><button type="button" className="icon-button" onClick={onClose} aria-label="Close dialog"><X size={18} /></button></header>
      <form onSubmit={(event) => { event.preventDefault(); onSubmit?.(event); }}><div className="modal__body">{children}</div>{onSubmit && <footer className="modal__footer"><Button type="button" variant="ghost" onClick={onClose}>Cancel</Button><Button type="submit" variant={submitVariant} loading={busy}>{submitLabel}</Button></footer>}</form>
    </section>
  </div>, document.body);
}
