import { useEffect, useMemo, useState, type ReactNode } from 'react';
import {
  ChevronDown, ChevronRight, FileCode2, FileText, Folder, FolderOpen, Search,
} from 'lucide-react';
import { api } from '@/lib/api';
import type { HubFile, HubRepository, HubTreeEntry, HubTreePage } from '@/types/idios';
import './RepositoryBrowser.css';

type TreeNode = {
  path: string;
  name: string;
  type: 'tree' | 'blob';
  size: number;
  sha?: string;
  children: TreeNode[];
  loaded: boolean;
  loading: boolean;
  page: number;
  hasMore: boolean;
  error: string;
  errorPage: number;
};

function buildTree(entries: HubTreeEntry[]): TreeNode[] {
  const roots: TreeNode[] = [];
  const directories = new Map<string, TreeNode>();
  const ordered = [...entries].filter((entry) => entry.path.trim()).sort((a, b) =>
    a.path.split('/').length - b.path.split('/').length || a.path.localeCompare(b.path));

  for (const entry of ordered) {
    const parts = entry.path.replace(/^\/+|\/+$/g, '').split('/').filter(Boolean);
    if (!parts.length) continue;
    let siblings = roots;
    let parentPath = '';
    for (let index = 0; index < parts.length; index += 1) {
      const name = parts[index];
      const path = parentPath ? `${parentPath}/${name}` : name;
      const isLast = index === parts.length - 1;
      const isFile = isLast && entry.type !== 'tree';
      let node = isFile
        ? siblings.find((candidate) => candidate.path === path && candidate.type === 'blob')
        : directories.get(path);

      if (!node) {
        node = {
          path,
          name,
          type: isFile ? 'blob' : 'tree',
          size: isFile ? entry.size : 0,
          sha: isLast ? entry.sha : undefined,
          children: [],
          loaded: false,
          loading: false,
          page: 0,
          hasMore: false,
          error: '',
          errorPage: 0,
        };
        siblings.push(node);
        if (!isFile) directories.set(path, node);
      } else if (isFile) {
        node.size = entry.size;
      } else if (isLast && entry.sha) {
        node.sha = entry.sha;
      }
      if (!isFile) siblings = node.children;
      parentPath = path;
    }
  }

  const sortNodes = (nodes: TreeNode[]) => {
    nodes.sort((a, b) => {
      if (a.type !== b.type) return a.type === 'tree' ? -1 : 1;
      return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    });
    nodes.forEach((node) => sortNodes(node.children));
  };
  sortNodes(roots);
  return roots;
}

function findTreeNode(nodes: TreeNode[], path: string): TreeNode | undefined {
  for (const node of nodes) {
    if (node.path === path) return node;
    const child = findTreeNode(node.children, path);
    if (child) return child;
  }
  return undefined;
}

function updateTreeNode(nodes: TreeNode[], path: string, update: (node: TreeNode) => TreeNode): TreeNode[] {
  return nodes.map((node) => node.path === path
    ? update(node)
    : { ...node, children: updateTreeNode(node.children, path, update) });
}

function mergeTreeNodes(current: TreeNode[], incoming: TreeNode[]): TreeNode[] {
  const merged = new Map(current.map((node) => [node.path, node]));
  for (const node of incoming) {
    const existing = merged.get(node.path);
    merged.set(node.path, existing ? {
      ...node,
      children: existing.children,
      loaded: existing.loaded,
      loading: existing.loading,
      page: existing.page,
      hasMore: existing.hasMore,
      error: existing.error,
      errorPage: existing.errorPage,
    } : node);
  }
  return [...merged.values()].sort((a, b) => {
    if (a.type !== b.type) return a.type === 'tree' ? -1 : 1;
    return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
  });
}

function filterTree(nodes: TreeNode[], query: string): TreeNode[] {
  if (!query) return nodes;
  return nodes.flatMap((node) => {
    if (node.path.toLowerCase().includes(query)) return [node];
    const children = filterTree(node.children, query);
    return children.length ? [{ ...node, children }] : [];
  });
}

