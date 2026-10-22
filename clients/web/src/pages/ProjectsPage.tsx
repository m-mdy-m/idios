import { useEffect, useState, type FormEvent } from 'react';
import { ArrowRight, ExternalLink, FolderKanban, GitBranch, Link2, Network, Plus, Search, Sparkles, Star, Trash2 } from 'lucide-react';
import { useNavigate } from 'react-router';
import { api } from '@/lib/api';
import { useWorkspaceStore } from '@/store/workspace-store';
import type { Issue, Project, ProjectRepository } from '@/types/idios';
import { Badge, Button, Card, EmptyState, Field, PageHeading } from '@/components/ui/primitives';
import { Modal } from '@/components/ui/Modal';
import { IssueTable } from '@/components/ui/IssueTable';

const repositorySlug = (name: string) => name.normalize('NFKD').toLowerCase().replace(/[\u0300-\u036f]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');

export function ProjectsPage() {
  const { dashboard, load } = useWorkspaceStore();
  const navigate = useNavigate();
  const [selected, setSelected] = useState<Project | null>(null);
  const [projectIssues, setProjectIssues] = useState<Issue[]>([]);
  const [projectRepositories, setProjectRepositories] = useState<ProjectRepository[]>([]);
  const [projectQuery, setProjectQuery] = useState('');
  const [open, setOpen] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const [linkOpen, setLinkOpen] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [issueRef, setIssueRef] = useState('');
  const [repositoryRef, setRepositoryRef] = useState('');
  const [relationship, setRelationship] = useState('');
  const [makePrimary, setMakePrimary] = useState(false);
  const [busy, setBusy] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<Project | null>(null);
  const [error, setError] = useState('');

  async function selectProject(project: Project) {
    setSelected(project); setError('');
    try {
      const [issues, repositories] = await Promise.all([api.projectIssues(project.id), api.projectRepositories(project.id)]);
      setProjectIssues(issues);
      setProjectRepositories(repositories);
    } catch (err) {
      setProjectIssues([]); setProjectRepositories([]);
      setError(err instanceof Error ? err.message : 'Unable to load project details.');
    }
  }
  useEffect(() => {
    if (selected) {
      const current = (dashboard?.projects ?? []).find((item) => item.id === selected.id);
      if (current) setSelected(current);
    }
  }, [dashboard?.projects, selected?.id]);

  const projects = (dashboard?.projects ?? []).filter((project) => `${project.name} ${project.description}`.toLowerCase().includes(projectQuery.toLowerCase()));
  const availableRepositories = (dashboard?.repositories ?? []).filter((repo) => !projectRepositories.some((link) => link.repository_id === repo.id));

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError('');
    try {
      const project = await api.createProject({ name: name.trim(), description: description.trim() });
      setOpen(false); setName(''); setDescription(''); await load(true); await selectProject(project);
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to create project.'); }
    finally { setBusy(false); }
  }
  async function addIssue(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!selected) return; setBusy(true); setError('');
    try {
      await api.addProjectIssue(selected.id, issueRef.trim());
      setAddOpen(false); setIssueRef(''); await selectProject(selected); await load(true);
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to add issue to project.'); }
    finally { setBusy(false); }
  }
  async function linkRepository(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!selected || !repositoryRef) return; setBusy(true); setError('');
    try {
      await api.linkProjectRepository(selected.id, { repository: repositoryRef, relationship: relationship.trim(), primary: makePrimary || projectRepositories.length === 0 });
      setLinkOpen(false); setRepositoryRef(''); setRelationship(''); setMakePrimary(false);
      await selectProject(selected); await load(true);
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to link repository to project.'); }
    finally { setBusy(false); }
  }
  async function makeRepositoryPrimary(repository: ProjectRepository) {
    if (!selected) return; setBusy(true); setError('');
    try { await api.setPrimaryProjectRepository(selected.id, repository.repository_id); await selectProject(selected); await load(true); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to change primary repository.'); }
    finally { setBusy(false); }
  }
  async function unlinkRepository(repository: ProjectRepository) {
    if (!selected) return; setBusy(true); setError('');
    try { await api.unlinkProjectRepository(selected.id, repository.repository_id); await selectProject(selected); await load(true); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to unlink repository.'); }
    finally { setBusy(false); }
  }
  async function removeProject(project: Project) {
    setBusy(true); setError('');
    try {
      await api.executeCommand('project.delete', { ref: project.id });
      setPendingDelete(null); setSelected(null); setProjectIssues([]); setProjectRepositories([]); await load(true);
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to delete project.'); }
    finally { setBusy(false); }
  }

  return <div className="page-stack">
    <PageHeading eyebrow="PLANNING" title="Projects" description="Group repositories and issues around one goal. Each project has a primary repository and its own focused knowledge graph." actions={<Button onClick={() => setOpen(true)}><Plus size={16} /> New project</Button>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    <div className="projects-layout">
      <Card className="project-browser">
        <div className="project-browser__head"><div><p className="eyebrow">PROJECT SPACE</p><h2>All projects <span>{projects.length}</span></h2></div><label className="inline-search"><Search size={15} /><input value={projectQuery} onChange={(event) => setProjectQuery(event.target.value)} placeholder="Search projects" /></label></div>
        {projects.length ? <div className="project-list">{projects.map((project, index) => <button className={`project-list-item ${selected?.id === project.id ? 'project-list-item--selected' : ''}`} onClick={() => void selectProject(project)} key={project.id}><span className={`project-color project-color--${index % 5}`}><FolderKanban size={17} /></span><span className="project-list-item__copy"><strong>{project.name}</strong><small>{project.description || 'No description'}</small></span><ArrowRight size={15} /></button>)}</div> : <EmptyState title="No projects yet" body="Create a project to group related repositories and issues." icon={<FolderKanban size={21} />} />}
        <div className="project-browser__footer"><Sparkles size={14} /><span>The project graph connects repositories, issues and relationships in one view.</span></div>
      </Card>

      <Card className="project-detail">
        {selected ? <>
          <div className="project-detail__top"><span className="project-detail-icon"><FolderKanban size={18} /></span><div className="project-detail__title"><p className="eyebrow">PROJECT DETAILS</p><h2>{selected.name}</h2></div><Button variant="secondary" onClick={() => setLinkOpen(true)}><Link2 size={15} /> Link repository</Button><Button variant="secondary" onClick={() => setAddOpen(true)}><Plus size={15} /> Add issue</Button><Button variant="danger" disabled={busy} onClick={() => setPendingDelete(selected)} title="Delete this project"><Trash2 size={14} /> Delete</Button></div>
          <p className="project-detail__description">{selected.description || 'Add a short description to explain the goal and scope of this project.'}</p>
          <div className="project-detail__metrics"><div><strong>{projectRepositories.length}</strong><span>Repositories</span></div><div><strong>{projectIssues.length}</strong><span>Issues</span></div><div><strong>{projectIssues.filter((item) => item.status === 'done').length}</strong><span>Completed</span></div></div>

          <section className="project-section"><div className="section-heading project-issues-heading"><div><p className="eyebrow">SOURCE CODE</p><h2>Linked repositories</h2></div><Badge tone="neutral">{projectRepositories.length} linked</Badge></div>
            {projectRepositories.length ? <div className="project-repository-list">{projectRepositories.map((repo) => <div className="project-repository-row" key={repo.repository_id}>
              <span className="project-repository-icon"><GitBranch size={16} /></span><div className="project-repository-copy"><button className="project-repository-name" onClick={() => navigate(`/repositories/${repositorySlug(repo.repository_name)}`)}>{repo.repository_name}<ExternalLink size={12} /></button><span>{repo.relationship || repo.repository_description || 'No role or relationship described yet.'}</span></div>
              {repo.is_primary ? <Badge tone="green"><Star size={11} /> Primary</Badge> : <Button variant="ghost" disabled={busy} onClick={() => void makeRepositoryPrimary(repo)}>Make primary</Button>}
              <Button variant="ghost" disabled={busy} title={`Unlink ${repo.repository_name}`} onClick={() => void unlinkRepository(repo)}><Trash2 size={14} /></Button>
            </div>)}</div> : <div className="project-inline-empty"><GitBranch size={16} /><span>Link the main repository, then add supporting or reference repositories such as 386bsd or xv6-riscv.</span><Button variant="secondary" onClick={() => setLinkOpen(true)}><Plus size={14} /> Link first repository</Button></div>}
          </section>

          <section className="project-section"><div className="section-heading project-issues-heading"><div><p className="eyebrow">CONNECTED WORK</p><h2>Issues in this project</h2></div><Badge tone="neutral">{projectIssues.length} linked</Badge></div><IssueTable issues={projectIssues} onChange={() => selected ? selectProject(selected) : undefined} emptyTitle="Nothing linked yet" emptyBody="Add an issue using its repository reference, such as XORON5#12, to bring it into this project." /></section>
          <section className="project-graph-callout"><span><Network size={17} /></span><div><strong>Project knowledge graph</strong><p>View this project with its repositories, issues and recorded relationships. These links are also part of the main workspace graph.</p></div><Button variant="secondary" onClick={() => navigate(`/graph?from=${encodeURIComponent(`project:${selected.name}`)}&depth=3`)}><Network size={14} /> Open graph</Button></section>
        </> : <div className="project-detail-empty"><span className="project-detail-empty__icon"><FolderKanban size={24} /></span><h2>Make the plan visible</h2><p>Select a project to see its linked repositories and issues, or create one to start grouping work.</p><Button variant="secondary" onClick={() => setOpen(true)}><Plus size={15} /> Create a project</Button></div>}
      </Card>
    </div>

    <Modal open={open} title="Create a project" description="Define a goal that groups repositories and issues." onClose={() => setOpen(false)} onSubmit={create} submitLabel="Create project" busy={busy}><>{error && <div className="inline-error">{error}</div>}<Field label="Project name"><input autoFocus required value={name} onChange={(event) => setName(event.target.value)} placeholder="e.g. XORON" /></Field><Field label="Short description"><textarea rows={3} value={description} onChange={(event) => setDescription(event.target.value)} placeholder="A short explanation of what this project is about." /></Field></></Modal>

    <Modal open={linkOpen} title="Link a repository" description={`Choose a repository and describe its role in ${selected?.name ?? 'this project'}.`} onClose={() => setLinkOpen(false)} onSubmit={linkRepository} submitLabel="Link repository" busy={busy}>
      <>{error && <div className="inline-error">{error}</div>}
        <Field label="Repository"><select autoFocus required value={repositoryRef} onChange={(event) => setRepositoryRef(event.target.value)}><option value="">Choose a repository…</option>{availableRepositories.map((repo) => <option value={repo.id} key={repo.id}>{repo.name}</option>)}</select></Field>
        {!availableRepositories.length && <p className="form-hint">All existing repositories are already linked to this project.</p>}
        <Field label="Relationship / role" hint="For example: main implementation, upstream reference, OS research, or design inspiration."><input value={relationship} onChange={(event) => setRelationship(event.target.value)} placeholder="Main implementation" /></Field>
        <label className="project-primary-check"><input type="checkbox" checked={makePrimary || projectRepositories.length === 0} disabled={projectRepositories.length === 0} onChange={(event) => setMakePrimary(event.target.checked)} /><span><strong>Primary repository</strong><small>This is the main codebase for the project.</small></span></label>
      </>
    </Modal>

    <Modal open={addOpen} title="Add an issue" description={`Link an existing issue to ${selected?.name ?? 'this project'}.`} onClose={() => setAddOpen(false)} onSubmit={addIssue} submitLabel="Add issue" busy={busy}><>{error && <div className="inline-error">{error}</div>}<Field label="Issue reference or ID"><input autoFocus required value={issueRef} onChange={(event) => setIssueRef(event.target.value)} placeholder="e.g. XORON5#12 or issue ID" list="project-issue-suggestions" /><datalist id="project-issue-suggestions">{(dashboard?.issues ?? []).map((item) => <option key={item.id} value={`${item.repository_name || (dashboard?.repositories ?? []).find((repo) => repo.id === item.repository_id)?.name || ''}#${item.number}`}>{item.title}</option>)}</datalist></Field><p className="form-hint">Issues remain owned by their repository, but can appear in multiple projects.</p></></Modal>
    <Modal open={Boolean(pendingDelete)} title={`Delete project “${pendingDelete?.name || ''}”?`} description="Its issue and repository links will be removed. The repositories and issues themselves will remain." onClose={() => setPendingDelete(null)} onSubmit={(event) => { event.preventDefault(); if (pendingDelete) void removeProject(pendingDelete); }} submitLabel="Delete project" submitVariant="danger" busy={busy}>
      <div className="delete-confirm-summary"><span className="delete-confirm-summary__icon"><Trash2 size={17} /></span><div><strong>{pendingDelete?.name}</strong><p>{pendingDelete?.description || 'No description provided.'}</p></div></div>
      {error && <div className="notice notice--error" role="alert">{error}</div>}
    </Modal>
  </div>;
}
