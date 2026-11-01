import { useEffect, useId, useRef, type FormEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { Button } from './primitives';

type OpenModalEntry = { close: () => void };
const openModalStack: OpenModalEntry[] = [];

const focusableSelector = 'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

export function Modal({ open, title, description, onClose, onSubmit, submitLabel = 'Save', submitVariant = 'primary', busy, children, wide = false, fullscreen = false, jobLayout = false }: {
  open: boolean; title: string; description?: string; onClose: () => void; onSubmit?: (event: FormEvent<HTMLFormElement>) => void;
  submitLabel?: string; submitVariant?: 'primary' | 'secondary' | 'ghost' | 'danger'; busy?: boolean; children: ReactNode; wide?: boolean; fullscreen?: boolean; jobLayout?: boolean;
}) {
  const titleId = useId();
  const onCloseRef = useRef(onClose);
  const dialogRef = useRef<HTMLElement | null>(null);
  useEffect(() => { onCloseRef.current = onClose; }, [onClose]);
  useEffect(() => {
    if (!open) return;
    const previousOverflow = document.body.style.overflow;
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const entry: OpenModalEntry = { close: () => onCloseRef.current() };
    openModalStack.push(entry);
    document.body.style.overflow = 'hidden';

    const dialog = dialogRef.current;
    const autofocus = dialog?.querySelector<HTMLElement>('[autofocus]');
    const first = dialog?.querySelector<HTMLElement>(focusableSelector);
    (autofocus || first || dialog)?.focus();

    const keyboard = (event: KeyboardEvent) => {
      if (openModalStack[openModalStack.length - 1] !== entry) return;
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopImmediatePropagation();
        entry.close();
        return;
      }
      if (event.key !== 'Tab') return;
      const currentDialog = dialogRef.current;
      if (!currentDialog) return;
      const items = Array.from(currentDialog.querySelectorAll<HTMLElement>(focusableSelector))
        .filter((item) => item.getClientRects().length > 0);
      if (!items.length) {
        event.preventDefault();
        currentDialog.focus();
        return;
      }
      const firstItem = items[0];
      const lastItem = items[items.length - 1];
      if (event.shiftKey && (document.activeElement === firstItem || !currentDialog.contains(document.activeElement))) {
        event.preventDefault();
        lastItem.focus();
      } else if (!event.shiftKey && (document.activeElement === lastItem || !currentDialog.contains(document.activeElement))) {
        event.preventDefault();
        firstItem.focus();
      }
    };
    window.addEventListener('keydown', keyboard);
    return () => {
      window.removeEventListener('keydown', keyboard);
      const index = openModalStack.lastIndexOf(entry);
      if (index >= 0) openModalStack.splice(index, 1);
      document.body.style.overflow = previousOverflow;
      if (previousFocus?.isConnected) previousFocus.focus();
    };
  }, [open]);
  if (!open) return null;
  const modalClass = 'modal ' + (wide ? 'modal--wide ' : '') + (fullscreen ? 'modal--fullscreen ' : '') + (jobLayout ? 'modal--job-layout' : '');
  return createPortal(<div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.currentTarget === event.target) onClose(); }}>
    <section ref={dialogRef} className={modalClass} role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1}>
      <header className="modal__header"><div><h2 id={titleId}>{title}</h2>{description && <p>{description}</p>}</div><button type="button" className="icon-button" onClick={onClose} aria-label="Close dialog"><X size={18} /></button></header>
      <form onSubmit={(event) => { event.preventDefault(); onSubmit?.(event); }}><div className="modal__body">{children}</div>{onSubmit && <footer className="modal__footer"><Button type="button" variant="ghost" onClick={onClose}>Cancel</Button><Button type="submit" variant={submitVariant} loading={busy}>{submitLabel}</Button></footer>}</form>
    </section>
  </div>, document.body);
}
