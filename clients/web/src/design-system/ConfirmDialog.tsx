import type { ReactNode } from 'react';
import { Modal } from './Modal';

type Props = {
  open: boolean;
  title: string;
  description?: string;
  children?: ReactNode;
  confirmLabel?: string;
  confirmVariant?: 'primary' | 'secondary' | 'ghost' | 'danger';
  busy?: boolean;
  onConfirm: () => void;
  onClose: () => void;
};

/** Shared confirmation dialog for destructive or irreversible actions. */
export function ConfirmDialog({ open, title, description, children, confirmLabel = 'Confirm', confirmVariant = 'danger', busy = false, onConfirm, onClose }: Props) {
  return <Modal open={open} title={title} description={description} onClose={onClose} onSubmit={() => onConfirm()} submitLabel={confirmLabel} submitVariant={confirmVariant} busy={busy}>
    {children}
  </Modal>;
}
