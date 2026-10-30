import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router';
import { Bell, BellRing, CheckCheck, Clock3, Info, ShieldAlert, TriangleAlert } from 'lucide-react';
import { api } from '@/lib/api';
import { useWorkspaceStore } from '@/store/workspace-store';
import { Badge, Button, Card, EmptyState, PageHeading } from '@/design-system';
import { formatRelative } from '@/lib/format-relative';
import { stopPlanReminderSequence } from '@/lib/notification-sound';

const levelIcon: Record<string, typeof Info> = { info: Info, warning: TriangleAlert, error: ShieldAlert };
export function NotificationsPage() {
  const { dashboard, load } = useWorkspaceStore();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notifications, setNotifications] = useState(dashboard?.notifications ?? []);
  const unread = notifications.filter((item) => !item.read_at).length;
  useEffect(() => { void api.notifications().then(setNotifications).catch((err: unknown) => setError(err instanceof Error ? err.message : 'Unable to load notifications.')); }, []);
  async function markRead() {
    setBusy(true); setError('');
    try { await api.markNotificationsRead(); await Promise.all([load(true), api.notifications().then(setNotifications)]); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to mark notifications as read.'); }
    finally { setBusy(false); }
  }
  return <div className="page-stack">
    <PageHeading eyebrow="WORKSPACE SIGNAL" title="Notifications" description="Important changes and reminders from the IDIOS Engine, collected in one place." actions={<Button variant="secondary" onClick={() => void markRead()} loading={busy} disabled={!unread}><CheckCheck size={15} /> Mark all read</Button>} />
    <div className="notification-overview"><span className="notification-overview__icon"><BellRing size={19} /></span><div><strong>{unread ? `${unread} unread notification${unread === 1 ? '' : 's'}` : 'You’re all caught up'}</strong><p>{unread ? 'A few things may need your attention.' : 'We’ll keep useful workflow updates here as they happen.'}</p></div><Badge tone={unread ? 'blue' : 'green'}>{unread ? 'Needs attention' : 'Up to date'}</Badge></div>
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    <Card className="notification-list-card"><div className="section-heading"><div><p className="eyebrow">INBOX</p><h2>Recent updates</h2></div><span className="table-secondary">Latest {notifications.length}</span></div>{notifications.length ? <div className="notification-list">{notifications.map((notification) => { const Icon = levelIcon[notification.level] || Info; const opensPlan = notification.related_entity_type === 'plan-date' && Boolean(notification.related_entity_id); const open = () => { if (opensPlan) { stopPlanReminderSequence(); navigate(`/plans?date=${encodeURIComponent(notification.related_entity_id ?? '')}`); } }; return <article className={`notification-row ${notification.read_at ? '' : 'notification-row--unread'} ${opensPlan ? 'notification-row--actionable' : ''}`} key={notification.id} role={opensPlan ? 'button' : undefined} tabIndex={opensPlan ? 0 : undefined} onClick={opensPlan ? open : undefined} onKeyDown={opensPlan ? (event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); open(); } } : undefined}><span className={`notification-row__icon notification-row__icon--${notification.level}`}><Icon size={17} /></span><div className="notification-row__copy"><div className="notification-row__title"><h3>{notification.title}</h3>{!notification.read_at && <span className="unread-pill">NEW</span>}</div><p>{notification.body || 'A workspace event has been recorded.'}</p><div className="notification-row__meta"><time><Clock3 size={12} />{formatRelative(notification.created_at)}</time><Badge tone={notification.level === 'error' ? 'red' : notification.level === 'warning' ? 'amber' : 'neutral'}>{notification.level || 'info'}</Badge>{opensPlan && <span className="notification-row__open-hint">Open today’s plans ↗</span>}</div></div></article>; })}</div> : <EmptyState title="No notifications yet" body="When IDIOS has a useful reminder or workflow update, you’ll find it here." icon={<Bell size={22} />} />}</Card>
  </div>;
}
