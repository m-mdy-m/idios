import { useMemo, useState, type FormEvent, type CSSProperties } from 'react';
import { BookOpen, FileText, Plus, Search, Trash2, X } from 'lucide-react';
import { api } from '@/lib/api';
import type { Note } from '@/types/idios';
import { useWorkspaceStore } from '@/store/workspace-store';
import { Button, EmptyState, Field, PageHeading } from '@/components/ui/primitives';
import { Modal } from '@/components/ui/Modal';
import { formatRelative } from '@/components/ui/IssueTable';

const labelPalette = ['#1f6feb', '#8957e5', '#238636', '#bc4c00', '#bf3989', '#0f8b8d', '#9a6700', '#6e7781'];
function defaultLabelColor(name: string) {
  let hash = 2166136261;
  for (const char of name.toLowerCase()) { hash ^= char.charCodeAt(0); hash = Math.imul(hash, 16777619); }
  return labelPalette[(hash >>> 0) % labelPalette.length];
}
function normalizeLabelNames(raw: string) {
  return [...new Map(raw.split(',').map((part) => part.trim().replace(/^#/, '')).filter(Boolean).map((name) => [name.toLowerCase(), name])).values()];
}

export function NotesPage() {
  const { dashboard, load } = useWorkspaceStore();
  const [query, setQuery] = useState('');
  const [activeLabel, setActiveLabel] = useState('all');
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Note | null>(null);
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [labels, setLabels] = useState('');
  const [labelColors, setLabelColors] = useState<Record<string, string>>({});
  const [showLabelColors, setShowLabelColors] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [deleting, setDeleting] = useState<string | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Note | null>(null);
  const allNotes = dashboard?.notes ?? [];
  const knownLabels = useMemo(() => {
    const items = new Map<string, { name: string; color: string; count: number }>();
    allNotes.forEach((note) => note.labels?.forEach((label) => {
      const key = label.name.toLowerCase();
      const current = items.get(key);
      items.set(key, { name: label.name, color: label.color || defaultLabelColor(label.name), count: (current?.count ?? 0) + 1 });
    }));
    return [...items.values()].sort((a, b) => a.name.localeCompare(b.name));
  }, [allNotes]);
  const filteredNotes = useMemo(() => {
    const text = query.trim().toLowerCase();
    return allNotes.filter((note) => {
      const textMatches = !text || `${note.title} ${note.content} ${note.labels?.map((label) => label.name).join(' ')}`.toLowerCase().includes(text);
      const labelMatches = activeLabel === 'all' || (activeLabel === 'untagged' ? !note.labels?.length : note.labels?.some((label) => label.name.toLowerCase() === activeLabel));
      return textMatches && labelMatches;
    });
  }, [allNotes, query, activeLabel]);
  const editorLabelNames = useMemo(() => normalizeLabelNames(labels), [labels]);

  function createNew() {
    setEditing(null); setTitle(''); setContent(''); setLabels(''); setLabelColors({}); setShowLabelColors(false); setError(''); setOpen(true);
  }
  function editNote(note: Note) {
    setEditing(note); setTitle(note.title); setContent(note.content); setLabels(note.labels?.map((label) => label.name).join(', ') ?? '');
    setLabelColors(Object.fromEntries((note.labels ?? []).map((label) => [label.name.toLowerCase(), /^#[0-9a-f]{6}$/i.test(label.color) ? label.color : defaultLabelColor(label.name)])));
    setShowLabelColors(false); setError(''); setOpen(true);
  }
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError('');
    const normalizedLabels = editorLabelNames;
    try {
      if (editing) await api.executeCommand('note.update', { id: editing.id, title: title.trim(), content: content.trim(), labels: normalizedLabels.join(', ') });
      else await api.createNote({ title: title.trim(), content: content.trim(), labels: normalizedLabels });
      for (const name of normalizedLabels) {
        await api.executeCommand('note.label.color', { name, color: labelColors[name.toLowerCase()] || defaultLabelColor(name) });
      }
      setOpen(false); setEditing(null); setTitle(''); setContent(''); setLabels(''); await load(true);
    } catch (err) { setError(err instanceof Error ? err.message : `Unable to ${editing ? 'update' : 'create'} note.`); }
    finally { setBusy(false); }
  }
  async function remove(id: string) {
    setDeleting(id); setError('');
    try { await api.deleteNote(id); setPendingDelete(null); await load(true); if (editing?.id === id) { setEditing(null); setOpen(false); } }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to delete note.'); }
    finally { setDeleting(null); }
  }

  return <div className="page-stack">
    <PageHeading eyebrow="PERSONAL KNOWLEDGE" title="Notes" description="Capture thoughts, decisions, and references. Use labels to keep related notes together." actions={<Button onClick={createNew}><Plus size={16} /> New note</Button>} />
    {error && !open && <div className="notice notice--error" role="alert">{error}</div>}
    <section className="notes-library panel" aria-label="Notes library">
      <div className="notes-library__toolbar">
        <div><h2>Notes</h2><span>{filteredNotes.length} shown · {allNotes.length} total</span></div>
        <label className="inline-search"><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search title, text, or label…" aria-label="Search notes" />{query && <button type="button" onClick={() => setQuery('')} aria-label="Clear search"><X size={13} /></button>}</label>
      </div>
      <div className="notes-label-filters" role="tablist" aria-label="Filter notes by label">
        <button type="button" role="tab" aria-selected={activeLabel === 'all'} className={activeLabel === 'all' ? 'notes-filter notes-filter--active' : 'notes-filter'} onClick={() => setActiveLabel('all')}>All <span>{allNotes.length}</span></button>
        <button type="button" role="tab" aria-selected={activeLabel === 'untagged'} className={activeLabel === 'untagged' ? 'notes-filter notes-filter--active' : 'notes-filter'} onClick={() => setActiveLabel('untagged')}>Unlabelled <span>{allNotes.filter((note) => !note.labels?.length).length}</span></button>
        {knownLabels.map((label) => <button type="button" role="tab" key={label.name} aria-selected={activeLabel === label.name.toLowerCase()} className={activeLabel === label.name.toLowerCase() ? 'notes-filter notes-filter--active' : 'notes-filter'} onClick={() => setActiveLabel(label.name.toLowerCase())}><i style={{ backgroundColor: label.color }} />{label.name}<span>{label.count}</span></button>)}
      </div>
      {filteredNotes.length ? <div className="notes-tile-grid">{filteredNotes.map((note) => {
        const color = note.labels?.[0]?.color || 'var(--accent)';
        return <article className="note-tile" key={note.id} style={{ borderTopColor: color }}>
          <button type="button" className="note-tile__open" onClick={() => editNote(note)} aria-label={`Open note ${note.title}`}>
            <span className="note-tile__heading"><span className="note-tile__icon"><FileText size={15} /></span><time>{formatRelative(note.updated_at)}</time></span>
            <strong>{note.title}</strong>
            <span className="note-tile__preview">{note.content.trim() || 'No content yet.'}</span>
            {note.labels?.length ? <span className="note-tile__labels">{note.labels.slice(0, 3).map((label) => <span className="note-color-label" key={label.id} style={{ '--label-color': /^#[0-9a-f]{6}$/i.test(label.color) ? label.color : defaultLabelColor(label.name) } as CSSProperties}>{label.name}</span>)}{note.labels.length > 3 && <span className="note-color-label note-color-label--more">+{note.labels.length - 3}</span>}</span> : <span className="note-tile__labels note-tile__labels--empty">Add a label to group this note</span>}
          </button>
          <button type="button" className="note-tile__delete" disabled={deleting === note.id} onClick={() => setPendingDelete(note)} aria-label={`Delete note ${note.title}`} title="Delete note"><Trash2 size={14} /></button>
        </article>;
      })}</div> : <EmptyState title={query || activeLabel !== 'all' ? 'No matching notes' : 'Make room for ideas'} body={query || activeLabel !== 'all' ? 'Try another search or label filter.' : 'Save a thought or decision. Notes will appear here as compact, searchable tiles.'} icon={<BookOpen size={22} />} />}
    </section>
    <Modal open={open} title={editing ? 'Edit note' : 'New note'} description={editing ? 'Update the note and its labels.' : 'Capture an idea, decision, or useful detail.'} onClose={() => setOpen(false)} onSubmit={save} submitLabel={editing ? 'Save changes' : 'Save note'} busy={busy} wide>
      {error && <div className="notice notice--error" role="alert">{error}</div>}
      <Field label="Title"><input autoFocus required maxLength={200} value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Give this thought a name" /></Field>
      <Field label="Content"><textarea rows={7} value={content} onChange={(event) => setContent(event.target.value)} placeholder="Write down the useful context…" /></Field>
      <Field label="Labels" hint="Separate labels with commas. Use the same label on multiple notes to group them."><input value={labels} onChange={(event) => setLabels(event.target.value)} placeholder="architecture, decision, research" /></Field>
      {editorLabelNames.length > 0 && <section className="note-label-color-editor"><div className="note-label-color-editor__heading"><div><strong>Label colors</strong><p>Choose colors used throughout the Notes library.</p></div><Button type="button" variant="ghost" onClick={() => setShowLabelColors((value) => !value)}>{showLabelColors ? 'Hide colors' : 'Customize'}</Button></div>{showLabelColors && <div className="note-label-color-editor__list">{editorLabelNames.map((name) => <label key={name} className="note-label-color-editor__item"><span><i style={{ backgroundColor: labelColors[name.toLowerCase()] || defaultLabelColor(name) }} />{name}</span><input type="color" aria-label={`Color for ${name}`} value={labelColors[name.toLowerCase()] || defaultLabelColor(name)} onChange={(event) => setLabelColors((current) => ({ ...current, [name.toLowerCase()]: event.target.value }))} /></label>)}</div>}</section>}
      {editing && <div className="note-editor-footer"><span>Last updated {formatRelative(editing.updated_at)}</span><Button type="button" variant="danger" onClick={() => setPendingDelete(editing)} loading={deleting === editing.id}><Trash2 size={14} /> Delete note</Button></div>}
    </Modal>
    <Modal open={Boolean(pendingDelete)} title={`Delete “${pendingDelete?.title || 'note'}”?`} description="This removes the note from your workspace. Any links to it will be detached; other records are kept." onClose={() => setPendingDelete(null)} onSubmit={(event) => { event.preventDefault(); if (pendingDelete) void remove(pendingDelete.id); }} submitLabel="Delete note" submitVariant="danger" busy={deleting === pendingDelete?.id}>
      {error && <div className="notice notice--error" role="alert">{error}</div>}
      <div className="delete-confirm-summary"><span className="delete-confirm-summary__icon"><Trash2 size={17} /></span><div><strong>{pendingDelete?.title || 'Untitled note'}</strong><p>{pendingDelete?.content?.slice(0, 180) || 'No content'}{(pendingDelete?.content?.length ?? 0) > 180 ? '…' : ''}</p></div></div>
    </Modal>
  </div>;
}
