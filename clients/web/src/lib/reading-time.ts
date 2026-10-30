import type { ShelfReadingSession } from '@/types/idios';

export interface ShelfReadingTotals {
  totalSeconds: number;
  weekSeconds: number;
  monthSeconds: number;
  sessions: number;
  books: number;
}

export function shelfReadingTotals(sessions: ShelfReadingSession[], now = new Date()): ShelfReadingTotals {
  const weekStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  weekStart.setDate(weekStart.getDate() - ((weekStart.getDay() + 6) % 7));
  const monthStart = new Date(now.getFullYear(), now.getMonth(), 1);
  let totalSeconds = 0;
  let weekSeconds = 0;
  let monthSeconds = 0;
  const bookIDs = new Set<string>();
  for (const session of sessions) {
    const seconds = Math.max(0, Number(session.duration_seconds) || 0);
    const started = new Date(session.started_at);
    totalSeconds += seconds;
    if (Number.isFinite(started.getTime()) && started >= weekStart) weekSeconds += seconds;
    if (Number.isFinite(started.getTime()) && started >= monthStart) monthSeconds += seconds;
    if (session.book_id) bookIDs.add(session.book_id);
  }
  return { totalSeconds, weekSeconds, monthSeconds, sessions: sessions.length, books: bookIDs.size };
}

export function formatReadingTime(seconds: number): string {
  const safe = Math.max(0, Math.floor(Number(seconds) || 0));
  if (safe < 60) return safe === 0 ? '0 min' : '<1 min';
  const hours = Math.floor(safe / 3600);
  const minutes = Math.floor((safe % 3600) / 60);
  const days = Math.floor(hours / 24);
  if (days) return `${days}d ${hours % 24}h`;
  return hours ? `${hours}h ${String(minutes).padStart(2, '0')}m` : `${minutes} min`;
}

export function readingSecondsByBook(sessions: ShelfReadingSession[]): Map<string, number> {
  const totals = new Map<string, number>();
  for (const session of sessions) totals.set(session.book_id, (totals.get(session.book_id) ?? 0) + Math.max(0, Number(session.duration_seconds) || 0));
  return totals;
}
