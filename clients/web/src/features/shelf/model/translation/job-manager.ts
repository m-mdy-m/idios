export type TranslationJobStatus = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled';
export type TranslationJobSnapshot = {
  id: string;
  bookID: string;
  pageNumber: number;
  status: TranslationJobStatus;
  completedParts: number;
  totalParts: number;
  error?: string;
  updatedAt: number;
};

type JobRecord = TranslationJobSnapshot & { controller: AbortController };
type Listener = (jobs: TranslationJobSnapshot[]) => void;

const MAX_RETAINED_JOBS = 40;
const TERMINAL = new Set<TranslationJobStatus>(['completed', 'failed', 'cancelled']);

/**
 * Reader-scoped translation job registry.
 * Panel visibility is intentionally not represented here: hiding the panel cannot cancel work.
 */
class TranslationJobManager {
  private jobs = new Map<string, JobRecord>();
  private listeners = new Set<Listener>();

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    listener(this.list());
    return () => this.listeners.delete(listener);
  }

  private toSnapshot(job: JobRecord): TranslationJobSnapshot {
    return {
      id: job.id,
      bookID: job.bookID,
      pageNumber: job.pageNumber,
      status: job.status,
      completedParts: job.completedParts,
      totalParts: job.totalParts,
      ...(job.error ? { error: job.error } : {}),
      updatedAt: job.updatedAt,
    };
  }

  list(): TranslationJobSnapshot[] {
    return [...this.jobs.values()]
      .map((job) => this.toSnapshot(job))
      .sort((a, b) => b.updatedAt - a.updatedAt);
  }

  get(id: string): TranslationJobSnapshot | undefined {
    const job = this.jobs.get(id);
    return job ? this.toSnapshot(job) : undefined;
  }

  start(input: { id: string; bookID: string; pageNumber: number; totalParts: number }): AbortSignal {
    const previous = this.jobs.get(input.id);
    if (previous && !TERMINAL.has(previous.status)) {
      throw new Error('A translation job with this ID is already active.');
    }
    this.pruneTerminalJobs();
    const record: JobRecord = {
      ...input,
      status: 'running',
      completedParts: 0,
      updatedAt: Date.now(),
      controller: new AbortController(),
    };
    this.jobs.set(input.id, record);
    this.emit();
    return record.controller.signal;
  }

  progress(id: string, completedParts: number, totalParts?: number): void {
    const job = this.jobs.get(id);
    if (!job || job.status !== 'running') return;
    this.jobs.set(id, {
      ...job,
      completedParts: Math.max(0, Math.min(totalParts ?? job.totalParts, Math.floor(completedParts))),
      totalParts: Math.max(0, Math.floor(totalParts ?? job.totalParts)),
      updatedAt: Date.now(),
    });
    this.emit();
  }

  complete(id: string): void {
    const job = this.jobs.get(id);
    if (!job || job.status !== 'running') return;
    this.jobs.set(id, { ...job, status: 'completed', completedParts: job.totalParts, updatedAt: Date.now() });
    this.emit();
  }

  fail(id: string, error: string): void {
    const job = this.jobs.get(id);
    if (!job || job.status !== 'running') return;
    this.jobs.set(id, { ...job, status: 'failed', error, updatedAt: Date.now() });
    this.emit();
  }

  /** Explicit user Stop action. Panel close/hide must never call this method. */
  cancel(id: string): void {
    const job = this.jobs.get(id);
    if (!job || TERMINAL.has(job.status)) return;
    job.controller.abort(new DOMException('Translation cancelled by the reader.', 'AbortError'));
    this.jobs.set(id, { ...job, status: 'cancelled', updatedAt: Date.now() });
    this.emit();
  }

  private pruneTerminalJobs(): void {
    const terminal = [...this.jobs.entries()]
      .filter(([, job]) => TERMINAL.has(job.status))
      .sort((left, right) => left[1].updatedAt - right[1].updatedAt);
    while (this.jobs.size >= MAX_RETAINED_JOBS && terminal.length) {
      const oldest = terminal.shift();
      if (oldest) this.jobs.delete(oldest[0]);
    }
  }

  private emit(): void {
    const snapshot = this.list();
    for (const listener of this.listeners) {
      try { listener(snapshot); } catch { /* A UI subscriber must not break job execution. */ }
    }
  }
}

export const translationJobs = new TranslationJobManager();