function formatSize(bytes: number): string {
  if (!bytes) return '';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function RemoteTreeView({
  tree,
  selectedPath,
  onOpenFile,
  emptyMessage,
  loadChildren,
  rootHasMore,
  rootLoading,
  onLoadMoreRoot,
}: {
  tree: HubTreeEntry[];
  selectedPath: string;
  onOpenFile: (path: string) => void;
  emptyMessage: string;
  loadChildren: (entry: HubTreeEntry, page: number) => Promise<HubTreePage>;
  rootHasMore: boolean;
  rootLoading: boolean;
  onLoadMoreRoot: () => void;
}) {
  const [query, setQuery] = useState('');
  const [nodes, setNodes] = useState<TreeNode[]>(() => buildTree(tree));
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const normalizedQuery = query.trim().toLowerCase();
  const visibleNodes = useMemo(() => filterTree(nodes, normalizedQuery), [nodes, normalizedQuery]);

  useEffect(() => {
    setNodes((current) => mergeTreeNodes(current, buildTree(tree)));
  }, [tree]);

  async function loadDirectoryPage(node: TreeNode, page: number) {
    setNodes((current) => updateTreeNode(current, node.path, (item) => ({
      ...item, loading: true, error: '', errorPage: page,
    })));
    try {
      const result = await loadChildren({
        path: node.path, name: node.name, type: 'tree', size: node.size, sha: node.sha,
      }, page);
      const responseTree = buildTree(result.items);
      const parent = findTreeNode(responseTree, node.path);
      const incoming = parent?.children ?? [];
      setNodes((current) => updateTreeNode(current, node.path, (item) => ({
        ...item,
        children: page === 1 ? incoming : mergeTreeNodes(item.children, incoming),
        loaded: true,
        loading: false,
        page: result.page,
        hasMore: result.has_more,
        error: '',
        errorPage: 0,
      })));
    } catch (cause) {
      setNodes((current) => updateTreeNode(current, node.path, (item) => ({
        ...item,
        loading: false,
        error: cause instanceof Error ? cause.message : 'Could not load this directory.',
        errorPage: page,
      })));
    }
  }

  function toggleDirectory(node: TreeNode) {
    const isOpen = expanded.has(node.path);
    setExpanded((current) => {
      const next = new Set(current);
      if (isOpen) next.delete(node.path);
      else next.add(node.path);
      return next;
    });
    if (!isOpen && !node.loaded && !node.loading) void loadDirectoryPage(node, 1);
  }

  function expandTo(path: string) {
    const parts = path.split('/');
    setExpanded((current) => {
      const next = new Set(current);
      let currentPath = '';
      for (const part of parts) {
        currentPath = currentPath ? `${currentPath}/${part}` : part;
        next.add(currentPath);
      }
      return next;
    });
  }

  function renderNodes(items: TreeNode[], depth = 0): ReactNode {
    return items.map((node) => {
      const isDirectory = node.type === 'tree';
      const isOpen = normalizedQuery ? true : expanded.has(node.path);
      const selected = selectedPath === node.path;
      return (
        <div className="hub-tree-node-group" key={node.path}>
          <button
            type="button"
            role="treeitem"
            className={`hub-tree-node ${selected ? 'is-selected' : ''} ${isDirectory ? 'is-directory' : 'is-file'}`}
            style={{ paddingInlineStart: `${10 + depth * 15}px` }}
            onClick={() => isDirectory ? toggleDirectory(node) : onOpenFile(node.path)}
            aria-expanded={isDirectory ? isOpen : undefined}
            aria-current={selected ? 'page' : undefined}
            aria-busy={isDirectory ? node.loading : undefined}
            title={node.path}
          >
            {isDirectory
              ? (isOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />)
              : <span className="hub-tree-node__spacer" />}
            {isDirectory
              ? (isOpen ? <FolderOpen size={14} /> : <Folder size={14} />)
              : (node.name.toLowerCase().endsWith('.md') || node.name.toLowerCase().endsWith('.txt')
                ? <FileText size={14} />
                : <FileCode2 size={14} />)}
            <span className="hub-tree-node__name">{node.name}</span>
            {!isDirectory && node.size > 0 && <small>{formatSize(node.size)}</small>}
          </button>
          {isDirectory && isOpen && (
            <div role="group" className="hub-tree-node-children">
              {node.loading && <p className="hub-tree-node-state">Loading directory…</p>}
              {node.error && (
                <div className="hub-tree-node-error">
                  <span>{node.error}</span>
                  <button type="button" onClick={() => void loadDirectoryPage(node, node.errorPage || 1)}>Retry</button>
                </div>
              )}
              {node.children.length > 0 && renderNodes(node.children, depth + 1)}
              {node.loaded && !node.children.length && !node.error && <p className="hub-tree-node-state">This folder is empty.</p>}
              {node.hasMore && !node.loading && (
                <button
                  type="button"
                  className="hub-tree-load-more"
                  onClick={() => void loadDirectoryPage(node, node.page + 1)}
                  style={{ paddingInlineStart: `${25 + depth * 15}px` }}
                >
                  Load more entries
                </button>
              )}
            </div>
          )}
        </div>
      );
    });
  }

  const crumbs = selectedPath.split('/').filter(Boolean);
  return (
    <div className="hub-tree-browser">
      <label className="hub-tree-search">
        <Search size={14} />
        <input
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search loaded paths"
          aria-label="Search loaded repository paths"
        />
      </label>
      <p className="hub-tree-search-hint">Search covers loaded paths. Expand folders to fetch their contents.</p>
      {selectedPath && (
        <nav className="hub-tree-breadcrumbs" aria-label="Selected file path">
          <button type="button" onClick={() => setExpanded(new Set())}>Repository</button>
          {crumbs.map((part, index) => {
            const path = crumbs.slice(0, index + 1).join('/');
            const isLast = index === crumbs.length - 1;
            return (
              <span className="hub-tree-breadcrumb" key={path}>
                <span aria-hidden="true">/</span>
                {isLast ? <strong>{part}</strong> : <button type="button" onClick={() => expandTo(path)}>{part}</button>}
              </span>
            );
          })}
        </nav>
      )}
      <div className="hub-tree-list" role="tree" aria-label="Remote repository files">
        {rootLoading && !nodes.length && <p className="hub-tree-empty">Loading repository tree…</p>}
        {renderNodes(visibleNodes)}
        {!rootLoading && !visibleNodes.length && (
          <p className="hub-tree-empty">{normalizedQuery ? 'No loaded paths match this search.' : emptyMessage}</p>
        )}
      </div>
      {rootHasMore && (
        <button type="button" className="hub-tree-load-more hub-tree-load-more--root" onClick={onLoadMoreRoot} disabled={rootLoading}>
          {rootLoading ? 'Loading repository entries…' : 'Load more repository entries'}
        </button>
      )}
    </div>
  );
}

type InlineToken = { value: string; kind: 'text' | 'bold' | 'italic' | 'code' | 'link' | 'image'; target?: string };

function safeReference(reference: string, base: string): string {
  try {
    const resolved = new URL(reference, base);
    return resolved.protocol === 'https:' || resolved.protocol === 'http:' ? resolved.href : '';
  } catch {
    return '';
  }
}

function parseInline(value: string): InlineToken[] {
  const result: InlineToken[] = [];
  const pattern = /(!\[[^\]]*\]\([^)]+\)|\[[^\]]+\]\([^)]+\)|\*\*[^*]+\*\*|__[^_]+__|\*[^*]+\*|_[^_]+_|\x60[^\x60]+\x60)/g;
  let offset = 0;
  for (const match of value.matchAll(pattern)) {
    const index = match.index ?? 0;
    if (index > offset) result.push({ value: value.slice(offset, index), kind: 'text' });
    const token = match[0];
    const image = /^!\[([^\]]*)\]\(([^)]+)\)$/.exec(token);
    const link = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(token);
    if (image) result.push({ value: image[1], target: image[2], kind: 'image' });
    else if (link) result.push({ value: link[1], target: link[2], kind: 'link' });
    else if (token.startsWith('**') || token.startsWith('__')) result.push({ value: token.slice(2, -2), kind: 'bold' });
    else if (token.startsWith('`')) result.push({ value: token.slice(1, -1), kind: 'code' });
    else result.push({ value: token.slice(1, -1), kind: 'italic' });
    offset = index + token.length;
  }
  if (offset < value.length) result.push({ value: value.slice(offset), kind: 'text' });
  return result;
}

