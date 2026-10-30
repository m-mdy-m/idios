import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router';
import { ArrowLeft, ArrowUpRight, BookMarked, BookOpen, Check, CheckCircle2, Clock3, FileText, FolderOpen, GitBranch, LibraryBig, LoaderCircle, MessageCircleQuestion, Network, Plus, Settings2, Tags, Trash2, X } from 'lucide-react';
import { api } from '@/lib/api';
import type { Note, Repository, ShelfAsk, ShelfBook, ShelfBookRelation } from '@/types/idios';
import { Button, EmptyState, ErrorNotice, Field } from '@/design-system';
import { AskCards } from '@/features/shelf/components/AskCards';
import { BookCover, ReadingBadge, formatShelfDate, parseTags, relationLabel, relationOptions, statusLabel, statusOptions } from '@/features/shelf/components/ShelfCommon';
import { MarkdownPreview } from '@/features/shelf/components/MarkdownPreview';
import { formatReadingTime } from '@/lib/reading-time';
import { syncPendingShelfReadingSessions } from '@/lib/reading-sessions';

type BookTab = 'read' | 'notes' | 'asks' | 'connections';

export function ShelfBookPage({ bookID }: { bookID: string }) {
  const [book, setBook] = useState<ShelfBook | null>(null);
  const [allBooks, setAllBooks] = useState<ShelfBook[]>([]);
  const [readme, setReadme] = useState('');
  const [notes, setNotes] = useState<Note[]>([]);
  const [asks, setAsks] = useState<ShelfAsk[]>([]);
  const [relations, setRelations] = useState<ShelfBookRelation[]>([]);
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [tab, setTab] = useState<BookTab>('read');
  const [loading, setLoading] = useState(true);
  const [readingTimeSeconds, setReadingTimeSeconds] = useState(0);
  const [readingSessionCount, setReadingSessionCount] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [tagDraft, setTagDraft] = useState('');
  const [tagEditOpen, setTagEditOpen] = useState(false);
  const [noteTitle, setNoteTitle] = useState('');
  const [noteContent, setNoteContent] = useState('');
  const [noteLabels, setNoteLabels] = useState('');
  const [askQuestion, setAskQuestion] = useState('');
  const [askContext, setAskContext] = useState('');
  const [askReason, setAskReason] = useState('');
  const [askFilter, setAskFilter] = useState<'open' | 'answered'>('open');
  const [answerDrafts, setAnswerDrafts] = useState<Record<string, string>>({});
  const [currentPage, setCurrentPage] = useState('0');
  const [totalPages, setTotalPages] = useState('0');
  const navigate = useNavigate();
  const [readingStatus, setReadingStatus] = useState('unread');
  const [progressOpen, setProgressOpen] = useState(false);
  const [targetBook, setTargetBook] = useState('');
  const [relationType, setRelationType] = useState('related_to');
  const [repoModalOpen, setRepoModalOpen] = useState(false);
  const [repoName, setRepoName] = useState('');
  const [repoDescription, setRepoDescription] = useState('');
  const [existingRepo, setExistingRepo] = useState('');

  const load = useCallback(async () => {
    setLoading(true); setError('');
    try { await syncPendingShelfReadingSessions();
      // Scan the folder once; book-related API operations then use the indexed record.
      let shelf = await api.shelf();
      let match = shelf.books.find((item) => item.id === bookID);
      if (!match) {
        shelf = await api.scanShelf();
        match = shelf.books.find((item) => item.id === bookID);
      }
      if (!match) throw new Error(`Book “${bookID}” is not indexed. Return to Shelf and check the connected folder.`);
      const [readmeData, noteItems, askItems, relationItems, repoItems, readingSessions] = await Promise.all([
        api.shelfReadme(bookID).catch(() => ({ content: '' })), api.shelfBookNotes(bookID), api.shelfBookAsks(bookID), api.shelfRelations(bookID), api.repositories(), api.shelfReadingSessions(bookID).catch(() => []),
      ]);
      setReadingTimeSeconds(readingSessions.reduce((sum, item) => sum + Math.max(0, Number(item.duration_seconds) || 0), 0));
      setReadingSessionCount(readingSessions.length);
      setBook(match); setAllBooks(shelf.books.filter((item) => item.available)); setReadme(readmeData.content);
      setNotes(noteItems); setAsks(askItems); setRelations(relationItems); setRepositories(repoItems);
      setTagDraft(match.tags.join(', ')); setCurrentPage(String(match.current_page)); setTotalPages(String(match.total_pages)); setReadingStatus(match.reading_status);
      setExistingRepo(match.repository_id || '');
      setRepoName(`${match.title.replace(/[^\p{L}\p{N}]+/gu, '-').replace(/^-|-$/g, '').toLowerCase() || 'book'}-lab`);
      setAnswerDrafts(Object.fromEntries(askItems.map((ask) => [ask.id, ask.answer ?? ''])));
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to load this book.'); }
    finally { setLoading(false); }
  }, [bookID]);
  useEffect(() => { void load(); }, [load]);

  const currentAsks = asks.filter((ask) => askFilter === 'open' ? ask.status === 'open' : ask.status === 'answered');
  const bookLinks = useMemo(() => relations.map((relation) => ({
    ...relation,
    otherBookID: relation.source_book_id === bookID ? relation.target_book_id : relation.source_book_id,
    otherBookTitle: relation.source_book_id === bookID ? relation.target_book_title : relation.source_book_title,
    direction: relation.source_book_id === bookID ? 'outgoing' : 'incoming',
  })), [relations, bookID]);

  async function saveTags(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!book) return; setBusy(true); setError('');
    try { const updated = await api.setShelfBookTags(book.id, parseTags(tagDraft)); setBook(updated); setTagDraft(updated.tags.join(', ')); setTagEditOpen(false); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to save tags.'); }
    finally { setBusy(false); }
  }
  async function saveProgress(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!book) return; setBusy(true); setError('');
    try {
      const updated = await api.setShelfBookProgress(book.id, { status: readingStatus, current_page: Math.max(0, Number(currentPage) || 0), total_pages: Math.max(0, Number(totalPages) || 0) });
      setBook(updated); setCurrentPage(String(updated.current_page)); setTotalPages(String(updated.total_pages)); setReadingStatus(updated.reading_status); setProgressOpen(false);
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to save progress.'); }
    finally { setBusy(false); }
  }
  async function createNote(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!book) return; setBusy(true); setError('');
    try { const note = await api.createShelfNote(book.id, { title: noteTitle.trim(), content: noteContent.trim(), labels: parseTags(noteLabels) }); setNotes((items) => [note, ...items]); setNoteTitle(''); setNoteContent(''); setNoteLabels(''); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to create note.'); }
    finally { setBusy(false); }
  }
  async function createAsk(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!book) return; setBusy(true); setError('');
    try { const ask = await api.createShelfAsk(book.id, { question: askQuestion.trim(), context: askContext.trim(), reason: askReason.trim() }); setAsks((items) => [ask, ...items]); setAnswerDrafts((items) => ({ ...items, [ask.id]: '' })); setAskQuestion(''); setAskContext(''); setAskReason(''); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to save this question.'); }
    finally { setBusy(false); }
  }
  async function saveAnswer(id: string, answer?: string) {
    setBusy(true); setError('');
    try { const updated = await api.answerShelfAsk(id, answer ?? (answerDrafts[id] ?? '')); setAsks((items) => items.map((ask) => ask.id === id ? updated : ask)); setAnswerDrafts((items) => ({ ...items, [id]: updated.answer ?? '' })); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to save the answer.'); }
    finally { setBusy(false); }
  }
  async function createRelation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!book || !targetBook) return; setBusy(true); setError('');
    try { await api.createShelfRelation({ source_book_id: book.id, target_book_id: targetBook, relation_type: relationType }); setRelations(await api.shelfRelations(book.id)); setTargetBook(''); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to connect these books.'); }
    finally { setBusy(false); }
  }
  async function removeRelation(id: string) {
    setBusy(true); setError('');
    try { await api.deleteShelfRelation(id); setRelations(await api.shelfRelations(bookID)); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to remove this relationship.'); }
    finally { setBusy(false); }
  }
  async function createRepository(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!book) return; setBusy(true); setError('');
    try { const repo = await api.createShelfBookRepository(book.id, { name: repoName.trim(), description: repoDescription.trim() || `Companion project for ${book.title}` }); setRepositories((items) => [...items, repo]); setExistingRepo(repo.id); setBook({ ...book, repository_id: repo.id, repository_name: repo.name }); setRepoModalOpen(false); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to create a companion repository.'); }
    finally { setBusy(false); }
  }
  async function linkRepository(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!book || !existingRepo) return; setBusy(true); setError('');
    try { const updated = await api.linkShelfBookRepository(book.id, existingRepo); setBook(updated); setRepoModalOpen(false); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to link the repository.'); }
    finally { setBusy(false); }
  }
  async function unlinkRepository() {
    if (!book) return; setBusy(true); setError('');
    try { const updated = await api.linkShelfBookRepository(book.id, ''); setBook(updated); setExistingRepo(''); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to unlink the repository.'); }
    finally { setBusy(false); }
  }

  if (loading) return <div className="page-stack shelf-page"><div className="shelf-loading"><LoaderCircle size={20} className="spin" /> Opening the book…</div></div>;
  if (!book) return <div className="page-stack shelf-page"><Link to="/shelf" className="shelf-back-link"><ArrowLeft size={15} /> Back to Shelf</Link>{error && <ErrorNotice message={error} onRetry={() => void load()} />}</div>;
  const progress = book.total_pages > 0 ? Math.min(100, Math.round((book.current_page / book.total_pages) * 100)) : book.reading_status === 'finished' ? 100 : 0;

  return <div className="page-stack shelf-page shelf-book-page shelf-v3-book-page">
    <div className="shelf-v3-breadcrumb"><Link to="/shelf"><LibraryBig size={14} /> Shelf</Link><span>/</span><span title={book.title}>{book.title}</span></div>
    {error && <ErrorNotice message={error} onRetry={() => void load()} />}
    {!book.available && <div className="shelf-offline-notice"><FolderOpen size={16} /><span>The source folder is offline. Your saved notes, tags and questions remain available.</span><Link className="button button--secondary" to="/shelf">Reconnect library</Link></div>}

    <header className="shelf-v3-book-hero">
      <div className="shelf-v3-book-hero__cover"><BookCover book={book} /></div>
      <div className="shelf-v3-book-hero__main">
        <span className="shelf-v3-eyebrow"><BookMarked size={13} /> BOOK DETAILS</span>
        <h1>{book.title}</h1>
        <p>{book.summary || 'A book in your local library.'}</p>
        <div className="shelf-v3-book-hero__meta"><ReadingBadge status={book.reading_status} />{book.pdf_file && <span><FileText size={13} /> PDF</span>}{book.readme_file && <span><BookOpen size={13} /> README</span>}{book.repository_name && <span><GitBranch size={13} /> {book.repository_name}</span>}<span title={`${readingSessionCount} completed focus sessions`}><Clock3 size={13} /> {formatReadingTime(readingTimeSeconds)} read</span></div>
        <div className="shelf-v3-book-hero__actions">
          {book.pdf_file && <Button onClick={() => navigate(`/shelf/${encodeURIComponent(book.id)}/read`)}><BookOpen size={14} /> Read PDF <ArrowUpRight size={13} /></Button>}
          <Button variant="secondary" onClick={() => setTab('notes')}><Plus size={14} /> New note</Button>
          <Button variant="secondary" onClick={() => setProgressOpen((open) => !open)}><CheckCircle2 size={14} /> Progress</Button>
          <Button variant="ghost" onClick={() => setRepoModalOpen(true)}><GitBranch size={14} /> {book.repository_id ? 'Repository' : 'Link project'}</Button>
        </div>
      </div>
      <aside className="shelf-v3-book-progress">
        <div className="shelf-v3-book-progress__top"><span>READING PROGRESS</span><strong>{book.total_pages > 0 ? `${progress}%` : statusLabel(book.reading_status)}</strong></div>
        <div className="shelf-v3-book-progress__track"><span style={{ width: `${progress}%` }} /></div>
        <div className="shelf-v3-book-progress__bottom"><span>{book.total_pages > 0 ? `Page ${book.current_page.toLocaleString()} of ${book.total_pages.toLocaleString()}` : 'Page count not set'}</span><button type="button" onClick={() => setProgressOpen((open) => !open)}>{progressOpen ? 'Close' : 'Edit'}</button></div>
        <div className="shelf-v3-book-progress__stat"><Clock3 size={13} /><span>{readingSessionCount} sessions</span><strong>{formatReadingTime(readingTimeSeconds)}</strong></div>
      </aside>
    </header>

    {progressOpen && <form className="panel shelf-inline-form shelf-progress-form" onSubmit={(event) => void saveProgress(event)}><div><h3>Reading progress</h3><p>Keep your place without changing the source PDF.</p></div><Field label="Status"><select className="input" value={readingStatus} onChange={(event) => setReadingStatus(event.target.value)}>{statusOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></Field><Field label="Current page"><input className="input" type="number" min="0" value={currentPage} onChange={(event) => setCurrentPage(event.target.value)} /></Field><Field label="Total pages"><input className="input" type="number" min="0" value={totalPages} onChange={(event) => setTotalPages(event.target.value)} /></Field><div className="shelf-inline-form__actions"><Button type="submit" loading={busy}><Check size={14} /> Save progress</Button><Button variant="ghost" type="button" onClick={() => setProgressOpen(false)}>Cancel</Button></div></form>}

    <section className="shelf-v3-book-meta">
      <div className="shelf-v3-book-meta__topics"><div className="shelf-v3-book-meta__heading"><span><Tags size={14} /> Topics</span><button type="button" onClick={() => setTagEditOpen((open) => !open)}><Settings2 size={13} /> {tagEditOpen ? 'Cancel' : 'Edit'}</button></div>
        {tagEditOpen ? <form className="shelf-tag-editor" onSubmit={(event) => void saveTags(event)}><input className="input" value={tagDraft} onChange={(event) => setTagDraft(event.target.value)} placeholder="systems, philosophy, architecture" /><p>Separate tags with commas.</p><Button type="submit" loading={busy}><Check size={13} /> Save tags</Button></form> : <div className="shelf-chip-list">{book.tags.length ? book.tags.map((tag) => <span className="shelf-chip" key={tag}>#{tag}</span>) : <span className="shelf-empty-inline">No tags added</span>}</div>}
      </div>
      <div className="shelf-v3-book-meta__source"><span><FolderOpen size={14} /> SOURCE</span><strong>{book.directory_name}/</strong><small>{book.readme_file || 'No README'} · {book.pdf_file || 'No PDF'}</small></div>
    </section>

    <nav className="shelf-v3-book-tabs" aria-label="Book workspace tabs">{([{ id: 'read', label: 'Overview', icon: BookOpen, detail: '' }, { id: 'notes', label: 'Notes', icon: FileText, detail: String(notes.length) }, { id: 'asks', label: 'Questions', icon: MessageCircleQuestion, detail: String(asks.filter((ask) => ask.status === 'open').length) }, { id: 'connections', label: 'Connections', icon: Network, detail: String(bookLinks.length) }] as const).map((item) => <button key={item.id} type="button" className={tab === item.id ? 'is-active' : ''} onClick={() => setTab(item.id)}><item.icon size={15} /><span>{item.label}</span>{item.detail && <small>{item.detail}</small>}</button>)}</nav>

    {tab === 'read' && <section className="panel shelf-reader-panel shelf-book-readme"><div className="shelf-panel-heading"><div><span className="eyebrow">BOOK NOTES</span><h2><FileText size={16} /> README preview</h2></div><span className="shelf-panel-heading__meta">MARKDOWN</span></div><div className="shelf-reader-panel__body"><MarkdownPreview markdown={readme} /></div></section>}

    {tab === 'notes' && <section className="shelf-tab-section"><div className="shelf-tab-heading"><div><span className="eyebrow">THOUGHTS THAT STAY WITH THE BOOK</span><h2>Reading notes</h2><p>Notes are linked to this book and remain available in IDIOS Notes and the knowledge graph.</p></div><span className="shelf-total-badge">{notes.length} notes</span></div>
      <form className="panel shelf-compose-card" onSubmit={(event) => void createNote(event)}><div className="shelf-compose-card__label"><Plus size={14} /> Capture a thought</div><Field label="Note title"><input className="input" value={noteTitle} onChange={(event) => setNoteTitle(event.target.value)} placeholder="e.g. Why the pipeline stalls" required maxLength={200} /></Field><Field label="Your note"><textarea className="input" value={noteContent} onChange={(event) => setNoteContent(event.target.value)} placeholder="Write the idea, insight, quote, or implementation detail…" rows={4} /></Field><Field label="Tags (comma-separated)"><input className="input" value={noteLabels} onChange={(event) => setNoteLabels(event.target.value)} placeholder="architecture, follow-up, experiment" /></Field><div className="shelf-compose-card__footer"><span>Saved in IDIOS Notes and linked to this book</span><Button type="submit" loading={busy}><Plus size={14} /> Save note</Button></div></form>
      {notes.length === 0 ? <EmptyState icon={<FileText size={22} />} title="Your first note starts here" body="Capture a detail, connection or implementation idea while it is fresh." /> : <div className="shelf-notes-list">{notes.map((note) => <article className="panel shelf-note-card" key={note.id}><div className="shelf-note-card__top"><span><FileText size={14} /> NOTE</span><time>{formatShelfDate(note.updated_at)}</time></div><h3>{note.title}</h3><p>{note.content || 'No note body.'}</p>{note.labels?.length ? <div className="shelf-chip-list">{note.labels.map((label) => <span className="shelf-chip" key={label.id}>#{label.name}</span>)}</div> : null}<div className="shelf-note-card__footer"><span><Link to="/notes"><ArrowUpRight size={12} /> Open all notes</Link></span><span><LibraryBig size={12} /> {book.title}</span></div></article>)}</div>}
    </section>}

    {tab === 'asks' && <section className="shelf-tab-section"><div className="shelf-tab-heading"><div><span className="eyebrow">CURIOSITY, CAPTURED</span><h2>Questions from this book</h2><p>Save what you don’t understand yet. Discover the answer later, then archive it without losing the trail.</p></div><span className="shelf-total-badge">{asks.filter((ask) => ask.status === 'open').length} open</span></div>
      <form className="panel shelf-compose-card shelf-ask-compose" onSubmit={(event) => void createAsk(event)}><div className="shelf-compose-card__label"><MessageCircleQuestion size={14} /> Ask a question</div><Field label="What came up while reading?"><textarea className="input" value={askQuestion} onChange={(event) => setAskQuestion(event.target.value)} placeholder="Why does the design use a five-stage pipeline instead of…?" required rows={2} /></Field><div className="shelf-ask-compose__fields"><Field label="Where in the book? (optional)"><input className="input" value={askContext} onChange={(event) => setAskContext(event.target.value)} placeholder="Chapter 4, page 126" /></Field><Field label="What prompted the question? (optional)"><input className="input" value={askReason} onChange={(event) => setAskReason(event.target.value)} placeholder="What seems unclear or surprising?" /></Field></div><div className="shelf-compose-card__footer"><span>Also visible in Shelf → Asks</span><Button type="submit" loading={busy}><Plus size={14} /> Save question</Button></div></form>
      <div className="shelf-asks-filter"><button className={askFilter === 'open' ? 'is-active' : ''} type="button" onClick={() => setAskFilter('open')}>Open <span>{asks.filter((ask) => ask.status === 'open').length}</span></button><button className={askFilter === 'answered' ? 'is-active' : ''} type="button" onClick={() => setAskFilter('answered')}>Answered <span>{asks.filter((ask) => ask.status === 'answered').length}</span></button></div>
      <AskCards asks={currentAsks} answerDrafts={answerDrafts} setAnswerDrafts={setAnswerDrafts} saveAnswer={(id, answer) => void saveAnswer(id, answer)} busy={busy} showBook={false} />
    </section>}

    {tab === 'connections' && <section className="shelf-tab-section"><div className="shelf-tab-heading"><div><span className="eyebrow">IDEAS ARE BETTER CONNECTED</span><h2>Book connections</h2><p>Express the relationship, not just a link. Connections also appear in Shelf’s and IDIOS’s knowledge graphs.</p></div><Link className="button button--secondary" to="/shelf/graph"><Network size={14} /> Open graph</Link></div>
      <form className="panel shelf-connection-form" onSubmit={(event) => void createRelation(event)}><div className="shelf-connection-form__intro"><span className="shelf-connection-form__icon"><Network size={16} /></span><div><h3>Connect another book</h3><p>Choose a target and label what connects them.</p></div></div><div className="shelf-connection-form__fields"><Field label="Related book"><select className="input" value={targetBook} onChange={(event) => setTargetBook(event.target.value)} required><option value="">Choose a book…</option>{allBooks.filter((item) => item.id !== book.id).map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}</select></Field><Field label="Relationship"><select className="input" value={relationType} onChange={(event) => setRelationType(event.target.value)}>{relationOptions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></Field><Button type="submit" loading={busy} disabled={!targetBook}><Plus size={14} /> Connect</Button></div><p className="shelf-connection-form__hint">{relationOptions.find((item) => item.value === relationType)?.description}</p></form>
      {bookLinks.length === 0 ? <EmptyState icon={<Network size={22} />} title="No connections yet" body="Start with a prerequisite, a contrasting approach, or a book that explains an idea more deeply." /> : <div className="shelf-connection-list">{bookLinks.map((relation) => { const relatedBook = allBooks.find((item) => item.id === relation.otherBookID); return <article className="panel shelf-connection-card" key={relation.id}><Link className="shelf-connection-card__node" to={`/shelf/${encodeURIComponent(relation.otherBookID)}`}><BookCover book={relatedBook ?? { id: relation.otherBookID, title: relation.otherBookTitle }} small /><span><strong>{relation.otherBookTitle}</strong><small>{relation.direction === 'outgoing' ? `This book → ${relationLabel(relation.relation_type).toLowerCase()}` : `${relationLabel(relation.relation_type)} → this book`}</small></span></Link><div className="shelf-connection-card__relation"><span>{relationLabel(relation.relation_type)}</span><Link to={`/shelf/${encodeURIComponent(relation.otherBookID)}`}>View book <ArrowUpRight size={12} /></Link></div><button type="button" className="shelf-icon-button shelf-icon-button--danger" aria-label="Remove relationship" title="Remove relationship" onClick={() => void removeRelation(relation.id)}><Trash2 size={14} /></button></article>; })}</div>}
    </section>}

    <div className="shelf-book-bottom-nav"><Link to="/shelf"><ArrowLeft size={14} /> Back to library</Link><span>Local-first · Connected knowledge</span><Link to="/shelf/asks">All open asks <ArrowUpRight size={13} /></Link></div>
    {repoModalOpen && <div className="shelf-modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setRepoModalOpen(false); }}><section className="panel shelf-repo-modal" role="dialog" aria-modal="true" aria-labelledby="shelf-repo-title"><button type="button" className="shelf-modal-close" aria-label="Close" onClick={() => setRepoModalOpen(false)}><X size={17} /></button><span className="shelf-kicker"><GitBranch size={14} /> BOOK + BUILD</span><h2 id="shelf-repo-title">Companion repository</h2><p>Keep the code you create while reading connected to the book that inspired it.</p>{book.repository_id ? <div className="shelf-linked-repo"><GitBranch size={17} /><div><strong>{book.repository_name}</strong><span>Already connected to this book</span></div><Button variant="danger" onClick={() => void unlinkRepository()} loading={busy}>Unlink</Button></div> : <><form className="shelf-repo-create-form" onSubmit={(event) => void createRepository(event)}><h3>Create a new repository</h3><Field label="Repository name"><input className="input" value={repoName} onChange={(event) => setRepoName(event.target.value)} required maxLength={100} /></Field><Field label="Description"><textarea className="input" value={repoDescription} onChange={(event) => setRepoDescription(event.target.value)} placeholder={`Experiments and implementations from ${book.title}`} rows={2} /></Field><Button type="submit" loading={busy}><Plus size={14} /> Create and link repository</Button></form><div className="shelf-repo-or"><span />or link an existing repository<span /></div><form className="shelf-repo-link-form" onSubmit={(event) => void linkRepository(event)}><Field label="Existing repository"><select className="input" value={existingRepo} onChange={(event) => setExistingRepo(event.target.value)}><option value="">Choose repository…</option>{repositories.map((repo) => <option key={repo.id} value={repo.id}>{repo.name}</option>)}</select></Field><Button type="submit" variant="secondary" disabled={!existingRepo} loading={busy}>Link repository</Button></form></>}</section></div>}
  </div>;
}
