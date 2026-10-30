import { Activity, BookOpen, CalendarClock, CheckSquare, FolderKanban, GitBranch, GitPullRequest, Quote, Tags, Terminal } from 'lucide-react';
import type { ActivityEvent } from '@/types/idios';
import { EmptyState } from '@/design-system';
import { formatRelative } from '@/lib/format-relative';
import type { LucideIcon } from 'lucide-react';

const entityIcons: Record<string, LucideIcon> = {
  issue: CheckSquare,
  repository: GitBranch,
  project: FolderKanban,
  note: BookOpen,
  shelf_book: BookOpen,
  quote: Quote,
  job: CalendarClock,
  milestone: CalendarClock,
  label: Tags,
  pull_request: GitPullRequest,
  command: Terminal,
};

export function ActivityList({ items }: { items: ActivityEvent[] }) {
  if (!items.length) return <EmptyState title="No activity yet" body="Meaningful changes will appear here as you use IDIOS." icon={<Activity size={20} />} />;

  const sorted = [...items].sort((a, b) => safeTime(b.created_at) - safeTime(a.created_at));
  const groups = sorted.reduce<Array<{ key: string; label: string; events: ActivityEvent[] }>>((all, item) => {
    const date = new Date(item.created_at);
    const key = Number.isFinite(date.getTime()) ? date.toDateString() : 'unknown';
    let group = all[all.length - 1];
    if (!group || group.key !== key) {
      group = { key, label: dayLabel(date), events: [] };
      all.push(group);
    }
    group.events.push(item);
    return all;
  }, []);

  return <div className="activity-list">
    {groups.map((group) => <section className="activity-day" key={group.key}>
      <h3 className="activity-day__heading">{group.label}<span>{group.events.length}</span></h3>
      <div className="activity-day__events">{group.events.map((item) => {
        const Icon = activityIconFor(item.entity_type);
        const typeLabel = item.entity_type.replaceAll('_', ' ').replace(/\b\w/g, (value) => value.toUpperCase());
        const timestamp = new Date(item.created_at);
        return <article className="activity-item" key={item.id}>
          <span className={`activity-item__marker activity-item__marker--${item.entity_type.toLowerCase().replace(/[^a-z0-9]+/g, '-')}`}><Icon size={14} /></span>
          <div className="activity-item__main">
            <div className="activity-item__title"><strong>{friendlyAction(item.action)}</strong><span className="activity-item__type">{typeLabel}</span></div>
            <p className="activity-item__summary">{activitySummary(item)}</p>
          </div>
          <div className="activity-item__meta"><time dateTime={item.created_at} title={timestamp.toLocaleString()}>{formatRelative(item.created_at)}</time><span>{Number.isFinite(timestamp.getTime()) ? timestamp.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' }) : 'Time unavailable'}</span></div>
        </article>;
      })}</div>
    </section>)}
  </div>;
}

function safeTime(value: string) {
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function dayLabel(date: Date) {
  if (!Number.isFinite(date.getTime())) return 'Date unavailable';
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const target = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
  const diff = Math.round((today - target) / 86_400_000);
  if (diff === 0) return 'Today';
  if (diff === 1) return 'Yesterday';
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: date.getFullYear() === now.getFullYear() ? undefined : 'numeric' }).format(date);
}

export function activityIconFor(entityType: string): LucideIcon {
  return entityIcons[entityType.toLowerCase().replaceAll('-', '_')] ?? Activity;
}

export function friendlyAction(action: string) {
  if (action === 'shelf.session_completed') return 'Completed a reading session';
  return action.replace(/[._-]+/g, ' ').replace(/\b\w/g, (value) => value.toUpperCase());
}


export function activitySummary(item: ActivityEvent): string {
  const fallback = `${item.entity_type.replaceAll('_', ' ')} updated`;
  if (!item.payload) return fallback;
  try {
    const parsed: unknown = JSON.parse(item.payload);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return fallback;
    const data = parsed as Record<string, unknown>;
    const text = (key: string) => typeof data[key] === 'string' ? String(data[key]).trim() : '';
    if (item.action === 'shelf.session_completed') {
      const book = text('book_title') || 'Book';
      const seconds = Math.max(0, Number(data.duration_seconds) || 0);
      const hours = Math.floor(seconds / 3600); const minutes = Math.floor((seconds % 3600) / 60);
      const duration = hours ? `${hours}h ${minutes}m` : minutes ? `${minutes} min` : '<1 min';
      const startPage = Math.max(0, Number(data.start_page) || 0); const endPage = Math.max(0, Number(data.end_page) || 0);
      return `${book} · ${duration}${startPage && endPage ? ` · pages ${startPage}–${endPage}` : ''}`;
    }
    const title = text('title') || text('name') || text('summary') || text('description') || text('branch') || text('default_branch');
    const repo = text('repository') || text('repository_name');
    const number = typeof data.number === 'number' || typeof data.number === 'string' ? String(data.number) : '';
    const reference = repo && number ? `${repo}#${number}` : repo;
    if (title && reference && !title.toLowerCase().includes(reference.toLowerCase())) return `${reference} · ${title}`;
    if (title) return title.length > 180 ? `${title.slice(0, 177)}…` : title;
    if (reference) return reference;
    if (typeof data.schedule_type === 'string') return `Schedule: ${data.schedule_type}`;
    if (typeof data.status === 'string') return `Status: ${data.status.replaceAll('_', ' ')}`;
    return fallback;
  } catch {
    // Some older rows may contain plain text rather than JSON.
    const plain = item.payload.trim();
    if (!plain || /[0-9a-f]{8}-[0-9a-f-]{27,}/i.test(plain)) return fallback;
    return plain.length > 180 ? `${plain.slice(0, 177)}…` : plain;
  }
}