function InlineMarkdown({ value, linkBase, imageBase }: { value: string; linkBase: string; imageBase: string }) {
  return <>{parseInline(value).map((part, index) => {
    if (part.kind === 'bold') return <strong key={index}>{part.value}</strong>;
    if (part.kind === 'italic') return <em key={index}>{part.value}</em>;
    if (part.kind === 'code') return <code key={index}>{part.value}</code>;
    if (part.kind === 'link') {
      const href = safeReference(part.target ?? '', linkBase);
      return href ? <a href={href} key={index} target="_blank" rel="noreferrer">{part.value}</a> : <span key={index}>{part.value}</span>;
    }
    if (part.kind === 'image') {
      const src = safeReference(part.target ?? '', imageBase);
      return src
        ? <img key={index} src={src} alt={part.value} loading="lazy" referrerPolicy="no-referrer" />
        : <span key={index}>{part.value}</span>;
    }
    return <span key={index}>{part.value}</span>;
  })}</>;
}

function tableCells(line: string): string[] {
  return line.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map((cell) => cell.trim());
}

function isTableDivider(line: string): boolean {
  const cells = tableCells(line);
  return cells.length > 0 && cells.every((cell) => /^:?-{3,}:?$/.test(cell));
}

function startsBlock(lines: string[], index: number): boolean {
  const line = lines[index] ?? '';
  return !line.trim() || /^#{1,6}\s/.test(line) || /^\x60{3}/.test(line.trim()) ||
    /^>\s?/.test(line) || /^\s*[-*+]\s+/.test(line) || /^\s*\d+\.\s+/.test(line) ||
    /^\s*(---+|\*\*\*|___)\s*$/.test(line);
}

