import { useCallback, useEffect, useState } from 'react';
import { GitBranch, GitCommitHorizontal, GitCompare, GitPullRequest, Plus, RefreshCw, Terminal } from 'lucide-react';
import { api } from '@/lib/api';
import type { LocalGitBranch, LocalGitCommit, LocalGitStatus, Repository } from '@/types/idios';
import { Badge, Button, EmptyState, Field } from '@/design-system';
import { Modal } from '@/design-system';
import './local-git-workspace.css';

function messageFrom(error: unknown): string {
  return error instanceof Error ? error.message : 'The local Git operation failed.';
}

export function LocalGitWorkspace({ repository }: { repository: Repository }) {
  const [status, setStatus] = useState<LocalGitStatus | null>(null);
  const [branches, setBranches] = useState<LocalGitBranch[]>([]);
  const [commits, setCommits] = useState<LocalGitCommit[]>([]);
  const [diff, setDiff] = useState('');
  const [diffStaged, setDiffStaged] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [initOpen, setInitOpen] = useState(false);
  const [cloneOpen, setCloneOpen] = useState(false);
  const [branchOpen, setBranchOpen] = useState(false);
  const [commitOpen, setCommitOpen] = useState(false);
  const [initialBranch, setInitialBranch] = useState('main');
  const [remoteURL, setRemoteURL] = useState('');
  const [cloneBranch, setCloneBranch] = useState('');
  const [newBranch, setNewBranch] = useState('');
  const [branchBase, setBranchBase] = useState('');
  const [commitMessage, setCommitMessage] = useState('');
  const [authorName, setAuthorName] = useState('');
  const [authorEmail, setAuthorEmail] = useState('');

  const refresh = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const nextStatus = await api.localGitStatus(repository.id);
      setStatus(nextStatus);
      if (nextStatus.initialized) {
        const [nextBranches, nextCommits] = await Promise.all([
          api.localGitBranches(repository.id),
          api.localGitLog(repository.id, 25),
        ]);
        setBranches(nextBranches);
        setCommits(nextCommits);
      } else {
        setBranches([]);
        setCommits([]);
        setDiff('');
      }
    } catch (err) {
      setError(messageFrom(err));
      setStatus(null);
    } finally {
      setLoading(false);
    }
  }, [repository.id]);

  useEffect(() => { void refresh(); }, [refresh]);

  async function perform(action: () => Promise<unknown>, success: string) {
    if (busy) return;
    setBusy(true);
    setError('');
    setNotice('');
    try {
      await action();
      setNotice(success);
      await refresh();
    } catch (err) {
      setError(messageFrom(err));
    } finally {
      setBusy(false);
    }
  }

  async function initialize() {
    await perform(
      () => api.initializeLocalGit(repository.id, { branch: initialBranch.trim() || 'main' }),
      'A real Git working copy was initialized locally.',
    );
    setInitOpen(false);
  }

  async function clone() {
    await perform(
      () => api.cloneLocalGit(repository.id, { remote_url: remoteURL.trim(), branch: cloneBranch.trim() }),
      'Repository cloned into the IDIOS-managed working-copy directory.',
    );
    setCloneOpen(false);
  }

  async function createBranch() {
    await perform(
      () => api.createLocalGitBranch(repository.id, { name: newBranch.trim(), base: branchBase.trim() }),
      'Local branch created and checked out.',
    );
    setNewBranch('');
    setBranchBase('');
    setBranchOpen(false);
  }

  async function switchBranch(name: string) {
    if (!name || name === status?.branch) return;
    await perform(() => api.switchLocalGitBranch(repository.id, { name }), `Switched to ${name}.`);
  }

  async function stageAll() {
    await perform(() => api.stageLocalGit(repository.id, { paths: [] }), 'All working-tree changes were staged locally.');
  }

  async function commit() {
    await perform(
      () => api.createLocalGitCommit(repository.id, {
        message: commitMessage.trim(),
        author_name: authorName.trim(),
        author_email: authorEmail.trim(),
      }),
      'Local Git commit created.',
    );
    setCommitMessage('');
    setCommitOpen(false);
  }

  async function loadDiff(staged: boolean) {
    setError('');
    setDiffStaged(staged);
    try {
      const result = await api.localGitDiff(repository.id, staged);
      setDiff(result.diff);
    } catch (err) {
      setError(messageFrom(err));
    }
  }

  if (loading && !status) {
    return <div className="local-git-workspace"><p className="repository-detail-loading">Checking local Git availability…</p></div>;
  }

  if (status && !status.available) {
    return <div className="local-git-workspace">
      <div className="local-git-workspace__unavailable">
        <Terminal size={22}/>
        <div>
          <strong>Local Git is unavailable on this device</strong>
          <p>{status.reason || 'The current platform has no configured LocalGitEngine backend.'}</p>
          <small>Provider connections are not used for local Git operations. Install Git on this desktop, or add a native engine for this platform.</small>
        </div>
      </div>
    </div>;
  }

  if (!status?.initialized) {
    return <div className="local-git-workspace">
      <div className="local-git-workspace__intro">
        <div className="local-git-workspace__mark"><GitBranch size={21}/></div>
        <div>
          <p className="eyebrow">REAL LOCAL GIT</p>
          <h3>{repository.name}</h3>
          <p>This working copy is separate from IDIOS simulated branches, commits and release snapshots. Initialize an empty repository or clone an upstream Git repository.</p>
        </div>
      </div>
      {error && <div className="notice notice--error" role="alert">{error}</div>}
      {notice && <div className="notice notice--success" role="status">{notice}</div>}
      <div className="local-git-workspace__setup">
        <section>
          <GitBranch size={19}/>
          <strong>Initialize local repository</strong>
          <p>Create a new Git repository in IDIOS-managed storage. This works without a network connection.</p>
          <Button onClick={() => setInitOpen(true)}><Plus size={14}/> Initialize repository</Button>
        </section>
        <section>
          <GitPullRequest size={19}/>
          <strong>Clone an existing repository</strong>
          <p>Fetch an upstream repository into a separate local working copy. Cloning requires network access.</p>
          <Button variant="secondary" onClick={() => setCloneOpen(true)}>Clone repository</Button>
        </section>
      </div>
      <Modal open={initOpen} title="Initialize local Git" description={`Create a working copy for ${repository.name}. No provider connection is required.`} onClose={() => setInitOpen(false)} onSubmit={initialize} submitLabel="Initialize" busy={busy}>
        <Field label="Initial branch"><input autoFocus value={initialBranch} onChange={(event) => setInitialBranch(event.target.value)} required maxLength={240} placeholder="main"/></Field>
      </Modal>
      <Modal open={cloneOpen} title="Clone repository" description="The HTTPS or SSH URL is used only for this clone operation. Credentials embedded in the URL are rejected." onClose={() => setCloneOpen(false)} onSubmit={clone} submitLabel="Clone" busy={busy} wide>
        <Field label="Clone URL"><input autoFocus value={remoteURL} onChange={(event) => setRemoteURL(event.target.value)} required placeholder="https://github.com/owner/repository.git"/></Field>
        <Field label="Branch (optional)"><input value={cloneBranch} onChange={(event) => setCloneBranch(event.target.value)} placeholder="Leave empty to use the remote default branch"/></Field>
      </Modal>
    </div>;
  }

  const hasChanges = !status.clean;
  return <div className="local-git-workspace">
    <header className="local-git-workspace__header">
      <div>
        <p className="eyebrow">LOCAL WORKING COPY</p>
        <h3>{repository.name}</h3>
        <p>Git operations below run against the local working copy and do not contact a provider unless you explicitly clone.</p>
      </div>
      <Button variant="secondary" onClick={() => void refresh()} loading={loading}><RefreshCw size={14}/> Refresh</Button>
    </header>
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    {notice && <div className="notice notice--success" role="status">{notice}</div>}
    <div className="local-git-workspace__summary">
      <div><span>Current branch</span><strong><GitBranch size={14}/>{status.branch || 'Detached HEAD'}</strong></div>
      <div><span>Working tree</span><strong><Badge tone={status.clean ? 'green' : 'amber'}>{status.clean ? 'Clean' : 'Changes present'}</Badge></strong></div>
      <div><span>Staged</span><strong>{status.staged}</strong></div>
      <div><span>Unstaged</span><strong>{status.unstaged}</strong></div>
      <div><span>Untracked</span><strong>{status.untracked}</strong></div>
    </div>
    <div className="local-git-workspace__toolbar">
      <Field label="Branch">
        <select value={status.branch || ''} onChange={(event) => void switchBranch(event.target.value)}>
          {branches.map((item) => <option key={item.name} value={item.name}>{item.name}{item.current ? ' · current' : ''}</option>)}
        </select>
      </Field>
      <Button variant="secondary" onClick={() => { setBranchBase(status.branch || ''); setBranchOpen(true); }}><Plus size={14}/> New branch</Button>
      <Button variant="secondary" disabled={!hasChanges || busy} onClick={() => void stageAll()}>Stage all changes</Button>
      <Button disabled={!status.staged || busy} onClick={() => setCommitOpen(true)}><GitCommitHorizontal size={14}/> Commit staged</Button>
    </div>
    <section className="local-git-workspace__section">
      <div className="local-git-workspace__section-heading">
        <div><strong>Diff</strong><span>Inspect unstaged or staged changes before committing.</span></div>
        <div className="local-git-workspace__actions">
          <Button size="sm" variant={!diffStaged ? 'secondary' : 'ghost'} onClick={() => void loadDiff(false)}>Unstaged diff</Button>
          <Button size="sm" variant={diffStaged ? 'secondary' : 'ghost'} onClick={() => void loadDiff(true)}>Staged diff</Button>
        </div>
      </div>
      <pre className="local-git-workspace__diff">{diff || 'Choose a diff view. A clean working tree has no changes to display.'}</pre>
    </section>
    <section className="local-git-workspace__section">
      <div className="local-git-workspace__section-heading">
        <div><strong>Local commit history</strong><span>{commits.length} recent commits · actual Git objects</span></div>
      </div>
      {commits.length ? <div className="local-git-workspace__commits">{commits.map((item) => <article key={item.sha}>
        <code>{item.sha.slice(0, 12)}</code>
        <div><strong>{item.message}</strong><span>{item.author} &lt;{item.author_email}&gt;</span><small>Committed by {item.committer} &lt;{item.commit_email}&gt; · {new Date(item.commit_date).toLocaleString()}</small></div>
      </article>)}</div> : <EmptyState title="No local commits yet" body="Stage your changes and create the first Git commit." icon={<GitCommitHorizontal size={20}/>}/>}
    </section>
    <Modal open={branchOpen} title="Create local branch" description="The new Git ref is created inside this working copy and is available offline." onClose={() => setBranchOpen(false)} onSubmit={createBranch} submitLabel="Create branch" busy={busy}>
      <Field label="Branch name"><input autoFocus required value={newBranch} onChange={(event) => setNewBranch(event.target.value)} placeholder="feature/my-change"/></Field>
      <Field label="Start from"><select value={branchBase} onChange={(event) => setBranchBase(event.target.value)}><option value="">Current HEAD</option>{branches.map((item) => <option key={item.name} value={item.name}>{item.name}</option>)}</select></Field>
    </Modal>
    <Modal open={commitOpen} title="Commit staged changes" description="This writes a real Git commit to the local object database. It does not push anything." onClose={() => setCommitOpen(false)} onSubmit={commit} submitLabel="Create commit" busy={busy} wide>
      <Field label="Commit message"><textarea autoFocus required rows={4} value={commitMessage} onChange={(event) => setCommitMessage(event.target.value)} placeholder="Describe the change"/></Field>
      <div className="local-git-workspace__identity">
        <Field label="Author name (optional)"><input value={authorName} onChange={(event) => setAuthorName(event.target.value)} placeholder="Use configured Git identity"/></Field>
        <Field label="Author email (optional)"><input value={authorEmail} onChange={(event) => setAuthorEmail(event.target.value)} placeholder="Use configured Git identity"/></Field>
      </div>
    </Modal>
  </div>;
}
