import { useEffect, useMemo, useState } from 'react';
import {
  Activity, ArrowRight, BookOpen, Blocks, CalendarDays, CheckCircle2, CircleDot,
  Clock3, FolderKanban, GitPullRequest, LibraryBig, Network, ScrollText, Settings2,
  Tags, Trash2, Workflow,
} from 'lucide-react';
import { Link } from 'react-router';
import { Badge, Button, ErrorNotice, Inline, LoadingState, PageHeading, Tile } from '@/design-system';
import { useWorkspaceStore } from '@/store/workspace-store';
import { api } from '@/lib/api';
import { formatRelative } from '@/lib/format-relative';
import { formatReadingTime, shelfReadingTotals } from '@/lib/reading-time';
import { syncPendingShelfReadingSessions } from '@/lib/reading-sessions';
import { activityIconFor, activitySummary, friendlyAction } from '@/features/activity';
import type { ActivityEvent, ShelfReadingSession } from '@/types/idios';
import './DashboardPage.css';

const modules = [
  { to: '/repositories', label: 'Repositories', detail: 'Sources and branches', icon: Blocks, group: 'Build' },
  { to: '/pull-requests', label: 'Pull requests', detail: 'Review changes', icon: GitPullRequest, group: 'Build' },
  { to: '/issues', label: 'Issues', detail: 'Work and follow-ups', icon: CircleDot, group: 'Build' },
  { to: '/projects', label: 'Projects', detail: 'Group related work', icon: FolderKanban, group: 'Build' },
  { to: '/shelf', label: 'Shelf', detail: 'Books and reading', icon: LibraryBig, group: 'Learn' },
  { to: '/notes', label: 'Notes', detail: 'Your knowledge base', icon: BookOpen, group: 'Learn' },
  { to: '/graph', label: 'Knowledge graph', detail: 'Explore connections', icon: Network, group: 'Learn' },
  { to: '/plans', label: 'Daily plans', detail: 'Decide what matters', icon: CalendarDays, group: 'Organize' },
  { to: '/planning', label: 'Labels & milestones', detail: 'Shape the roadmap', icon: Tags, group: 'Organize' },
  { to: '/jobs', label: 'Scheduled jobs', detail: 'Automate routine work', icon: Workflow, group: 'Operate' },
  { to: '/logs', label: 'Logs', detail: 'Inspect local activity', icon: ScrollText, group: 'Operate' },
  { to: '/remove', label: 'Remove items', detail: 'Clean up old data', icon: Trash2, group: 'Operate' },
] as const;