function renderBlocks(markdown: string, linkBase: string, imageBase: string): ReactNode[] {
  const lines = markdown.replace(/\r\n?/g, '\n').split('\n');
  const result: ReactNode[] = [];
  let index = 0;
  const inline = (value: string) => <InlineMarkdown value={value} linkBase={linkBase} imageBase={imageBase} />;

  while (index < lines.length) {
    const line = lines[index];
    if (!line.trim()) { index += 1; continue; }
    if (/^\x60{3}/.test(line.trim())) {
      const code: string[] = [];
      index += 1;
      while (index < lines.length && !/^\x60{3}/.test(lines[index].trim())) { code.push(lines[index]); index += 1; }
      if (index < lines.length) index += 1;
      result.push(<pre key={`code-${index}`}><code>{code.join('\n')}</code></pre>);
      continue;
    }
    if (index + 1 < lines.length && line.includes('|') && isTableDivider(lines[index + 1])) {
      const headers = tableCells(line);
      index += 2;
      const rows: string[][] = [];
      while (index < lines.length && lines[index].includes('|') && lines[index].trim()) { rows.push(tableCells(lines[index])); index += 1; }
      result.push(
        <div className="hub-readme-table-wrap" key={`table-${index}`}>
          <table className="hub-readme-table">
            <thead><tr>{headers.map((cell, cellIndex) => <th key={cellIndex}>{inline(cell)}</th>)}</tr></thead>
            <tbody>{rows.map((row, rowIndex) => (
              <tr key={rowIndex}>{headers.map((_, cellIndex) => <td key={cellIndex}>{inline(row[cellIndex] ?? '')}</td>)}</tr>
            ))}</tbody>
          </table>
        </div>,
      );
      continue;
    }
    const heading = /^(#{1,6})\s+(.*)$/.exec(line.trim());
    if (heading) {
      const level = heading[1].length;
      const content = inline(heading[2]);
      if (level === 1) result.push(<h1 key={index}>{content}</h1>);
      else if (level === 2) result.push(<h2 key={index}>{content}</h2>);
      else if (level === 3) result.push(<h3 key={index}>{content}</h3>);
      else if (level === 4) result.push(<h4 key={index}>{content}</h4>);
      else if (level === 5) result.push(<h5 key={index}>{content}</h5>);
      else result.push(<h6 key={index}>{content}</h6>);
      index += 1;
      continue;
    }
    if (/^\s*(---+|\*\*\*|___)\s*$/.test(line)) { result.push(<hr key={index} />); index += 1; continue; }
    if (/^>\s?/.test(line)) {
      const quote: string[] = [];
      while (index < lines.length && /^>\s?/.test(lines[index])) { quote.push(lines[index].replace(/^>\s?/, '')); index += 1; }
      result.push(<blockquote key={`quote-${index}`}>{quote.map((part, partIndex) => <p key={partIndex}>{inline(part)}</p>)}</blockquote>);
      continue;
    }
    if (/^\s*[-*+]\s+/.test(line)) {
      const items: string[] = [];
      while (index < lines.length && /^\s*[-*+]\s+/.test(lines[index])) { items.push(lines[index].replace(/^\s*[-*+]\s+/, '')); index += 1; }
      result.push(<ul key={`ul-${index}`}>{items.map((item, itemIndex) => <li key={itemIndex}>{inline(item)}</li>)}</ul>);
      continue;
    }
    if (/^\s*\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (index < lines.length && /^\s*\d+\.\s+/.test(lines[index])) { items.push(lines[index].replace(/^\s*\d+\.\s+/, '')); index += 1; }
      result.push(<ol key={`ol-${index}`}>{items.map((item, itemIndex) => <li key={itemIndex}>{inline(item)}</li>)}</ol>);
      continue;
    }
    const paragraph = [line.trim()];
    index += 1;
    while (index < lines.length && !startsBlock(lines, index)) { paragraph.push(lines[index].trim()); index += 1; }
    result.push(<p key={`p-${index}`}>{inline(paragraph.join(' '))}</p>);
  }
  return result;
}

function readmeCandidates(tree: HubTreeEntry[]): HubTreeEntry[] {
  const rootFiles = tree.filter((entry) => entry.type === 'blob' && !entry.path.includes('/'));
  const byName = (name: string) => rootFiles.find((entry) => entry.name.toLowerCase() === name.toLowerCase());
  const preferred = ['README.md', 'README', 'README.txt'].map(byName).filter((entry): entry is HubTreeEntry => Boolean(entry));
  const known = new Set(preferred.map((entry) => entry.path));
  const alternatives = rootFiles
    .filter((entry) => /^readme(?:[._-].*)?$/i.test(entry.name) && !known.has(entry.path))
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }));
  return [...preferred, ...alternatives];
}

