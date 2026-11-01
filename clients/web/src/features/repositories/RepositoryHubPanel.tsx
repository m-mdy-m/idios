import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router';
import {
  ArrowUpRight, Check, ChevronDown, Copy, FileCode2, GitBranch, GitCommitHorizontal,
  Link2, LockKeyhole, MoreHorizontal, Pin, Plus, RefreshCw, Search, ShieldCheck, Trash2, Unplug,
} from 'lucide-react';
import { api } from '@/lib/api';
import type {
  HubBranch, HubChangeRequest, HubCollection, HubCommit, HubConnection, HubDiscoveredRepository, HubFile,
  HubIssue, HubProjectRepository, HubProvider, HubRelease, HubRepository, HubRepositoryRelation,
  HubStatus, HubTag, HubTagComparison, HubTreeEntry, Project,
} from '@/types/idios';
import { Badge, Button, EmptyState, Field, Modal } from '@/design-system';
import './RepositoryHubPanel.css';
import { RemoteReadmePreview, RemoteTreeView } from './RepositoryBrowser';

type RemoteTab = 'Overview' | 'Code' | 'Issues' | 'Change Requests' | 'Commits' | 'Releases' | 'Tags' | 'Settings';

type DiffLine = { kind: 'context' | 'remove' | 'add'; text: string };

function previewDiff(before: string, after: string): DiffLine[] {
  const oldLines = before.replace(/\r\n?/g, '\n').split('\n');
  const newLines = after.replace(/\r\n?/g, '\n').split('\n');
  let prefix = 0;
  while (prefix < oldLines.length && prefix < newLines.length && oldLines[prefix] === newLines[prefix]) prefix += 1;
  let suffix = 0;
  while (suffix < oldLines.length - prefix && suffix < newLines.length - prefix &&
    oldLines[oldLines.length - 1 - suffix] === newLines[newLines.length - 1 - suffix]) suffix += 1;
  const lines: DiffLine[] = [];
  oldLines.slice(Math.max(0, prefix - 3), prefix).forEach((text) => lines.push({ kind: 'context', text }));
  oldLines.slice(prefix, oldLines.length - suffix).forEach((text) => lines.push({ kind: 'remove', text }));
  newLines.slice(prefix, newLines.length - suffix).forEach((text) => lines.push({ kind: 'add', text }));
  newLines.slice(Math.max(prefix, newLines.length - suffix), newLines.length - Math.max(0, suffix - 3))
    .forEach((text) => lines.push({ kind: 'context', text }));
  return lines;
}

function messageFrom(error: unknown): string {
  return error instanceof Error ? error.message : 'The Repository Hub request failed.';
}

function remoteStateTone(state: string): 'green' | 'blue' | 'amber' | 'red' | 'neutral' {
  if (['merged', 'closed', 'done'].includes(state)) return 'green';
  if (['opened', 'open', 'in_progress'].includes(state)) return 'blue';
  if (['locked', 'blocked'].includes(state)) return 'amber';
  if (['error', 'failed'].includes(state)) return 'red';
  return 'neutral';
}

function providerSettingsURL(repository: HubRepository): string {
  try {
    if (repository.provider === 'sourceforge') {
      const origin = new URL(repository.host);
      const project = repository.namespace || repository.full_path.split('/')[0];
      if (!['http:', 'https:'].includes(origin.protocol) || !project) return '';
      return `${origin.origin}/p/${encodeURIComponent(project)}/admin/`;
    }
    const page = new URL(repository.web_url);
    if (!['http:', 'https:'].includes(page.protocol)) return '';
    const base = page.toString().replace(/\/+$/, '');
    return repository.provider === 'github' ? `${base}/settings` : `${base}/-/edit`;
  } catch {
    return '';
  }
}

type RepositoryHubPanelProps = {
  repositoryID?: string;
  repositoryView?: string;
  standalone?: boolean;
};

function remoteTabFromView(view?: string): RemoteTab {
  switch ((view || 'overview').toLowerCase()) {
    case 'code': return 'Code';
    case 'issues': return 'Issues';
    case 'changes':
    case 'change-requests': return 'Change Requests';
    case 'commits': return 'Commits';
    case 'releases': return 'Releases';
    case 'tags': return 'Tags';
    case 'settings': return 'Settings';
    default: return 'Overview';
  }
}

function remoteViewFromTab(tab: RemoteTab): string {
  switch (tab) {
    case 'Code': return 'code';
    case 'Issues': return 'issues';
    case 'Change Requests': return 'changes';
    case 'Commits': return 'commits';
    case 'Releases': return 'releases';
    case 'Tags': return 'tags';
    case 'Settings': return 'settings';
    default: return 'overview';
  }
}

function syncStateLabel(state: string): string {
  switch (state) {
    case 'ok': return 'Up to date';
    case 'stale': return 'Update available';
    case 'error': return 'Sync failed';
    case 'never': return 'Not synced';
    default: return 'Unknown';
  }
}

