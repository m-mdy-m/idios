import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { Link, useParams } from 'react-router';
import {
  ArrowDownAZ, ArrowUpRight, BookOpen, Check, Clock3, FolderOpen, FileText,
  LibraryBig, LoaderCircle, MessageCircleQuestion, Network, RotateCcw,
  Search, Settings2, Tags, X, SlidersHorizontal, ChevronDown,
} from 'lucide-react';
import { ShelfBookPage } from '@/features/shelf';
import { api } from '@/lib/api';
import type { ShelfAsk, ShelfReadingSession, ShelfState } from '@/types/idios';
import { formatReadingTime, readingSecondsByBook, shelfReadingTotals } from '@/lib/reading-time';
import { syncPendingShelfReadingSessions } from '@/lib/reading-sessions';
import { Button, EmptyState, ErrorNotice, Field } from '@/design-system';
import { BookCover, ReadingBadge } from '@/features/shelf/components/ShelfCommon';
import { PrivatePath } from '@/components/privacy/PrivatePath';

type ShelfStatus = 'all' | 'unread' | 'reading' | 'finished';
type ShelfFormat = 'all' | 'pdf' | 'readme' | 'complete';
type ShelfSort = 'recent' | 'title' | 'progress';

export function ShelfPage() {
  const { slug } = useParams();
  if (slug) return <ShelfBookPage bookID={slug} />;
  return <ShelfLibraryPage />;
}

