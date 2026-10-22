import { useEffect, useMemo, useState } from 'react';
import { Activity, ArrowRight, BookOpen, Check, ChevronLeft, ChevronRight, CircleHelp, Copy, Pause, Play, Quote as QuoteIcon } from 'lucide-react';
import { Link } from 'react-router';
import { useWorkspaceStore } from '@/store/workspace-store';
import { ErrorNotice, LoadingState } from '@/components/ui/primitives';
import { formatRelative } from '@/components/ui/IssueTable';
import { activityIconFor, activitySummary, friendlyAction } from '@/components/ActivityList';
import { api } from '@/lib/api';
import type { ActivityEvent, Quote } from '@/types/idios';

export function DashboardPage() {
  const { dashboard, state, error, load } = useWorkspaceStore();
  const [quotes, setQuotes] = useState<Quote[]>([]);
  const [quotesLoading, setQuotesLoading] = useState(true);
  const [activeQuoteIndex, setActiveQuoteIndex] = useState(0);
  const [quotePaused, setQuotePaused] = useState(false);
  const [quoteCopyNotice, setQuoteCopyNotice] = useState('');
  const [activeNoteIndex, setActiveNoteIndex] = useState(0);
  const [notesPaused, setNotesPaused] = useState(false);
  const [viewport, setViewport] = useState(() => ({
    width: typeof window === 'undefined' ? 1280 : window.innerWidth,
    height: typeof window === 'undefined' ? 720 : window.innerHeight,
  }));
  useEffect(() => {
    let active = true;
    void api.quotes().then((items) => {
      if (!active) return;
      const ordered = [...items].sort((a, b) => safeTime(b.created_at) - safeTime(a.created_at));
      setQuotes(ordered);
      setActiveQuoteIndex(0);
    }).catch(() => undefined).finally(() => { if (active) setQuotesLoading(false); });
    return () => { active = false; };
  }, []);
  useEffect(() => {
    if (quotePaused || quotes.length < 2 || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const timer = window.setInterval(() => setActiveQuoteIndex((index) => (index + 1) % quotes.length), 12000);
    return () => window.clearInterval(timer);
  }, [quotePaused, quotes.length, activeQuoteIndex]);
  useEffect(() => {
    const updateViewport = () => setViewport({ width: window.innerWidth, height: window.innerHeight });
    updateViewport();
    window.addEventListener('resize', updateViewport, { passive: true });
    return () => window.removeEventListener('resize', updateViewport);
  }, []);
  const activeQuote = quotes.length ? quotes[activeQuoteIndex % quotes.length] : null;
  const recentNotes = dashboard?.notes.slice(0, 6) ?? [];
  const activeNote = recentNotes.length ? recentNotes[activeNoteIndex % recentNotes.length] : null;
  useEffect(() => {
    if (notesPaused || recentNotes.length < 2 || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const timer = window.setInterval(() => setActiveNoteIndex((index) => (index + 1) % recentNotes.length), 8500);
    return () => window.clearInterval(timer);
  }, [notesPaused, recentNotes.length]);
  const orderedActivity = useMemo(() => [...(dashboard?.activity ?? [])]
    .sort((a, b) => safeTime(b.created_at) - safeTime(a.created_at)), [dashboard?.activity]);
  const recentActivity = useMemo(() => orderedActivity.slice(0, getOverviewActivityLimit(viewport.width, viewport.height, orderedActivity)), [orderedActivity, viewport.width, viewport.height]);
  const activityGroups = useMemo(() => groupActivityByDay(recentActivity), [recentActivity]);

  if (!dashboard && state === 'loading') return <LoadingState />;
  if (!dashboard && state === 'error') return <div className="page-stack"><ErrorNotice message={error || 'The local server is not responding.'} onRetry={() => void load()} /><div className="server-help"><CircleHelp size={18} /><div><strong>Is the IDIOS server running?</strong><p>Run <code>idios web</code> from a terminal to start the local server and open this workspace.</p></div></div></div>;
  if (!dashboard) return null;


  async function copyActiveQuote() {
    if (!activeQuote) return;
    try {
      await navigator.clipboard.writeText(`“${activeQuote.text}”${activeQuote.author ? ` — ${activeQuote.author}` : ''}`);
      setQuoteCopyNotice('Copied');
      window.setTimeout(() => setQuoteCopyNotice(''), 1800);
    } catch {
      setQuoteCopyNotice('Clipboard unavailable');
      window.setTimeout(() => setQuoteCopyNotice(''), 2200);
    }
  }

  function previousQuote() {
    if (quotes.length > 1) setActiveQuoteIndex((index) => (index - 1 + quotes.length) % quotes.length);
  }

  function advanceQuote() {
    if (quotes.length > 1) setActiveQuoteIndex((index) => (index + 1) % quotes.length);
  }

  return <div className="page-stack dashboard-page">
    {error && <div className="notice notice--warning"><span>Refresh failed. Showing the latest available data: {error}</span></div>}

    <div className="dashboard-overview-layout">
      <div className="dashboard-overview-top">
        <section className="overview-quote-minimal overview-quote-atmosphere" aria-label="Quotes">
          <div className="overview-quote-minimal__header">
            <div className="overview-quote-minimal__label"><span className="overview-quote-minimal__mark"><QuoteIcon size={15} /></span><span>Quotes</span></div>
          </div>
          {activeQuote ? <div className="overview-quote-minimal__body" key={activeQuote.id} aria-live="polite" aria-atomic="true">
            <blockquote>{activeQuote.text}</blockquote>
            {(activeQuote.author || activeQuote.source || activeQuote.category || quoteCopyNotice) && <div className="overview-quote-minimal__attribution">{activeQuote.author && <><span className="overview-quote-minimal__dash" /><strong>{activeQuote.author}</strong></>}{(activeQuote.source || activeQuote.category) && <><span className="overview-quote-minimal__separator">·</span><span className="overview-quote-minimal__source">{activeQuote.source || activeQuote.category}</span></>}{quoteCopyNotice && <span className="overview-quote-minimal__notice">{quoteCopyNotice}</span>}</div>}
          </div> : quotesLoading ? <div className="overview-quote-minimal__empty">Collecting your saved quotes…</div> : <div className="overview-quote-minimal__empty"><span>No quotes saved yet.</span><Link to="/quotes">Add a quote <ArrowRight size={12} /></Link></div>}
          <div className="overview-quote-minimal__footer">
            <div className="overview-quote-minimal__footer-controls" role="group" aria-label="Quote controls">
              {quotes.length > 1 && <button type="button" className="overview-quote-minimal__button" onClick={previousQuote} aria-label="Previous quote" title="Previous quote"><ChevronLeft size={14} /></button>}
              {quotes.length > 1 && <button type="button" className="overview-quote-minimal__button" onClick={() => setQuotePaused((value) => !value)} aria-label={quotePaused ? 'Resume quote rotation' : 'Pause quote rotation'} title={quotePaused ? 'Resume rotation' : 'Pause rotation'}>{quotePaused ? <Play size={13} /> : <Pause size={13} />}</button>}
              {quotes.length > 1 && <button type="button" className="overview-quote-minimal__button" onClick={advanceQuote} aria-label="Next quote" title="Next quote"><ChevronRight size={14} /></button>}
              {activeQuote && <button type="button" className="overview-quote-minimal__button" onClick={() => void copyActiveQuote()} aria-label="Copy quote" title="Copy quote">{quoteCopyNotice === 'Copied' ? <Check size={13} /> : <Copy size={13} />}</button>}
            </div>
            <span className="overview-quote-minimal__count">{quotes.length ? `${String(activeQuoteIndex + 1).padStart(2, '0')} / ${String(quotes.length).padStart(2, '0')}` : '—'}</span>
          </div>
        </section>
        <div className="overview-side-stack">
          <section className="overview-note-spotlight panel overview-note-atmosphere" aria-labelledby="overview-notes-title">
            <div className="overview-note-spotlight__heading">
              <div className="overview-note-spotlight__heading-title"><BookOpen size={15} /><h2 id="overview-notes-title">Notes</h2></div>
              <div className="overview-note-spotlight__heading-meta"><span>{recentNotes.length ? `${String((activeNoteIndex % recentNotes.length) + 1).padStart(2, '0')} / ${String(recentNotes.length).padStart(2, '0')}` : '—'}</span><Link to="/notes">View all <ArrowRight size={12} /></Link></div>
            </div>
            {activeNote ? <Link to="/notes" className="overview-note-spotlight__current" key={activeNote.id} aria-live="polite">
              <span className="overview-note-spotlight__icon"><BookOpen size={15} /></span>
              <span className="overview-note-spotlight__content"><strong>{activeNote.title || 'Untitled note'}</strong><span>{activeNote.content?.trim() || 'No content yet.'}</span>{activeNote.labels?.length ? <span className="overview-note-spotlight__tags">{activeNote.labels.slice(0, 3).map((label) => <span className="overview-note-spotlight__tag" key={label.id}>{label.name}</span>)}</span> : null}<small>{formatRelative(activeNote.updated_at)}</small></span>
              <ArrowRight className="overview-note-spotlight__open" size={14} />
            </Link> : <div className="overview-note-spotlight__empty"><BookOpen size={17} /><span>No notes yet</span><Link to="/notes">Create a note <ArrowRight size={11} /></Link></div>}
            <div className="overview-note-spotlight__footer">
              <div className="overview-note-spotlight__controls" role="group" aria-label="Note controls">
                {recentNotes.length > 1 && <button type="button" onClick={() => setActiveNoteIndex((index) => (index - 1 + recentNotes.length) % recentNotes.length)} aria-label="Previous note" title="Previous note"><ChevronLeft size={14} /></button>}
                {recentNotes.length > 1 && <button type="button" onClick={() => setNotesPaused((value) => !value)} aria-label={notesPaused ? 'Resume note rotation' : 'Pause note rotation'} title={notesPaused ? 'Resume rotation' : 'Pause rotation'}>{notesPaused ? <Play size={13} /> : <Pause size={13} />}</button>}
                {recentNotes.length > 1 && <button type="button" onClick={() => setActiveNoteIndex((index) => (index + 1) % recentNotes.length)} aria-label="Next note" title="Next note"><ChevronRight size={14} /></button>}
              </div>
              <span>{activeNote ? (!notesPaused && recentNotes.length > 1 ? 'ROTATING NOTES' : 'NOTE PREVIEW') : 'EMPTY LIBRARY'}</span>
            </div>
          </section>


        </div>
      </div>
      <section className="overview-activity overview-activity-tree panel" aria-labelledby="overview-activity-title">
        <div className="overview-tree-heading">
          <div className="overview-tree-heading__title"><span className="overview-tree-heading__icon"><Activity size={16} /></span><span><h2 id="overview-activity-title">Latest changes</h2><p>Recent activity across your workspace.</p></span></div>
          <Link className="subtle-link" to="/activity">View all <ArrowRight size={13} /></Link>
        </div>
        {activityGroups.length ? <div className="overview-timeline">
          {activityGroups.map((group) => <section className="overview-timeline__day" key={group.key} aria-label={group.label}>
            <div className="overview-timeline__day-heading"><span className="overview-timeline__day-dot" /><strong>{group.label}</strong><span>{group.items.length} {group.items.length === 1 ? 'event' : 'events'}</span></div>
            <div className="overview-timeline__events">
              {group.items.map((item) => {
                const Icon = activityIconFor(item.entity_type);
                const typeLabel = friendlyAction(item.entity_type.replaceAll('_', ' '));
                return <Link className="overview-timeline__event" to="/activity" key={item.id} title="Open activity history">
                  <span className="overview-timeline__event-icon"><Icon size={12} /></span>
                  <span className="overview-timeline__event-copy"><span className="overview-timeline__event-top"><strong>{friendlyAction(item.action)}</strong><time dateTime={item.created_at} title={new Date(item.created_at).toLocaleString()}>{formatRelative(item.created_at)}</time></span><span className="overview-timeline__event-summary">{activitySummary(item)}</span><span className="overview-timeline__event-type">{typeLabel}</span></span>
                  <ArrowRight className="overview-timeline__event-open" size={12} />
                </Link>;
              })}
            </div>
          </section>)}
        </div> : <div className="overview-activity-empty"><Activity size={17} /><div><strong>No recent changes</strong><p>Your workspace timeline will appear here as you work.</p></div></div>}
      </section>
    </div>
  </div>;
}

function safeTime(value: string): number {
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : 0;
}


function groupActivityByDay(items: ActivityEvent[]): Array<{ key: string; label: string; items: ActivityEvent[] }> {
  const groups = new Map<string, { key: string; label: string; items: ActivityEvent[] }>();
  for (const item of items) {
    const date = new Date(item.created_at);
    if (!Number.isFinite(date.getTime())) continue;
    const key = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
    if (!groups.has(key)) groups.set(key, { key, label: activityDayLabel(date), items: [] });
    groups.get(key)?.items.push(item);
  }
  return [...groups.values()];
}

function activityDayLabel(date: Date): string {
  const today = new Date();
  const target = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
  const current = new Date(today.getFullYear(), today.getMonth(), today.getDate()).getTime();
  const offset = Math.round((current - target) / 86_400_000);
  if (offset === 0) return 'Today';
  if (offset === 1) return 'Yesterday';
  if (offset === -1) return 'Tomorrow';
  return date.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
}

/** Choose enough timeline rows to use a desktop viewport without stretching an empty card. */
function getOverviewActivityLimit(width: number, height: number, items: ActivityEvent[]): number {
  if (items.length <= 7) return items.length;
  if (width < 1050) return 7;

  const topHeight = height <= 520 ? 142 : Math.min(210, Math.max(164, Math.round(height * 0.27)));
  const bodyBudget = Math.max(0, height - 46 - 15 - 12 - topHeight - 13 - 50 - 10 - 14);
  const rowCost = 30;
  const dayHeadingCost = 21;
  let used = 0;
  let count = 0;
  let previousDay = '';

  for (const item of items) {
    const date = new Date(item.created_at);
    if (!Number.isFinite(date.getTime())) continue;
    const day = `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`;
    const headingCost = day === previousDay ? 0 : dayHeadingCost;
    if (count >= 7 && used + headingCost + rowCost > bodyBudget) break;
    used += headingCost + rowCost;
    count += 1;
    previousDay = day;
  }

  return Math.min(items.length, Math.max(Math.min(7, items.length), count));
}