export function RepositoryHubPanel({ repositoryID, repositoryView, standalone = false }: RepositoryHubPanelProps) {
  const navigate = useNavigate();
  const [status, setStatus] = useState<HubStatus | null>(null);
  const [connections, setConnections] = useState<HubConnection[]>([]);
  const [repositories, setRepositories] = useState<HubRepository[]>([]);
  const [collections, setCollections] = useState<HubCollection[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [hubProjectLinks, setHubProjectLinks] = useState<HubProjectRepository[]>([]);
  const [projectToLink, setProjectToLink] = useState('');
  const [projectLinkRelationship, setProjectLinkRelationship] = useState('implementation');
  const [selected, setSelected] = useState<HubRepository | null>(null);
  const [tab, setTab] = useState<RemoteTab>('Overview');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [provider, setProvider] = useState<HubProvider>('github');
  const [host, setHost] = useState('https://github.com');
  const [accessToken, setAccessToken] = useState('');
  const [search, setSearch] = useState('');
  const [repositorySearch, setRepositorySearch] = useState('');
  const [repositoryProviderFilter, setRepositoryProviderFilter] = useState<HubProvider | 'all'>('all');
  const [issueSearch, setIssueSearch] = useState('');
  const [issueStateFilter, setIssueStateFilter] = useState<'all' | 'open' | 'closed'>('all');
  const [discoveryConnection, setDiscoveryConnection] = useState('');
  const [discovery, setDiscovery] = useState<HubDiscoveredRepository[]>([]);
  const [discoveryBusy, setDiscoveryBusy] = useState(false);
  const [discoveryPage, setDiscoveryPage] = useState(1);
  const [discoveryHasMore, setDiscoveryHasMore] = useState(false);
  const [collectionFilter, setCollectionFilter] = useState('');
  const [pinnedOnly, setPinnedOnly] = useState(false);
  const [collectionName, setCollectionName] = useState('');
  const [collectionDescription, setCollectionDescription] = useState('');
  const [repositoryAlias, setRepositoryAlias] = useState('');
  const [repositoryNote, setRepositoryNote] = useState('');
  const [repositoryPinned, setRepositoryPinned] = useState(false);
  const [providerSetupOpen, setProviderSetupOpen] = useState(false);
  const [branches, setBranches] = useState<HubBranch[]>([]);
  const [branch, setBranch] = useState('');
  const [tree, setTree] = useState<HubTreeEntry[]>([]);
  const [treePage, setTreePage] = useState(1);
  const [treeHasMore, setTreeHasMore] = useState(false);
  const [treeLoading, setTreeLoading] = useState(false);
  const [file, setFile] = useState<HubFile | null>(null);
  const [filePath, setFilePath] = useState('');
  const [fileContent, setFileContent] = useState('');
  const [fileOriginalContent, setFileOriginalContent] = useState('');
  const [showDiffPreview, setShowDiffPreview] = useState(false);
  const [fileCommitMessage, setFileCommitMessage] = useState('');
  const [fileCreateOnly, setFileCreateOnly] = useState(false);
  const [issues, setIssues] = useState<HubIssue[]>([]);
  const [changes, setChanges] = useState<HubChangeRequest[]>([]);
  const [commits, setCommits] = useState<HubCommit[]>([]);
  const [commitPage, setCommitPage] = useState(1);
  const [commitHasMore, setCommitHasMore] = useState(false);
  const [commitLoading, setCommitLoading] = useState(false);
  const [expandedCommitSHA, setExpandedCommitSHA] = useState('');
  const [commitDetails, setCommitDetails] = useState<Record<string, HubCommit>>({});
  const [commitDetailLoading, setCommitDetailLoading] = useState('');
  const [commitDetailErrors, setCommitDetailErrors] = useState<Record<string, string>>({});
  const [latestRelease, setLatestRelease] = useState<HubRelease | null>(null);
  const [latestReleaseAvailability, setLatestReleaseAvailability] = useState<{ available: boolean; state: string; reason?: string } | null>(null);
  const [releases, setReleases] = useState<HubRelease[]>([]);
  const [releasePage, setReleasePage] = useState(1);
  const [releaseHasMore, setReleaseHasMore] = useState(false);
  const [releaseLoading, setReleaseLoading] = useState(false);
  const [releaseAvailability, setReleaseAvailability] = useState<{ available: boolean; state: string; reason?: string } | null>(null);
  const [tags, setTags] = useState<HubTag[]>([]);
  const [tagPage, setTagPage] = useState(1);
  const [tagHasMore, setTagHasMore] = useState(false);
  const [tagLoading, setTagLoading] = useState(false);
  const [tagAvailability, setTagAvailability] = useState<{ available: boolean; state: string; reason?: string } | null>(null);
  const [compareFrom, setCompareFrom] = useState('');
  const [compareTo, setCompareTo] = useState('');
  const [tagComparison, setTagComparison] = useState<HubTagComparison | null>(null);
  const [tagCompareLoading, setTagCompareLoading] = useState(false);
  const [tagCompareError, setTagCompareError] = useState('');
  const [relations, setRelations] = useState<HubRepositoryRelation[]>([]);
  const [targetRelation, setTargetRelation] = useState('');
  const [relationType, setRelationType] = useState('related_to');
  const [relationDescription, setRelationDescription] = useState('');
  const [issueCreateOpen, setIssueCreateOpen] = useState(false);
  const [changeCreateOpen, setChangeCreateOpen] = useState(false);
  const [issueTitle, setIssueTitle] = useState('');
  const [issueBody, setIssueBody] = useState('');
  const [issueLabels, setIssueLabels] = useState('');
  const [issueAssignee, setIssueAssignee] = useState('');
  const [issueStatus, setIssueStatus] = useState('');
  const [issuePrivate, setIssuePrivate] = useState(false);
  const [issueDiscussionDisabled, setIssueDiscussionDisabled] = useState(false);
  const [issueCustomFieldsJSON, setIssueCustomFieldsJSON] = useState('{}');
  const [editingIssueID, setEditingIssueID] = useState('');
  const [editIssueTitle, setEditIssueTitle] = useState('');
  const [editIssueBody, setEditIssueBody] = useState('');
  const [editIssueState, setEditIssueState] = useState('');
  const [editIssueAssignee, setEditIssueAssignee] = useState('');
  const [editIssueLabels, setEditIssueLabels] = useState('');
  const [editIssuePrivate, setEditIssuePrivate] = useState(false);
  const [editIssueDiscussionDisabled, setEditIssueDiscussionDisabled] = useState(false);
  const [editIssueCustomFieldsJSON, setEditIssueCustomFieldsJSON] = useState('{}');
  const [commentNumber, setCommentNumber] = useState<number | null>(null);
  const [commentBody, setCommentBody] = useState('');
  const [changeTitle, setChangeTitle] = useState('');
  const [changeBody, setChangeBody] = useState('');
  const [changeSource, setChangeSource] = useState('');
  const [changeTarget, setChangeTarget] = useState('');
  const [fileLoading, setFileLoading] = useState(false);
  const diffLines = useMemo(() => previewDiff(fileOriginalContent, fileContent), [fileOriginalContent, fileContent]);

  const reload = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [nextStatus, nextConnections, nextRepositories, nextCollections, nextProjects] = await Promise.all([
        api.hubStatus(), api.hubConnections(), api.hubRepositories(), api.hubCollections(), api.projects(),
      ]);
      setStatus(nextStatus);
      setConnections(nextConnections);
      setRepositories(nextRepositories);
      setCollections(nextCollections);
      setProjects(nextProjects);
      setDiscoveryConnection((current) => nextConnections.some((item) => item.id === current) ? current : nextConnections[0]?.id ?? '');
      setSelected((current) => current ? nextRepositories.find((item) => item.id === current.id) ?? null : null);
    } catch (err) {
      setError(messageFrom(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void reload(); }, [reload]);

  useEffect(() => {
    if (!standalone || !repositoryID) {
      if (!standalone) {
        setSelected(null);
        setTab('Overview');
      }
      return;
    }
    const match = repositories.find((item) => item.id === repositoryID);
    if (match) {
      setSelected((current) => current?.id === match.id ? current : match);
      setTab(remoteTabFromView(repositoryView));
    } else if (!loading) {
      setSelected(null);
      setError('This linked repository is no longer available in this IDIOS workspace.');
    }
  }, [standalone, repositoryID, repositoryView, repositories, loading]);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const connectedProvider = params.get('hub_connected');
    const callbackError = params.get('hub_error');
    if (connectedProvider) setNotice(`Connected ${connectedProvider} account successfully.`);
    if (callbackError) setError(callbackError);
    if (connectedProvider || callbackError) {
      const url = new URL(window.location.href);
      url.searchParams.delete('hub_connected');
      url.searchParams.delete('hub_error');
      window.history.replaceState({}, '', url.pathname + url.search + url.hash);
      void reload();
    }
  }, [reload]);

  useEffect(() => {
    if (!selected) {
      setLatestRelease(null);
      setLatestReleaseAvailability(null);
      return;
    }
    let active = true;
    setLatestRelease(null);
    setLatestReleaseAvailability(null);
    if (!selected.capabilities.includes('releases.read')) return () => { active = false; };
    void api.hubReleases(selected.id, 1, 1).then((page) => {
      if (!active) return;
      setLatestRelease(page.items[0] ?? null);
      setLatestReleaseAvailability(page.availability);
    }).catch((err) => {
      if (active) setLatestReleaseAvailability({ available: false, state: 'error', reason: messageFrom(err) });
    });
    return () => { active = false; };
  }, [selected?.id, selected?.capabilities]);

  useEffect(() => {
    if (!selected) {
      setBranches([]); setTree([]); setFile(null); setIssues([]); setChanges([]); setCommits([]); setRelations([]);
      setRepositoryAlias(''); setRepositoryNote(''); setRepositoryPinned(false);
      setHubProjectLinks([]); setProjectToLink('');
      return;
    }
    let active = true;
    setError('');
    setRepositoryAlias(selected.local_alias ?? '');
    setRepositoryPinned(selected.pinned);
    void api.hubRepositoryPreferences(selected.id).then((prefs) => {
      if (!active) return;
      setRepositoryAlias(prefs.local_alias ?? '');
      setRepositoryNote(prefs.private_note ?? '');
      setRepositoryPinned(prefs.pinned);
    }).catch((err) => { if (active) setError(messageFrom(err)); });
    void api.hubRepositoryRelations(selected.id).then((items) => { if (active) setRelations(items); }).catch((err) => { if (active) setError(messageFrom(err)); });
    void api.hubRepositoryProjects(selected.id).then((items) => {
      if (!active) return;
      setHubProjectLinks(items);
      setProjectToLink((current) => current && projects.some((project) => project.id === current && !items.some((link) => link.project_id === current)) ? current : '');
    }).catch((err) => { if (active) setError(messageFrom(err)); });
    return () => { active = false; };
  }, [selected?.id, projects]);

  useEffect(() => {
    if (!selected || (tab !== 'Code' && tab !== 'Change Requests')) return;
    if (!selected.capabilities.includes('branches.read')) { setBranches([]); setTree([]); return; }
    let active = true;
    void api.hubBranches(selected.id).then((items) => {
      if (!active) return;
      setBranches(items);
      const requestedBranch = new URLSearchParams(window.location.search).get('branch') ?? '';
      setBranch((current) => {
        if (requestedBranch && items.some((item) => item.name === requestedBranch)) return requestedBranch;
        return items.some((item) => item.name === current) ? current : items.find((item) => item.default)?.name ?? items[0]?.name ?? '';
      });
    }).catch((err) => { if (active) setError(messageFrom(err)); });
    return () => { active = false; };
  }, [selected?.id, tab]);

  useEffect(() => {
    if (!standalone || !selected || tab !== 'Code' || !branch) return;
    const url = new URL(window.location.href);
    url.searchParams.set('branch', branch);
    window.history.replaceState(window.history.state, '', url.pathname + url.search + url.hash);
  }, [standalone, selected?.id, tab, branch]);

  useEffect(() => {
    if (!selected || tab !== 'Code' || !branch) {
      setTree([]); setTreePage(1); setTreeHasMore(false); setTreeLoading(false);
      return;
    }
    let active = true;
    setTree([]); setTreePage(1); setTreeHasMore(false); setTreeLoading(true);
    setFile(null); setFilePath(''); setFileContent(''); setFileOriginalContent(''); setShowDiffPreview(false);
    void api.hubTree(selected.id, branch).then((result) => {
      if (!active) return;
      setTree(result.items); setTreePage(result.page); setTreeHasMore(result.has_more);
    }).catch((err) => { if (active) setError(messageFrom(err)); })
      .finally(() => { if (active) setTreeLoading(false); });
    return () => { active = false; };
  }, [selected?.id, tab, branch]);

  useEffect(() => {
    if (!selected) return;
    let active = true;
    if (tab === 'Issues') {
      setIssues([]);
      if (!selected.capabilities.includes('issues.read')) return () => { active = false; };
      void api.hubIssues(selected.id).then((items) => { if (active) setIssues(items); }).catch((err) => { if (active) setError(messageFrom(err)); });
    } else if (tab === 'Change Requests') {
      setChanges([]);
      if (!selected.capabilities.includes('changes.read')) return () => { active = false; };
      void api.hubChanges(selected.id).then((items) => { if (active) setChanges(items); }).catch((err) => { if (active) setError(messageFrom(err)); });
    } else if (tab === 'Commits') {
      setCommits([]);
      setCommitPage(1); setCommitHasMore(false); setExpandedCommitSHA('');
      if (!selected.capabilities.includes('commits.read')) return () => { active = false; };
      setCommitLoading(true);
      void api.hubCommits(selected.id, branch, 1).then((items) => {
        if (!active) return;
        setCommits(items); setCommitPage(1); setCommitHasMore(items.length === 50);
      }).catch((err) => { if (active) setError(messageFrom(err)); })
        .finally(() => { if (active) setCommitLoading(false); });
    } else if (tab === 'Releases') {
      setReleases([]);
      setReleasePage(1); setReleaseHasMore(false); setReleaseAvailability(null);
      if (!selected.capabilities.includes('releases.read')) return () => { active = false; };
      setReleaseLoading(true);
      void api.hubReleases(selected.id, 1, 20).then((page) => {
        if (!active) return;
        setReleases(page.items); setReleasePage(page.page); setReleaseHasMore(page.has_more);
        setReleaseAvailability(page.availability);
      }).catch((err) => {
        if (active) setReleaseAvailability({ available: false, state: 'error', reason: messageFrom(err) });
      }).finally(() => { if (active) setReleaseLoading(false); });
    } else if (tab === 'Tags') {
      setTags([]);
      setTagPage(1); setTagHasMore(false); setTagAvailability(null);
      setCompareFrom(''); setCompareTo(''); setTagComparison(null); setTagCompareError('');
      if (!selected.capabilities.includes('tags.read')) return () => { active = false; };
      setTagLoading(true);
      void api.hubTags(selected.id, 1, 20).then((page) => {
        if (!active) return;
        setTags(page.items); setTagPage(page.page); setTagHasMore(page.has_more);
        setTagAvailability(page.availability);
      }).catch((err) => {
        if (active) setTagAvailability({ available: false, state: 'error', reason: messageFrom(err) });
      }).finally(() => { if (active) setTagLoading(false); });
    } else if (tab === 'Settings') {
      void api.hubRepositoryRelations(selected.id).then((items) => { if (active) setRelations(items); }).catch((err) => { if (active) setError(messageFrom(err)); });
    }
    return () => { active = false; };
  }, [selected?.id, tab, branch]);

  const filteredRepositories = useMemo(() => {
    const query = repositorySearch.trim().toLowerCase();
    return repositories.filter((item) =>
      (!collectionFilter || (item.collection_ids ?? []).includes(collectionFilter)) &&
      (!pinnedOnly || item.pinned) &&
      (repositoryProviderFilter === 'all' || item.provider === repositoryProviderFilter) &&
      (!query || [item.full_path, item.local_alias, item.description, item.host, item.namespace, item.name, item.provider]
        .some((value) => (value ?? '').toLowerCase().includes(query))));
  }, [repositories, collectionFilter, pinnedOnly, repositorySearch, repositoryProviderFilter]);

  const providerGroups = useMemo(() => connections.map((connection) => ({
    connection,
    items: filteredRepositories.filter((item) => item.connection_id === connection.id),
  })).filter((group) => group.items.length > 0), [connections, filteredRepositories]);

  const filteredIssues = useMemo(() => {
    const query = issueSearch.trim().toLowerCase();
    return issues.filter((item) => {
      const matchesState = issueStateFilter === 'all' ||
        (issueStateFilter === 'open' ? ['open', 'opened'].includes(item.state) : ['closed', 'done'].includes(item.state));
      const searchable = [item.title, item.body, item.author, item.assignee, String(item.number), ...(item.labels ?? [])];
      return matchesState && (!query || searchable.some((value) => (value ?? '').toLowerCase().includes(query)));
    });
  }, [issues, issueSearch, issueStateFilter]);

  async function perform(action: () => Promise<void>, success?: string) {
    if (busy) return;
    setBusy(true); setError(''); setNotice('');
    try { await action(); if (success) setNotice(success); }
    catch (err) { setError(messageFrom(err)); }
    finally { setBusy(false); }
  }

  function openRepository(repository: HubRepository) {
    setSelected(repository);
    setTab('Overview');
    setFile(null);
    setFileOriginalContent('');
    setShowDiffPreview(false);
    navigate(`/repositories/remote/${encodeURIComponent(repository.id)}/overview`);
  }

  async function toggleRepositoryPinned(repository: HubRepository) {
    await perform(async () => {
      const preferences = await api.hubRepositoryPreferences(repository.id);
      await api.saveHubRepositoryPreferences(repository.id, {
        local_alias: preferences.local_alias || '',
        private_note: preferences.private_note || '',
        pinned: !repository.pinned,
      });
      await reload();
    }, repository.pinned ? 'Repository unpinned in this IDIOS workspace.' : 'Repository pinned in this IDIOS workspace.');
  }

  async function removeRepositoryFromCollection(repository: HubRepository, collectionID: string) {
    const collection = collections.find((item) => item.id === collectionID);
    if (!collection) return;
    await perform(async () => {
      await api.removeHubCollectionRepository(collectionID, repository.id);
      await reload();
    }, `Removed from collection ${collection.name}. The repository remains linked.`);
  }

  async function unlinkRepository(repository: HubRepository) {
    const confirmed = window.confirm(
      `Unlink ${repository.full_path} from IDIOS? This removes its local Hub link and cached provider metadata. It does not delete the upstream repository or a separate local Git working copy.`,
    );
    if (!confirmed) return;
    await perform(async () => {
      await api.unlinkHubRepository(repository.id);
      if (selected?.id === repository.id) setSelected(null);
      await reload();
    }, 'Repository unlinked from this IDIOS workspace. The upstream repository was not changed.');
  }

  async function copyCloneURL(value: string, kind: 'HTTPS' | 'SSH') {
    try {
      await navigator.clipboard.writeText(value);
      setNotice(`${kind} clone URL copied.`);
      setError('');
    } catch {
      setError('Clipboard access is unavailable. Select and copy the clone URL manually.');
    }
  }

  async function copyRemoteFileValue(value: string, label: string) {
    try {
      await navigator.clipboard.writeText(value);
      setNotice(`${label} copied.`);
      setError('');
    } catch {
      setError('Clipboard access is unavailable. Select and copy the value manually.');
    }
  }

  async function startOAuth() {
    const configured = status?.providers.find((item) => item.id === provider)?.oauth_available;
    if (!configured) {
      setError(`OAuth is not configured for ${provider}. Use a provider token or configure its OAuth client in the IDIOS server environment.`);
      return;
    }
    setError(''); setNotice('');
    try {
      const result = await api.hubOAuthStart(provider, host.trim());
      window.location.assign(result.authorization_url);
    } catch (err) { setError(messageFrom(err)); }
  }

  async function connect(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken.trim()) return;
    await perform(async () => {
      await api.connectHubProvider({ provider, host: host.trim(), access_token: accessToken.trim() });
      setAccessToken('');
      await reload();
    }, 'Provider account verified and connected.');
  }

  async function discover(page = 1) {
    if (!discoveryConnection) return;
    setDiscoveryBusy(true); setError('');
    if (page === 1) setDiscovery([]);
    try {
      const result = await api.discoverHubRepositories(discoveryConnection, search, page);
      setDiscovery((current) => page === 1 ? result.items : [...current, ...result.items]);
      setDiscoveryPage(result.page);
      setDiscoveryHasMore(result.items.length === 100);
      if (page === 1 && !result.items.length) setNotice('No repositories were returned for this account and search.');
    } catch (err) { setError(messageFrom(err)); }
    finally { setDiscoveryBusy(false); }
  }

  async function importRepository(remote: HubDiscoveredRepository) {
    if (!discoveryConnection) return;
    await perform(async () => {
      const linked = await api.importHubRepository({ connection_id: discoveryConnection, external_id: remote.external_id, full_path: remote.full_path });
      await reload();
      setSelected(linked);
      setTab('Overview');
      setDiscovery((items) => items.filter((item) => item.external_id !== remote.external_id));
    }, 'Repository explicitly linked to IDIOS.');
  }

  async function savePreferences(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected) return;
    await perform(async () => {
      const prefs = await api.saveHubRepositoryPreferences(selected.id, {
        local_alias: repositoryAlias, private_note: repositoryNote, pinned: repositoryPinned,
      });
      setRepositoryNote(prefs.private_note ?? '');
      setRepositoryPinned(prefs.pinned);
      await reload();
    }, 'Repository preferences saved locally.');
  }

  async function linkSelectedProject() {
    if (!selected || !projectToLink) return;
    await perform(async () => {
      await api.linkHubRepositoryProject(selected.id, {
        project_id: projectToLink, relationship: projectLinkRelationship,
      });
      setHubProjectLinks(await api.hubRepositoryProjects(selected.id));
      setProjectToLink('');
    }, 'Hub repository linked to the IDIOS Project.');
  }

  async function unlinkSelectedProject(projectID: string) {
    if (!selected) return;
    await perform(async () => {
      await api.unlinkHubRepositoryProject(selected.id, projectID);
      setHubProjectLinks(await api.hubRepositoryProjects(selected.id));
    }, 'Project association removed locally.');
  }

  async function makeSelectedProjectPrimary(projectID: string) {
    if (!selected) return;
    await perform(async () => {
      await api.setHubRepositoryProjectPrimary(selected.id, projectID);
      setHubProjectLinks(await api.hubRepositoryProjects(selected.id));
    }, 'Primary Project repository updated locally.');
  }

  async function createCollection(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!collectionName.trim()) return;
    await perform(async () => {
      await api.createHubCollection({ name: collectionName.trim(), description: collectionDescription.trim() });
      setCollectionName(''); setCollectionDescription('');
      await reload();
    }, 'Collection created.');
  }

  async function addToCollection(collectionID: string) {
    if (!selected) return;
    await perform(async () => {
      await api.addHubCollectionRepository(collectionID, selected.id);
      await reload();
    }, 'Repository added to collection.');
  }

  async function deleteCollection(collection: HubCollection) {
    if (!window.confirm(`Delete collection “${collection.name}”? Linked repositories will remain in the Hub.`)) return;
    await perform(async () => {
      await api.deleteHubCollection(collection.id);
      if (collectionFilter === collection.id) setCollectionFilter('');
      await reload();
    }, 'Collection deleted.');
  }

  async function loadMoreCommits() {
    if (!selected || !supports('commits.read') || commitLoading || !commitHasMore) return;
    const repositoryID = selected.id;
    const currentBranch = branch;
    const nextPage = commitPage + 1;
    setCommitLoading(true);
    setError('');
    try {
      const items = await api.hubCommits(repositoryID, currentBranch, nextPage);
      if (selected?.id !== repositoryID || branch !== currentBranch) return;
      setCommits((current) => [...current, ...items]);
      setCommitPage(nextPage);
      setCommitHasMore(items.length === 50);
    } catch (err) {
      setError(messageFrom(err));
    } finally {
      setCommitLoading(false);
    }
  }

  async function loadCommitDetails(item: HubCommit) {
    if (!selected) return;
    const repositoryID = selected.id;
    const currentBranch = branch;
    setCommitDetailLoading(item.sha);
    setCommitDetailErrors((current) => ({ ...current, [item.sha]: '' }));
    try {
      const details = await api.hubCommit(repositoryID, item.sha, currentBranch);
      if (selected?.id !== repositoryID || branch !== currentBranch) return;
      setCommitDetails((current) => ({ ...current, [item.sha]: details }));
    } catch (err) {
      setCommitDetailErrors((current) => ({ ...current, [item.sha]: messageFrom(err) }));
    } finally {
      setCommitDetailLoading((current) => current === item.sha ? '' : current);
    }
  }

  async function toggleCommitDetails(item: HubCommit) {
    if (expandedCommitSHA === item.sha) {
      setExpandedCommitSHA('');
      return;
    }
    setExpandedCommitSHA(item.sha);
    if (!commitDetails[item.sha]) await loadCommitDetails(item);
  }

  async function loadMoreReleases() {
    if (!selected || releaseLoading || !releaseHasMore) return;
    const repositoryID = selected.id;
    const nextPage = releasePage + 1;
    setReleaseLoading(true);
    try {
      const result = await api.hubReleases(repositoryID, nextPage, 20);
      if (selected?.id !== repositoryID) return;
      setReleases((current) => [...current, ...result.items]);
      setReleasePage(result.page);
      setReleaseHasMore(result.has_more);
      setReleaseAvailability(result.availability);
    } catch (err) {
      setReleaseAvailability({ available: false, state: 'error', reason: messageFrom(err) });
    } finally {
      setReleaseLoading(false);
    }
  }

  async function loadMoreTags() {
    if (!selected || tagLoading || !tagHasMore) return;
    const repositoryID = selected.id;
    const nextPage = tagPage + 1;
    setTagLoading(true);
    try {
      const result = await api.hubTags(repositoryID, nextPage, 20);
      if (selected?.id !== repositoryID) return;
      setTags((current) => [...current, ...result.items]);
      setTagPage(result.page);
      setTagHasMore(result.has_more);
      setTagAvailability(result.availability);
    } catch (err) {
      setTagAvailability({ available: false, state: 'error', reason: messageFrom(err) });
    } finally {
      setTagLoading(false);
    }
  }

  async function compareSelectedTags() {
    if (!selected || !compareFrom || !compareTo || compareFrom === compareTo) return;
    const comparisonRepositoryID = selected.id;
    setTagCompareLoading(true);
    setTagCompareError('');
    setTagComparison(null);
    try {
      const comparison = await api.hubCompareTags(comparisonRepositoryID, compareFrom, compareTo);
      if (selected?.id === comparisonRepositoryID) setTagComparison(comparison);
    } catch (err) {
      setTagCompareError(messageFrom(err));
    } finally {
      setTagCompareLoading(false);
    }
  }

  async function openRemoteFile(path: string) {
    if (!selected || !branch) return;
    setFileLoading(true); setError(''); setFile(null); setShowDiffPreview(false);
    try {
      const result = await api.hubFile(selected.id, path, branch);
      setFile(result); setFilePath(result.path); setFileContent(result.content); setFileOriginalContent(result.content);
      setFileCreateOnly(false); setShowDiffPreview(false); setFileCommitMessage('');
    } catch (err) { setError(messageFrom(err)); }
    finally { setFileLoading(false); }
  }

  async function loadMoreRemoteTree() {
    if (!selected || !branch || !treeHasMore || treeLoading) return;
    const repositoryID = selected.id;
    const currentBranch = branch;
    setTreeLoading(true);
    try {
      const result = await api.hubTree(repositoryID, currentBranch, '', '', treePage + 1);
      if (selected?.id !== repositoryID || branch !== currentBranch) return;
      setTree((current) => [...current, ...result.items]);
      setTreePage(result.page);
      setTreeHasMore(result.has_more);
    } catch (err) {
      setError(messageFrom(err));
    } finally {
      setTreeLoading(false);
    }
  }

  async function saveRemoteFile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected || !branch || !filePath.trim() || !fileCommitMessage.trim()) return;
    await perform(async () => {
      const result = await api.saveHubFile(selected.id, {
        path: filePath.trim(), branch, content: fileContent, commit_message: fileCommitMessage.trim(),
        expected_sha: file?.sha || file?.last_commit_id, create_only: fileCreateOnly || !file,
      });
      setFile(result); setFileOriginalContent(result.content); setShowDiffPreview(false); setFileCreateOnly(false); setFileCommitMessage('');
      const rootPage = await api.hubTree(selected.id, branch);
      setTree(rootPage.items); setTreePage(rootPage.page); setTreeHasMore(rootPage.has_more);
    }, 'Remote file committed successfully.');
  }

  async function createIssue(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected || !issueTitle.trim()) return;
    const customFields: Record<string, string> = {};
    if (selected.provider === 'sourceforge') {
      try {
        const parsed: unknown = JSON.parse(issueCustomFieldsJSON || '{}');
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
          throw new Error('Tracker custom fields must be a JSON object.');
        }
        for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
          if (typeof value !== 'string') throw new Error(`Custom field ${key} must have a string value.`);
          customFields[key] = value;
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Invalid Tracker custom fields JSON.');
        return;
      }
    }
    await perform(async () => {
      await api.createHubIssue(selected.id, {
        title: issueTitle.trim(), body: issueBody,
        labels: issueLabels.split(',').map((s) => s.trim()).filter(Boolean),
        status: selected.provider === 'sourceforge' ? issueStatus.trim() : undefined,
        assignee: selected.provider === 'sourceforge' ? issueAssignee.trim() : undefined,
        private: selected.provider !== 'github' ? issuePrivate : undefined,
        discussion_disabled: selected.provider === 'sourceforge' ? issueDiscussionDisabled : undefined,
        custom_fields: selected.provider === 'sourceforge' ? customFields : undefined,
      });
      setIssueTitle(''); setIssueBody(''); setIssueLabels('');
      setIssueAssignee(''); setIssueStatus(''); setIssuePrivate(false);
      setIssueDiscussionDisabled(false); setIssueCustomFieldsJSON('{}');
      setIssues(await api.hubIssues(selected.id));
      setIssueCreateOpen(false);
    }, 'Issue created on the provider.');
  }

  function beginEditIssue(item: HubIssue) {
    setEditingIssueID(item.id);
    setEditIssueTitle(item.title);
    setEditIssueBody(item.body);
    setEditIssueState(item.state);
    setEditIssueAssignee(item.assignee ?? '');
    setEditIssueLabels((item.labels ?? []).join(', '));
    setEditIssuePrivate(Boolean(item.private));
    setEditIssueDiscussionDisabled(Boolean(item.discussion_disabled));
    const fields: Record<string, string> = {};
    for (const [key, value] of Object.entries(item.custom_fields ?? {})) {
      fields[key] = typeof value === 'string' ? value : String(value ?? '');
    }
    setEditIssueCustomFieldsJSON(JSON.stringify(fields, null, 2));
  }

  async function saveIssueEdit(event: FormEvent<HTMLFormElement>, item: HubIssue) {
    event.preventDefault();
    if (!selected) return;
    const customFields: Record<string, string> = {};
    if (selected.provider === 'sourceforge') {
      try {
        const parsed: unknown = JSON.parse(editIssueCustomFieldsJSON || '{}');
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
          throw new Error('Tracker custom fields must be a JSON object.');
        }
        for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
          if (typeof value !== 'string') throw new Error(`Custom field ${key} must have a string value.`);
          customFields[key] = value;
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Invalid Tracker custom fields JSON.');
        return;
      }
    }
    await perform(async () => {
      const updated = await api.updateHubIssue(selected.id, item.number, {
        title: editIssueTitle, body: editIssueBody, state: editIssueState,
        assignee: selected.provider === 'gitlab' ? undefined : editIssueAssignee,
        labels: editIssueLabels.split(',').map((value) => value.trim()).filter(Boolean),
        private: selected.provider === 'github' ? undefined : editIssuePrivate,
        discussion_disabled: selected.provider === 'sourceforge' ? editIssueDiscussionDisabled : undefined,
        custom_fields: selected.provider === 'sourceforge' ? customFields : undefined,
      });
      setIssues((items) => items.map((current) => current.id === item.id ? updated : current));
      setEditingIssueID('');
    }, 'Provider issue updated.');
  }

  async function addComment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected || !commentNumber || !commentBody.trim()) return;
    await perform(async () => {
      await api.commentHubIssue(selected.id, commentNumber, commentBody.trim());
      setCommentNumber(null); setCommentBody('');
    }, 'Comment sent to the provider.');
  }

  async function createChange(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected || !changeTitle.trim() || !changeSource || !changeTarget) return;
    if (changeSource === changeTarget) {
      setError('Choose two different branches for a change request.');
      return;
    }
    await perform(async () => {
      await api.createHubChange(selected.id, { title: changeTitle.trim(), body: changeBody, source_branch: changeSource, target_branch: changeTarget });
      setChangeTitle(''); setChangeBody('');
      setChanges(await api.hubChanges(selected.id));
      setChangeCreateOpen(false);
    }, 'Change request created on the provider.');
  }

  async function createRelation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected || !targetRelation || selected.id === targetRelation) return;
    await perform(async () => {
      await api.createHubRepositoryRelation(selected.id, { target_repository_id: targetRelation, relation_type: relationType, description: relationDescription.trim() });
      setTargetRelation(''); setRelationDescription('');
      setRelations(await api.hubRepositoryRelations(selected.id));
    }, 'Repository relation saved.');
  }

  const capabilities = selected?.capabilities ?? [];
  const supports = (name: string) => capabilities.includes(name);
  const otherRepositories = repositories.filter((item) => item.id !== selected?.id);
  const selectedCollectionIDs = selected?.collection_ids ?? [];

  return <div className={standalone ? 'hub-workspace hub-workspace--standalone' : 'hub-workspace'}>
    <div className="hub-heading">
      <div>
        <p className="hub-heading__context">{standalone ? selected?.provider.toUpperCase() || 'REMOTE REPOSITORY' : 'REMOTE SOURCES · PROVIDER WORKSPACES'}</p>
        {standalone && <strong className="hub-heading__title">{selected?.local_alias || selected?.full_path || 'Repository'}</strong>}
      </div>
      <div className="hub-heading__actions">
        {!standalone && <Button variant="secondary" onClick={() => setProviderSetupOpen((open) => !open)}>{providerSetupOpen ? 'Hide provider setup' : 'Manage providers'}</Button>}
        <Button variant="secondary" onClick={() => void reload()} loading={loading}><RefreshCw size={14}/> Refresh</Button>
      </div>
    </div>

    {error && <div className="notice notice--error" role="alert">{error}</div>}
    {notice && <div className="notice notice--success" role="status">{notice}</div>}

    {status && !status.credential_storage_ready && <div className="notice notice--warning hub-credential-warning" role="status">
      <LockKeyhole size={16}/>
      <div><strong>Encrypted credential storage is locked</strong>
        <p>{status.credential_storage_error || 'The IDIOS server has not loaded a valid provider encryption key.'}</p>
        <small>Set the key in the environment that launches IDIOS, stop the current server process, and start it again. The key is read when the server starts.</small>
      </div>
    </div>}

    {!selected && !standalone && providerSetupOpen && <div className="hub-setup-grid">
      <section className="hub-section">
        <div className="hub-section-heading"><div><h3>Provider connections</h3><p>Use a personal access token or provider bearer token. IDIOS verifies it before saving an encrypted copy.</p></div><Badge>{connections.length} connected</Badge></div>
        <form className="hub-form" onSubmit={connect}>
          <div className="hub-form-grid">
            <Field label="Provider"><select value={provider} onChange={(event) => { const next = event.target.value as HubProvider; setProvider(next); setHost(next === 'github' ? 'https://github.com' : next === 'sourceforge' ? 'https://sourceforge.net' : 'https://gitlab.com'); }}><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="sourceforge">SourceForge / Allura</option></select></Field>
            <Field label="Host" hint={provider === 'gitlab' ? 'Use your GitLab origin, including self-managed instances.' : 'Hosted provider endpoint.'}><input value={host} onChange={(event) => setHost(event.target.value)} placeholder="https://gitlab.example.com" /></Field>
          </div>
          {status?.providers.find((item) => item.id === provider)?.oauth_available && <Button type="button" variant="secondary" disabled={!status?.credential_storage_ready || busy} onClick={() => void startOAuth()}><ShieldCheck size={14}/> Continue with {provider === 'github' ? 'GitHub' : 'GitLab'} OAuth</Button>}
          {provider === 'sourceforge' && <p className="hub-muted">SourceForge Allura accepts a Bearer token generated from <a href="https://sourceforge.net/auth/oauth/" target="_blank" rel="noreferrer">SourceForge OAuth settings</a>; the account's API token is entered here directly.</p>}
          <Field label="Access token / Bearer token" hint="Fallback for personal access tokens and SourceForge. Sent only to the local IDIOS API and encrypted before storage."><input type="password" autoComplete="off" value={accessToken} onChange={(event) => setAccessToken(event.target.value)} required placeholder="Paste a provider token" /></Field>
          <Button type="submit" disabled={!status?.credential_storage_ready || !accessToken.trim() || busy}><ShieldCheck size={14}/> {busy ? 'Verifying…' : 'Verify & connect with token'}</Button>
          {!status?.credential_storage_ready && <p className="hub-muted hub-disabled-reason">Token verification unlocks after IDIOS confirms encrypted credential storage is ready.</p>}
        </form>
        {connections.length > 0 && <div className="hub-connections-list">{connections.map((item) => <div className="hub-connection-row" key={item.id}>
          <div><strong>{item.display_name || item.account_name}</strong><span>{item.provider} · {item.host} · {item.account_name}</span></div>
          <Badge tone={item.status === 'connected' ? 'green' : 'amber'}>{item.status}</Badge>
          <Button variant="ghost" disabled={busy} onClick={() => {
            const confirmed = window.confirm(`Disconnect ${item.display_name || item.account_name} from IDIOS? Its saved credentials and linked remote references will be removed from this workspace. This does not delete upstream repositories or separate local Git working copies.`);
            if (confirmed) void perform(async () => { await api.deleteHubConnection(item.id); await reload(); }, 'Provider disconnected. Upstream repositories and local Git working copies were not deleted.');
          }}><Unplug size={13}/> Disconnect</Button>
        </div>)}</div>}
      </section>

      <section className="hub-section">
        <div className="hub-section-heading"><div><h3>Discover repositories</h3><p>Choose a connection and search. Nothing is imported until you click Link.</p></div></div>
        {connections.length > 0 ? <>
          <div className="hub-discovery-controls">
            <Field label="Connection"><select value={discoveryConnection} onChange={(event) => setDiscoveryConnection(event.target.value)}><option value="">Choose provider connection…</option>{connections.map((item) => <option key={item.id} value={item.id}>{item.provider} · {item.account_name}</option>)}</select></Field>
            <Field label="Search"><input value={search} onChange={(event) => setSearch(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); void discover(); } }} placeholder="namespace / repository" /></Field>
            <Button onClick={() => void discover()} disabled={!discoveryConnection || discoveryBusy}><Search size={14}/> {discoveryBusy ? 'Searching…' : 'Search'}</Button>
          </div>
          <div className="hub-discovery-list">
            {discovery.map((item) => <div className="hub-discovery-row" key={item.external_id}>
              <div className="hub-discovery-main"><strong>{item.full_path}</strong><span>{item.description || item.visibility || 'Provider repository'} · {item.default_branch || 'No default branch'} · {item.permission || 'permission unknown'}</span><small>{item.capabilities.join(' · ')}</small></div>
              <Button disabled={busy || repositories.some((saved) => saved.provider === item.provider && saved.host === item.host && saved.external_id === item.external_id)} onClick={() => void importRepository(item)}><Plus size={13}/> Link</Button>
            </div>)}
            {!discovery.length && <p className="hub-muted">Search a provider account to choose specific repositories.</p>}
            {discoveryHasMore && <Button type="button" variant="secondary" disabled={discoveryBusy} onClick={() => void discover(discoveryPage + 1)}>{discoveryBusy ? 'Loading…' : 'Load more repositories'}</Button>}
          </div>
        </> : <div className="hub-discovery-locked"><GitBranch size={18}/><div><strong>Connect a provider first</strong><p>Once GitHub, GitLab or SourceForge is connected, search its account and link only the repositories you choose.</p></div></div>}
      </section>
    </div>}

    {repositories.length > 0 && (standalone || !loading) && <div className={standalone ? 'hub-browser hub-browser--standalone' : 'hub-browser'}>
      <aside className="hub-sidebar">
        <div className="hub-section-heading"><div><h3>Providers</h3><p>{connections.length} accounts · {repositories.length} linked repositories</p></div><Badge>{repositories.filter((item) => item.pinned).length} pinned</Badge></div>
        <div className="hub-work-item hub-search-row"><Field label="Search repositories"><input value={repositorySearch} onChange={(event) => setRepositorySearch(event.target.value)} placeholder="Search name, owner or description" /></Field><Field label="Provider"><select value={repositoryProviderFilter} onChange={(event) => setRepositoryProviderFilter(event.target.value as HubProvider | 'all')}><option value="all">All providers</option><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="sourceforge">SourceForge</option></select></Field></div>
        <Field label="Collection"><select value={collectionFilter} onChange={(event) => setCollectionFilter(event.target.value)}><option value="">All collections</option>{collections.map((item) => <option key={item.id} value={item.id}>{item.name} ({item.repository_count})</option>)}</select></Field>
        <label className="hub-checkbox"><input type="checkbox" checked={pinnedOnly} onChange={(event) => setPinnedOnly(event.target.checked)}/><Pin size={13}/> Pinned only</label>
        {collections.length > 0 && <div className="hub-collection-management">{collections.map((item) => <div key={item.id}><span>{item.name} · {item.repository_count}</span><button type="button" disabled={busy} onClick={() => void deleteCollection(item)} aria-label={`Delete collection ${item.name}`}><Trash2 size={12}/></button></div>)}</div>}
        <div className="hub-provider-groups">
          {providerGroups.map(({ connection, items }) => <section className="hub-provider-group" key={connection.id}>
            <header><div><strong>{connection.display_name || connection.account_name}</strong><small>{connection.provider.toUpperCase()} · {connection.host}</small></div><div className="hub-provider-group__badges"><Badge tone={connection.status === 'connected' ? 'green' : 'amber'}>{connection.status}</Badge><Badge>{items.length} repos</Badge></div></header>
            {items.map((item) => <div className="hub-provider-repository" key={item.id}>
              <button type="button" className={selected?.id === item.id ? 'hub-repository-choice is-active' : 'hub-repository-choice'} onClick={() => openRepository(item)}>
                <span className="hub-repository-provider">{item.pinned ? 'PINNED' : item.visibility || 'REPOSITORY'}</span>
                <strong>{item.local_alias || item.full_path}</strong>
                <small>{item.default_branch || 'Default branch unavailable'} · {item.language || item.repository_type}</small>
              </button>
              <details className="hub-repository-menu">
                <summary aria-label={`Actions for ${item.full_path}`} title="Repository actions"><MoreHorizontal size={15}/></summary>
                <div className="hub-repository-menu__content">
                  <button type="button" disabled={busy} onClick={() => void toggleRepositoryPinned(item)}><Pin size={13}/>{item.pinned ? 'Unpin repository' : 'Pin repository'}</button>
                  {(item.collection_ids ?? []).map((collectionID) => {
                    const collection = collections.find((entry) => entry.id === collectionID);
                    if (!collection) return null;
                    return <button type="button" key={collectionID} disabled={busy} onClick={() => void removeRepositoryFromCollection(item, collectionID)}><Trash2 size={13}/>Remove from {collection.name}</button>;
                  })}
                  <button type="button" className="hub-repository-menu__danger" disabled={busy} onClick={() => void unlinkRepository(item)}><Unplug size={13}/>Unlink from IDIOS</button>
                </div>
              </details>
            </div>)}
          </section>)}
          {!providerGroups.length && <p className="hub-muted">No linked repositories match these filters.</p>}
        </div>
        <form className="hub-create-collection" onSubmit={createCollection}>
          <label className="hub-field-label">New collection</label>
          <input value={collectionName} onChange={(event) => setCollectionName(event.target.value)} placeholder="Collection name" maxLength={80}/>
          <input value={collectionDescription} onChange={(event) => setCollectionDescription(event.target.value)} placeholder="Description (optional)" maxLength={2000}/>
          <Button type="submit" variant="secondary" disabled={!collectionName.trim() || busy}><Plus size={13}/> Create collection</Button>
        </form>
      </aside>

      <section className="hub-main">
        {!selected ? <EmptyState title="Choose a linked repository" body="Link a repository from one of your provider connections, then inspect its real remote code and work items here." icon={<GitBranch size={22}/>} /> : <>
          <div className="hub-repository-header">
            <div><p className="eyebrow">{selected.provider.toUpperCase()} · {selected.visibility || 'REMOTE REPOSITORY'}</p><h3>{selected.local_alias || selected.full_path}</h3><p>{selected.description || selected.web_url}</p></div>
            <a href={selected.web_url} target="_blank" rel="noreferrer" className="button button--secondary">Open provider <ArrowUpRight size={13}/></a>
          </div>
          <div className="hub-repository-header__actions"><Button variant="secondary" disabled={busy} onClick={() => void perform(async () => { const updated = await api.syncHubRepository(selected.id); await reload(); setSelected(updated); }, 'Repository metadata synchronized.') }><RefreshCw size={13}/> Sync metadata</Button></div>
          <div className="hub-repository-meta"><span>{selected.permission || 'Permission unknown'}</span><span>{selected.default_branch || 'Default branch unknown'}</span><span>{selected.sync_state}</span><span>{selected.open_issue_count} open issues</span></div>
          <nav className="hub-tabs" aria-label="Remote repository views">{(['Overview','Code','Issues','Change Requests','Commits','Releases','Tags','Settings'] as RemoteTab[]).map((item) => <button key={item} type="button" className={tab === item ? 'is-active' : ''} onClick={() => {
            setTab(item);
            if (standalone && selected) navigate(`/repositories/remote/${encodeURIComponent(selected.id)}/${remoteViewFromTab(item)}`);
          }}>{item}</button>)}</nav>

          {tab === 'Overview' && <section className="hub-overview">
            <div className="hub-overview-hero">
              <div><p className="eyebrow">{selected.provider.toUpperCase()} · {selected.visibility || 'REMOTE REPOSITORY'}</p><h3>{selected.local_alias || selected.full_path}</h3><p>{selected.description || 'No description provided by the provider.'}</p></div>
              <a href={selected.web_url} target="_blank" rel="noreferrer" className="button button--secondary">Open provider <ArrowUpRight size={13}/></a>
            </div>
            <div className="hub-overview-metrics">
              <article><span>Default branch</span><strong><GitBranch size={14}/>{selected.default_branch || 'Unavailable'}</strong><small>{selected.permission ? `Permission: ${selected.permission}` : 'Permission not reported'}</small></article>
              <article><span>Open issues</span><strong>{selected.capabilities.includes('issues.read') ? selected.open_issue_count : '—'}</strong><small>{selected.capabilities.includes('issues.read') ? 'Reported by provider' : 'Not supported by this connection'}</small></article>
              <article><span>Change requests</span><strong>{selected.capabilities.includes('changes.read') ? selected.open_change_request_count : '—'}</strong><small>{selected.capabilities.includes('changes.read') ? 'Reported by provider' : 'Not supported by this connection'}</small></article>
              <article><span>Synchronization</span><strong><Badge tone={selected.sync_state === 'ok' ? 'green' : selected.sync_state === 'error' ? 'red' : selected.sync_state === 'stale' ? 'amber' : 'neutral'}>{syncStateLabel(selected.sync_state)}</Badge></strong><small>{selected.last_synced_at ? `Last synced ${new Date(selected.last_synced_at).toLocaleString()}` : 'No successful sync recorded'}</small></article>
            </div>
            <section className="hub-latest-release">
              <div className="hub-detail-toolbar">
                <div><strong>Latest release</strong><p className="hub-muted">The newest provider release is fetched independently from the full release history.</p></div>
                <Button type="button" variant="ghost" disabled={!selected.capabilities.includes('releases.read')} onClick={() => setTab('Releases')}>Release history <ArrowUpRight size={12}/></Button>
              </div>
              {!selected.capabilities.includes('releases.read')
                ? <p className="hub-muted">This provider does not expose a Releases API. Git tags can still be browsed separately.</p>
                : latestRelease
                  ? <article className="hub-latest-release__item">
                      <div><strong>{latestRelease.name || latestRelease.tag_name}</strong><span>{latestRelease.tag_name}</span></div>
                      <div className="hub-latest-release__meta">
                        <Badge>{latestRelease.state}</Badge>
                        <small>{latestRelease.published_at ? new Date(latestRelease.published_at).toLocaleDateString() : 'Publication date not reported'}</small>
                        <small>{latestRelease.assets.length} assets</small>
                      </div>
                      {latestRelease.url && <a href={latestRelease.url} target="_blank" rel="noreferrer">Open release <ArrowUpRight size={12}/></a>}
                    </article>
                  : latestReleaseAvailability?.state === 'unsupported'
                    ? <p className="hub-muted">{latestReleaseAvailability.reason}</p>
                    : latestReleaseAvailability?.state === 'error'
                      ? <p className="hub-muted">{latestReleaseAvailability.reason || 'Latest release could not be loaded.'}</p>
                      : <p className="hub-muted">No published releases were returned.</p>}
            </section>
            <section className="hub-overview-description">
              <div><span>Description</span><p>{selected.description || 'No description provided by the provider.'}</p></div>
              <div><span>Repository</span><code>{selected.full_path}</code></div>
              <div><span>Host</span><code>{selected.host}</code></div>
              <div><span>Primary language</span><strong>{selected.language || 'Not reported'}</strong></div>
              <div><span>Repository visibility</span><strong>{selected.visibility || 'Not reported'}</strong></div>
              <div><span>Last metadata update</span><strong>{selected.last_synced_at ? new Date(selected.last_synced_at).toLocaleString() : 'Not synced yet'}</strong></div>
            </section>
            <section className="hub-overview-clone">
              <div><strong>Clone repository</strong><p>Use the URL with a local Git client or copy it for another device.</p></div>
              <div className="hub-overview-clone__actions">
                {selected.clone_url || selected.clone_ssh_url ? <details className="hub-clone-menu">
                  <summary><Copy size={13}/> Clone options <ChevronDown size={13}/></summary>
                  <div className="hub-clone-menu__content">
                    {selected.clone_url && <button type="button" onClick={(event) => { void copyCloneURL(selected.clone_url || '', 'HTTPS'); event.currentTarget.closest('details')?.removeAttribute('open'); }}>Copy HTTPS clone URL</button>}
                    {selected.clone_ssh_url && <button type="button" onClick={(event) => { void copyCloneURL(selected.clone_ssh_url || '', 'SSH'); event.currentTarget.closest('details')?.removeAttribute('open'); }}>Copy SSH clone URL</button>}
                  </div>
                </details> : <span>Clone URLs are not available for this repository.</span>}
              </div>
            </section>
            <RemoteReadmePreview repository={selected} />
          </section>}

          {tab === 'Code' && !supports('code.read') && <div className="hub-provider-limitation"><GitBranch size={24}/><h4>Remote code browser unavailable</h4><p>{selected.provider === 'sourceforge' ? 'SourceForge Allura does not expose a REST tree/file API. Use the repository clone endpoint with Git for checkout, branches, commits, and pushes.' : 'This connection does not have permission to read repository code.'}</p>{selected.clone_url && <a href={selected.clone_url}>{selected.clone_url}</a>}</div>}
          {tab === 'Code' && supports('code.read') && <div className="hub-code-layout">
            <div className="hub-code-tree">
              <div className="hub-detail-toolbar"><strong>Remote tree</strong><Field label="Branch"><select value={branch} onChange={(event) => setBranch(event.target.value)}>{branches.map((item) => <option key={item.name} value={item.name}>{item.name}{item.default ? ' · default' : ''}</option>)}</select></Field></div>
              <RemoteTreeView
                key={`${selected.id}:${branch}`}
                tree={tree}
                selectedPath={filePath}
                onOpenFile={(path) => void openRemoteFile(path)}
                emptyMessage={branches.length ? "No files returned for this branch." : "This provider does not expose a REST tree browser for this repository."}
                loadChildren={(entry, page) => api.hubTree(selected.id, branch, entry.path, entry.sha ?? '', page)}
                rootHasMore={treeHasMore}
                rootLoading={treeLoading}
                onLoadMoreRoot={() => void loadMoreRemoteTree()}
              />
            </div>
            <div className="hub-file-editor">
              <div className="hub-detail-toolbar"><strong>{file?.path || (fileCreateOnly ? 'New remote file' : 'Remote file viewer')}</strong><div className="hub-file-toolbar-actions">{fileLoading && <span>Loading…</span>}<Button type="button" variant="secondary" disabled={!supports('code.write') || busy} onClick={() => { setFile(null); setFilePath(''); setFileContent(''); setFileOriginalContent(''); setShowDiffPreview(false); setFileCreateOnly(true); setFileCommitMessage(''); }}><Plus size={13}/> New file</Button></div></div>
              {file || fileCreateOnly ? <form onSubmit={saveRemoteFile}>
                <div className="hub-file-actions">
                  <span>{file ? `Revision ${file.sha || file.last_commit_id || '—'} · ${file.size.toLocaleString()} bytes` : 'New file · not yet committed'}</span>
                  {file && <div className="hub-item-actions">
                    <Button type="button" variant="ghost" onClick={() => void copyRemoteFileValue(file.path, 'File path')}><Copy size={13}/> Copy path</Button>
                    <Button type="button" variant="ghost" onClick={() => void copyRemoteFileValue(fileContent, 'File content')}><Copy size={13}/> Copy content</Button>
                  </div>}
                </div>
                <Field label="File path"><input value={filePath} onChange={(event) => setFilePath(event.target.value)} disabled={!fileCreateOnly} required /></Field>
                <Field label="Content"><textarea className="hub-code-textarea" value={fileContent} onChange={(event) => { setFileContent(event.target.value); setShowDiffPreview(false); }} rows={18} spellCheck={false}/></Field>
                <Field label="Commit message"><input value={fileCommitMessage} onChange={(event) => setFileCommitMessage(event.target.value)} required placeholder="Describe this remote commit" /></Field>
                <div className="hub-item-actions"><Button type="button" variant="secondary" disabled={!supports('code.write') || fileContent === fileOriginalContent} onClick={() => setShowDiffPreview(true)}>Preview diff</Button><Button type="submit" disabled={busy || !supports('code.write') || !fileCommitMessage.trim() || !showDiffPreview}><GitCommitHorizontal size={14}/> Commit file</Button></div>
                {showDiffPreview && <div className="hub-diff-preview"><strong>Uncommitted diff · {filePath}</strong><pre>{diffLines.map((line, index) => <div key={index} className={`hub-diff-line hub-diff-line--${line.kind}`}>{line.kind === 'add' ? '+' : line.kind === 'remove' ? '−' : ' '} {line.text}</div>)}</pre></div>}
                {!supports('code.write') && <p className="hub-muted">This connection is read-only for code changes.</p>}
              </form> : <div className="hub-file-empty"><FileCode2 size={25}/><strong>Select a file in the remote tree</strong><p>Files are fetched from the provider. Writes require provider permission and a matching file revision.</p></div>}
            </div>
          </div>}

          {tab === 'Issues' && <div className="hub-work-items">
            <div className="hub-detail-toolbar">
              <Field label="Search issues"><input value={issueSearch} onChange={(event) => setIssueSearch(event.target.value)} placeholder="title, body, label, author or number" /></Field>
              <Field label="State"><select value={issueStateFilter} onChange={(event) => setIssueStateFilter(event.target.value as 'all' | 'open' | 'closed')}><option value="all">All</option><option value="open">Open</option><option value="closed">Closed</option></select></Field>
              {supports('issues.write') && <Button type="button" onClick={() => { setError(''); setIssueCreateOpen(true); }} disabled={busy}><Plus size={13}/> New issue</Button>}
            </div>
            {!supports('issues.read') && <p className="hub-muted">{selected.provider === 'sourceforge' ? 'This SourceForge project has no accessible Tracker tool mounted.' : 'This connection does not have permission to read Issues.'}</p>}
            {filteredIssues.map((item) => <article className="hub-work-item hub-issue-item" key={item.id}>
              <div className="hub-issue-main">
                <strong>#{item.number} · {item.title}</strong>
                <Badge tone={remoteStateTone(item.state)}>{item.state}</Badge>
                <p>{item.body}</p>
                <small>{item.author || 'Unknown author'}{item.assignee ? ` · assigned to ${item.assignee}` : ''}{item.private ? ' · private' : ''}{item.discussion_disabled ? ' · discussion disabled' : ''} · {item.url ? <a href={item.url} target="_blank" rel="noreferrer">Open issue</a> : 'Provider issue'}</small>
                {item.custom_fields && Object.keys(item.custom_fields).length > 0 && <small>Custom fields: {Object.entries(item.custom_fields).map(([key, value]) => `${key.replace(/^_/, '')}: ${String(value)}`).join(' · ')}</small>}
              </div>
              <div className="hub-item-actions">
                {supports('issues.write') && <Button variant="ghost" disabled={busy} onClick={() => editingIssueID === item.id ? setEditingIssueID('') : beginEditIssue(item)}>{editingIssueID === item.id ? 'Cancel edit' : 'Edit'}</Button>}
                <Button variant="ghost" disabled={item.discussion_disabled} onClick={() => { setCommentNumber(item.number); setCommentBody(''); }}><Plus size={13}/> {item.discussion_disabled ? 'Discussion disabled' : 'Comment'}</Button>
                {selected.provider !== 'sourceforge' && ['open','opened'].includes(item.state) && supports('issues.write') && <Button variant="ghost" disabled={busy} onClick={() => { if (window.confirm(`Close issue #${item.number} on ${selected.full_path}?`)) void perform(async () => { await api.closeHubIssue(selected.id,item.number); setIssues(await api.hubIssues(selected.id)); },'Provider issue closed.'); }}>Close</Button>}
              </div>
              {editingIssueID === item.id && <form className="hub-inline-create hub-issue-edit" onSubmit={(event) => void saveIssueEdit(event,item)}>
                <Field label="Title"><input value={editIssueTitle} onChange={(event) => setEditIssueTitle(event.target.value)} required maxLength={256}/></Field>
                <Field label="Description"><textarea rows={4} value={editIssueBody} onChange={(event) => setEditIssueBody(event.target.value)}/></Field>
                {selected.provider === 'sourceforge' ? <Field label="Tracker status"><input value={editIssueState} onChange={(event) => setEditIssueState(event.target.value)} placeholder="Project-defined status"/></Field> : <Field label="State"><select value={editIssueState === 'opened' ? 'open' : editIssueState} onChange={(event) => setEditIssueState(event.target.value)}><option value="open">Open</option><option value="closed">Closed</option></select></Field>}
                {selected.provider !== 'gitlab' && <Field label={selected.provider === 'sourceforge' ? 'Assigned to (username)' : 'Assignee (username)'}><input value={editIssueAssignee} onChange={(event) => setEditIssueAssignee(event.target.value)}/></Field>}
                <Field label="Labels (comma-separated)"><input value={editIssueLabels} onChange={(event) => setEditIssueLabels(event.target.value)}/></Field>
                {selected.provider !== 'github' && <label className="hub-checkbox"><input type="checkbox" checked={editIssuePrivate} onChange={(event) => setEditIssuePrivate(event.target.checked)}/>{selected.provider === 'gitlab' ? 'Confidential issue' : 'Private ticket'}</label>}
                {selected.provider === 'sourceforge' && <>
                  <label className="hub-checkbox"><input type="checkbox" checked={editIssueDiscussionDisabled} onChange={(event) => setEditIssueDiscussionDisabled(event.target.checked)}/> Disable ticket discussion</label>
                  <Field label="Tracker custom fields (JSON)" hint="Allura keys start with underscore. Values must be strings."><textarea rows={3} value={editIssueCustomFieldsJSON} onChange={(event) => setEditIssueCustomFieldsJSON(event.target.value)} spellCheck={false}/></Field>
                </>}
                <div className="hub-item-actions"><Button type="submit" disabled={busy || !editIssueTitle.trim()}><Check size={13}/> Save Issue</Button><Button type="button" variant="ghost" onClick={() => setEditingIssueID('')}>Cancel</Button></div>
              </form>}
            </article>)}
            {commentNumber !== null && <form className="hub-inline-create" onSubmit={addComment}><Field label={`Comment on #${commentNumber}`}><textarea rows={3} value={commentBody} onChange={(event) => setCommentBody(event.target.value)} required/></Field><Button type="submit" disabled={busy || !commentBody.trim()}>Send comment</Button><Button type="button" variant="ghost" onClick={() => setCommentNumber(null)}>Cancel</Button></form>}
            {supports('issues.read') && !filteredIssues.length && <p className="hub-muted">{issues.length ? 'No issues match these filters.' : 'No remote Issues were returned for this repository.'}</p>}
            {supports('issues.write') && <Modal
              open={issueCreateOpen}
              title="New issue"
              description={`Create an issue on ${selected.full_path} through ${selected.provider}.`}
              onClose={() => setIssueCreateOpen(false)}
              onSubmit={createIssue}
              submitLabel="Create issue"
              busy={busy}
              wide
            >
              {error && <div className="notice notice--error" role="alert">{error}</div>}
              <Field label="Title"><input autoFocus value={issueTitle} onChange={(event) => setIssueTitle(event.target.value)} required maxLength={256} placeholder="Describe the issue" /></Field>
              <Field label="Description"><textarea rows={5} value={issueBody} onChange={(event) => setIssueBody(event.target.value)} placeholder="Add context, expected behavior and reproduction steps." /></Field>
              <Field label="Labels (comma-separated)"><input value={issueLabels} onChange={(event) => setIssueLabels(event.target.value)} placeholder="bug, enhancement" /></Field>
              {selected.provider !== 'github' && <label className="hub-checkbox"><input type="checkbox" checked={issuePrivate} onChange={(event) => setIssuePrivate(event.target.checked)}/>{selected.provider === 'gitlab' ? 'Confidential issue' : 'Private ticket'}</label>}
              {selected.provider === 'sourceforge' && <>
                <div className="hub-form-grid">
                  <Field label="Tracker status (optional)"><input value={issueStatus} onChange={(event) => setIssueStatus(event.target.value)} placeholder="Project-defined status"/></Field>
                  <Field label="Assigned to (optional)"><input value={issueAssignee} onChange={(event) => setIssueAssignee(event.target.value)} placeholder="SourceForge username"/></Field>
                </div>
                <label className="hub-checkbox"><input type="checkbox" checked={issueDiscussionDisabled} onChange={(event) => setIssueDiscussionDisabled(event.target.checked)}/> Disable ticket discussion</label>
                <Field label="Tracker custom fields (JSON)" hint="Allura custom-field values must be strings."><textarea rows={3} value={issueCustomFieldsJSON} onChange={(event) => setIssueCustomFieldsJSON(event.target.value)} spellCheck={false}/></Field>
              </>}
            </Modal>}
          </div>}

          {tab === 'Change Requests' && <div className="hub-work-items">
            <div className="hub-detail-toolbar">
              <div><strong>Change requests</strong><p className="hub-muted">Review proposed changes between repository branches.</p></div>
              {supports('changes.write') && <Button type="button" onClick={() => { setError(''); setChangeCreateOpen(true); }} disabled={busy || !branches.length}><Plus size={13}/> New change request</Button>}
            </div>
            {changes.map((item) => <article className="hub-work-item" key={item.id}><div><strong>#{item.number} · {item.title}</strong><Badge tone={remoteStateTone(item.state)}>{item.draft ? 'Draft' : item.state}</Badge><p>{item.source_branch} → {item.target_branch}</p><small>{item.author || 'Unknown author'} · {item.url ? <a href={item.url} target="_blank" rel="noreferrer">Open review</a> : 'Provider change request'}</small></div><div className="hub-item-actions"><Button variant="ghost" disabled={busy || !supports('changes.write')} onClick={() => void perform(async () => { await api.approveHubChange(selected.id,item.number); setChanges(await api.hubChanges(selected.id)); },'Review approved.')}>Approve</Button><Button variant="secondary" disabled={busy || !supports('changes.merge') || !['open','opened'].includes(item.state)} onClick={() => { if (window.confirm(`Merge ${selected.full_path} change #${item.number} on the provider?`)) void perform(async () => { await api.mergeHubChange(selected.id,item.number); setChanges(await api.hubChanges(selected.id)); },'Provider merged the change.'); }}>Merge</Button><Button variant="ghost" disabled={busy || !supports('changes.write') || !['open','opened'].includes(item.state)} onClick={() => { if (window.confirm(`Close change request #${item.number} on ${selected.full_path}?`)) void perform(async () => { await api.closeHubChange(selected.id,item.number); setChanges(await api.hubChanges(selected.id)); },'Provider change request closed.'); }}>Close</Button></div></article>)}
            {!supports('changes.read') && <p className="hub-muted">This provider does not expose merge/change requests through the current API. SourceForge Allura supports Tracker and Discussion workflows, not Pull/Merge Requests.</p>}
            {supports('changes.read') && !changes.length && <p className="hub-muted">No remote change requests were returned.</p>}
            {supports('changes.write') && <Modal
              open={changeCreateOpen}
              title="New change request"
              description={`Propose changes to ${selected.full_path}.`}
              onClose={() => setChangeCreateOpen(false)}
              onSubmit={createChange}
              submitLabel="Create change request"
              busy={busy}
              wide
            >
              {error && <div className="notice notice--error" role="alert">{error}</div>}
              <Field label="Title"><input autoFocus value={changeTitle} onChange={(event) => setChangeTitle(event.target.value)} required maxLength={256} placeholder="Summarize the proposed change" /></Field>
              <Field label="Description"><textarea rows={5} value={changeBody} onChange={(event) => setChangeBody(event.target.value)} placeholder="Explain the change and its impact." /></Field>
              <div className="hub-form-grid">
                <Field label="Source branch"><select value={changeSource} onChange={(event) => setChangeSource(event.target.value)} required><option value="">Choose source…</option>{branches.map((item) => <option key={item.name} value={item.name}>{item.name}</option>)}</select></Field>
                <Field label="Target branch"><select value={changeTarget} onChange={(event) => setChangeTarget(event.target.value)} required><option value="">Choose target…</option>{branches.map((item) => <option key={item.name} value={item.name}>{item.name}</option>)}</select></Field>
              </div>
              {changeSource && changeTarget && changeSource === changeTarget && <p className="notice notice--warning">Choose two different branches for a change request.</p>}
            </Modal>}
          </div>}

          {tab === 'Commits' && <div className="hub-work-items">
            <div className="hub-detail-toolbar">
              <div><strong>Commit history</strong><p className="hub-muted">Author and committer are shown separately. Signature verification and CI checks are independent signals.</p></div>
              {branch && <Badge>{branch}</Badge>}
            </div>
            {!supports('commits.read') ? <p className="hub-muted">Commit browsing is not available through this provider's API. Use the clone URL with Git when provided.</p> : <>
              {commits.map((item) => {
                const expanded = expandedCommitSHA === item.sha;
                const detail = commitDetails[item.sha];
                const signature = item.signature_status || 'unknown';
                const signatureLabel = signature === 'unavailable' ? 'Signature unavailable' : signature === 'unknown' ? 'Signature unknown' : signature[0].toUpperCase() + signature.slice(1);
                return <article className="hub-commit-card" key={item.sha}>
                  <div className="hub-commit-card__summary">
                    <code title={item.sha}>{item.sha.slice(0, 12)}</code>
                    <div className="hub-commit-card__message">
                      <strong>{item.message.split('\n')[0]}</strong>
                      <span>{item.author || 'Author not reported'}{item.email ? ` · ${item.email}` : ''} · {item.authored_at ? new Date(item.authored_at).toLocaleString() : 'Author date unavailable'}</span>
                      <span>Committer: {item.committer || 'Not reported'}{item.committer_email ? ` · ${item.committer_email}` : ''} · {item.committed_at ? new Date(item.committed_at).toLocaleString() : 'Commit date unavailable'}</span>
                    </div>
                    <div className="hub-commit-card__signals">
                      <Badge tone={signature === 'verified' ? 'green' : signature === 'unverified' ? 'amber' : 'neutral'}>{signatureLabel}</Badge>
                      {item.checks_available
                        ? <Badge tone={item.checks_state === 'success' ? 'green' : item.checks_state === 'failure' || item.checks_state === 'failed' ? 'red' : 'neutral'}>Checks: {item.checks_state || 'reported'}</Badge>
                        : <Badge>Checks on details</Badge>}
                    </div>
                    <div className="hub-commit-card__actions">
                      <Button type="button" variant="secondary" onClick={() => void toggleCommitDetails(item)} disabled={commitDetailLoading === item.sha}>
                        {expanded ? 'Hide details' : commitDetailLoading === item.sha ? 'Loading…' : 'Details'}
                      </Button>
                      {item.url && <a href={item.url} target="_blank" rel="noreferrer">Open <ArrowUpRight size={12}/></a>}
                    </div>
                  </div>
                  {expanded && <div className="hub-commit-details">
                    {commitDetailLoading === item.sha && <p className="hub-muted">Loading commit details…</p>}
                    {commitDetailErrors[item.sha] && <div className="notice notice--error" role="alert">{commitDetailErrors[item.sha]} <button type="button" onClick={() => {
                      setCommitDetails((current) => { const next = { ...current }; delete next[item.sha]; return next; });
                      void loadCommitDetails(item);
                    }}>Retry</button></div>}
                    {detail && <>
                      <section className="hub-commit-detail-section">
                        <h4>Commit identity</h4>
                        <div className="hub-commit-provenance-grid">
                          <div className="hub-commit-provenance-grid__wide"><span>Full SHA</span><code>{detail.sha}</code></div>
                          <div className="hub-commit-provenance-grid__wide"><span>Full message</span><pre>{detail.message}</pre></div>
                        </div>
                      </section>
                      <section className="hub-commit-detail-section">
                        <h4>Cryptographic signature</h4>
                        <div className="hub-commit-provenance-grid">
                          <div><span>Status</span><strong>{detail.signature_status || 'unknown'}</strong></div>
                          <div><span>Type</span><strong>{detail.signature_type || 'Not reported'}</strong></div>
                          <div><span>Verification source</span><strong>{detail.signature_source || 'Not reported'}</strong></div>
                          <div><span>Verified at</span><strong>{detail.signature_verified_at ? new Date(detail.signature_verified_at).toLocaleString() : 'Not reported'}</strong></div>
                        </div>
                        <p>{detail.signature_reason || (detail.signature_status === 'unavailable' ? 'This provider does not expose commit signature verification.' : 'The provider did not supply a verification reason.')}</p>
                      </section>
                      <section className="hub-commit-detail-section">
                        <h4>CI and status checks</h4>
                        {!detail.checks_available
                          ? <p>Checks unavailable through this provider connection.</p>
                          : !detail.checks?.length ? <p>No status checks were reported for this commit.</p>
                          : <div className="hub-commit-check-list">{detail.checks.map((check, index) => <div key={`${check.name}-${index}`}>
                            <strong>{check.name}</strong><Badge tone={check.conclusion === 'success' || check.status === 'success' ? 'green' : check.conclusion === 'failure' || check.status === 'failed' ? 'red' : 'neutral'}>{check.conclusion || check.status}</Badge>
                            {check.description && <span>{check.description}</span>}
                            {check.url && <a href={check.url} target="_blank" rel="noreferrer">Open check</a>}
                          </div>)}</div>}
                      </section>
                      <section className="hub-commit-detail-section">
                        <h4>Changed files · {detail.stats?.total ?? 0} lines</h4>
                        <div className="hub-commit-provenance-grid">
                          <div><span>Additions</span><strong>+{detail.stats?.additions ?? 0}</strong></div>
                          <div><span>Deletions</span><strong>−{detail.stats?.deletions ?? 0}</strong></div>
                          <div><span>Files</span><strong>{detail.files?.length ?? 0}</strong></div>
                        </div>
                        {detail.files?.map((file) => <div className="hub-commit-file" key={file.path}>
                          <div><code>{file.path}</code><Badge>{file.status}</Badge><span>+{file.additions} / −{file.deletions}</span></div>
                          {file.patch && <pre>{file.patch}</pre>}
                        </div>)}
                        {detail.diff && <details className="hub-commit-full-diff"><summary>Full commit diff</summary><pre>{detail.diff}</pre></details>}
                        {!detail.diff_available && <p>{detail.diff_unavailable_reason || 'Commit diff unavailable from this provider.'}</p>}
                      </section>
                    </>}
                  </div>}
                </article>;
              })}
              {!commits.length && !commitLoading && <p className="hub-muted">No commits returned for this repository.</p>}
              {commitHasMore && <Button type="button" variant="secondary" onClick={() => void loadMoreCommits()} loading={commitLoading}>Load more commits</Button>}
              {commitLoading && !commits.length && <p className="hub-muted">Loading commit history…</p>}
            </>}
          </div>}

          {tab === 'Settings' && <div className="hub-work-items">
            <section className="hub-provider-settings hub-work-item">
              <h4>{selected.provider.toUpperCase()} repository settings</h4>
              <p className="hub-muted">Provider-owned settings are not changed by IDIOS unless a documented endpoint and the required permission are available. No such settings operation is currently enabled for this repository.</p>
              {providerSettingsURL(selected) ? <a href={providerSettingsURL(selected)} target="_blank" rel="noreferrer">Open provider settings <ArrowUpRight size={12}/></a> : <a href={selected.web_url} target="_blank" rel="noreferrer">Open provider repository <ArrowUpRight size={12}/></a>}
            </section>
            <section className="hub-relations-section">
              <h4>Repository relationships</h4>
            <form className="hub-inline-create" onSubmit={createRelation}><div className="hub-form-grid"><Field label="Target repository"><select value={targetRelation} onChange={(event) => setTargetRelation(event.target.value)}><option value="">Choose repository…</option>{otherRepositories.map((item) => <option key={item.id} value={item.id}>{item.full_path}</option>)}</select></Field><Field label="Relation"><select value={relationType} onChange={(event) => setRelationType(event.target.value)}><option value="related_to">Related to</option><option value="depends_on">Depends on</option><option value="implements">Implements</option><option value="documentation_for">Documentation for</option></select></Field></div><Field label="Description"><input value={relationDescription} onChange={(event) => setRelationDescription(event.target.value)}/></Field><Button type="submit" disabled={busy || !targetRelation}><Link2 size={13}/> Add relation</Button></form>
            {relations.map((item) => <div className="hub-work-item" key={item.id}><div><strong>{item.relation_type.replaceAll('_',' ')} · {item.target_full_path || item.target_name}</strong><p>{item.description || 'No description'}</p></div><Button variant="ghost" disabled={busy} onClick={() => void perform(async () => { await api.deleteHubRepositoryRelation(item.id); setRelations(await api.hubRepositoryRelations(selected.id)); },'Relation removed.') }><Trash2 size={13}/> Remove</Button></div>)}
            {!relations.length && <p className="hub-muted">No typed repository relations yet.</p>}
            </section>
          </div>}

          {tab === 'Settings' && <details className="hub-preferences" open><summary>IDIOS-local preferences & organization</summary>
            <form onSubmit={savePreferences} className="hub-inline-create">
              <Field label="Local alias"><input value={repositoryAlias} onChange={(event) => setRepositoryAlias(event.target.value)} maxLength={160} placeholder={selected.full_path}/></Field>
              <Field label="Private workspace note" hint="Stored only in this IDIOS workspace. It is never sent to GitHub, GitLab, or SourceForge."><textarea rows={4} value={repositoryNote} onChange={(event) => setRepositoryNote(event.target.value)} maxLength={20000} placeholder="Markdown notes about this repository…"/></Field>
              <label className="hub-checkbox"><input type="checkbox" checked={repositoryPinned} onChange={(event) => setRepositoryPinned(event.target.checked)}/><Pin size={13}/> Pin repository</label>
              <Button type="submit" disabled={busy}><Check size={13}/> Save preferences</Button>
            </form>
            <div className="hub-collection-links"><strong>Collections</strong>{collections.map((item) => <label className="hub-checkbox" key={item.id}><input type="checkbox" checked={selectedCollectionIDs.includes(item.id)} disabled={busy} onChange={() => void (selectedCollectionIDs.includes(item.id) ? perform(async () => { await api.removeHubCollectionRepository(item.id,selected.id); await reload(); }) : addToCollection(item.id))}/>{item.name}</label>)}</div>
            <div className="hub-project-links">
              <strong>Linked IDIOS Projects</strong>
              {hubProjectLinks.map((item) => <div className="hub-project-link-row" key={item.project_id}>
                <span><b>{item.project_name}</b><small>{item.relationship || 'implementation'}{item.is_primary ? ' · Primary repository' : ''}</small></span>
                {!item.is_primary && <Button variant="ghost" disabled={busy} onClick={() => void makeSelectedProjectPrimary(item.project_id)}>Make primary</Button>}
                <Button variant="ghost" disabled={busy} onClick={() => void unlinkSelectedProject(item.project_id)}>Unlink</Button>
              </div>)}
              {!hubProjectLinks.length && <p className="hub-muted">This Hub repository is not linked to an IDIOS Project.</p>}
              <form className="hub-inline-create" onSubmit={(event) => { event.preventDefault(); void linkSelectedProject(); }}>
                <Field label="Existing IDIOS Project"><select value={projectToLink} onChange={(event) => setProjectToLink(event.target.value)}>
                  <option value="">Choose a Project…</option>
                  {projects.filter((project) => !hubProjectLinks.some((link) => link.project_id === project.id)).map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
                </select></Field>
                <Field label="Relationship"><select value={projectLinkRelationship} onChange={(event) => setProjectLinkRelationship(event.target.value)}>
                  <option value="implementation">Implementation</option><option value="documentation">Documentation</option><option value="tooling">Tooling</option><option value="shared_component">Shared component</option>
                </select></Field>
                <Button type="submit" disabled={busy || !projectToLink}><Plus size={13}/> Link Project</Button>
              </form>
            </div>
            <Button variant="danger" disabled={busy} onClick={() => void perform(async () => { await api.unlinkHubRepository(selected.id); setSelected(null); await reload(); },'Repository unlinked from IDIOS.')}><Unplug size={13}/> Unlink repository</Button>
          </details>}
        </>}
      </section>
    </div>}
    {repositories.length === 0 && !loading && !standalone && <div className="hub-empty-workspace">
      <GitBranch size={19}/>
      <div><strong>No remote repositories linked</strong><p>Connect a provider, search its repositories, then choose Link. Nothing is added until you explicitly select it.</p></div>
    </div>}
  </div>;
}