function ShelfLibraryPage() {
  const [shelf, setShelf] = useState<ShelfState | null>(null);
  const [rootPath, setRootPath] = useState('');
  const [showPathEditor, setShowPathEditor] = useState(false);
  const [asks, setAsks] = useState<ShelfAsk[]>([]);
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState<ShelfStatus>('all');
  const [activeTag, setActiveTag] = useState('all');
  const [formatFilter, setFormatFilter] = useState<ShelfFormat>('all');
  const [sortBy, setSortBy] = useState<ShelfSort>('recent');
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [view, setView] = useState<'grid' | 'list'>('grid');
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [sessions, setSessions] = useState<ShelfReadingSession[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      await syncPendingShelfReadingSessions();
      const [data, openAsks, readingSessions] = await Promise.all([
        api.shelf(), api.shelfAsks('open'), api.shelfReadingSessions().catch(() => []),
      ]);
      setShelf(data);
      setAsks(openAsks);
      setSessions(readingSessions);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to load Shelf.');
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { void load(); }, [load]);

  const books = useMemo(() => shelf?.books.filter((book) => book.available) ?? [], [shelf]);
  const readingTotals = useMemo(() => shelfReadingTotals(sessions), [sessions]);
  const bookReadingSeconds = useMemo(() => readingSecondsByBook(sessions), [sessions]);
  const tags = useMemo(() => [...new Set(books.flatMap((book) => book.tags ?? []))].sort((a, b) => a.localeCompare(b)), [books]);
  const readingCount = books.filter((book) => book.reading_status === 'reading').length;
  const finishedCount = books.filter((book) => book.reading_status === 'finished').length;
  const unreadCount = books.filter((book) => book.reading_status === 'unread').length;

  const filteredBooks = useMemo(() => {
    const text = query.trim().toLocaleLowerCase();
    const filtered = books.filter((book) => {
      const haystack = `${book.title} ${book.summary ?? ''} ${book.tags.join(' ')} ${book.directory_name}`.toLocaleLowerCase();
      const matchesText = !text || haystack.includes(text);
      const matchesStatus = status === 'all' || book.reading_status === status;
      const matchesTag = activeTag === 'all' || book.tags.some((tag) => tag.toLocaleLowerCase() === activeTag.toLocaleLowerCase());
      const hasPDF = Boolean(book.pdf_file);
      const hasReadme = Boolean(book.readme_file);
      const matchesFormat = formatFilter === 'all'
        || (formatFilter === 'pdf' && hasPDF)
        || (formatFilter === 'readme' && hasReadme)
        || (formatFilter === 'complete' && hasPDF && hasReadme);
      return matchesText && matchesStatus && matchesTag && matchesFormat;
    });
    return filtered.sort((a, b) => {
      if (sortBy === 'title') return a.title.localeCompare(b.title, undefined, { sensitivity: 'base' });
      if (sortBy === 'progress') {
        const progress = (book: typeof a) => book.total_pages > 0 ? book.current_page / book.total_pages : book.reading_status === 'finished' ? 1 : 0;
        return progress(b) - progress(a) || a.title.localeCompare(b.title);
      }
      return (Date.parse(b.updated_at || b.created_at) || 0) - (Date.parse(a.updated_at || a.created_at) || 0);
    });
  }, [books, query, status, activeTag, formatFilter, sortBy]);

  const filtersActive = Boolean(query.trim()) || status !== 'all' || activeTag !== 'all' || formatFilter !== 'all';
  const advancedFilterCount = Number(activeTag !== 'all') + Number(formatFilter !== 'all');
  function clearFilters() {
    setQuery(''); setStatus('all'); setActiveTag('all'); setFormatFilter('all'); setSortBy('recent');
  }

  async function rescanLibrary() {
    setBusy(true);
    setError('');
    try {
      const result = await api.scanShelf();
      setShelf(result);
      const [nextAsks, nextSessions] = await Promise.all([
        api.shelfAsks('open'), api.shelfReadingSessions().catch(() => []),
      ]);
      setAsks(nextAsks);
      setSessions(nextSessions);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to rescan the library.');
    } finally {
      setBusy(false);
    }
  }

  async function connectFolder(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError('');
    try {
      const result = await api.setShelfRoot(rootPath.trim());
      setShelf(result);
      setRootPath('');
      setShowPathEditor(false);
      const [nextAsks, nextSessions] = await Promise.all([
        api.shelfAsks('open'), api.shelfReadingSessions().catch(() => []),
      ]);
      setAsks(nextAsks);
      setSessions(nextSessions);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to connect that folder.');
    } finally {
      setBusy(false);
    }
  }

  return <div className="page-stack shelf-page shelf-v3-library">
    <header className="shelf-v3-page-header">
      <div className="shelf-v3-page-header__intro">
        <span className="shelf-v3-eyebrow"><LibraryBig size={14} /> PERSONAL LIBRARY</span>
        <h1>Shelf</h1>
        <p>Your books, notes and reading progress — together, without the clutter.</p>
      </div>
      <div className="shelf-v3-page-header__actions">
        <Link className="shelf-v3-quiet-action" to="/shelf/asks"><MessageCircleQuestion size={15} /> Questions <span>{asks.length}</span></Link>
        <Link className="shelf-v3-quiet-action" to="/shelf/graph"><Network size={15} /> Knowledge graph <ArrowUpRight size={13} /></Link>
        <Button onClick={() => { setShowPathEditor((open) => !open); setRootPath(''); setError(''); }}><FolderOpen size={15} /> {shelf?.configured ? 'Library source' : 'Connect library'}</Button>
      </div>
    </header>

    {error && <ErrorNotice message={error} onRetry={() => void load()} />}

    {showPathEditor && <section className="shelf-v3-source panel">
      <div className="shelf-v3-source__heading"><span className="shelf-v3-icon-tile"><FolderOpen size={17} /></span><div><h2>Library source</h2><p>IDIOS reads files in place. Your PDFs are not moved or duplicated.</p></div><button className="shelf-v3-icon-button" type="button" aria-label="Close library source settings" onClick={() => setShowPathEditor(false)}><X size={16} /></button></div>
      {shelf?.configured && <div className="shelf-v3-source__current"><span>Current folder</span><PrivatePath value={shelf.root_path} /></div>}
      <form className="shelf-v3-source__form" onSubmit={(event) => void connectFolder(event)}>
        <Field label="Folder path" hint="Choose a folder containing one subfolder per book."><input className="input" value={rootPath} onChange={(event) => setRootPath(event.target.value)} placeholder="D:\\Books or /home/user/books" autoComplete="off" required /></Field>
        <div className="shelf-v3-source__actions"><Button type="submit" loading={busy}><Check size={14} /> Connect and scan</Button><Button variant="ghost" type="button" onClick={() => setShowPathEditor(false)}>Cancel</Button></div>
      </form>
    </section>}

    {!loading && !shelf?.configured && <section className="shelf-v3-empty-library panel">
      <div className="shelf-v3-empty-library__icon"><BookOpen size={24} /></div>
      <span className="shelf-v3-eyebrow">LOCAL-FIRST READING</span>
      <h2>Start with your own bookshelf.</h2>
      <p>Connect an existing folder. IDIOS indexes each book folder and opens its original PDF alongside your notes.</p>
      <Button onClick={() => { setRootPath(''); setShowPathEditor(true); }}><FolderOpen size={15} /> Choose library folder</Button>
      <div className="shelf-v3-empty-library__hint"><span>01</span> Connect a folder <i /> <span>02</span> Browse books <i /> <span>03</span> Read and annotate</div>
    </section>}

    {shelf?.configured && <>
      <section className="shelf-v3-summary" aria-label="Library overview">
        <div className="shelf-v3-summary__source"><span className="shelf-v3-source-dot" /><div><strong>Library connected</strong><span><PrivatePath value={shelf.root_path} /></span></div><button type="button" className="shelf-v3-icon-button" onClick={() => { setRootPath(''); setShowPathEditor(true); }} aria-label="Change library folder" title="Change library folder"><Settings2 size={15} /></button></div>
        <div className="shelf-v3-summary__metrics"><div><strong>{books.length}</strong><span>Total books</span></div><div><strong>{readingCount}</strong><span>Reading</span></div><div><strong>{unreadCount}</strong><span>To read</span></div><div><strong>{finishedCount}</strong><span>Finished</span></div></div>
        <div className="shelf-v3-summary__time"><span>Reading time</span><strong>{formatReadingTime(readingTotals.weekSeconds)}</strong><small>This week</small></div>
      </section>

      <section className="shelf-v3-catalog" aria-label="Book catalog">
        <div className="shelf-v3-catalog__heading"><div><h2>Your books</h2><p>Search and filter the library to find what you need.</p></div><span className="shelf-v3-result-count">{filteredBooks.length} <span>/ {books.length}</span></span></div>
        <div className="shelf-v3-search-row">
          <label className="shelf-v3-search"><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search books, topics or tags" aria-label="Search books" />{query && <button type="button" onClick={() => setQuery('')} aria-label="Clear search"><X size={14} /></button>}</label>
          <button className={`shelf-v3-filter-toggle${filtersOpen || advancedFilterCount > 0 ? ' is-active' : ''}`} type="button" aria-expanded={filtersOpen} onClick={() => setFiltersOpen((open) => !open)}><SlidersHorizontal size={15} /><span>Filters</span>{advancedFilterCount > 0 && <b>{advancedFilterCount}</b>}<ChevronDown size={14} className={filtersOpen ? 'is-rotated' : ''} /></button>
          <div className="shelf-v3-view-toggle" aria-label="Catalog view"><button className={view === 'grid' ? 'is-active' : ''} type="button" onClick={() => setView('grid')} aria-label="Grid view"><BookOpen size={15} /></button><button className={view === 'list' ? 'is-active' : ''} type="button" onClick={() => setView('list')} aria-label="List view"><span className="shelf-v3-list-glyph"><i /><i /><i /></span></button></div>
          <Button variant="secondary" onClick={() => void rescanLibrary()} loading={busy}><RotateCcw size={14} /> Rescan</Button>
        </div>

        <div className="shelf-v3-status-filters" role="group" aria-label="Filter by reading status">
          {([{ value: 'all', label: 'All books', count: books.length }, { value: 'unread', label: 'To read', count: unreadCount }, { value: 'reading', label: 'Reading', count: readingCount }, { value: 'finished', label: 'Finished', count: finishedCount }] as const).map((item) => <button key={item.value} type="button" className={status === item.value ? 'is-active' : ''} onClick={() => setStatus(item.value)}><span>{item.label}</span><b>{item.count}</b></button>)}
        </div>

        {!filtersOpen && (advancedFilterCount > 0 || sortBy !== 'recent') && <div className="shelf-v3-active-filters" aria-label="Active filters">
          <span>Active:</span>
          {activeTag !== 'all' && <button type="button" onClick={() => setActiveTag('all')}>Topic: {activeTag}<X size={12} /></button>}
          {formatFilter !== 'all' && <button type="button" onClick={() => setFormatFilter('all')}>Files: {formatFilter === 'complete' ? 'PDF + README' : formatFilter.toUpperCase()}<X size={12} /></button>}
          {sortBy !== 'recent' && <button type="button" onClick={() => setSortBy('recent')}>Sort: {sortBy === 'title' ? 'Title A–Z' : 'Progress'}<X size={12} /></button>}
          <button className="shelf-v3-active-filters__reset" type="button" onClick={clearFilters}>Reset all</button>
        </div>}

        {filtersOpen && <div className="shelf-v3-filter-row">
          <label className="shelf-v3-select"><span><Tags size={13} /> Topic</span><select value={activeTag} onChange={(event) => setActiveTag(event.target.value)} aria-label="Filter by topic"><option value="all">All topics</option>{tags.map((tag) => <option key={tag} value={tag}>{tag}</option>)}</select></label>
          <label className="shelf-v3-select"><span><FileText size={13} /> Files</span><select value={formatFilter} onChange={(event) => setFormatFilter(event.target.value as ShelfFormat)} aria-label="Filter by available files"><option value="all">Any file set</option><option value="pdf">Has PDF</option><option value="readme">Has README</option><option value="complete">PDF + README</option></select></label>
          <label className="shelf-v3-select shelf-v3-select--sort"><span><ArrowDownAZ size={13} /> Sort</span><select value={sortBy} onChange={(event) => setSortBy(event.target.value as ShelfSort)} aria-label="Sort books"><option value="recent">Recently updated</option><option value="title">Title A–Z</option><option value="progress">Reading progress</option></select></label>
          {filtersActive && <button className="shelf-v3-clear-filters" type="button" onClick={clearFilters}><X size={13} /> Reset</button>}
        </div>}

        {loading ? <div className="shelf-v3-loading"><LoaderCircle size={18} className="spin" /> Loading your library…</div>
          : books.length === 0 ? <EmptyState icon={<BookOpen size={23} />} title="No book folders found" body="Put each book in its own folder, then rescan the library." />
          : filteredBooks.length === 0 ? <div className="shelf-v3-no-results"><Search size={21} /><strong>No books match these filters</strong><p>Try a different search or clear the selected filters.</p><Button variant="secondary" onClick={clearFilters}>Clear all filters</Button></div>
          : <div className={`shelf-v3-book-grid${view === 'list' ? ' shelf-v3-book-grid--list' : ''}`}>{filteredBooks.map((book) => {
            const progress = book.total_pages > 0 ? Math.min(100, Math.round((book.current_page / book.total_pages) * 100)) : book.reading_status === 'finished' ? 100 : 0;
            return <Link to={`/shelf/${encodeURIComponent(book.id)}`} className="shelf-v3-book-card" key={book.id}>
              <div className="shelf-v3-book-card__cover"><BookCover book={book} /></div>
              <div className="shelf-v3-book-card__body">
                <div className="shelf-v3-book-card__top"><ReadingBadge status={book.reading_status} /><span>{book.pdf_file ? 'PDF' : 'No PDF'}</span></div>
                <h3>{book.title}</h3>
                <p className="shelf-v3-book-card__summary">{book.summary || 'No summary yet.'}</p>
                {book.tags.length > 0 && <div className="shelf-v3-book-card__tags">{book.tags.slice(0, 2).map((tag) => <span key={tag}>{tag}</span>)}{book.tags.length > 2 && <span>+{book.tags.length - 2}</span>}</div>}
                <div className="shelf-v3-book-card__progress"><div className="shelf-v3-progress"><span style={{ width: `${progress}%` }} /></div><span>{book.total_pages > 0 ? `${progress}%` : book.reading_status === 'finished' ? 'Done' : 'Not started'}</span></div>
                <div className="shelf-v3-book-card__footer"><span><BookOpen size={12} /> {book.total_pages > 0 ? `${book.current_page} of ${book.total_pages} pages` : 'Reading progress'}</span><span title="Time spent reading"><Clock3 size={12} /> {formatReadingTime(bookReadingSeconds.get(book.id) ?? 0)}</span></div>
              </div>
              <span className="shelf-v3-book-card__arrow"><ArrowUpRight size={14} /></span>
            </Link>;
          })}</div>}
        <footer className="shelf-v3-catalog__footer"><span>Showing <strong>{filteredBooks.length}</strong> of {books.length} books</span><span>{tags.length} topics · {formatReadingTime(readingTotals.totalSeconds)} total reading time</span></footer>
      </section>
    </>}
  </div>;
}
