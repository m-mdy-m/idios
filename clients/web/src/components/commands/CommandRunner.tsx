import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { AlertTriangle, CheckCircle2, LoaderCircle, RotateCcw, XCircle } from 'lucide-react';
import { api } from '@/lib/api';
import type { ProtocolCommand, ProtocolOption } from '@/types/idios';
import { Button, Field } from '@/components/ui/primitives';
import { Modal } from '@/components/ui/Modal';

export function CommandRunner({ command, open, initialValues, onClose, onSuccess }: {
  command: ProtocolCommand | null;
  open: boolean;
  initialValues?: Record<string, unknown>;
  onClose: () => void;
  onSuccess?: (result: unknown) => void | Promise<void>;
}) {
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [busy, setBusy] = useState(false);
  const submittingRef = useRef(false);
  const [error, setError] = useState('');
  const [dynamicOptions, setDynamicOptions] = useState<Record<string, ProtocolOption[]>>({});
  const [loadingOptions, setLoadingOptions] = useState(false);
  const [bulkPreview, setBulkPreview] = useState(false);
  const [bulkResult, setBulkResult] = useState<{ created: number; total: number; failed: Array<{ item: string; error: string }> } | null>(null);
  const destructive = Boolean(command?.id.endsWith('.delete'));

  useEffect(() => {
    if (!open || !command) return;
    const defaults: Record<string, unknown> = {};
    for (const field of command.fields ?? []) defaults[field.name] = field.default ?? (field.type === 'boolean' ? false : field.type === 'color' ? '#7c9cff' : '');
    setValues({ ...defaults, ...(initialValues ?? {}) });
    setDynamicOptions({});
    setError(''); setBulkPreview(false); setBulkResult(null);
  }, [open, command, initialValues]);

  useEffect(() => {
    if (!open || !command) return;
    const sourced = (command.fields ?? []).filter((field) => field.source);
    if (!sourced.length) {
      setDynamicOptions({});
      setLoadingOptions(false);
      return;
    }
    let active = true;
    setDynamicOptions({});
    setLoadingOptions(true);
    void Promise.all(sourced.map(async (field) => {
      const source = field.source!;
      const dependentType = source === 'entities' ? String(values.entity_type ?? 'issue')
        : source === 'source_entities' ? String(values.source_type ?? 'concept')
          : source === 'target_entities' ? String(values.target_type ?? 'issue')
            : source === 'owners' ? String(values.scope ?? 'repository') : '';

      // Branch choices must belong to the active repository. Otherwise a form
      // can offer a perfectly valid branch from a different repository and only
      // fail after submit. Resolve the repository from the form context first.
      if (source === 'branches') {
        let repositoryID = '';
        let mustBeScoped = false;
        if (command.id === 'issue.create') {
          repositoryID = String(values.repo ?? '');
          mustBeScoped = true;
        } else if (command.id === 'branch.create') {
          repositoryID = String(values.repository ?? '');
          mustBeScoped = true;
        } else if (command.id === 'issue.branch.assign' || command.id === 'pull_request.create') {
          const issueID = String(values.issue ?? '');
          mustBeScoped = true;
          if (issueID) {
            const issue = await api.issue(issueID);
            repositoryID = issue.repository_id;
          }
        }
        if (mustBeScoped) {
          if (!repositoryID) return [field.name, []] as const;
          const branches = await api.repositoryBranches(repositoryID);
          return [field.name, branches.map((branch) => ({
            label: `${branch.name}${branch.is_default ? ' · default' : ''}`,
            value: branch.id,
          }))] as const;
        }
      }

      const options = await api.protocolOptions(source, dependentType);
      // Parent issues are only valid inside the selected repository. Keep the
      // picker consistent with the Engine rule instead of exposing invalid choices.
      if (command.id === 'issue.create' && field.name === 'parent' && source === 'issues') {
        const repositoryID = String(values.repo ?? '');
        if (!repositoryID) return [field.name, []] as const;
        const repositories = await api.repositories();
        const repositoryName = repositories.find((item) => item.id === repositoryID)?.name;
        return [field.name, repositoryName ? options.filter((option) => option.label.startsWith(`${repositoryName}#`)) : []] as const;
      }
      if (command.id === 'issue.parent.assign' && field.name === 'parent' && source === 'issues') {
        const issueID = String(values.issue ?? '');
        if (!issueID) return [field.name, []] as const;
        const issue = await api.issue(issueID);
        const repositories = await api.repositories();
        const repositoryName = repositories.find((item) => item.id === issue.repository_id)?.name;
        return [field.name, repositoryName ? options.filter((option) => option.value !== issue.id && option.label.startsWith(`${repositoryName}#`)) : []] as const;
      }
      return [field.name, options] as const;
    })).then((pairs) => {
      if (active) setDynamicOptions(Object.fromEntries(pairs));
    }).catch((err: unknown) => {
      if (active) {
        setDynamicOptions({});
        setError(err instanceof Error ? err.message : 'Unable to load choices from the local Engine.');
      }
    }).finally(() => { if (active) setLoadingOptions(false); });
    return () => { active = false; };
  }, [open, command, values.entity_type, values.source_type, values.target_type, values.scope, values.repo, values.repository, values.issue]);

  function updateField(name: string, value: unknown) {
    setValues((current) => {
      const next = { ...current, [name]: value };
      if (name === 'entity_type') next.entity_ref = '';
      if (name === 'source_type') next.source = '';
      if (name === 'target_type') next.target = '';
      if (name === 'scope') {
        next.owner = '';
        if (value !== 'repository') next.repository = '';
      }
      return next;
    });
  }

  const title = command?.title ?? 'Run command';
  const bulkFieldName = command?.id === 'issue.bulk_create' ? 'titles' : command?.id === 'quote.bulk_create' ? 'quotes' : command?.id === 'job.bulk_create' ? 'names' : '';
  const bulkLines = bulkFieldName ? String(values[bulkFieldName] ?? '').split(/\r?\n/).map((line) => line.trim()).filter(Boolean) : [];
  const isBulk = Boolean(bulkFieldName);
  const isJobSchedule = Boolean(command?.id.startsWith('job.') && (command.id === 'job.create' || command.id === 'job.update' || command.id === 'job.bulk_create'));
  const fields = useMemo(() => command?.fields ?? [], [command]);
  if (!command) return null;

  const fixedRepositoryTarget = command?.id === 'repository.delete' ? String(initialValues?.__confirm_target ?? '') : '';
  const repositoryConfirmationTarget = command?.id === 'repository.delete'
    ? fixedRepositoryTarget || (dynamicOptions.ref ?? []).find((option) => option.value === String(values.ref ?? ''))?.label || String(values.ref ?? '')
    : '';

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!command || submittingRef.current) return;
    // React state updates are asynchronous; a ref blocks two submit events in
    // the same tick so destructive protocol commands are never sent twice.
    submittingRef.current = true;
    setBusy(true); setError('');
    try {
      const input: Record<string, unknown> = {};
      for (const field of fields) {
        const value = values[field.name];
        const required = field.required || (field.name === 'schedule_at' && String(values.schedule_type ?? '') === 'once') || (command.id === 'label.create' && field.name === 'repository' && String(values.scope ?? 'global') === 'repository');
        if (required && (value === undefined || value === null || String(value).trim() === '')) throw new Error(`${field.label} is required.`);
        if (value !== undefined && value !== '') input[field.name] = field.type === 'number' ? Number(value) : value;
      }
      if (isBulk && !bulkPreview) {
        if (bulkLines.length === 0) throw new Error('Enter at least one non-empty line.');
        if (bulkLines.length > 100) throw new Error('A batch can contain at most 100 items.');
        setBulkPreview(true);
        return;
      }
      if (bulkResult) {
        onClose();
        return;
      }
      const response = await api.executeCommand(command.id, input);
      await onSuccess?.(response.result);
      if (isBulk && response.result && typeof response.result === 'object' && 'created' in response.result) {
        const result = response.result as { created?: number; total?: number; failed?: Array<{ item?: string; error?: string }> };
        setBulkResult({
          created: Number(result.created ?? 0),
          total: Number(result.total ?? bulkLines.length),
          failed: Array.isArray(result.failed) ? result.failed.map((item) => ({ item: String(item.item ?? 'Unknown item'), error: String(item.error ?? 'Unknown error') })) : [],
        });
      } else {
        onClose();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : `Unable to run ${command.title.toLowerCase()}.`);
    } finally { submittingRef.current = false; setBusy(false); }
  }

  function retryFailedItems() {
    if (!bulkResult || !bulkFieldName) return;
    const failedItems = bulkResult.failed.map((item) => item.item).filter(Boolean);
    if (!failedItems.length) return;
    setValues((current) => ({ ...current, [bulkFieldName]: failedItems.join('\n') }));
    setBulkResult(null);
    setBulkPreview(false);
    setError('');
  }

  const primaryLabel = busy ? (bulkResult ? 'Finishing…' : 'Running…')
    : bulkResult ? 'Done'
      : isBulk && !bulkPreview ? `Review ${bulkLines.length || 0} items`
        : isBulk ? `Create ${bulkLines.length} items`
          : command.id.endsWith('.delete') ? `Delete ${command.id === 'pull_request.delete' ? 'pull request' : command.id.split('.')[0]}` : 'Run action';

  return <Modal open={open} title={bulkResult ? 'Batch results' : title} description={bulkResult ? 'The Engine reports each successful and failed item. Retrying will submit only failed items.' : command.description} onClose={onClose} onSubmit={submit} submitLabel={primaryLabel} submitVariant={destructive ? 'danger' : 'primary'} busy={busy} wide fullscreen={isJobSchedule} jobLayout={isJobSchedule}>
    {destructive && <div className="command-danger-note"><AlertTriangle size={17} /><div><strong>{command.id === 'repository.delete' ? `Delete ${repositoryConfirmationTarget || 'this repository'}?` : `Delete ${title.replace(/^delete\s+/i, '').toLowerCase()}?`}</strong><p>{command.id === 'repository.delete' ? 'This permanently removes the repository and its owned issues, pull requests, branches, labels, milestones, completions and settings.' : 'This action removes the selected record. IDIOS will enforce its relationship rules before deleting it.'}</p></div></div>}
    {command.id === 'repository.delete' && fixedRepositoryTarget && <div className="command-delete-target"><span>Repository</span><strong>{fixedRepositoryTarget}</strong><small>The name is shown for clarity; the stable repository ID is used for the deletion.</small></div>}
    {error && <div className="inline-error" role="alert">{error}</div>}
    {bulkResult ? <section className="command-batch-report" aria-live="polite">
      <div className="command-batch-report__summary"><CheckCircle2 size={19} /><strong>{bulkResult.created} of {bulkResult.total} created successfully</strong><span>{bulkResult.failed.length ? `${bulkResult.failed.length} failed` : 'All items completed'}</span></div>
      {bulkResult.failed.length > 0 && <>
        <div className="command-batch-report__failures">{bulkResult.failed.map((item, index) => <div className="command-batch-report__failure" key={`${item.item}-${index}`}><XCircle size={16} /><div><strong>{item.item}</strong><p>{item.error}</p></div></div>)}</div>
        <Button type="button" variant="secondary" onClick={retryFailedItems}><RotateCcw size={15} /> Retry failed items only</Button>
      </>}
    </section> : bulkPreview ? <section className="command-batch-preview">
      <div className="command-batch-preview__header"><span className="command-batch-preview__count">{bulkLines.length}</span><div><strong>Review before creating</strong><p>Only non-empty lines are included. The Engine handles each item separately and reports failures without hiding successes.</p></div></div>
      <ol>{bulkLines.slice(0, 8).map((line, index) => <li key={`${index}-${line}`}>{line}</li>)}</ol>
      {bulkLines.length > 8 && <p className="muted">And {bulkLines.length - 8} more items…</p>}
      <Button type="button" variant="ghost" onClick={() => setBulkPreview(false)}>Back to edit</Button>
    </section> : <>
    {fields.filter((field) => {
      const schedule = String(values.schedule_type ?? '');
      if (field.name === 'schedule_at') return schedule === 'once';
      if (field.name === 'interval_minutes') return schedule === 'interval';
      if (field.name === 'schedule_time') return ['daily', 'weekly', 'monthly'].includes(schedule);
      if (field.name === 'schedule_weekday') return schedule === 'weekly';
      if (field.name === 'schedule_day') return schedule === 'monthly';
      if (command.id === 'label.create' && field.name === 'repository') return String(values.scope ?? 'global') === 'repository';
      return true;
    }).filter((field) => !(command.id === 'repository.delete' && fixedRepositoryTarget && field.name === 'ref')).map((field) => {
      const required = field.required || (field.name === 'schedule_at' && String(values.schedule_type ?? '') === 'once') || (command.id === 'label.create' && field.name === 'repository' && String(values.scope ?? 'global') === 'repository');
      const options = field.source ? dynamicOptions[field.name] ?? [] : (field.options ?? []).map((option) => ({ label: option.replaceAll('_', ' '), value: option }));
      const isChoice = field.type === 'select' || Boolean(field.source);
      return <Field key={field.name} label={`${field.label}${required ? ' *' : ''}`} hint={field.help}>
        {field.type === 'textarea' ? <textarea rows={4} autoFocus={field === fields[0]} required={required} value={String(values[field.name] ?? '')} placeholder={field.placeholder} onChange={(event) => updateField(field.name, event.target.value)} />
          : isChoice ? <select autoFocus={field === fields[0]} required={required} value={String(values[field.name] ?? '')} disabled={Boolean(field.source && loadingOptions && !dynamicOptions[field.name])} onChange={(event) => updateField(field.name, event.target.value)}><option value="">{field.source ? (loadingOptions ? 'Loading choices…' : options.length ? 'Choose an item…' : 'No matching items') : 'Choose…'}</option>{options.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select>
            : field.type === 'boolean' ? <label className="command-checkbox"><input type="checkbox" checked={Boolean(values[field.name])} onChange={(event) => updateField(field.name, event.target.checked)} /><span>{field.help || field.label}</span></label>
              : <input autoFocus={field === fields[0]} required={required} type={field.type === 'number' ? 'number' : field.type === 'date' ? 'date' : field.type === 'datetime' ? 'datetime-local' : field.type === 'color' || (command.id === 'label.create' && field.name === 'color') ? 'color' : 'text'} value={String(values[field.name] ?? '')} placeholder={field.placeholder} onChange={(event) => updateField(field.name, event.target.value)} />}
      </Field>;
    })}
    </>}
    {busy && <p className="command-running"><LoaderCircle size={14} className="spin" /> The local Engine is processing this action…</p>}
  </Modal>;
}
