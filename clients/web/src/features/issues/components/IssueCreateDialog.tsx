import { useEffect, useRef, useState, type FormEvent } from 'react';
import { api } from '@/lib/api';
import { Field, LoadingState } from '@/design-system';
import { Modal } from '@/design-system';
import type { AppConfig, ProtocolOption, Repository } from '@/types/idios';

type Props = {
  open: boolean;
  repositories: Repository[];
  initialRepositoryId?: string;
  onClose: () => void;
  onCreated?: () => void | Promise<void>;
};

/** One issue-creation flow shared by the dashboard, issue list, and repository detail. */
export function IssueCreateDialog({ open, repositories, initialRepositoryId, onClose, onCreated }: Props) {
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [parentOptions, setParentOptions] = useState<ProtocolOption[]>([]);
  const [referencesReady, setReferencesReady] = useState(false);
  const [repo, setRepo] = useState('');
  const [branches, setBranches] = useState<Array<{ id: string; name: string; is_default: boolean; repository_id: string }>>([]);
  const [branch, setBranch] = useState('');
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState('normal');
  const [issueType, setIssueType] = useState<'task' | 'bug' | 'feature' | 'research'>('task');
  const [parent, setParent] = useState('');
  const [dueDate, setDueDate] = useState('');
  const [templateKey, setTemplateKey] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [referenceError, setReferenceError] = useState('');
  const initializedForOpen = useRef(false);

  useEffect(() => {
    let active = true;
    void Promise.allSettled([api.config(), api.protocolOptions('issues')]).then(([configResult, optionsResult]) => {
      if (!active) return;
      if (configResult.status === 'fulfilled') setConfig(configResult.value.config);
      else setReferenceError(configResult.reason instanceof Error ? configResult.reason.message : 'Could not load the shared issue-template configuration.');
      if (optionsResult.status === 'fulfilled') setParentOptions(optionsResult.value);
      else setReferenceError((current) => current || (optionsResult.reason instanceof Error ? optionsResult.reason.message : 'Could not load the issue relationship choices.'));
      setReferencesReady(true);
    });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    if (!open) {
      initializedForOpen.current = false;
      return;
    }
    if (!referencesReady || initializedForOpen.current) return;
    initializedForOpen.current = true;
    const repositoryID = repositories.some((item) => item.id === initialRepositoryId)
      ? initialRepositoryId!
      : repositories[0]?.id ?? '';
    const templates = config?.ISSUE_TEMPLATE.templates ?? {};
    const preferred = config?.ISSUE_TEMPLATE.default || Object.keys(templates)[0] || '';
    const definition = templates[preferred];
    setRepo(repositoryID);
    setBranch('');
    setParent('');
    setDueDate('');
    setPriority('normal');
    setIssueType('task');
    setTemplateKey(preferred);
    setTitle(definition?.title_prefix ?? '');
    setDescription(definition?.body ?? '');
    setError('');
  }, [open, referencesReady, config, repositories, initialRepositoryId]);

  useEffect(() => {
    if (!open || !repo) {
      setBranches([]);
      return;
    }
    let active = true;
    void api.repositoryBranches(repo).then((items) => {
      if (active) setBranches(items);
    }).catch((err: unknown) => {
      if (active) {
        setBranches([]);
        setError((current) => current || (err instanceof Error ? `Could not load branches: ${err.message}` : 'Could not load branches for this repository.'));
      }
    });
    return () => { active = false; };
  }, [open, repo]);

  function applyTemplate(key: string) {
    setTemplateKey(key);
    const definition = config?.ISSUE_TEMPLATE.templates[key];
    setTitle(definition?.title_prefix ?? '');
    setDescription(definition?.body ?? '');
  }

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!repo) { setError('Select a repository before creating an issue.'); return; }
    setBusy(true);
    setError('');
    try {
      await api.createIssue({
        repository: repo,
        title: title.trim(),
        description: description.trim(),
        priority,
        issue_type: issueType,
        parent,
        branch,
        due_date: dueDate,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to create the issue.');
      setBusy(false);
      return;
    }

    setBusy(false);
    onClose();
    try {
      await onCreated?.();
    } catch (err) {
      setError(err instanceof Error ? `Issue was created, but the view could not refresh: ${err.message}` : 'Issue was created, but the view could not refresh.');
    }
  }

  const repositoryName = repositories.find((item) => item.id === repo)?.name ?? '';
  const availableParents = parentOptions.filter((item) => item.value !== '' && item.label.startsWith(`${repositoryName}#`));
  const templates = config?.ISSUE_TEMPLATE.templates ?? {};

  return <Modal open={open} title="Create an issue" description="Create work from the same shared flow wherever you are in IDIOS." onClose={onClose} onSubmit={create} submitLabel="Create issue" busy={busy || !referencesReady} wide>
    {!referencesReady ? <LoadingState /> : <>
      {(error || referenceError) && <div className="inline-error" role="alert">{error || referenceError}</div>}
      <div className="issue-create-grid">
        <Field label="Repository"><select value={repo} onChange={(event) => { setRepo(event.target.value); setBranch(''); setParent(''); }} required disabled={busy || !repositories.length}><option value="" disabled>Select repository</option>{repositories.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</select></Field>
        <Field label="Issue template" hint="Templates are shared through your local IDIOS configuration."><select value={templateKey} onChange={(event) => applyTemplate(event.target.value)} disabled={busy}><option value="">No template</option>{Object.entries(templates).sort(([a], [b]) => a.localeCompare(b)).map(([key, definition]) => <option value={key} key={key}>{definition.name || key}</option>)}</select></Field>
      </div>
      <Field label="Branch" hint="Optional workflow branch in the selected repository; it does not create a Git branch on disk."><select value={branch} onChange={(event) => setBranch(event.target.value)} disabled={busy || !repo}><option value="">No branch</option>{branches.filter((item) => item.repository_id === repo).map((item) => <option value={item.id} key={item.id}>{item.name}{item.is_default ? ' · default' : ''}</option>)}</select></Field>
      <Field label="Title"><input autoFocus value={title} onChange={(event) => setTitle(event.target.value)} required maxLength={200} disabled={busy} placeholder="What needs to be done?" /></Field>
      <Field label="Description / issue brief"><textarea rows={5} value={description} onChange={(event) => setDescription(event.target.value)} maxLength={30000} disabled={busy} placeholder="Context, acceptance criteria, or notes…" /></Field>
      <div className="issue-create-grid issue-create-grid--three issue-create-grid--types">
        <Field label="Issue type" hint="Used for repository completion statistics."><select value={issueType} onChange={(event) => setIssueType(event.target.value as typeof issueType)} disabled={busy}><option value="task">Task</option><option value="bug">Bug</option><option value="feature">Feature</option><option value="research">Research</option></select></Field>
        <Field label="Priority"><select value={priority} onChange={(event) => setPriority(event.target.value)} disabled={busy}><option value="low">Low</option><option value="normal">Normal</option><option value="high">High</option><option value="urgent">Urgent</option></select></Field>
        <Field label="Parent issue" hint="Optional; the parent must belong to the same repository and remain open."><select value={parent} onChange={(event) => setParent(event.target.value)} disabled={busy || !repo}><option value="">No parent</option>{availableParents.map((item) => <option value={item.value} key={item.value}>{item.label}</option>)}</select></Field>
        <Field label="Due date"><input type="date" value={dueDate} onChange={(event) => setDueDate(event.target.value)} disabled={busy} /></Field>
      </div>
    </>}
  </Modal>;
}
