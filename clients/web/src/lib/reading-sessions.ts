import { api } from '@/lib/api';

export interface ShelfReadingSessionInput {
  id: string;
  book_id: string;
  started_at: string;
  ended_at: string;
  start_page: number;
  end_page: number;
}

const pendingKey = 'idios.shelf.readingSessions.pending.v1';
let syncPromise: Promise<void> | null = null;

function readPending(): ShelfReadingSessionInput[] {
  try {
    const raw: unknown = JSON.parse(localStorage.getItem(pendingKey) ?? '[]');
    if (!Array.isArray(raw)) return [];
    return raw.filter((item): item is ShelfReadingSessionInput => Boolean(
      item && typeof item === 'object' && typeof item.id === 'string' && item.id.length <= 128
      && typeof item.book_id === 'string' && typeof item.started_at === 'string' && typeof item.ended_at === 'string'
      && Number.isFinite(Number(item.start_page)) && Number.isFinite(Number(item.end_page)),
    ));
  } catch { return []; }
}

function writePending(items: ShelfReadingSessionInput[]): boolean {
  try { localStorage.setItem(pendingKey, JSON.stringify(items.slice(-200))); return true; }
  catch { return false; }
}

export function queueShelfReadingSession(item: ShelfReadingSessionInput): boolean {
  const current = readPending().filter((candidate) => candidate.id !== item.id);
  current.push(item);
  return writePending(current);
}

/** Retry local outbox entries whenever a workspace page loads. IDs make retries idempotent. */
export function syncPendingShelfReadingSessions(): Promise<void> {
  if (syncPromise) return syncPromise;
  syncPromise = (async () => {
    const pending = readPending();
    if (!pending.length) return;
    const failed: ShelfReadingSessionInput[] = [];
    for (const session of pending) {
      try { await api.createShelfReadingSession(session); }
      catch { failed.push(session); }
    }
    writePending(failed);
  })().finally(() => { syncPromise = null; });
  return syncPromise;
}

export function newReadingSessionID(): string {
  try { return crypto.randomUUID(); }
  catch { return `session_${Date.now()}_${Math.random().toString(36).slice(2, 12)}`; }
}
