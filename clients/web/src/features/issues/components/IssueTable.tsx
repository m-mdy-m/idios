import { useEffect, useState, type CSSProperties } from 'react';
import { ArrowUpRight, Circle, CircleCheck, CircleDashed, Clock3, Play, Trash2, XCircle } from 'lucide-react';
import type { Issue, Label } from '@/types/idios';
import { Badge, Button, EmptyState } from '@/design-system';
import { api } from '@/lib/api';
import { useWorkspaceStore } from '@/store/workspace-store';
import { Modal } from '@/design-system';
import { formatRelative } from '@/lib/format-relative';

function statusTone(status: string): 'neutral' | 'green' | 'blue' | 'amber' {
  if (status === 'done') return 'green';
  if (status === 'in_progress') return 'blue';
  if (status === 'cancelled') return 'neutral';
  return 'amber';
}
function statusLabel(status: string) {
  return ({ open: 'Open', in_progress: 'In progress', done: 'Done', cancelled: 'Cancelled' } as Record<string, string>)[status] || status;
}
function StatusIcon({ status }: { status: string }) {
  if (status === 'done') return <CircleCheck size={15} />;
  if (status === 'in_progress') return <Clock3 size={15} />;
  if (status === 'cancelled') return <XCircle size={15} />;
  return <CircleDashed size={15} />;
}

export function IssueTable({ issues, emptyTitle = 'Nothing on your plate', emptyBody = 'New work will appear here when you create an issue.', onChange, onSelect }: { issues: Issue[]; emptyTitle?: string; emptyBody?: string; onChange?: () => void | Promise<void>; onSelect?: (issue: Issue) => void }) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [pendingDelete, setPendingDelete] = useState<Issue | null>(null);
  const [labelsByIssue, setLabelsByIssue] = useState<Record<string, Label[]>>({});
  const load = useWorkspaceStore((state) => state.load);
  useEffect(() => {
    let active = true;
    void api.issueLabels().then((value) => { if (active) setLabelsByIssue(value); }).catch(() => undefined);
    return () => { active = false; };
  }, [issues]);
  async function update(issue: Issue, action: 'start' | 'cancel') {
    setBusy(issue.id); setError('');
    try { if (action === 'start') await api.startIssue(issue.id); else await api.cancelIssue(issue.id); await load(true); await onChange?.(); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to update issue.'); }
    finally { setBusy(null); }
  }
  async function remove(issue: Issue) {
    const reference = issue.id;
    setBusy(issue.id); setError('');
    try { await api.executeCommand('issue.delete', { ref: reference }); setPendingDelete(null); await load(true); await onChange?.(); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to delete issue.'); }
    finally { setBusy(null); }
  }
  if (!issues.length) return <EmptyState title={emptyTitle} body={emptyBody} icon={<Circle size={21} />} />;
  return <div className="table-wrap"><table className="data-table"><thead><tr><th>Issue</th><th>Status</th><th>Priority</th><th>Repository</th><th>Updated</th><th aria-label="Actions" /></tr></thead><tbody>{issues.map((issue) => {
    const issueLabels = labelsByIssue[issue.id] ?? [];
    const highlight = issueLabels.find((label) => /^(important|hotfix|critical|urgent|security|blocker|blocked)$/i.test(label.name));
    const highlightStyle = highlight ? ({ '--issue-label-color': /^#[0-9a-f]{6}$/i.test(highlight.color) ? highlight.color : 'var(--accent)' } as CSSProperties) : undefined;
    return <tr key={issue.id} className={highlight ? 'data-table__highlighted-issue' : undefined} style={highlightStyle}>
    <td><div className="issue-cell"><span className={`issue-status-icon issue-status-icon--${issue.status}`}><StatusIcon status={issue.status} /></span><div className="issue-cell__text"><strong>{issue.title}</strong><span>{issue.repository_name || 'Repository'} #{issue.number}{issue.blocked ? <span className="inline-blocked"> · BLOCKED</span> : null}</span>{issueLabels.length > 0 && <span className="issue-inline-labels">{issueLabels.slice(0, 4).map((label) => <span key={label.id} className="issue-inline-label" style={{ '--chip-color': label.color || 'var(--accent)' } as CSSProperties}>{label.name}</span>)}{issueLabels.length > 4 && <span className="issue-inline-label issue-inline-label--more">+{issueLabels.length - 4}</span>}</span>}{issue.branch_name && <small className="issue-branch-hint"><span aria-hidden="true">⌥</span> {issue.branch_name}</small>}{issue.blockers?.length ? <small className="blocker-note">Waiting on {issue.blockers.length} prerequisite{issue.blockers.length === 1 ? '' : 's'}</small> : null}</div></div></td>
    <td><Badge tone={statusTone(issue.status)}>{statusLabel(issue.status)}</Badge></td><td><span className={`priority priority--${(issue.priority || 'normal').toLowerCase()}`}><span />{issue.priority || 'Normal'}</span></td><td><span className="table-secondary">{issue.repository_name || '—'}</span></td><td><span className="table-secondary">{formatRelative(issue.updated_at)}</span></td><td><div className="row-actions">{issue.status === 'open' && !issue.blocked && <Button variant="ghost" className="table-action" onClick={() => void update(issue, 'start')} loading={busy === issue.id}><Play size={13} /> Start</Button>}{issue.status === 'in_progress' && <Button variant="ghost" className="table-action" onClick={() => void update(issue, 'cancel')} loading={busy === issue.id}>Cancel</Button>}{onSelect ? <button type="button" className="table-open-button" aria-label={`Open ${issue.title}`} title="Open issue details" onClick={() => onSelect(issue)}><ArrowUpRight size={15} /></button> : <ArrowUpRight className="table-open-icon" size={15} />}<button type="button" className="table-open-button table-delete-button" aria-label={`Delete ${issue.title}`} title="Delete issue" disabled={busy === issue.id} onClick={() => setPendingDelete(issue)}><Trash2 size={14} /></button></div></td>
  </tr>;
  })}</tbody></table>{error && <p className="inline-error" role="alert">{error}</p>}
    <Modal open={Boolean(pendingDelete)} title={`Delete issue ${pendingDelete?.repository_name || 'Repository'}#${pendingDelete?.number ?? ''}?`} description="The Engine will keep linked work safe. An issue with child issues, blockers, pull requests, or completion history cannot be deleted until those relationships are resolved." onClose={() => setPendingDelete(null)} onSubmit={(event) => { event.preventDefault(); if (pendingDelete) void remove(pendingDelete); }} submitLabel="Delete issue" submitVariant="danger" busy={busy === pendingDelete?.id}>
      <div className="delete-confirm-summary"><span className="delete-confirm-summary__icon"><Trash2 size={17} /></span><div><strong>{pendingDelete?.title}</strong><p>{pendingDelete?.description || 'No description'} </p></div></div>
      {error && <div className="notice notice--error" role="alert">{error}</div>}
    </Modal>
  </div>;
}