export function DashboardPage() {
  const { dashboard, health, state, error, load } = useWorkspaceStore();
  const [readingSessions, setReadingSessions] = useState<ShelfReadingSession[]>([]);
  const readingSummary = useMemo(() => shelfReadingTotals(readingSessions), [readingSessions]);
  const recentActivity = useMemo(() => [...(dashboard?.activity ?? [])]
    .sort((a, b) => safeTime(b.created_at) - safeTime(a.created_at))
    .slice(0, 6), [dashboard?.activity]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    let active = true;
    void syncPendingShelfReadingSessions()
      .then(() => api.shelfReadingSessions())
      .then((sessions) => { if (active) setReadingSessions(sessions); })
      .catch(() => undefined);
    return () => { active = false; };
  }, []);

  if (!dashboard && state === 'loading') return <LoadingState label="Starting local workspace…" />;
  if (!dashboard && state === 'error') return <div className="page-stack"><ErrorNotice message={error || 'The local server is not responding.'} onRetry={() => void load()} /><Tile className="dashboard-server-help"><strong>Start the local engine</strong><p>Run <code>idios web</code> from a terminal, then refresh this page.</p></Tile></div>;
  if (!dashboard) return null;

  const metrics = [
    { label: 'Repositories', value: dashboard.metrics.repositories, detail: 'Connected sources', to: '/repositories', icon: Blocks },
    { label: 'Open issues', value: dashboard.metrics.open_issues, detail: `${dashboard.metrics.in_progress_issues} in progress`, to: '/issues', icon: CircleDot },
    { label: 'Projects', value: dashboard.metrics.projects, detail: 'Active workspaces', to: '/projects', icon: FolderKanban },
    { label: 'Notes', value: dashboard.metrics.notes, detail: 'Saved knowledge', to: '/notes', icon: BookOpen },
  ];
  const groups = ['Build', 'Learn', 'Organize', 'Operate'] as const;

  return <div className="page-stack dashboard-minimal">
    <PageHeading
      eyebrow="IDIOS / LOCAL WORKBENCH"
      title="A calm place to build and learn."
      description="One small workspace for your code, reading, and ideas. Choose a module and stay focused on one thing at a time."
      actions={<Inline gap={7}><Badge tone={health ? 'green' : 'amber'}><span className={`dashboard-status-dot ${health ? 'is-online' : ''}`} />{health ? 'Local engine ready' : 'Engine offline'}</Badge><Button variant="secondary" onClick={() => void load()}>Refresh</Button></Inline>}
    />

    {error && <div className="notice notice--warning" role="status">Showing the latest local data: {error}</div>}

    <section className="dashboard-metrics" aria-label="Workspace summary">
      {metrics.map(({ label, value, detail, to, icon: Icon }) => <Link to={to} key={label} className="dashboard-metric">
        <span className="dashboard-metric__icon"><Icon size={16} strokeWidth={1.7} /></span>
        <span className="dashboard-metric__label">{label}</span>
        <strong className="dashboard-metric__value">{value.toLocaleString()}</strong>
        <span className="dashboard-metric__detail">{detail}</span>
        <ArrowRight className="dashboard-metric__arrow" size={14} />
      </Link>)}
    </section>

    <Tile className="dashboard-reading-line">
      <div className="dashboard-reading-line__identity"><span className="dashboard-reading-line__mark"><Clock3 size={16} /></span><span><strong>Reading time</strong><small>A quiet measure of your learning</small></span></div>
      <div className="dashboard-reading-line__stat"><small>This week</small><strong>{formatReadingTime(readingSummary.weekSeconds)}</strong></div>
      <div className="dashboard-reading-line__stat"><small>This month</small><strong>{formatReadingTime(readingSummary.monthSeconds)}</strong></div>
      <div className="dashboard-reading-line__stat"><small>All time</small><strong>{formatReadingTime(readingSummary.totalSeconds)}</strong></div>
      <Link to="/shelf" className="dashboard-reading-line__link" aria-label="Open Shelf"><ArrowRight size={16} /></Link>
    </Tile>

    <section className="dashboard-modules" aria-label="Workspace modules">
      <div className="dashboard-section-heading"><div><span className="dashboard-kicker">WORKSPACE</span><h2>Modules</h2></div><span className="dashboard-section-count">{String(modules.length).padStart(2, '0')} available</span></div>
      <div className="dashboard-module-groups">
        {groups.map((group) => <section className="dashboard-module-group" key={group}>
          <h3>{group}</h3>
          <div className="dashboard-module-grid">
            {modules.filter((module) => module.group === group).map(({ to, label, detail, icon: Icon }) => <Link className="dashboard-module" to={to} key={to}>
              <span className="dashboard-module__icon"><Icon size={16} strokeWidth={1.7} /></span>
              <span className="dashboard-module__copy"><strong>{label}</strong><small>{detail}</small></span>
              <ArrowRight className="dashboard-module__arrow" size={13} />
            </Link>)}
          </div>
        </section>)}
      </div>
    </section>

    <section className="dashboard-activity">
      <div className="dashboard-section-heading"><div><span className="dashboard-kicker">LOCAL HISTORY</span><h2>Recent activity</h2></div><Link to="/activity" className="dashboard-text-link">View history <ArrowRight size={13} /></Link></div>
      <Tile className="dashboard-activity-list">
        {recentActivity.length ? recentActivity.map((item: ActivityEvent) => {
          const Icon = activityIconFor(item.entity_type);
          return <Link className="dashboard-activity-row" to="/activity" key={item.id}>
            <span className="dashboard-activity-row__icon"><Icon size={14} /></span>
            <span className="dashboard-activity-row__copy"><strong>{friendlyAction(item.action)}</strong><small>{activitySummary(item)}</small></span>
            <time dateTime={item.created_at}>{formatRelative(item.created_at)}</time>
            <ArrowRight className="dashboard-activity-row__arrow" size={12} />
          </Link>;
        }) : <div className="dashboard-activity-empty"><Activity size={16} /><span>No changes recorded yet. Your local activity will appear here.</span></div>}
      </Tile>
    </section>

    <footer className="dashboard-footer"><span><CheckCircle2 size={13} /> Local-first by design</span><span>CLI <span className="dashboard-footer__slash">/</span> WEB</span><Link to="/settings">Preferences <Settings2 size={13} /></Link></footer>
  </div>;
}

function safeTime(value: string): number {
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : 0;
}
