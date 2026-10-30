import type { ButtonHTMLAttributes, CSSProperties, HTMLAttributes, ReactNode } from 'react';
import { LoaderCircle } from 'lucide-react';
import styles from './primitives.module.css';
import layout from './layout.module.css';

type Tone = 'neutral' | 'green' | 'blue' | 'amber' | 'red' | 'violet';
type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

/** A low-gloss, bordered surface. Use for independent pieces of information, not as a wrapper for every div. */
export function Tile({ className = '', interactive = false, children, ...props }: HTMLAttributes<HTMLElement> & { interactive?: boolean; children?: ReactNode }) {
  return <section className={`${styles.tile} panel ${className}`.trim()} data-interactive={interactive || undefined} {...props}>{children}</section>;
}

/** Backwards-compatible alias while features migrate from the legacy Card name. */
export const Card = Tile;

export function Button({ className = '', variant = 'primary', loading = false, children, disabled, type = 'button', ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant; loading?: boolean }) {
  const variantClass = variant === 'secondary' ? '' : variant === 'ghost' ? styles.ghost : variant === 'danger' ? styles.danger : styles.primary;
  return <button className={`${styles.button} button button--${variant} ${variantClass} ${className}`.trim()} disabled={disabled || loading} type={type} {...props}>
    {loading && <LoaderCircle className="spin" size={15} aria-hidden="true" />}{children}
  </button>;
}

export function IconButton(props: ButtonHTMLAttributes<HTMLButtonElement> & { label: string }) {
  const { label, className = '', children, ...rest } = props;
  return <button aria-label={label} title={label} className={`${styles.button} icon-button ${className}`.trim()} type="button" {...rest}>{children}</button>;
}

export function Badge({ children, tone = 'neutral', className = '' }: { children: ReactNode; tone?: Tone; className?: string }) {
  return <span className={`${styles.badge} badge badge--${tone} ${className}`.trim()}>{children}</span>;
}

export function PageHeading({ eyebrow, title, description, actions }: { eyebrow?: string; title: string; description?: string; actions?: ReactNode }) {
  return <header className={`page-heading ${styles.heading}`}><div className={`page-heading__copy ${styles.headingCopy}`}>{eyebrow && <p className={`eyebrow ${styles.eyebrow}`}>{eyebrow}</p>}<h1 className={styles.title}>{title}</h1>{description && <p className={`page-description ${styles.description}`}>{description}</p>}</div>{actions && <div className={`page-heading__actions ${styles.actions}`}>{actions}</div>}</header>;
}

export function EmptyState({ title, body, icon }: { title: string; body: string; icon?: ReactNode }) {
  return <div className={`empty-state ${styles.state}`}><div className="empty-state__icon">{icon}</div><h3>{title}</h3><p>{body}</p></div>;
}

export function LoadingState({ label = 'Loading your workspace…' }: { label?: string } = {}) {
  return <div className="loading-state"><LoaderCircle className="spin" size={20} /><span>{label}</span></div>;
}

export function ErrorNotice({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return <div className={`notice notice--error ${styles.state} ${styles.error}`} role="alert"><div><strong>Couldn’t load this view</strong><p>{message}</p></div>{onRetry && <button className="text-button" onClick={onRetry} type="button">Try again</button>}</div>;
}

export function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return <label className={`field ${styles.field}`}><span className={`field__label ${styles.fieldLabel}`}>{label}</span>{children}{hint && <span className={`field__hint ${styles.fieldHint}`}>{hint}</span>}</label>;
}

export function Box({ className = '', ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={`${layout.box} ${className}`.trim()} {...props} />;
}
export function Stack({ className = '', gap = 12, ...props }: HTMLAttributes<HTMLDivElement> & { gap?: number }) {
  return <div className={`${layout.stack} ${className}`.trim()} style={{ ...props.style, '--stack-gap': `${gap}px` } as CSSProperties} {...props} />;
}
export function Inline({ className = '', gap = 8, ...props }: HTMLAttributes<HTMLDivElement> & { gap?: number }) {
  return <div className={`${layout.inline} ${className}`.trim()} style={{ ...props.style, '--inline-gap': `${gap}px` } as CSSProperties} {...props} />;
}
export function Text({ className = '', tone = 'default', mono = false, ...props }: HTMLAttributes<HTMLSpanElement> & { tone?: 'default' | 'muted'; mono?: boolean }) {
  return <span className={`${layout.text} ${tone === 'muted' ? layout.muted : ''} ${mono ? layout.mono : ''} ${className}`.trim()} {...props} />;
}
export function Divider({ className = '', ...props }: HTMLAttributes<HTMLHRElement>) {
  return <hr className={`${styles.divider} ${className}`.trim()} {...props} />;
}
