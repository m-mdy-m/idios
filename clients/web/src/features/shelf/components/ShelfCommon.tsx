import { useMemo, type CSSProperties } from 'react';
import { BookOpen } from 'lucide-react';
import type { ShelfBook } from '@/types/idios';

export const relationOptions = [
  { value: 'related_to', label: 'Related to', description: 'Shares a topic or idea' },
  { value: 'prerequisite_for', label: 'Prerequisite for', description: 'Read this book first' },
  { value: 'contrasts_with', label: 'Contrasts with', description: 'Different approach or conclusion' },
  { value: 'implements', label: 'Implements', description: 'Builds or applies an idea from' },
  { value: 'references', label: 'References', description: 'Cites or points toward' },
  { value: 'uses', label: 'Uses', description: 'Uses concepts from' },
] as const;
export const statusOptions = [
  { value: 'unread', label: 'To read' },
  { value: 'reading', label: 'Reading now' },
  { value: 'finished', label: 'Finished' },
] as const;
export const statusLabel = (status: string) => statusOptions.find((item) => item.value === status)?.label ?? status;
export const relationLabel = (type: string) => relationOptions.find((item) => item.value === type)?.label ?? type.replaceAll('_', ' ');
export const parseTags = (value: string) => [...new Map(value.split(',').map((tag) => tag.trim().replace(/^#/, '')).filter(Boolean).map((tag) => [tag.toLowerCase(), tag])).values()];
export function formatShelfDate(value: string) {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}

export function BookCover({ book, small = false }: { book: Pick<ShelfBook, 'id' | 'title'>; small?: boolean }) {
  const style = useMemo(() => {
    let hash = 0;
    for (const char of book.id) hash = (Math.imul(hash, 31) + char.charCodeAt(0)) | 0;
    return { '--cover-hue': Math.abs(hash) % 360 } as CSSProperties;
  }, [book.id]);
  return <div className={`shelf-book-cover${small ? ' shelf-book-cover--small' : ''}`} style={style} aria-hidden="true">
    <div className="shelf-book-cover__spine" /><div className="shelf-book-cover__ornament"><span /><span /><span /></div>
    <div className="shelf-book-cover__title">{book.title}</div><div className="shelf-book-cover__mark"><BookOpen size={small ? 15 : 20} /></div>
  </div>;
}

export function ReadingBadge({ status }: { status: string }) {
  const kind = status === 'finished' ? 'finished' : status === 'reading' ? 'reading' : 'unread';
  return <span className={`shelf-status shelf-status--${kind}`}><span />{statusLabel(status)}</span>;
}
