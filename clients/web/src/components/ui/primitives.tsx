import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from 'react';
import { LoaderCircle } from 'lucide-react';

export function Card({ className = '', children, ...props }: HTMLAttributes<HTMLElement>) {
  return <section className={`panel ${className}`} {...props}>{children}</section>;
}

export function Button({ className = '', variant = 'primary', loading = false, children, disabled, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'secondary' | 'ghost' | 'danger'; loading?: boolean }) {
  return <button className={`button button--${variant} ${className}`} disabled={disabled || loading} {...props}>
    {loading && <LoaderCircle className="spin" size={15} aria-hidden="true" />}{children}
  </button>;
}

export function Badge({ children, tone = 'neutral' }: { children: ReactNode; tone?: 'neutral' | 'green' | 'blue' | 'amber' | 'red' | 'violet' }) {
  return <span className={`badge badge--${tone}`}>{children}</span>;
}

export function PageHeading({ eyebrow, title, description, actions }: { eyebrow?: string; title: string; description?: string; actions?: ReactNode }) {
  return <div className="page-heading"><div className="page-heading__copy">{eyebrow && <p className="eyebrow">{eyebrow}</p>}<h1>{title}</h1>{description && <p className="page-description">{description}</p>}</div>{actions && <div className="page-heading__actions">{actions}</div>}</div>;
}

export function EmptyState({ title, body, icon }: { title: string; body: string; icon?: ReactNode }) {
  return <div className="empty-state"><div className="empty-state__icon">{icon}</div><h3>{title}</h3><p>{body}</p></div>;
}

export function LoadingState() {
  return <div className="loading-state"><LoaderCircle className="spin" size={20} /><span>Loading your workspace…</span></div>;
}

export function ErrorNotice({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return <div className="notice notice--error" role="alert"><div><strong>Couldn’t load this view</strong><p>{message}</p></div>{onRetry && <button className="text-button" onClick={onRetry}>Try again</button>}</div>;
}

export function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return <label className="field"><span className="field__label">{label}</span>{children}{hint && <span className="field__hint">{hint}</span>}</label>;
}
