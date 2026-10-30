import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import {
  ArrowUpRight, CalendarClock, CheckCircle2, CircleDashed, GitBranch, GitPullRequest, Link2,
  LoaderCircle, Pin, Play, Plus, Trash2, Unlink, XCircle,
} from 'lucide-react';
import { api } from '@/lib/api';
import type { Issue, Label, Milestone, ProtocolCommand } from '@/types/idios';
import { Badge, Button } from '@/design-system';
import { CommandRunner } from '@/features/commands';

interface IssueDetailsProps {
  issue: Issue | null;
  commands: ProtocolCommand[];
  open: boolean;
  onClose: () => void;
  onChanged: (commandId?: string) => void | Promise<void>;
  onOpenGraph: () => void;
  pullRequestTemplate?: { title_prefix: string; body: string };
}

function issueRef(issue: Issue) { return `${issue.repository_name || 'Repository'}#${issue.number}`; }
function tone(status: string): 'neutral' | 'green' | 'blue' | 'amber' {
  if (status === 'done') return 'green';
  if (status === 'in_progress') return 'blue';
  if (status === 'cancelled') return 'neutral';
  return 'amber';
}

export function IssueDetails({ issue, commands, open, onClose, onChanged, onOpenGraph, pullRequestTemplate }: IssueDetailsProps) {
  const [current, setCurrent] = useState<Issue | null>(issue);
  const [dependencies, setDependencies] = useState<Issue[]>([]);
  const [labelsByIssue, setLabelsByIssue] = useState<Record<string, Label[]>>({});
  const [milestonesByIssue, setMilestonesByIssue] = useState<Record<string, Milestone[]>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [activeCommand, setActiveCommand] = useState<ProtocolCommand | null>(null);
  const [initialValues, setInitialValues] = useState<Record<string, unknown>>({});

  const selectedRef = current ? issueRef(current) : '';
  const issueLabels = current ? labelsByIssue[current.id] ?? [] : [];
  const issueMilestones = current ? milestonesByIssue[current.id] ?? [] : [];

  async function refresh() {
    if (!issue) return;
    setLoading(true); setError('');
    try {
      const [fresh, deps, allLabels, allMilestones] = await Promise.all([
        api.issue(issueRef(issue)), api.issueDependencies(issueRef(issue)), api.issueLabels(), api.issueMilestones(),
      ]);
      setCurrent(fresh); setDependencies(deps); setLabelsByIssue(allLabels); setMilestonesByIssue(allMilestones);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to load issue details.');
      setCurrent(issue);
    } finally { setLoading(false); }
  }

  useEffect(() => {
    if (!open) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => { document.body.style.overflow = previousOverflow; };
  }, [open]);

  useEffect(() => {
    if (!open || !issue) return;
    setCurrent(issue);
    void refresh();
    // Refresh is scoped to the selected issue and intentionally doesn't depend on its own function identity.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, issue?.id]);

  function run(commandId: string, values: Record<string, unknown> = {}) {
    const command = commands.find((candidate) => candidate.id === commandId);
    if (!command) { setError(`The shared IDIOS protocol does not expose “${commandId}”.`); return; }
    setError(''); setInitialValues(values); setActiveCommand(command);
  }

  async function commandComplete() {
    const commandId = activeCommand?.id;
    setActiveCommand(null);
    if (commandId === 'issue.delete') { onClose(); await onChanged(commandId); return; }
    await refresh(); await onChanged(commandId);
  }

  if (!open || !issue || !current) {
    return <CommandRunner command={activeCommand} open={Boolean(activeCommand)} initialValues={initialValues} onClose={() => setActiveCommand(null)} onSuccess={commandComplete} />;
  }

  return <>
    {createPortal(<div className="issue-detail-overlay" role="presentation" onMouseDown={(event) => { if (event.currentTarget === event.target) onClose(); }}>
      <section className="issue-detail-dialog" role="dialog" aria-modal="true" aria-labelledby="issue-detail-title">
        <header className="issue-detail-header">
          <div className="issue-detail-header__icon"><CircleDashed size={19} /></div>
          <div className="issue-detail-header__copy">
            <p className="eyebrow">{current.repository_name || 'Repository'} · ISSUE #{current.number}</p>
            <h2 id="issue-detail-title">{current.title}</h2>
            <div className="issue-detail-meta"><Badge tone={tone(current.status)}>{current.status.replace('_', ' ')}</Badge><span>{current.priority || 'normal'} priority</span>{current.due_date && <span><CalendarClock size={12} /> Due {current.due_date}</span>}</div>
          </div>
          <button type="button" className="icon-button" onClick={onClose} aria-label="Close issue details"><XCircle size={18} /></button>
        </header>
        <div className="issue-detail-scroll">
          {error && <div className="notice notice--error" role="alert">{error}</div>}
          <div className="issue-detail-actionbar">
            {current.status === 'open' && <Button onClick={() => run('issue.start', { ref: current.id })} disabled={current.blocked}><Play size={14} /> Start issue</Button>}
            {current.status === 'in_progress' && <Button onClick={() => run('pull_request.create', { issue: current.id, title: `${pullRequestTemplate?.title_prefix ?? ''}${current.title}`, description: pullRequestTemplate?.body ?? '', source_branch: current.branch_id ?? '' })}><GitPullRequest size={14} /> Create pull request</Button>}
            {current.status === 'open' && current.blocked && <span className="issue-detail-blocked"><CircleDashed size={14} /> Blocked by prerequisites</span>}
            {current.status !== 'done' && current.status !== 'cancelled' && <Button variant="secondary" onClick={() => run('issue.cancel', { ref: current.id })}><XCircle size={14} /> Cancel</Button>}
            <Button variant="ghost" onClick={() => run('workspace.pin.toggle', { entity_type: 'issue', entity_ref: current.id, label: current.title, color: 'accent' })}><Pin size={14} /> Pin</Button>
            <Button variant="ghost" onClick={onOpenGraph}><ArrowUpRight size={14} /> View graph</Button>
          </div>
          <section className="issue-detail-section">
            <div className="issue-detail-section__heading"><div><p className="eyebrow">OVERVIEW</p><h3>Description</h3></div></div>
            <p className="issue-detail-description">{current.description || 'No description yet.'}</p>
            <div className="issue-detail-info-grid">
              <div><span>Repository</span><strong>{current.repository_name || '—'}</strong></div>
              <div><span>Parent issue</span><strong>{current.parent_reference || 'None'}</strong></div>
              <div><span>Branch</span><strong>{current.branch_name || 'Not assigned'}</strong></div>
              <div><span>Created</span><strong>{current.created_at ? new Date(current.created_at).toLocaleDateString() : '—'}</strong></div>
              <div><span>Updated</span><strong>{current.updated_at ? new Date(current.updated_at).toLocaleString() : '—'}</strong></div>
            </div>
            <div className="issue-detail-inline-actions issue-detail-inline-actions--bottom"><Button variant="secondary" onClick={() => run('issue.parent.assign', { issue: current.id, parent: current.parent_issue_id ?? '' })}>{current.parent_issue_id ? 'Change parent / Sub-issue' : 'Set parent issue'}</Button></div>
          </section>
          <section className="issue-detail-section">
            <div className="issue-detail-section__heading"><div><p className="eyebrow">WORKFLOW</p><h3>Dependencies <span>{dependencies.length}</span></h3></div><Button variant="secondary" onClick={() => run('issue.dependency.add', { issue: current.id })}><Plus size={13} /> Add dependency</Button></div>
            <div className="issue-detail-branch-row"><div><GitBranch size={15} /><div><strong>{current.branch_name || 'No branch assigned'}</strong><small>Track this issue as part of a repository branch timeline.</small></div></div><Button variant="secondary" onClick={() => run('issue.branch.assign', { issue: current.id, branch: current.branch_id ?? '' })}>{current.branch_id ? 'Change branch' : 'Assign branch'}</Button></div>
            {current.blockers?.length ? <div className="issue-dependency-list">{current.blockers.map((blocker) => <div className="issue-dependency-row" key={blocker.issue_id}><div className="issue-dependency-status"><CircleDashed size={15} /></div><div className="issue-dependency-copy"><strong>{blocker.repository_name}#{blocker.number} · {blocker.title}</strong><small>{blocker.status.replace('_', ' ')} · {blocker.reason || 'Blocking prerequisite'}</small></div><Button variant="ghost" onClick={() => run('issue.dependency.remove', { issue: current.id, prerequisite: blocker.issue_id })} title="Remove prerequisite"><Unlink size={14} /></Button></div>)}</div>
              : dependencies.length ? <div className="issue-dependency-list">{dependencies.map((dependency) => <div className="issue-dependency-row" key={dependency.id}><div className="issue-dependency-status"><CheckCircle2 size={15} /></div><div className="issue-dependency-copy"><strong>{issueRef(dependency)} · {dependency.title}</strong><small>Prerequisite · {dependency.status.replace('_', ' ')}</small></div><Button variant="ghost" onClick={() => run('issue.dependency.remove', { issue: current.id, prerequisite: dependency.id })}><Unlink size={14} /></Button></div>)}</div>
              : <p className="issue-detail-empty-inline">No prerequisites connected. Work can proceed as soon as the Engine’s other rules allow it.</p>}
            {loading && <span className="issue-detail-muted"><LoaderCircle className="spin" size={13} /> Refreshing workflow…</span>}
          </section>
          <section className="issue-detail-section">
            <div className="issue-detail-section__heading"><div><p className="eyebrow">KNOWLEDGE</p><h3>Labels & milestones</h3></div><div className="issue-detail-inline-actions"><Button variant="secondary" onClick={() => run('issue.label.assign', { issue: current.id })}>Assign label</Button><Button variant="secondary" onClick={() => run('issue.milestone.assign', { issue: current.id })}>Assign milestone</Button></div></div>
            <div className="issue-detail-tags-row"><span className="issue-detail-tags-label">Labels</span>{issueLabels.length ? issueLabels.map((label) => <span className="issue-detail-tag" key={label.id}><i style={{ backgroundColor: label.color || 'var(--accent)' }} />{label.name}</span>) : <small>None assigned</small>}</div>
            <div className="issue-detail-tags-row"><span className="issue-detail-tags-label">Milestones</span>{issueMilestones.length ? issueMilestones.map((milestone) => <span className="issue-detail-tag issue-detail-tag--milestone" key={milestone.id}><CalendarClock size={12} />{milestone.title}{milestone.due_date ? ` · ${milestone.due_date}` : ''}</span>) : <small>None assigned</small>}</div>
            <div className="issue-detail-inline-actions issue-detail-inline-actions--bottom"><Button variant="ghost" onClick={() => run('issue.concept.link', { issue: current.id })}><Link2 size={14} /> Link concept</Button><Button variant="danger" onClick={() => run('issue.delete', { ref: current.id })}><Trash2 size={14} /> Delete issue</Button></div>
          </section>
          {loading && <div className="issue-detail-loading"><LoaderCircle className="spin" size={16} /> Syncing latest Engine state…</div>}
        </div>
        <footer className="issue-detail-footer"><span>{selectedRef} · Actions run through the shared Go Engine.</span><Button variant="secondary" onClick={() => void refresh()} disabled={loading}><LoaderCircle size={14} className={loading ? 'spin' : ''} /> Refresh details</Button></footer>
      </section>
    </div>, document.body)}
    <CommandRunner command={activeCommand} open={Boolean(activeCommand)} initialValues={initialValues} onClose={() => setActiveCommand(null)} onSuccess={commandComplete} />
  </>;
}
