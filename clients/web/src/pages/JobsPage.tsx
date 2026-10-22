import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';
import { CalendarClock, CheckCircle2, Clock3, ListPlus, Pause, Play, Plus, RotateCw, Search, Trash2, Pencil, BellRing } from 'lucide-react';
import { api } from '@/lib/api';
import type { ProtocolCommand, ScheduledJob } from '@/types/idios';
import { Badge, Button, EmptyState, PageHeading } from '@/components/ui/primitives';
import { CommandRunner } from '@/components/commands/CommandRunner';
import { formatRelative } from '@/components/ui/IssueTable';
import { useWorkspaceStore } from '@/store/workspace-store';

type JobFilter = 'all' | 'enabled' | 'paused' | 'completed';
const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

export function JobsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [items, setItems] = useState<ScheduledJob[]>([]);
  const [commands, setCommands] = useState<ProtocolCommand[]>([]);
  const [notificationsEnabled, setNotificationsEnabled] = useState(true);
  const [selectedCommand, setSelectedCommand] = useState<ProtocolCommand | null>(null);
  const [initialValues, setInitialValues] = useState<Record<string, unknown>>({});
  const [error, setError] = useState('');
  const [busy, setBusy] = useState<string | null>(null);
  const [bulkCreateOpen, setBulkCreateOpen] = useState(false);
  const [filter, setFilter] = useState<JobFilter>('all');
  const [query, setQuery] = useState('');
  const { load } = useWorkspaceStore();

  async function refresh() {
    try {
      const [jobs, catalog, configPayload] = await Promise.all([api.jobs(), api.protocol(), api.config()]);
      setItems(jobs);
      setCommands(catalog.commands);
      setNotificationsEnabled(configPayload.config.notifications.enabled);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to load scheduled jobs.');
    }
  }
  useEffect(() => { void refresh(); }, []);
  useEffect(() => {
    if (searchParams.get('bulk') !== '1' || !commands.some((item) => item.id === 'job.bulk_create')) return;
    setBulkCreateOpen(true);
    const nextParams = new URLSearchParams(searchParams);
    nextParams.delete('bulk');
    setSearchParams(nextParams, { replace: true });
  }, [searchParams, commands, setSearchParams]);
  useEffect(() => {
    if (window.location.hash === '#queue') document.getElementById('job-list')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }, []);

  const pendingJobs = items.filter((job): job is ScheduledJob & { next_run_at: string } => job.enabled && !job.completed_at && typeof job.next_run_at === 'string' && job.next_run_at.length > 0);
  const counts = {
    all: items.length,
    enabled: items.filter((job) => job.enabled && !job.completed_at).length,
    paused: items.filter((job) => !job.enabled && !job.completed_at).length,
    completed: items.filter((job) => Boolean(job.completed_at)).length,
  };
  const filteredJobs = useMemo(() => {
    const text = query.trim().toLowerCase();
    return items.filter((job) => {
      const statusMatches = filter === 'all' || (filter === 'enabled' && job.enabled && !job.completed_at) || (filter === 'paused' && !job.enabled && !job.completed_at) || (filter === 'completed' && Boolean(job.completed_at));
      const textMatches = !text || `${job.name} ${job.description ?? ''} ${job.action_type} ${job.schedule_type}`.toLowerCase().includes(text);
      return statusMatches && textMatches;
    }).sort((a, b) => {
      if (a.completed_at && !b.completed_at) return 1;
      if (!a.completed_at && b.completed_at) return -1;
      const aTime = a.next_run_at ? Date.parse(a.next_run_at) : Number.MAX_SAFE_INTEGER;
      const bTime = b.next_run_at ? Date.parse(b.next_run_at) : Number.MAX_SAFE_INTEGER;
      return aTime - bTime;
    });
  }, [items, filter, query]);

  function run(id: string, values: Record<string, unknown> = {}) {
    const command = commands.find((item) => item.id === id);
    if (!command) return;
    setInitialValues(values);
    setSelectedCommand(command);
  }
  async function quick(id: string, input: Record<string, unknown>) {
    setBusy(String(input.id));
    setError('');
    try {
      await api.executeCommand(id, id === 'job.run' ? { ...input, notify: notificationsEnabled } : input);
      await Promise.all([refresh(), load(true)]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to run scheduled job action.');
    } finally { setBusy(null); }
  }
  async function complete() { await Promise.all([refresh(), load(true)]); }
  function jobInput(job: ScheduledJob) {
    return { id: job.id, name: job.name, description: job.description ?? '', action_type: job.action_type, schedule_type: job.schedule_type, interval_minutes: job.interval_minutes ?? 60, schedule_at: job.schedule_type === 'once' ? localDateTime(job.next_run_at) : '', schedule_time: job.schedule_time ?? '09:00', schedule_weekday: job.schedule_weekday ?? 1, schedule_day: job.schedule_day ?? 1, enabled: job.enabled, notify: job.notify };
  }

  return <div className="page-stack jobs-page">
    <PageHeading eyebrow="AUTOMATION" title="Scheduled jobs" description="Plan one-time reminders or repeating work. Pause, edit, or run any job from its row." actions={<><Button variant="secondary" onClick={() => setBulkCreateOpen(true)} disabled={!commands.some((item) => item.id === 'job.bulk_create')}><ListPlus size={16} /> Add many</Button><Button onClick={() => run('job.create')}><Plus size={16} /> New schedule</Button></>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}

    <div className="job-overview-strip" role="tablist" aria-label="Filter scheduled jobs">
      {([['all', 'All jobs'], ['enabled', 'Enabled'], ['paused', 'Paused'], ['completed', 'Completed']] as const).map(([value, label]) => <button type="button" key={value} className={`job-overview-stat ${filter === value ? 'job-overview-stat--active' : ''}`} onClick={() => setFilter(value)} role="tab" aria-selected={filter === value}><span>{label}</span><strong>{counts[value]}</strong></button>)}
    </div>

    <section id="job-list" className="jobs-browser panel" aria-label="Scheduled jobs list">
      <div className="jobs-browser__toolbar"><div><h2>{filter === 'all' ? 'All schedules' : filter === 'enabled' ? 'Enabled schedules' : filter === 'paused' ? 'Paused schedules' : 'Completed one-time jobs'}</h2><span>{filteredJobs.length} shown · {pendingJobs.length} upcoming</span></div><label className="inline-search"><Search size={15} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Find a job…" aria-label="Search scheduled jobs" /></label></div>
      {filteredJobs.length ? <div className="jobs-list">
        {filteredJobs.map((job) => {
          const isBusy = busy === job.id;
          const status = job.completed_at ? 'Completed' : job.enabled ? 'Enabled' : 'Paused';
          const schedule = scheduleSummary(job);
          return <article className="job-list-row" key={job.id}>
            <span className={`job-list-row__icon ${job.completed_at ? 'job-list-row__icon--done' : ''}`}><CalendarClock size={16} /></span>
            <div className="job-list-row__main"><div className="job-list-row__title"><strong>{job.name}</strong><Badge tone={job.completed_at ? 'green' : job.enabled ? 'blue' : 'neutral'}>{status}</Badge></div><p>{job.description || job.action_type.replaceAll('-', ' ')}</p><div className="job-list-row__meta"><span>{schedule}</span><span>{job.notify ? <><BellRing size={12} /> Notify</> : 'Silent'}</span>{job.last_run_at && <span><CheckCircle2 size={12} /> Last run {formatRelative(job.last_run_at)}</span>}</div></div>
            <div className="job-list-row__next">{job.completed_at ? <><strong>Finished</strong><time>{formatRelative(job.completed_at)}</time></> : job.next_run_at ? <><strong>Next run</strong><time title={new Date(job.next_run_at).toLocaleString()}>{formatRelative(job.next_run_at)}</time></> : <><strong>Next run</strong><time>Not scheduled</time></>}</div>
            <div className="job-list-row__actions">
              {!job.completed_at && <button type="button" className="icon-button" disabled={isBusy} onClick={() => void quick('job.toggle', { id: job.id })} aria-label={job.enabled ? `Pause ${job.name}` : `Enable ${job.name}`} title={job.enabled ? 'Pause schedule' : 'Enable schedule'}>{job.enabled ? <Pause size={15} /> : <Play size={15} />}</button>}
              {!job.completed_at && <button type="button" className="icon-button" disabled={isBusy} onClick={() => void quick('job.run', { id: job.id })} aria-label={`Run ${job.name} now`} title="Run now">{isBusy ? <RotateCw size={15} className="spin" /> : <Play size={15} />}</button>}
              <button type="button" className="icon-button" onClick={() => run('job.update', jobInput(job))} aria-label={`Edit ${job.name}`} title="Edit schedule"><Pencil size={15} /></button>
              <button type="button" className="icon-button job-list-row__delete" onClick={() => run('job.delete', { id: job.id })} aria-label={`Delete ${job.name}`} title="Delete schedule"><Trash2 size={15} /></button>
            </div>
          </article>;
        })}
      </div> : <div className="jobs-empty"><EmptyState title={query ? 'No matching jobs' : filter === 'completed' ? 'No completed jobs' : 'Nothing scheduled here'} body={query ? 'Try a different name or search term.' : 'Create a schedule for a reminder, a daily plan, or a recurring review.'} icon={<Clock3 size={21} />} />{!query && filter !== 'completed' && <Button onClick={() => run('job.create')}><Plus size={15} /> Create a schedule</Button>}</div>}
    </section>
    <CommandRunner command={commands.find((item) => item.id === 'job.bulk_create') ?? null} open={bulkCreateOpen} onClose={() => setBulkCreateOpen(false)} onSuccess={complete} />
    <CommandRunner command={selectedCommand} open={Boolean(selectedCommand)} initialValues={initialValues} onClose={() => setSelectedCommand(null)} onSuccess={complete} />
  </div>;
}

function scheduleSummary(job: ScheduledJob) {
  switch (job.schedule_type) {
    case 'interval': return `Every ${job.interval_minutes || 60} minutes`;
    case 'once': return 'One time';
    case 'daily': return `Daily · ${job.schedule_time || '09:00'}`;
    case 'weekly': return `Every ${weekdays[job.schedule_weekday] ?? 'week'} · ${job.schedule_time || '09:00'}`;
    case 'monthly': return `Day ${job.schedule_day || 1} each month · ${job.schedule_time || '09:00'}`;
    default: return job.schedule_type.replaceAll('_', ' ');
  }
}

function localDateTime(raw?: string) { if (!raw) return ''; const date = new Date(raw); if (!Number.isFinite(date.getTime())) return ''; const pad = (part: number) => String(part).padStart(2, '0'); return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`; }
