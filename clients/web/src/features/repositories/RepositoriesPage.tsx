import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { Activity, ArrowUpRight, Box, CheckCircle2, ChevronDown, ChevronRight, FileText, Folder, FolderOpen, GitBranch, GitPullRequest, Layers3, Plus, Search, Tags, Pencil } from 'lucide-react';
import { api } from '@/lib/api';
import { useNavigate, useParams, useSearchParams } from 'react-router';
import { useWorkspaceStore } from '@/store/workspace-store';
import type { ActivityEvent, Branch, CompletionRecord, Issue, Label, Milestone, Project, ProjectRepository, ProtocolCommand, PullRequest, Repository, RepositoryFile, RepositoryTag, RepositoryTagComparison, ScheduledJob } from '@/types/idios';
import { Badge, Button, Card, EmptyState, Field, PageHeading } from '@/design-system';
import { Modal } from '@/design-system';
import { CommandRunner } from '@/features/commands';
import { IssueCreateDialog, IssueDetails } from '@/features/issues';
import { RepositoryHubPanel } from '@/features/repositories/RepositoryHubPanel';
import { LocalGitWorkspace } from '@/features/repositories/LocalGitWorkspace';
import { formatRelative } from '@/lib/format-relative';

const tabs = ['Overview', 'Files', 'Git', 'Issues', 'Reviews', 'Branches', 'History', 'Labels', 'Milestones', 'Projects', 'Settings', 'Tags', 'Activity', 'Graph'] as const;
const visibleTabs: readonly RepoTab[] = ['Files', 'Git', 'Issues', 'Reviews', 'History', 'Projects', 'Settings'];
type RepoTab = typeof tabs[number];
const repositorySlug = (name: string) => name.normalize('NFKD').toLowerCase().replace(/[\u0300-\u036f]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');

type VirtualFileTreeNode = { name: string; path: string; isDirectory: boolean; file?: RepositoryFile; children: VirtualFileTreeNode[] };

function buildVirtualFileTree(files: RepositoryFile[]): VirtualFileTreeNode[] {
  const root: VirtualFileTreeNode[] = [];
  for (const file of files) {
    const segments = file.path.split('/').filter(Boolean);
    let children = root;
    let parentPath = '';
    segments.forEach((segment, index) => {
      parentPath = parentPath ? `${parentPath}/${segment}` : segment;
      const isFile = index === segments.length - 1;
      let node = children.find((candidate) => candidate.name === segment);
      if (!node) {
        node = { name: segment, path: parentPath, isDirectory: !isFile, children: [] };
        children.push(node);
      }
      if (isFile) { node.isDirectory = false; node.file = file; }
      else { node.isDirectory = true; children = node.children; }
    });
  }
  const sort = (nodes: VirtualFileTreeNode[]) => {
    nodes.sort((a, b) => Number(b.isDirectory) - Number(a.isDirectory) || a.name.localeCompare(b.name));
    nodes.forEach((node) => sort(node.children));
  };
  sort(root);
  return root;
}

function renderInlineMarkdown(source: string, keyPrefix: string): ReactNode[] {
  const tokenPattern = /(`[^`]+`|\*\*[^*]+\*\*|__[^_]+__|~~[^~]+~~|\[[^\]]+\]\([^)]+\)|\*[^*\n]+\*|_[^_\n]+_)/g;
  const result: ReactNode[] = [];
  let cursor = 0;
  let tokenIndex = 0;
  for (const match of source.matchAll(tokenPattern)) {
    const token = match[0];
    const index = match.index ?? 0;
    if (index > cursor) result.push(source.slice(cursor, index));
    const key = `${keyPrefix}-${tokenIndex++}`;
    if (token.startsWith('`') && token.endsWith('`')) {
      result.push(<code key={key}>{token.slice(1, -1)}</code>);
    } else if ((token.startsWith('**') && token.endsWith('**')) || (token.startsWith('__') && token.endsWith('__'))) {
      result.push(<strong key={key}>{token.slice(2, -2)}</strong>);
    } else if (token.startsWith('~~') && token.endsWith('~~')) {
      result.push(<del key={key}>{token.slice(2, -2)}</del>);
    } else if ((token.startsWith('*') && token.endsWith('*')) || (token.startsWith('_') && token.endsWith('_'))) {
      result.push(<em key={key}>{token.slice(1, -1)}</em>);
    } else if (token.startsWith('[')) {
      const link = token.match(/^\[([^\]]+)\]\(([^)]+)\)$/);
      const href = link?.[2]?.trim() ?? '';
      if (link && /^(https?:\/\/|mailto:)/i.test(href)) {
        result.push(<a key={key} href={href} target="_blank" rel="noreferrer noopener">{link[1]}</a>);
      } else {
        result.push(token);
      }
    } else {
      result.push(token);
    }
    cursor = index + token.length;
  }
  if (cursor < source.length) result.push(source.slice(cursor));
  return result;
}

function RepositoryMarkdownPreview({ source }: { source: string }) {
  const lines = source.replace(/\r\n?/g, '\n').split('\n');
  const blocks: ReactNode[] = [];
  let index = 0;
  let blockIndex = 0;
  const isBlockStart = (line: string) => /^\s*```/.test(line) || /^#{1,6}\s+/.test(line) || /^\s*(?:[-*_]\s*){3,}$/.test(line) || /^\s*>/.test(line) || /^\s*(?:[-+*]|\d+\.)\s+/.test(line);

  while (index < lines.length) {
    const line = lines[index];
    if (!line.trim()) { index += 1; continue; }
    const key = `md-block-${blockIndex++}`;

    if (/^\s*```/.test(line)) {
      const language = line.trim().slice(3).trim();
      index += 1;
      const codeLines: string[] = [];
      while (index < lines.length && !/^\s*```/.test(lines[index])) codeLines.push(lines[index++]);
      if (index < lines.length) index += 1;
      blocks.push(<pre className="simrepo-readme-code" key={key} data-language={language || undefined}><code>{codeLines.join('\n')}</code></pre>);
      continue;
    }

    const heading = line.match(/^(#{1,6})\s+(.+?)\s*#*\s*$/);
    if (heading) {
      const level = heading[1].length;
      const content = renderInlineMarkdown(heading[2], key);
      const common = { key, className: `simrepo-readme-h${level}` };
      if (level === 1) blocks.push(<h1 {...common}>{content}</h1>);
      else if (level === 2) blocks.push(<h2 {...common}>{content}</h2>);
      else if (level === 3) blocks.push(<h3 {...common}>{content}</h3>);
      else if (level === 4) blocks.push(<h4 {...common}>{content}</h4>);
      else if (level === 5) blocks.push(<h5 {...common}>{content}</h5>);
      else blocks.push(<h6 {...common}>{content}</h6>);
      index += 1;
      continue;
    }

    if (/^\s*(?:[-*_]\s*){3,}$/.test(line)) {
      blocks.push(<hr key={key} />); index += 1; continue;
    }

    if (/^\s*>/.test(line)) {
      const quote: string[] = [];
      while (index < lines.length && /^\s*>/.test(lines[index])) quote.push(lines[index++].replace(/^\s*>\s?/, ''));
      blocks.push(<blockquote key={key}>{quote.map((entry, entryIndex) => <p key={`${key}-${entryIndex}`}>{renderInlineMarkdown(entry, `${key}-${entryIndex}`)}</p>)}</blockquote>);
      continue;
    }

    const listMatch = line.match(/^\s*([-+*]|\d+\.)\s+(.+)$/);
    if (listMatch) {
      const ordered = /^\d+\.$/.test(listMatch[1]);
      const items: string[] = [];
      while (index < lines.length) {
        const item = lines[index].match(/^\s*([-+*]|\d+\.)\s+(.+)$/);
        if (!item || /^\d+\.$/.test(item[1]) !== ordered) break;
        items.push(item[2]); index += 1;
      }
      blocks.push(ordered
        ? <ol key={key}>{items.map((item, itemIndex) => <li key={`${key}-${itemIndex}`}>{renderInlineMarkdown(item, `${key}-${itemIndex}`)}</li>)}</ol>
        : <ul key={key}>{items.map((item, itemIndex) => <li key={`${key}-${itemIndex}`}>{renderInlineMarkdown(item, `${key}-${itemIndex}`)}</li>)}</ul>);
      continue;
    }

    const paragraph: string[] = [line];
    index += 1;
    while (index < lines.length && lines[index].trim() && !isBlockStart(lines[index])) paragraph.push(lines[index++]);
    blocks.push(<p key={key}>{renderInlineMarkdown(paragraph.join(' '), key)}</p>);
  }

  return <div className="simrepo-readme-preview">{blocks.length ? blocks : <p className="simrepo-muted">This README is empty. Edit README.md to introduce this repository.</p>}</div>;
}

export function RepositoriesPage() {
  const { dashboard, load } = useWorkspaceStore();
  const navigate = useNavigate();
  const { slug } = useParams();
  const repositoryRoute = Boolean(slug);
  const [searchParams, setSearchParams] = useSearchParams();
  const [query, setQuery] = useState('');
  const [workspaceMode, setWorkspaceMode] = useState<'local' | 'connected'>('connected');
  const [open, setOpen] = useState(false);
  const [issueCreateOpen, setIssueCreateOpen] = useState(false);
  const [selectedIssue, setSelectedIssue] = useState<Issue | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [initialBranch, setInitialBranch] = useState('main');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [selected, setSelected] = useState<Repository | null>(null);
  const [tab, setTab] = useState<RepoTab>('Files');
  const [branches, setBranches] = useState<Branch[]>([]);
  const [jobs, setJobs] = useState<ScheduledJob[]>([]);
  const [activeBranchID, setActiveBranchID] = useState('');
  const [pullRequests, setPullRequests] = useState<PullRequest[]>([]);
  const [labels, setLabels] = useState<Label[]>([]);
  const [milestones, setMilestones] = useState<Milestone[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectLinks, setProjectLinks] = useState<ProjectRepository[]>([]);
  const [projectsBusy, setProjectsBusy] = useState(false);
  const [projectBusy, setProjectBusy] = useState(false);
  const projectActionLock = useRef(false);
  const [projectToLink, setProjectToLink] = useState('');
  const [projectCreateOpen, setProjectCreateOpen] = useState(false);
  const [newProjectName, setNewProjectName] = useState('');
  const [newProjectDescription, setNewProjectDescription] = useState('');
  const [projectForIssues, setProjectForIssues] = useState('');
  const [projectIssues, setProjectIssues] = useState<Issue[]>([]);
  const [issueToAddToProject, setIssueToAddToProject] = useState('');
  const [activity, setActivity] = useState<ActivityEvent[]>([]);
  const [completions, setCompletions] = useState<CompletionRecord[]>([]);
  const [files, setFiles] = useState<RepositoryFile[]>([]);
  const [tags, setTags] = useState<RepositoryTag[]>([]);
  const [filePath, setFilePath] = useState('');
  const [fileContent, setFileContent] = useState('');
  const [fileEditorOpen, setFileEditorOpen] = useState(false);
  const [expandedDirectories, setExpandedDirectories] = useState<Record<string, boolean>>({});
  const [fileBusy, setFileBusy] = useState(false);
  const [fileMessage, setFileMessage] = useState('');
  const [tagName, setTagName] = useState('');
  const [tagMessage, setTagMessage] = useState('');
  const [tagBusy, setTagBusy] = useState(false);
  const [tagCompare, setTagCompare] = useState<RepositoryTagComparison | null>(null);
  const [expandedCompletionID, setExpandedCompletionID] = useState('');
  const [insightsOpen, setInsightsOpen] = useState(false);
  const [historyIssues, setHistoryIssues] = useState<Issue[]>([]);
  const [expandedHistoryIssueID, setExpandedHistoryIssueID] = useState('');
  const [detailBusy, setDetailBusy] = useState(false);
  const [commands, setCommands] = useState<ProtocolCommand[]>([]);
  const [selectedCommand, setSelectedCommand] = useState<ProtocolCommand | null>(null);
  const [initialValues, setInitialValues] = useState<Record<string, unknown>>({});
  const repositories = useMemo(() => (dashboard?.repositories ?? []).filter((repository) => `${repository.name} ${repository.description}`.toLowerCase().includes(query.toLowerCase())), [dashboard, query]);

  useEffect(() => { let active = true; void api.protocol().then((payload) => { if (active) setCommands(payload.commands); }).catch(() => undefined); return () => { active = false; }; }, []);

  const refreshDetail = useCallback(async (repo: Repository, activeCheck?: () => boolean) => {
    setDetailBusy(true); setError('');
    try {
      const [repoBranches, prs, repoLabels, repoMilestones, repoActivity, repoCompletions, allJobs, repoFiles, repoTags] = await Promise.all([
        api.repositoryBranches(repo.id), api.pullRequests({ repo: repo.id }), api.labels(repo.id), api.milestones('repository', repo.id),
        api.activity({ type: 'repository', id: repo.id, limit: 30 }), api.completions(repo.id), api.jobs(), api.repositoryFiles(repo.id), api.repositoryTags(repo.id),
      ]);
      if (activeCheck && !activeCheck()) return;
      setBranches(repoBranches); setJobs(allJobs); setActiveBranchID((current) => repoBranches.some((item) => item.id === current) ? current : (repoBranches.find((item) => item.is_default)?.id || repoBranches[0]?.id || '')); setPullRequests(prs); setLabels(repoLabels.filter((item) => item.repository_id === repo.id));
      setMilestones(repoMilestones); setActivity(repoActivity); setCompletions(repoCompletions); setFiles(repoFiles); setTags(repoTags);
    } catch (err) { if (!activeCheck || activeCheck()) setError(err instanceof Error ? err.message : 'Unable to load repository details.'); }
    finally { if (!activeCheck || activeCheck()) setDetailBusy(false); }
  }, []);

  const selectedRepositoryID = selected?.id;

  useEffect(() => {
    setProjectForIssues('');
    setProjectIssues([]);
    setProjects([]);
    setProjectLinks([]);
  }, [selectedRepositoryID]);

  useEffect(() => {
    if (!selectedRepositoryID || tab !== 'Projects') return;
    let active = true;
    setProjectsBusy(true);
    setError('');
    void (async () => {
      const projectCatalog = await api.projects();
      const projectMappings = await Promise.all(projectCatalog.map((project) => api.projectRepositories(project.id)));
      if (!active) return;
      const repositoryProjectLinks = projectMappings.flat().filter((link) => link.repository_id === selectedRepositoryID);
      setProjects(projectCatalog);
      setProjectLinks(repositoryProjectLinks);
      setProjectForIssues((current) => repositoryProjectLinks.some((link) => link.project_id === current) ? current : repositoryProjectLinks[0]?.project_id ?? '');
    })().catch((err) => {
      if (active) setError(err instanceof Error ? err.message : 'Unable to load repository projects.');
    }).finally(() => {
      if (active) setProjectsBusy(false);
    });
    return () => { active = false; };
  }, [selectedRepositoryID, tab]);

  useEffect(() => {
    let active = true;
    if (!projectForIssues) { setProjectIssues([]); return; }
    void api.projectIssues(projectForIssues).then((items) => { if (active) setProjectIssues(items); }).catch((err) => { if (active) setError(err instanceof Error ? err.message : 'Unable to load project issues.'); });
    return () => { active = false; };
  }, [projectForIssues]);

  async function runProjectAction(action: () => Promise<void>, fallback: string) {
    if (projectActionLock.current) return;
    projectActionLock.current = true;
    setProjectBusy(true);
    setError('');
    try {
      await action();
    } catch (err) {
      setError(err instanceof Error ? err.message : fallback);
    } finally {
      projectActionLock.current = false;
      setProjectBusy(false);
    }
  }

  async function addRepositoryIssueToProject() {
    if (!selected || !projectForIssues || !issueToAddToProject) return;
    const repository = selected;
    const projectID = projectForIssues;
    const issue = repositoryIssues.find((item) => item.id === issueToAddToProject);
    if (!issue) return;
    await runProjectAction(async () => {
      await api.addProjectIssue(projectID, `${repository.name}#${issue.number}`);
      const updated = await api.projectIssues(projectID);
      setProjectIssues(updated);
      setIssueToAddToProject('');
    }, 'Unable to add issue to project.');
  }

  async function createRepositoryProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected || !newProjectName.trim()) return;
    setBusy(true); setError('');
    try {
      const project = await api.createProject({ name: newProjectName.trim(), description: newProjectDescription.trim() });
      await api.linkProjectRepository(project.id, { repository: selected.id, relationship: 'implementation', primary: true });
      setProjects((current) => [...current, project]);
      const links = await api.projectRepositories(project.id);
      setProjectLinks((current) => [...current, ...links.filter((item) => item.repository_id === selected.id)]);
      setProjectForIssues(project.id);
      setProjectCreateOpen(false); setNewProjectName(''); setNewProjectDescription('');
      await load(true);
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to create project for this repository.'); }
    finally { setBusy(false); }
  }

  async function linkRepositoryProject() {
    if (!selected || !projectToLink) return;
    const repository = selected;
    const projectID = projectToLink;
    await runProjectAction(async () => {
      const existingLinks = await api.projectRepositories(projectID);
      if (!existingLinks.some((link) => link.repository_id === repository.id)) {
        await api.linkProjectRepository(projectID, { repository: repository.id, relationship: 'implementation', primary: existingLinks.length === 0 });
      }
      const links = await api.projectRepositories(projectID);
      setProjectLinks((current) => [...current.filter((item) => item.project_id !== projectID), ...links.filter((item) => item.repository_id === repository.id)]);
      setProjectForIssues(projectID);
      setProjectToLink('');
    }, 'Unable to link project.');
  }

  async function makeRepositoryPrimary(projectID: string) {
    if (!selected) return;
    const repository = selected;
    await runProjectAction(async () => {
      await api.setPrimaryProjectRepository(projectID, repository.id);
      const links = await api.projectRepositories(projectID);
      setProjectLinks((current) => [...current.filter((item) => item.project_id !== projectID), ...links.filter((item) => item.repository_id === repository.id)]);
    }, 'Unable to make this repository primary.');
  }

  async function unlinkRepositoryProject(projectID: string) {
    if (!selected) return;
    const repository = selected;
    await runProjectAction(async () => {
      await api.unlinkProjectRepository(projectID, repository.id);
      setProjectLinks((current) => current.filter((item) => item.project_id !== projectID));
      if (projectForIssues === projectID) {
        const next = projectLinks.find((item) => item.project_id !== projectID)?.project_id ?? '';
        setProjectForIssues(next);
      }
    }, 'Unable to unlink project.');
  }

  function selectTab(next: RepoTab) {
    setTab(next);
    const params = new URLSearchParams(searchParams);
    params.set('tab', next);
    setSearchParams(params, { replace: true });
  }

  function selectRepository(repository: Repository) {
    setSelected(repository);
    setInsightsOpen(false);
    setTab('Files');
    setFileEditorOpen(false); setExpandedDirectories({}); setTagCompare(null); setExpandedCompletionID(''); setHistoryIssues([]);
    const slugPath = repositorySlug(repository.name);
    setSearchParams({}, { replace: true });
    navigate(`/repositories/${slugPath}`);
  }

  useEffect(() => {
    if (!slug) {
      // A deleted repository leaves the former detail URL behind briefly while
      // the dashboard refreshes. Clear only that route-level error on list view.
      setError((current) => current.includes('was not found') ? '' : current);
      return;
    }
    if (!dashboard) return;
    const match = dashboard.repositories.find((item) => repositorySlug(item.name) === slug.toLowerCase() || item.id === slug || item.name.toLowerCase() === slug.toLowerCase());
    if (match) {
      setSelected((current) => current?.id === match.id ? current : match);
      setTab(tabs.find((item) => item.toLowerCase() === (searchParams.get('tab')?.toLowerCase() === 'pull requests' ? 'reviews' : searchParams.get('tab')?.toLowerCase())) ?? 'Files');
      setError('');
    } else {
      setSelected(null);
      setError(`Repository "${slug}" was not found. Return to Repositories and open an existing repository.`);
    }
  }, [slug, dashboard?.repositories, searchParams]);

  useEffect(() => {
    const target = searchParams.get('repo');
    if (!target || !dashboard?.repositories.length) return;
    const match = dashboard.repositories.find((item) => item.id === target || item.name === target);
    if (!match) return;
    setSelected(match);
    const requested = tabs.find((item) => item.toLowerCase() === searchParams.get('tab')?.toLowerCase()) ?? 'Overview';
    setTab(requested);
    const next = new URLSearchParams(searchParams);
    next.delete('repo');
    next.delete('tab');
    setSearchParams(next, { replace: true });
  }, [searchParams, dashboard?.repositories, setSearchParams]);

  useEffect(() => {
    if (!selected) return;
    let active = true;
    void refreshDetail(selected, () => active);
    return () => { active = false; };
  }, [selected, refreshDetail]);

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError('');
    try {
      const created = await api.createRepository({ name: name.trim(), description: description.trim(), default_branch: initialBranch.trim() || 'main' });
      setOpen(false); setName(''); setDescription(''); setInitialBranch('main'); await load(true); selectRepository(created);
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to create repository.'); } finally { setBusy(false); }
  }

  function run(id: string, values: Record<string, unknown> = {}) {
    const command = commands.find((item) => item.id === id);
    if (!command) { setError(`Action ${id} is unavailable in the shared Go protocol.`); return; }
    setInitialValues(values); setSelectedCommand(command);
  }
  async function complete() {
    const completedCommand = selectedCommand?.id;
    setSelectedCommand(null);
    if (completedCommand === 'repository.delete') {
      // Leave the deleted repository route before refreshing the workspace list.
      setSelected(null);
      setError('');
      navigate('/repositories', { replace: true });
      setSearchParams({}, { replace: true });
      await load(true);
      return;
    }
    await load(true);
    if (selected) {
      const latest = (useWorkspaceStore.getState().dashboard?.repositories ?? []).find((item) => item.id === selected.id) ?? selected;
      setSelected(latest);
      await refreshDetail(latest);
    }
  }

  const activeBranch = branches.find((item) => item.id === activeBranchID) ?? branches.find((item) => item.is_default) ?? branches[0];
  const branchTimeline = activeBranch ? [
    { key: `branch:${activeBranch.id}`, time: activeBranch.created_at, kind: 'Branch created', title: activeBranch.name, detail: activeBranch.is_default ? 'Default workflow branch' : `Based on ${branches.find((item) => item.id === activeBranch.base_branch_id)?.name ?? 'another branch'}` },
    ...(dashboard?.issues ?? []).filter((item) => item.repository_id === selected?.id && item.branch_id === activeBranch.id).map((item) => ({ key: `issue:${item.id}`, time: item.updated_at, kind: 'Issue', title: `${selected?.name}#${item.number} · ${item.title}`, detail: item.status.replace('_', ' ') })),
    ...jobs.filter((item) => item.branch_id === activeBranch.id).map((item) => ({ key: `job:${item.id}`, time: item.updated_at, kind: 'Scheduled job', title: item.name, detail: item.enabled ? `Next run ${item.next_run_at ? formatRelative(item.next_run_at) : 'not scheduled'}` : 'Paused' })),
    ...pullRequests.filter((item) => item.source_branch_id === activeBranch.id || item.target_branch_id === activeBranch.id).map((item) => ({ key: `pr:${item.id}`, time: item.updated_at, kind: item.is_draft ? 'Draft review' : 'Review', title: `${selected?.name}!${item.number} · ${item.title}`, detail: `${item.source_branch_name || 'source unset'} → ${item.target_branch_name || 'target unset'} · ${item.status}` })),
    ...completions.filter((item) => item.branch_id === activeBranch.id).map((item) => ({ key: `completion:${item.id}`, time: item.created_at, kind: 'Accepted completion', title: `${selected?.name}#${item.issue_number}`, detail: item.summary })),
  ].sort((a, b) => Date.parse(b.time) - Date.parse(a.time)) : [];

  const repositoryIssues = (dashboard?.issues ?? []).filter((issue) => issue.repository_id === selected?.id);
  const fileTree = useMemo(() => buildVirtualFileTree(files), [files]);

  function renderFileTree(nodes: VirtualFileTreeNode[], depth = 0): ReactNode[] {
    return nodes.flatMap((node) => {
      if (node.isDirectory) {
        const expanded = expandedDirectories[node.path] ?? depth === 0;
        return [
          <button type="button" key={node.path} className="simrepo-file-row simrepo-file-row--directory" style={{ paddingLeft: 12 + depth * 16 }} aria-expanded={expanded} onClick={() => setExpandedDirectories((current) => ({ ...current, [node.path]: !(current[node.path] ?? depth === 0) }))}>
            <span className="simrepo-file-tree__caret">{expanded ? <ChevronDown size={13} /> : <ChevronRight size={13} />}</span><span className="simrepo-file-icon simrepo-file-icon--folder">{expanded ? <FolderOpen size={14} /> : <Folder size={14} />}</span><span>{node.name}</span>
          </button>,
          ...(expanded ? renderFileTree(node.children, depth + 1) : []),
        ];
      }
      const file = node.file;
      if (!file) return [];
      const active = fileEditorOpen && filePath === file.path;
      return [<button type="button" key={file.path} className={`simrepo-file-row ${active ? 'simrepo-file-row--active' : ''}`} style={{ paddingLeft: 18 + depth * 16 }} onClick={() => { setFilePath(file.path); setFileContent(file.content); setFileEditorOpen(true); setFileMessage(''); }}>
        <span className="simrepo-file-tree__caret"><FileText size={13} /></span><span className="simrepo-file-icon">{file.path.toLowerCase().endsWith('.md') ? 'MD' : file.path.includes('.') ? '<>' : 'FILE'}</span><span>{node.name}</span><small>{file.content.length.toLocaleString()} B</small>
      </button>];
    });
  }
  const completedByType = {
    task: repositoryIssues.filter((item) => item.status === 'done' && item.type === 'task').length,
    bug: repositoryIssues.filter((item) => item.status === 'done' && item.type === 'bug').length,
    feature: repositoryIssues.filter((item) => item.status === 'done' && item.type === 'feature').length,
    research: repositoryIssues.filter((item) => item.status === 'done' && item.type === 'research').length,
  };
  async function saveFile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!selected) return; setFileBusy(true); setFileMessage(''); setError('');
    try { await api.saveRepositoryFile(selected.id, { path: filePath, content: fileContent }); setFileEditorOpen(false); setFileMessage(`Saved ${filePath}`); await refreshDetail(selected); }
    catch (err) { setFileMessage(err instanceof Error ? err.message : 'Could not save file'); }
    finally { setFileBusy(false); }
  }
  async function removeFile(file: RepositoryFile) {
    if (!selected || !window.confirm(`Delete ${file.path} from this simulated repository?`)) return;
    try { await api.deleteRepositoryFile(selected.id, file.path); setFileMessage(`Deleted ${file.path}`); await refreshDetail(selected); }
    catch (err) { setFileMessage(err instanceof Error ? err.message : 'Could not delete file'); }
  }
  async function createTag(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!selected) return; setTagBusy(true); setError('');
    try { await api.createRepositoryTag(selected.id, { name: tagName.trim(), message: tagMessage.trim() }); setTagName(''); setTagMessage(''); setTagCompare(null); await refreshDetail(selected); }
    catch (err) { setError(err instanceof Error ? err.message : 'Could not create tag'); }
    finally { setTagBusy(false); }
  }
  async function toggleCompletion(item: CompletionRecord) {
    if (expandedCompletionID === item.id) { setExpandedCompletionID(''); setHistoryIssues([]); setExpandedHistoryIssueID(''); return; }
    setExpandedCompletionID(item.id); setExpandedHistoryIssueID('');
    try { setHistoryIssues(await api.pullRequestIssues(item.pull_request_id)); }
    catch (err) { setError(err instanceof Error ? err.message : 'Could not load merged issues'); setHistoryIssues([]); }
  }
  async function openCompletionInHistory(item: CompletionRecord) {
    setExpandedCompletionID(item.id);
    setExpandedHistoryIssueID('');
    setHistoryIssues([]);
    selectTab('History');
    try { setHistoryIssues(await api.pullRequestIssues(item.pull_request_id)); }
    catch (err) { setError(err instanceof Error ? err.message : 'Could not load merged issues'); }
  }

  function editReadme() {
    const readme = files.find((file) => file.path.toLowerCase() === 'readme.md');
    setFilePath('README.md');
    setFileContent(readme?.content ?? `# ${selected?.name ?? 'Repository'}\n\n${selected?.description ?? ''}\n`);
    setFileEditorOpen(true);
    setFileMessage('');
    selectTab('Files');
  }

  async function showTagChanges(tag: RepositoryTag) {
    if (!selected) return;
    try { setTagCompare(await api.compareRepositoryTag(selected.id, tag.name)); }
    catch (err) { setError(err instanceof Error ? err.message : 'Could not compare this tag'); }
  }

  return <div className="page-stack">
    <PageHeading eyebrow={repositoryRoute ? 'REPOSITORY' : workspaceMode === 'connected' ? 'REMOTE SOURCES' : 'WORKSPACES'} title={repositoryRoute && selected ? selected.name : workspaceMode === 'connected' ? 'Repository Hub' : 'Repositories'} description={repositoryRoute && selected ? (selected.description || 'Repository workspace, issues, branches and history.') : workspaceMode === 'connected' ? 'Connect GitHub, GitLab and SourceForge repositories without mixing them with local workspaces.' : 'Each repository has its own URL and a focused view of its work.'} actions={<>{!repositoryRoute && <Button variant="secondary" onClick={() => setWorkspaceMode((current) => current === 'local' ? 'connected' : 'local')}>{workspaceMode === 'local' ? 'Connected repositories' : 'Local repositories'}</Button>}{repositoryRoute && <Button variant="secondary" onClick={() => { setSelected(null); navigate('/repositories'); }}>All repositories</Button>}{workspaceMode === 'local' && <Button onClick={() => setOpen(true)}><Plus size={16} /> New repository</Button>}</>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    {workspaceMode === 'connected' && !repositoryRoute ? <RepositoryHubPanel /> : <>
    {!repositoryRoute && <><div className="repository-toolbar"><div className="repository-total"><span className="section-symbol"><Layers3 size={17} /></span><div><strong>{dashboard?.repositories.length ?? 0} repositories</strong><p>Independent workspaces · Main branch workflow</p></div></div><label className="inline-search"><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Find a repository…" /></label></div>
    {repositories.length ? <div className="repository-grid">{repositories.map((repository, index) => {
      const issues = (dashboard?.issues ?? []).filter((issue) => issue.repository_id === repository.id);
      const active = issues.filter((issue) => issue.status === 'open' || issue.status === 'in_progress').length;
      return <Card className={`repository-card ${selected?.id === repository.id ? 'repository-card--selected' : ''}`} key={repository.id}><div className="repository-card__top"><span className={`repository-avatar repository-avatar--${index % 5}`}><Box size={19} /></span><button className="icon-button" aria-label={`Open ${repository.name} details`} title="Open repository workspace" onClick={() => selectRepository(repository)}><ArrowUpRight size={16} /></button></div><button className="repository-card-select" onClick={() => selectRepository(repository)}><span className="repository-title"><h2>{repository.name}</h2><Badge tone="neutral">Local workspace</Badge></span><span className="repository-description">{repository.description || 'No description added yet.'}</span></button><div className="repository-stats"><div><strong>{issues.length}</strong><span>Issues</span></div><div><strong>{active}</strong><span>Open work</span></div><div><strong>{issues.filter((item) => item.status === 'done').length}</strong><span>Completed</span></div></div><div className="repository-card__footer"><span><GitBranch size={14} /> {repository.default_branch_id ? 'Default branch configured' : 'Workflow repository'}</span><span>Updated {formatRelative(repository.updated_at)}</span></div></Card>;
    })}</div> : <Card><EmptyState title={query ? 'No matching repositories' : 'Start with a workspace'} body={query ? 'Try changing your search.' : 'Create a repository to group issues and establish a clear working context.'} icon={<Layers3 size={22} />} /></Card>}</>}
    {repositoryRoute && !selected && !error && <Card><EmptyState title="Loading repository" body="Looking up this repository…" icon={<Layers3 size={22} />} /></Card>}
    {repositoryRoute && !selected && error && <Card><EmptyState title="Repository not found" body={error} icon={<Layers3 size={22} />} /></Card>}

    {selected && <Card className="repository-inspector"><div className="repository-inspector__header"><div className="repository-inspector__identity"><span className="repository-avatar repository-avatar--0"><Box size={19} /></span><div><p className="eyebrow">REPOSITORY WORKSPACE</p><h2>{selected.name}</h2><p>{selected.description || 'No description provided.'}</p></div></div><div className="repository-inspector__actions"><Button variant="secondary" onClick={() => run('repository.description.update', { repository: selected.id, description: selected.description })}><Pencil size={14} /> Edit description</Button><Button variant="ghost" onClick={() => { setSelected(null); if (repositoryRoute) navigate('/repositories'); }}>Close</Button></div></div>
      <div className="repository-tabs" role="tablist" aria-label="Repository views">{visibleTabs.map((item) => <button key={item} role="tab" aria-selected={tab === item} className={tab === item ? 'repository-tab repository-tab--active' : 'repository-tab'} onClick={() => selectTab(item)}>{item}</button>)}</div>
      {detailBusy && <p className="repository-detail-loading">Refreshing repository data…</p>}
      <div className="repository-tab-content">
      {tab === 'Overview' && <div className="simrepo-overview">
        <div className="simrepo-overview__hero"><div><p className="eyebrow">SIMULATED REPOSITORY</p><h3>{selected.name}<span className="simrepo-branch-pill"><GitBranch size={13}/>{branches.find((item) => item.is_default)?.name || 'main'}</span></h3><p>{selected.description || 'A local IDIOS workspace with its own simulated files, changes and release tags.'}</p></div><Button variant="secondary" onClick={() => selectTab('Files')}><Box size={14}/> Browse files</Button></div>
        <section className="simrepo-panel simrepo-readme-panel">
          <div className="simrepo-panel__heading"><div><p className="eyebrow">README.md</p><h3>Repository overview</h3></div><Button variant="secondary" onClick={editReadme}><Pencil size={14}/> Edit README</Button></div>
          {files.some((file) => file.path.toLowerCase() === 'readme.md')
            ? <RepositoryMarkdownPreview source={files.find((file) => file.path.toLowerCase() === 'readme.md')?.content ?? ''}/>
            : <div className="simrepo-readme-empty"><FileText size={22}/><div><strong>README.md is missing</strong><p>Add a README to describe what this repository is for.</p></div><Button variant="secondary" onClick={editReadme}><Plus size={14}/> Create README</Button></div>}
        </section>

        <section className={`simrepo-insights ${insightsOpen ? 'simrepo-insights--open' : ''}`}>
          <button className="simrepo-insights__toggle" type="button" aria-expanded={insightsOpen} onClick={() => setInsightsOpen((current) => !current)}>
            <span className="simrepo-insights__mark"><Activity size={17}/></span>
            <span className="simrepo-insights__copy"><strong>Insights</strong><small>{completions.length} commits · {repositoryIssues.filter((item) => item.status === 'done').length} completed issues · {pullRequests.filter((item) => item.status === 'open').length} open reviews · {tags.length} tags</small></span>
            <span className="simrepo-insights__action">{insightsOpen ? 'Hide insights' : 'Show insights'}<ChevronDown size={16}/></span>
          </button>
          {insightsOpen && <div className="simrepo-insights__content">
            <div className="simrepo-metrics"><div><span>Commits / completions</span><strong>{completions.length}</strong><small>Each merged PR creates one record</small></div><div><span>Issues completed</span><strong>{repositoryIssues.filter((item) => item.status === 'done').length}</strong><small>Across all issue types</small></div><div><span>Open pull requests</span><strong>{pullRequests.filter((item) => item.status === 'open').length}</strong><small>{pullRequests.length} total PRs</small></div><div><span>Release tags</span><strong>{tags.length}</strong><small>{tags[0] ? `Latest ${tags[0].name}` : 'No releases yet'}</small></div></div>
            <section className="simrepo-panel"><div className="simrepo-panel__heading"><div><p className="eyebrow">DELIVERED WORK</p><h3>Completed issues by type</h3></div><button className="simrepo-text-action" type="button" onClick={() => selectTab('Issues')}>Browse issues <ArrowUpRight size={14}/></button></div><div className="simrepo-type-grid">{([{key:'task',label:'Tasks',tone:'blue'},{key:'bug',label:'Bugs',tone:'red'},{key:'feature',label:'Features',tone:'violet'},{key:'research',label:'Research',tone:'teal'}] as const).map((item) => <div className={`simrepo-type-card simrepo-type-card--${item.tone}`} key={item.key}><span>{item.label}</span><strong>{completedByType[item.key]}</strong><small>completed</small></div>)}</div></section>
            <section className="simrepo-panel"><div className="simrepo-panel__heading"><div><p className="eyebrow">LATEST MERGES</p><h3>Commit history</h3></div><button className="simrepo-text-action" type="button" onClick={() => selectTab('History')}>Full history <ArrowUpRight size={14}/></button></div>{completions.length ? <div className="simrepo-recent-commits">{completions.slice(0,3).map((item) => { const pr = pullRequests.find((candidate) => candidate.id === item.pull_request_id); return <button key={item.id} type="button" onClick={() => void openCompletionInHistory(item)} className="simrepo-commit-row" title="Open full history at this commit"><span className="simrepo-commit-icon"><GitPullRequest size={16}/></span><span className="simrepo-commit-main"><strong>{pr ? `Review #${pr.number} · ${pr.title}` : `Completion #${item.sequence}`}</strong><small>{item.summary}</small></span><code>{(item.commit_hash || item.id.slice(0,12)).slice(0,12)}</code><time>{formatRelative(item.created_at)}</time></button>; })}</div> : <EmptyState title="No simulated commits yet" body="When you merge a PR, IDIOS records one commit/completion and captures the included issues." icon={<GitPullRequest size={20}/>}/>}</section>
          </div>}
        </section>

        <div className="simrepo-overview-actions"><Button variant="secondary" onClick={() => selectTab('Issues')}>Browse issues</Button><Button variant="secondary" onClick={() => setIssueCreateOpen(true)}>New issue</Button><Button variant="danger" onClick={() => run('repository.delete', { ref: selected.id, __confirm_target: selected.name })}>Delete repository</Button></div>
        <p className="simrepo-footnote"><CheckCircle2 size={14}/> IDIOS simulates repository history internally. It does not run Git or change files on your disk; all file snapshots, commit records and tags are stored in the local Engine database.</p>
      </div>}
      {tab === 'Issues' && <div className="repository-issues-workspace"><div className="repository-detail-toolbar"><div><strong>Issues in {selected.name}</strong><p>Create and track work scoped to this repository.</p></div><div className="repository-detail-toolbar__actions"><Button variant="secondary" onClick={() => selectTab('Labels')}><Tags size={14}/> Labels</Button><Button variant="secondary" onClick={() => selectTab('Milestones')}><Layers3 size={14}/> Milestones</Button><Button onClick={() => setIssueCreateOpen(true)}><Plus size={14} /> New issue</Button></div></div><div className="repository-embedded-list">{(dashboard?.issues ?? []).filter((issue) => issue.repository_id === selected.id).length ? (dashboard?.issues ?? []).filter((issue) => issue.repository_id === selected.id).map((issue) => <div className="repository-simple-row" key={issue.id} role="button" tabIndex={0} onClick={() => setSelectedIssue(issue)} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); setSelectedIssue(issue); } }}><div><strong>{issue.title}</strong><span>{selected.name}#{issue.number} · {issue.status}{issue.blocked ? ' · BLOCKED' : ''} · {(issue.type || 'task').toUpperCase()}</span></div><Badge tone={issue.status === 'done' ? 'green' : issue.status === 'in_progress' ? 'blue' : issue.blocked ? 'amber' : 'neutral'}>{issue.status}</Badge></div>) : <EmptyState title="No issues in this repository" body="Create an issue to start tracking work." icon={<CheckCircle2 size={20} />} />}</div></div>}
      {tab === 'Reviews' && <div className="repository-reviews-workspace">
        <div className="repository-detail-toolbar"><div><strong>Reviews</strong><p>Review records and their linked issues.</p></div><span className="repository-detail-count">{pullRequests.filter((pr) => pr.status === 'open').length} open</span></div>
        <div className="repository-embedded-list">{pullRequests.length ? pullRequests.map((pr) => <article className="repository-review-row" key={pr.id}>
          <div className="repository-review-row__main"><div className="repository-review-row__title"><strong>{pr.title}</strong><Badge tone={pr.is_draft ? 'neutral' : pr.status === 'accepted' ? 'green' : pr.status === 'open' ? 'blue' : 'neutral'}>{pr.is_draft ? 'Draft' : pr.status}</Badge></div>
            <span>{selected.name}!{pr.number} · {pr.source_branch_name || 'source unset'} → {pr.target_branch_name || 'main'} · {(pr.issue_ids?.length ?? 1)} linked issue{(pr.issue_ids?.length ?? 1) === 1 ? '' : 's'}</span>
            {pr.description && <p>{pr.description}</p>}
          </div>
          <div className="repository-review-row__actions">
            {pr.issue_id && <Button variant="ghost" onClick={() => { const issue = repositoryIssues.find((item) => item.id === pr.issue_id); if (issue) setSelectedIssue(issue); else navigate(`/issues?issue=${encodeURIComponent(pr.issue_id)}`); }}>Issue</Button>}
            {pr.status === 'open' && <Button variant="ghost" onClick={() => run('pull_request.edit', { ref: pr.id, title: pr.title, description: pr.description })}>Edit</Button>}
            {pr.status === 'open' && pr.is_draft && <Button variant="secondary" onClick={() => run('pull_request.mark_ready', { ref: pr.id })}>Ready</Button>}
            {pr.status === 'open' && !pr.is_draft && <Button variant="secondary" onClick={() => run('pull_request.accept', { ref: pr.id })}>Accept</Button>}
            {pr.status === 'open' && <Button variant="ghost" onClick={() => run('pull_request.close', { ref: pr.id })}>Close</Button>}
            <Button variant="ghost" onClick={() => selectTab('History')}>History</Button>
          </div>
        </article>) : <EmptyState title="No reviews yet" body="Start work from an issue, then create a review request when it is ready." icon={<GitPullRequest size={20} />} />}</div>
      </div>}
      {tab === 'Branches' && <div className="repository-branches-workspace">
        <div className="repository-detail-toolbar"><Button variant="ghost" onClick={() => selectTab('Files')}>← Files</Button><div><strong>{branches.length} workflow branches</strong><p>Branch records group IDIOS issues, scheduled jobs and review records. They do not create Git refs on disk.</p></div><Button onClick={() => run('branch.create', { repository: selected.id, base_branch: selected.default_branch_id || branches.find((item) => item.is_default)?.id || '' })}><Plus size={14} /> Create branch</Button></div>
        {branches.length ? <div className="repository-branch-layout"><nav className="repository-branch-list" aria-label="Repository branches">{branches.map((branch) => <button type="button" key={branch.id} className={activeBranch?.id === branch.id ? 'repository-branch-choice repository-branch-choice--active' : 'repository-branch-choice'} onClick={() => setActiveBranchID(branch.id)}><GitBranch size={15} /><span><strong>{branch.name}</strong><small>{branch.is_default ? 'Default branch' : `from ${branches.find((item) => item.id === branch.base_branch_id)?.name || 'base branch'}`}</small></span>{branch.is_default && <Badge tone="green">Default</Badge>}</button>)}</nav>
          <section className="branch-timeline"><div className="branch-timeline__header"><div><p className="eyebrow">BRANCH TIMELINE</p><h3>{activeBranch?.name}</h3></div><span>{branchTimeline.length} records</span></div>{branchTimeline.length ? <div className="branch-timeline__items">{branchTimeline.map((item) => <article className="branch-timeline__item" key={item.key}><span className="branch-timeline__dot"/><div><div className="branch-timeline__item-heading"><strong>{item.kind}</strong><time>{formatRelative(item.time)}</time></div><p>{item.title}</p><small>{item.detail}</small></div></article>)}</div> : <EmptyState title="No timeline events" body="Assign issues or scheduled jobs to this branch to see its workflow here." icon={<GitBranch size={20} />} />}</section>
        </div> : <EmptyState title="No workflow branches" body="Create a branch record to group related issues, jobs and pull requests." icon={<GitBranch size={20} />} />}
      </div>}
      {tab === 'History' && <div className="simrepo-history-workspace">
        <div className="simrepo-section-intro"><div><p className="eyebrow">IMMUTABLE WORKFLOW HISTORY</p><h3>Merge history</h3><p>One merged PR creates one simulated commit. Expand a PR to inspect every included issue and its original description.</p></div><Badge tone="green">{completions.length} commits</Badge></div>
        {completions.length ? <div className="simrepo-history-tree">{completions.map((item) => { const pr=pullRequests.find((candidate)=>candidate.id===item.pull_request_id); const expanded=expandedCompletionID===item.id; return <article className={`simrepo-history-commit ${expanded?'simrepo-history-commit--open':''}`} key={item.id}><button type="button" className="simrepo-history-commit__head" onClick={()=>void toggleCompletion(item)} aria-expanded={expanded}><span className="simrepo-history-commit__glyph"><GitPullRequest size={17}/></span><span className="simrepo-history-commit__main"><strong>{pr?`PR #${pr.number} · ${pr.title}`:`Commit ${item.sequence}`}</strong><small>{item.summary}</small><code>{(item.commit_hash||item.id.slice(0,12)).slice(0,12)} · {item.branch_name||'main'}</code></span><span className="simrepo-history-commit__meta"><Badge tone="green">Merged</Badge><time>{formatRelative(item.created_at)}</time></span><span className="simrepo-tree-caret">{expanded?'−':'+'}</span></button>{expanded&&<div className="simrepo-history-commit__body"><div className="simrepo-tree-root"><span className="simrepo-tree-line"/><strong>{historyIssues.length} included issue{historyIssues.length===1?'':'s'}</strong><small>{pr?.description||'No PR description was saved.'}</small></div>{historyIssues.map((issue)=>{const issueExpanded=expandedHistoryIssueID===issue.id;return <section className="simrepo-tree-issue" key={issue.id}><button type="button" className="simrepo-tree-issue__button" onClick={()=>setExpandedHistoryIssueID(issueExpanded?'':issue.id)}><span className="simrepo-tree-issue__connector"/><span className={`simrepo-issue-type-dot simrepo-issue-type-dot--${issue.type||'task'}`}/><span className="simrepo-tree-issue__text"><strong>{selected.name}#{issue.number} · {issue.title}</strong><small>{(issue.type||'task').toUpperCase()} · {issue.status}</small></span><span className="simrepo-tree-caret">{issueExpanded?'−':'+'}</span></button>{issueExpanded&&<div className="simrepo-tree-issue__detail"><p>{issue.description||'No description was saved for this issue.'}</p><dl><div><dt>Priority</dt><dd>{issue.priority||'normal'}</dd></div><div><dt>Created</dt><dd>{new Date(issue.created_at).toLocaleString()}</dd></div><div><dt>Completed</dt><dd>{issue.closed_at?new Date(issue.closed_at).toLocaleString():new Date(item.created_at).toLocaleString()}</dd></div></dl></div>}</section>;})}</div>}</article>;})}</div> : <EmptyState title="No merge history yet" body="Once a pull request is merged, it appears here as an immutable simulated commit." icon={<GitBranch size={20} />} />}
      </div>}
      {tab === 'Git' && <LocalGitWorkspace repository={selected} />}
      {tab === 'Files' && <div className="simrepo-files-workspace"><div className="simrepo-section-intro"><div><p className="eyebrow">FILES</p><h3>{selected.name}</h3><p>Browse the repository's local file snapshot.</p></div><div className="repository-detail-toolbar__actions"><Button variant="secondary" onClick={() => selectTab('Branches')}><GitBranch size={14}/> Branches</Button><Button variant="secondary" onClick={() => selectTab('Tags')}><Tags size={14}/> Tags</Button><Button onClick={()=>{setFilePath('');setFileContent('');setFileEditorOpen(true);setFileMessage('');}}><Plus size={14}/> New file</Button></div></div>{fileMessage&&<div className="notice notice--info">{fileMessage}</div>}
        <div className="simrepo-file-layout"><div className="simrepo-file-tree"><div className="simrepo-file-tree__head"><Box size={15}/><strong>{selected.name}</strong><Badge>{files.length} files</Badge></div>{fileTree.length ? renderFileTree(fileTree) : <p className="simrepo-muted">No files in this virtual repository.</p>}</div>
          <section className="simrepo-file-editor">{fileEditorOpen?<form onSubmit={saveFile}><div className="simrepo-file-editor__head"><div><p className="eyebrow">FILE EDITOR</p><h4>{filePath||'New file'}</h4></div><div><Button type="button" variant="ghost" onClick={()=>setFileEditorOpen(false)}>Cancel</Button><Button type="submit" disabled={fileBusy||!filePath.trim()}>{fileBusy?'Saving…':'Save file'}</Button></div></div><Field label="Path"><input value={filePath} onChange={(event)=>setFilePath(event.target.value)} disabled={fileBusy} placeholder="src/main.c" required maxLength={512}/></Field><Field label="Content"><textarea className="simrepo-code-editor" value={fileContent} onChange={(event)=>setFileContent(event.target.value)} disabled={fileBusy} rows={18} spellCheck={false} /></Field>{files.some((file)=>file.path===filePath)&&<Button type="button" variant="danger" onClick={()=>{const file=files.find((item)=>item.path===filePath);if(file)void removeFile(file);setFileEditorOpen(false);}}>Delete file</Button>}</form>:<div className="simrepo-file-editor__empty"><Box size={28}/><strong>Select a file to inspect it</strong><p>Or create a new virtual file. Tags snapshot the current file tree so each release can be downloaded later.</p></div>}</section></div>
      </div>}
      {tab === 'Tags' && <div className="simrepo-tags-workspace"><div className="simrepo-section-intro"><div><p className="eyebrow">RELEASE SNAPSHOTS</p><h3>Tags & versions</h3><p>Create a named snapshot like <code>0.0.0</code> or <code>0.1.0</code>. Every tag snapshots the latest merged commit, keeps its own file tree, and records simulated commits since the previous tag. Unmerged file edits are not released.</p></div><Badge>{tags.length} tags</Badge></div>
        <form className="simrepo-tag-form" onSubmit={createTag}><Field label="Tag name"><input value={tagName} onChange={(event)=>setTagName(event.target.value)} placeholder={tags.length?'0.1.0':'0.0.0'} required pattern="[A-Za-z0-9][A-Za-z0-9._+-]{0,99}" maxLength={100}/></Field><Field label="Release notes"><input value={tagMessage} onChange={(event)=>setTagMessage(event.target.value)} placeholder="First public milestone" maxLength={10000}/></Field><Button type="submit" disabled={tagBusy}>{tagBusy?'Creating…':'Create tag snapshot'}</Button></form>
        {tags.length?<div className="simrepo-tags-list">{tags.map((tag)=> <article key={tag.id} className="simrepo-tag-card"><div className="simrepo-tag-card__version"><span className="simrepo-tag-icon"><Tags size={17}/></span><div><strong>{tag.name}</strong><small>{tag.message||'No release notes'} · {new Date(tag.created_at).toLocaleString()}</small></div></div><div className="simrepo-tag-card__meta"><span>{tag.completion_sequence ? `Latest included commit #${tag.completion_sequence}` : 'No commits yet'}</span><span>{tag.file_count??0} files</span>{tag.previous_tag_name&&<span>from {tag.previous_tag_name}</span>}</div><div className="simrepo-tag-card__actions"><button className="simrepo-text-action" type="button" onClick={()=>void showTagChanges(tag)}>Compare changes</button>{tag.previous_tag_name&&<a className="simrepo-download-link" href={api.repositoryTagArchiveURL(selected.id,tag.previous_tag_name)}>Download {tag.previous_tag_name}</a>}<a className="simrepo-download-link" href={api.repositoryTagArchiveURL(selected.id,tag.name)}><ArrowUpRight size={14}/> Download ZIP</a></div></article>)}</div>:<EmptyState title="No tags yet" body="Create the first tag (for example 0.0.0) to snapshot the repository's current virtual files and simulated history." icon={<Tags size={20}/>}/>}
        {tagCompare&&<section className="simrepo-tag-compare"><div className="simrepo-panel__heading"><div><p className="eyebrow">TAG DIFF</p><h3>{tagCompare.previous_tag?`${tagCompare.previous_tag} → ${tagCompare.tag.name}`:`Initial snapshot · ${tagCompare.tag.name}`}</h3></div><Button variant="ghost" onClick={()=>setTagCompare(null)}>Close</Button></div><p>{tagCompare.changes.length} file changes · {tagCompare.history.length} simulated commits in this release interval.</p>{tagCompare.changes.length?<div className="simrepo-diff-list">{tagCompare.changes.map((change)=><article key={change.path}><Badge tone={change.status==='added'?'green':change.status==='deleted'?'red':'blue'}>{change.status}</Badge><strong>{change.path}</strong></article>)}</div>:<p className="simrepo-muted">No file differences from the previous snapshot.</p>}<div className="simrepo-release-history">{tagCompare.history.map((commit)=><div key={commit.id}><GitPullRequest size={14}/><span><strong>PR commit · {commit.summary}</strong><small>{(commit.commit_hash||commit.id.slice(0,12)).slice(0,12)} · {formatRelative(commit.created_at)}</small></span></div>)}</div>{tagCompare.previous_tag&&<a className="simrepo-download-link" href={api.repositoryTagArchiveURL(selected.id,tagCompare.previous_tag)}>Download previous tag {tagCompare.previous_tag}</a>}</section>}
      </div>}
      {tab === 'Labels' && <div className="repository-detail-toolbar"><Button variant="ghost" onClick={() => selectTab('Issues')}>← Issues</Button><p>Labels are stored locally and can be assigned to issues in this repository.</p><div className="repository-detail-toolbar__actions"><Button variant="secondary" onClick={() => run('label.create', { scope: 'repository', repository: selected.id, color: '#1f6feb' })}><Plus size={14} /> New label</Button></div></div>}
      {tab === 'Labels' && <div className="repository-embedded-list">{labels.length ? labels.map((label) => <div className="repository-simple-row" key={label.id}><div><strong><span className="label-color-dot" style={{ background: label.color || '#7c9cff' }} /> {label.name}</strong><span>{label.description || 'No description'}</span></div><Badge>{label.scope_type}</Badge></div>) : <EmptyState title="No repository labels" body="Create a scoped label from Labels & milestones." icon={<Tags size={20} />} />}</div>}
      {tab === 'Milestones' && <div className="repository-detail-toolbar"><Button variant="ghost" onClick={() => selectTab('Issues')}>← Issues</Button><p>Group related issues under a repository milestone.</p><div className="repository-detail-toolbar__actions"><Button variant="secondary" onClick={() => run('milestone.create', { scope: 'repository', owner: selected.id })}><Plus size={14} /> New milestone</Button></div></div>}
      {tab === 'Milestones' && <div className="repository-embedded-list">{milestones.length ? milestones.map((item) => <div className="repository-simple-row" key={item.id}><div><strong>{item.title}</strong><span>{item.description || 'No description'}{item.due_date ? ` · Due ${item.due_date}` : ''}</span></div><Badge>{item.scope_type}</Badge></div>) : <EmptyState title="No milestones yet" body="Create a repository milestone to group work against a target." icon={<GitBranch size={20} />} />}</div>}
      {tab === 'Projects' && <section className="repository-projects-workspace">
        <div className="repository-detail-toolbar"><div><strong>Projects</strong><p>Connect this repository to existing IDIOS projects.</p></div>
          <div className="repository-detail-toolbar__actions"><Button variant="secondary" disabled={projectsBusy || projectBusy || busy} onClick={() => setProjectCreateOpen(true)}><Plus size={14}/> New project</Button><select aria-label="Select project" value={projectToLink} disabled={projectsBusy || projectBusy} onChange={(event) => setProjectToLink(event.target.value)}><option value="">Choose a project…</option>{projects.filter((project) => !projectLinks.some((link) => link.project_id === project.id)).map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}</select><Button onClick={() => void linkRepositoryProject()} disabled={projectsBusy || projectBusy || !projectToLink}><Plus size={14}/> {projectBusy ? 'Working…' : 'Link project'}</Button></div>
        </div>
        {projectsBusy ? <p className="repository-detail-loading">Loading project links…</p> : <div className="repository-embedded-list">{projectLinks.length ? projectLinks.map((link) => <div className="repository-simple-row" key={link.project_id}><div><strong>{projects.find((project) => project.id === link.project_id)?.name ?? 'Project'}</strong><span>{selected.name} · {link.relationship || 'implementation'}{link.is_primary ? ' · Primary repository' : ''}</span></div>{!link.is_primary && <Button variant="ghost" disabled={projectBusy} onClick={() => void makeRepositoryPrimary(link.project_id)}>Make primary</Button>}<Button variant="ghost" disabled={projectBusy} onClick={() => void unlinkRepositoryProject(link.project_id)}>Unlink</Button></div>) : <EmptyState title="No linked projects" body="Link an existing project to keep its repositories and issues together." icon={<Layers3 size={20}/>} />}</div>}
        {!projectsBusy && projectLinks.length > 0 && <section className="repository-project-issues">
          <div className="repository-detail-toolbar"><div><strong>Project issues</strong><p>Keep this repository's issues connected to the selected project.</p></div>
            <div className="repository-detail-toolbar__actions"><select aria-label="Select linked project" value={projectForIssues} disabled={projectBusy} onChange={(event) => setProjectForIssues(event.target.value)}>{projectLinks.map((link) => <option key={link.project_id} value={link.project_id}>{projects.find((project) => project.id === link.project_id)?.name ?? 'Project'}</option>)}</select>
              <select aria-label="Select repository issue" value={issueToAddToProject} disabled={projectBusy} onChange={(event) => setIssueToAddToProject(event.target.value)}><option value="">Choose issue…</option>{repositoryIssues.filter((issue) => !projectIssues.some((item) => item.id === issue.id)).map((issue) => <option key={issue.id} value={issue.id}>#{issue.number} · {issue.title}</option>)}</select>
              <Button onClick={() => void addRepositoryIssueToProject()} disabled={projectBusy || !issueToAddToProject}><Plus size={14}/> {projectBusy ? 'Working…' : 'Add issue'}</Button>
            </div>
          </div>
          <div className="repository-embedded-list">{projectIssues.filter((issue) => issue.repository_id === selected.id).length ? projectIssues.filter((issue) => issue.repository_id === selected.id).map((issue) => <button className="repository-project-issue-row" type="button" key={issue.id} onClick={() => setSelectedIssue(issue)}><span>{selected.name}#{issue.number} · {issue.title}</span><Badge tone={issue.status === 'done' ? 'green' : issue.status === 'in_progress' ? 'blue' : 'neutral'}>{issue.status.replace('_', ' ')}</Badge></button>) : <EmptyState title="No linked issues from this repository" body="Add an existing issue above to include it in this project." icon={<Layers3 size={20}/>} />}</div>
        </section>}
      </section>}
      {tab === 'Settings' && <div className="repository-settings-grid">
        <section className="repository-settings-card"><div><p className="eyebrow">GENERAL</p><h3>Repository details</h3><p className="repository-settings-card__description">The name identifies this workspace in issue references and links. Keep the description short and useful.</p></div><dl><div><dt>Name</dt><dd>{selected.name}</dd></div><div><dt>Description</dt><dd>{selected.description || 'No description added yet.'}</dd></div><div><dt>Created</dt><dd>{new Date(selected.created_at).toLocaleString()}</dd></div><div><dt>Last updated</dt><dd>{new Date(selected.updated_at).toLocaleString()}</dd></div></dl><Button variant="secondary" onClick={() => run('repository.description.update', { repository: selected.id, description: selected.description })}><Pencil size={14} /> Edit description</Button></section>
        <section className="repository-settings-card"><div><p className="eyebrow">WORKFLOW</p><h3>Default branch</h3><p className="repository-settings-card__description">New pull requests use the default branch as their target unless you choose another branch.</p></div><div className="repository-default-branch"><GitBranch size={16} /><strong>{branches.find((branch) => branch.id === selected.default_branch_id)?.name || branches.find((branch) => branch.is_default)?.name || 'main'}</strong><Badge tone="green">Default</Badge></div><Button variant="secondary" onClick={() => selectTab('Branches')}>Manage branches</Button></section>
        <section className="repository-settings-card"><div><p className="eyebrow">PLANNING</p><h3>Labels & milestones</h3><p className="repository-settings-card__description">Organize issues with colors, labels, and target milestones scoped to this repository.</p></div><div className="repository-settings-links"><Button variant="secondary" onClick={() => selectTab('Labels')}><Tags size={14} /> Manage labels</Button><Button variant="secondary" onClick={() => selectTab('Milestones')}><Layers3 size={14} /> Manage milestones</Button></div></section>
        <section className="repository-settings-card repository-settings-card--danger"><div><p className="eyebrow">DANGER ZONE</p><h3>Delete repository</h3><p className="repository-settings-card__description">Removes this local workspace and its owned issues, pull requests, branches, labels and milestones. Notes are preserved but detached.</p></div><Button variant="danger" onClick={() => run('repository.delete', { ref: selected.id, __confirm_target: selected.name })}>Delete {selected.name}</Button></section>
      </div>}
      {tab === 'Activity' && <div className="repository-embedded-list">{activity.length ? activity.map((event) => <div className="repository-simple-row" key={event.id}><div><strong>{event.action.replaceAll('_', ' ')}</strong><span>{event.payload || `${event.entity_type} · ${event.entity_id}`}</span></div><time>{formatRelative(event.created_at)}</time></div>) : completions.length ? completions.map((event) => <div className="repository-simple-row" key={event.id}><div><strong>Accepted #{event.issue_number}</strong><span>{event.summary || `Completed issue with PR ${event.pull_request_id}`}</span></div><time>{formatRelative(event.created_at)}</time></div>) : <EmptyState title="No recorded activity yet" body="Actions and accepted completions will be listed here." icon={<Activity size={20} />} />}</div>}
      {tab === 'Graph' && <div className="repository-graph-entry"><span className="repository-graph-entry__icon"><GitBranch size={22} /></span><p className="eyebrow">REPOSITORY KNOWLEDGE</p><h3>Explore connected records</h3><p>Open the shared graph focused on <strong>{selected.name}</strong>. All stored relations are shown; only <code>depends_on</code> can block work.</p><Button onClick={() => navigate(`/graph?from=${encodeURIComponent(`repo:${selected.name}`)}`)}><GitBranch size={14} /> Open focused graph</Button></div>}
      </div>
    </Card>}

    <Modal open={open} title="New repository" description="Create a workspace in your local IDIOS Engine." onClose={() => setOpen(false)} onSubmit={create} submitLabel="Create repository" busy={busy}>
      {error && <div className="inline-error" role="alert">{error}</div>}
      <Field label="Name"><input autoFocus required value={name} onChange={(event) => setName(event.target.value)} maxLength={80} placeholder="e.g. XORON5" /></Field>
      <Field label="Description" hint="A short sentence about the purpose of this workspace."><textarea rows={3} value={description} onChange={(event) => setDescription(event.target.value)} placeholder="What are you building?" /></Field>
      <Field label="Initial branch" hint="The default branch name inside IDIOS. This is workflow metadata, not a filesystem Git branch."><input value={initialBranch} onChange={(event) => setInitialBranch(event.target.value)} required placeholder="main" /></Field>
      <div className="form-note"><GitBranch size={15} /><span>Issues, jobs and review records can be linked to branches. IDIOS keeps simulated files, commits and tags in its own local database; it never invokes Git.</span></div>
    </Modal>
    <CommandRunner command={selectedCommand} open={Boolean(selectedCommand)} initialValues={initialValues} onClose={() => setSelectedCommand(null)} onSuccess={complete} />
    <IssueCreateDialog open={issueCreateOpen} repositories={dashboard?.repositories ?? []} initialRepositoryId={selected?.id} onClose={() => setIssueCreateOpen(false)} onCreated={complete} />
    <Modal open={projectCreateOpen} title="New project" description={selected ? `Create a project for ${selected.name}. The repository will be linked as its primary codebase.` : 'Create a project.'} onClose={() => setProjectCreateOpen(false)} onSubmit={createRepositoryProject} submitLabel="Create project" busy={busy}>
      <Field label="Project name"><input autoFocus required value={newProjectName} onChange={(event) => setNewProjectName(event.target.value)} maxLength={100} placeholder="e.g. Runtime hardening" /></Field>
      <Field label="Description"><textarea rows={3} value={newProjectDescription} onChange={(event) => setNewProjectDescription(event.target.value)} placeholder="What is this project meant to deliver?" /></Field>
    </Modal>
    <IssueDetails issue={selectedIssue} commands={commands} open={Boolean(selectedIssue)} onClose={() => setSelectedIssue(null)} onChanged={complete} onOpenGraph={() => { const ref = selectedIssue ? `${selectedIssue.repository_name || 'Repository'}#${selectedIssue.number}` : ''; setSelectedIssue(null); navigate(`/graph?from=${encodeURIComponent(ref)}`); }} />
    </>}
  </div>;
}
