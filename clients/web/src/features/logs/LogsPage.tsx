import { useCallback, useEffect, useMemo, useState } from 'react';
import { Activity, AlertTriangle, Check, Clipboard, Database, FileText, RefreshCw, ScrollText, Server, Trash2 } from 'lucide-react';
import { api } from '@/lib/api';
import { Badge, Button, Card, EmptyState, PageHeading } from '@/design-system';
import { ConfirmDialog } from '@/design-system';
import type { LogPayload, MonitoringSnapshot } from '@/types/idios';
import { PrivatePath } from '@/components/privacy/PrivatePath';
import { redactLocalPaths } from '@/lib/privacy';

const levels = ['ALL', 'ERROR', 'WARN', 'INFO', 'DEBUG'] as const;
type LogLevel = typeof levels[number];
const categories = ['ALL', 'HTTP/API', 'Workflow', 'Scheduler', 'Updates', 'Configuration', 'Storage', 'Runtime'] as const;
type LogCategory = typeof categories[number];

function levelOf(line: string): Exclude<LogLevel, 'ALL'> {
  const match = line.match(/\s(ERROR|WARN|INFO|DEBUG)\s/);
  return (match?.[1] as Exclude<LogLevel, 'ALL'> | undefined) ?? 'INFO';
}

function categoryOf(line: string): Exclude<LogCategory, 'ALL'> {
  const lower = line.toLowerCase();
  if (/\b(get|post|put|patch|delete)\s+\/api\//.test(lower)) return 'HTTP/API';
  if (/scheduler|scheduled job|reminder|next run|due job/.test(lower)) return 'Scheduler';
  if (/update|release|download|checksum|restart/.test(lower)) return 'Updates';
  if (/config|setting|theme|keyboard|preference/.test(lower)) return 'Configuration';
  if (/sqlite|database|migration|storage|store|query/.test(lower)) return 'Storage';
  if (/issue|repository|pull request|branch|project|concept|note|label|milestone|quote|profile/.test(lower)) return 'Workflow';
  return 'Runtime';
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
}

function formatDuration(seconds: number): string {
  const safe = Math.max(0, Math.floor(seconds || 0));
  const days = Math.floor(safe / 86400);
  const hours = Math.floor((safe % 86400) / 3600);
  const minutes = Math.floor((safe % 3600) / 60);
  const secs = safe % 60;
  if (days) return `${days}d ${hours}h`;
  if (hours) return `${hours}h ${minutes}m`;
  if (minutes) return `${minutes}m ${secs}s`;
  return `${secs}s`;
}

export function LogsPage() {
  const [payload, setPayload] = useState<LogPayload | null>(null);
  const [monitoring, setMonitoring] = useState<MonitoringSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [clearing, setClearing] = useState(false);
  const [confirmClearOpen, setConfirmClearOpen] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [filter, setFilter] = useState<LogLevel>('ALL');
  const [category, setCategory] = useState<LogCategory>('ALL');
  const [query, setQuery] = useState('');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [copied, setCopied] = useState(false);

  const refresh = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    setError('');
    const [logResult, monitoringResult] = await Promise.allSettled([api.logs(1000), api.monitoring()]);
    if (logResult.status === 'fulfilled') setPayload(logResult.value);
    else setError(logResult.reason instanceof Error ? logResult.reason.message : 'Unable to read local diagnostics.');
    if (monitoringResult.status === 'fulfilled') setMonitoring(monitoringResult.value);
    else if (logResult.status === 'fulfilled') setError((current) => current || 'Logs are available, but the monitoring snapshot could not be loaded.');
    setLoading(false);
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);
  useEffect(() => {
    if (!autoRefresh) return;
    const timer = window.setInterval(() => void refresh(true), 5000);
    return () => window.clearInterval(timer);
  }, [autoRefresh, refresh]);

  const lines = useMemo(() => [...(payload?.lines ?? [])].reverse().filter((line) => {
    const level = levelOf(line);
    return (filter === 'ALL' || filter === level)
      && (category === 'ALL' || categoryOf(line) === category)
      && line.toLowerCase().includes(query.trim().toLowerCase());
  }), [payload, filter, category, query]);

  async function copyLogs() {
    try {
      await navigator.clipboard.writeText(lines.map(redactLocalPaths).join('\n'));
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      setError('Clipboard access is not available in this browser context.');
    }
  }

  async function clearLogs() {
    setClearing(true);
    setError('');
    setNotice('');
    try {
      await api.clearLogs();
      setConfirmClearOpen(false);
      setNotice('Local log history was cleared. New events will be recorded as usual.');
      await refresh(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to clear the local log file.');
    } finally {
      setClearing(false);
    }
  }

  return <div className="page-stack">
    <PageHeading eyebrow="LOCAL DIAGNOSTICS" title="Logs & monitoring" description="Inspect the local log stream, filter entries, watch runtime health, and clear diagnostic history. IDIOS does not send these metrics to a remote service." actions={<>
      <Button variant={autoRefresh ? 'secondary' : 'ghost'} onClick={() => setAutoRefresh((value) => !value)}><Activity size={15} /> {autoRefresh ? 'Live refresh on' : 'Live refresh off'}</Button>
      <Button variant="secondary" onClick={() => void refresh()} disabled={loading}><RefreshCw size={15} className={loading ? 'spin' : ''} /> Refresh</Button>
      <Button onClick={() => void copyLogs()} disabled={!lines.length}>{copied ? <Check size={15} /> : <Clipboard size={15} />} {copied ? 'Copied' : 'Copy visible'}</Button>
      <Button variant="danger" onClick={() => setConfirmClearOpen(true)} disabled={clearing}><Trash2 size={15} /> {clearing ? 'Clearing…' : 'Clear logs'}</Button>
    </>} />
    {error && <div className="notice notice--error" role="alert"><AlertTriangle size={15} /> {error}</div>}
    {notice && <div className="notice notice--success" role="status">{notice}</div>}

    <div className="logs-summary-grid">
      <Card className="logs-summary-card"><span className="section-symbol section-symbol--blue"><ScrollText size={17} /></span><div><span>Matching entries</span><strong>{lines.length}</strong></div></Card>
      <Card className="logs-summary-card"><span className="section-symbol section-symbol--amber"><AlertTriangle size={17} /></span><div><span>Errors in buffer</span><strong>{(payload?.lines ?? []).filter((line) => levelOf(line) === 'ERROR').length}</strong></div></Card>
      <Card className="logs-summary-card"><span className="section-symbol section-symbol--violet"><FileText size={17} /></span><div><span>Log file size</span><strong>{formatBytes(monitoring?.logs.size_bytes ?? 0)}</strong></div></Card>
      <Card className="logs-summary-card"><span className="section-symbol section-symbol--green"><Database size={17} /></span><div><span>Database</span><strong className="logs-status-value">{monitoring?.database.status === 'ok' ? 'Healthy' : monitoring ? 'Degraded' : 'Checking'}</strong><small>{monitoring?.database.status === 'error' ? ('Database ping failed') : monitoring ? `${monitoring.database.latency_ms.toFixed(1)} ms ping` : 'Awaiting health snapshot'}</small></div></Card>
      <Card className="logs-summary-card"><span className="section-symbol section-symbol--blue"><Server size={17} /></span><div><span>Process uptime</span><strong>{formatDuration(monitoring?.process.uptime_seconds ?? 0)}</strong><small>{monitoring?.process.goroutines ?? '—'} goroutines · {formatBytes(monitoring?.process.memory_alloc_bytes ?? 0)} allocated</small></div></Card>
      <Card className="logs-summary-card"><span className="section-symbol section-symbol--amber"><Activity size={17} /></span><div><span>HTTP requests</span><strong>{monitoring?.http.requests_total ?? '—'}</strong><small>{monitoring ? `${monitoring.http.requests_failed} failed · ${monitoring.http.average_request_ms.toFixed(2)} ms avg` : 'Awaiting health snapshot'}</small></div></Card>
    </div>

    <Card className="logs-panel">
      <div className="logs-panel-header"><div><p className="eyebrow">SHARED LOG STREAM</p><h2>Recent events</h2><div className="logs-path"><PrivatePath value={payload?.path ?? monitoring?.logs.path} hiddenLabel="Local log file · path hidden" /></div></div><Badge tone={autoRefresh ? 'green' : monitoring?.status === 'ok' ? 'neutral' : 'amber'}>{autoRefresh ? 'LIVE · 5 SEC' : monitoring?.status === 'degraded' ? 'DEGRADED' : 'LOCAL MONITORING'}</Badge></div>
      <div className="logs-toolbar">
        <label className="inline-search"><FileText size={15} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search log text…" /></label>
        <label className="logs-category-select"><span>Category</span><select value={category} onChange={(event) => setCategory(event.target.value as LogCategory)}>{categories.map((value) => <option value={value} key={value}>{value}</option>)}</select></label>
        <div className="logs-level-filter" role="group" aria-label="Filter by log level">{levels.map((level) => <button type="button" key={level} className={filter === level ? 'logs-level-filter__active' : ''} aria-pressed={filter === level} onClick={() => setFilter(level)}>{level}</button>)}</div>
      </div>
      <div className="logs-stream" aria-live="polite">
        {loading && !payload ? <div className="loading-state"><Activity size={16} className="spin" /> Reading local log…</div>
          : lines.length ? lines.map((line, index) => <div className={`log-entry log-entry--${levelOf(line).toLowerCase()}`} key={`${index}-${line}`}><span className="log-entry__level">{levelOf(line)}</span><code><span className="log-entry__category">{categoryOf(line)}</span>{redactLocalPaths(line)}</code></div>)
            : <EmptyState title="No matching log entries" body="Change the level, category, or search term. Enable Debug in Settings if you need request-level details." icon={<ScrollText size={21} />} />}
      </div>
      <div className="logs-panel-footer"><span>Showing {lines.length} of up to {payload?.count ?? 0} loaded lines, newest first.</span><span>Local file · no cloud logging</span></div>
    </Card>
    <ConfirmDialog open={confirmClearOpen} title="Clear local log history?" description="This action cannot be undone." confirmLabel={clearing ? 'Clearing…' : 'Clear logs'} busy={clearing} onClose={() => { if (!clearing) setConfirmClearOpen(false); }} onConfirm={() => void clearLogs()}>
      <p>Only the local diagnostic log file will be truncated. Repositories, issues, notes, settings, and other workspace data are not changed.</p>
    </ConfirmDialog>
  </div>;
}