function contentBases(repository: HubRepository): { linkBase: string; imageBase: string } {
  const root = repository.web_url.replace(/\/+$/, '');
  const branch = encodeURIComponent(repository.default_branch || 'main');
  if (repository.provider === 'github') {
    const linkBase = `${root}/blob/${branch}/`;
    const imageBase = repository.host.replace(/\/+$/, '') === 'https://github.com'
      ? `https://raw.githubusercontent.com/${repository.full_path}/${branch}/`
      : linkBase;
    return { linkBase, imageBase };
  }
  if (repository.provider === 'gitlab') {
    const linkBase = `${root}/-/blob/${branch}/`;
    return { linkBase, imageBase: `${root}/-/raw/${branch}/` };
  }
  return { linkBase: `${root}/`, imageBase: `${root}/` };
}

export function RemoteReadmePreview({ repository }: { repository: HubRepository }) {
  const [readme, setReadme] = useState<HubFile | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [plainText, setPlainText] = useState(false);

  useEffect(() => {
    let active = true;
    setReadme(null);
    setError('');
    if (!repository.capabilities.includes('code.read')) {
      setLoading(false);
      setError('README preview is not available through this provider connection.');
      return () => { active = false; };
    }
    setLoading(true);
    const branch = repository.default_branch || 'main';
    void (async () => {
      try {
        let page = await api.hubTree(repository.id, branch);
        const rootEntries = [...page.items];
        while (page.has_more && page.page < 20) {
          page = await api.hubTree(repository.id, branch, '', '', page.page + 1);
          rootEntries.push(...page.items);
        }
        const candidates = readmeCandidates(rootEntries);
        if (!candidates.length) {
          if (active) setError(page.has_more
            ? 'No README was found in the first 2,000 root entries. Open the Code view to continue browsing.'
            : 'No root-level README file was found on the default branch.');
          return;
        }
        let lastError = '';
        for (const candidate of candidates) {
          try {
            const file = await api.hubFile(repository.id, candidate.path, branch);
            if (!active) return;
            const byteLength = new TextEncoder().encode(file.content).length;
            if (byteLength > 512 * 1024) {
              setError('The README is larger than the 512 KB preview limit. Open it in Code or use the provider page.');
              return;
            }
            setReadme(file);
            setPlainText(/\.txt$/i.test(file.path) || !/\.md$|\.markdown$|^README$/i.test(file.path.split('/').pop() ?? ''));
            return;
          } catch (requestError) {
            lastError = requestError instanceof Error ? requestError.message : 'The README could not be loaded.';
          }
        }
        if (active) setError(lastError || 'A README candidate exists, but the provider did not return its contents.');
      } catch (requestError) {
        if (active) setError(requestError instanceof Error ? requestError.message : 'The repository README could not be loaded.');
      } finally {
        if (active) setLoading(false);
      }
    })();
    return () => { active = false; };
  }, [repository.id, repository.default_branch, repository.capabilities]);

  const bases = useMemo(() => contentBases(repository), [repository]);
  const rendered = useMemo(() => readme && !plainText
    ? renderBlocks(readme.content, bases.linkBase, bases.imageBase)
    : [], [readme, plainText, bases]);

  return (
    <section className="hub-readme-preview">
      <header className="hub-readme-preview__header">
        <div><span className="eyebrow">DEFAULT BRANCH</span><h3>README</h3></div>
        {readme && <small>{readme.path} · {formatSize(readme.size)}</small>}
      </header>
      {loading && <p className="hub-readme-preview__state">Loading README…</p>}
      {!loading && error && <p className="hub-readme-preview__state">{error}</p>}
      {!loading && readme && (
        <div className={plainText ? 'hub-readme-content is-plain-text' : 'hub-readme-content'}>
          {plainText ? <pre>{readme.content}</pre> : rendered.length ? rendered : <p className="hub-readme-preview__state">This README is empty.</p>}
        </div>
      )}
    </section>
  );
}
