import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router';
import { ArrowLeft, ArrowUpRight, ChevronLeft, LibraryBig, LoaderCircle, MessageCircleQuestion, Network } from 'lucide-react';
import { api } from '@/lib/api';
import type { ShelfAsk, ShelfBook } from '@/types/idios';
import { ErrorNotice, PageHeading } from '@/design-system';
import { AskCards } from '@/features/shelf/components/AskCards';

export function ShelfAsksPage() {
  const [status, setStatus] = useState<'open' | 'answered' | 'all'>('open');
  const [asks, setAsks] = useState<ShelfAsk[]>([]);
  const [books, setBooks] = useState<ShelfBook[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [answerDrafts, setAnswerDrafts] = useState<Record<string, string>>({});
  const load = useCallback(async () => {
    setLoading(true); setError('');
    try {
      const shelf = await api.shelf();
      const items = await api.shelfAsks('all');
      setBooks(shelf.books); setAsks(items); setAnswerDrafts(Object.fromEntries(items.map((ask) => [ask.id, ask.answer ?? ''])));
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to load Shelf asks.'); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  async function saveAnswer(id: string, answer?: string) {
    setBusy(true); setError('');
    try { await api.answerShelfAsk(id, answer ?? (answerDrafts[id] ?? '')); const items = await api.shelfAsks('all'); setAsks(items); setAnswerDrafts(Object.fromEntries(items.map((ask) => [ask.id, ask.answer ?? '']))); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to save this answer.'); }
    finally { setBusy(false); }
  }
  const openCount = asks.filter((ask) => ask.status === 'open').length;
  const answeredCount = asks.filter((ask) => ask.status === 'answered').length;
  const visibleAsks = asks.filter((ask) => status === 'all' || ask.status === status);
  return <div className="page-stack shelf-page shelf-asks-page">
    <div className="shelf-breadcrumb"><Link to="/shelf"><LibraryBig size={14} /> Shelf</Link><ChevronLeft size={13} className="shelf-breadcrumb__separator" /><span>Asks</span></div>
    <PageHeading eyebrow="YOUR QUESTION ARCHIVE" title="Asks" description="Questions that happened while you were reading — kept with their source book, context and eventual answer." actions={<Link className="button button--secondary" to="/shelf/graph"><Network size={14} /> Explore connections</Link>} />
    {error && <ErrorNotice message={error} onRetry={() => void load()} />}
    <section className="shelf-asks-hero"><div className="shelf-asks-hero__glyph"><MessageCircleQuestion size={25} /></div><div><span className="eyebrow">CURIOSITY LOG</span><h2>Don’t lose a good question.</h2><p>Unanswered questions stay in the inbox. Once you discover the answer — in code, another book, or an experiment — save it here and it moves into the archive.</p></div><div className="shelf-asks-hero__stats"><div><strong>{openCount}</strong><span>Open now</span></div><div><strong>{answeredCount}</strong><span>In archive</span></div></div></section>
    <div className="shelf-global-asks-toolbar"><div className="shelf-segmented">{[{ value: 'open', label: 'Open inbox' }, { value: 'answered', label: 'Answered archive' }, { value: 'all', label: 'Everything' }].map((item) => <button key={item.value} type="button" className={status === item.value ? 'is-active' : ''} onClick={() => setStatus(item.value as typeof status)}>{item.label}</button>)}</div><span>{visibleAsks.length} {visibleAsks.length === 1 ? 'question' : 'questions'}{status === 'open' ? ' waiting on an answer' : status === 'answered' ? ' saved in the archive' : ' across your library'}</span></div>
    {loading ? <div className="shelf-loading"><LoaderCircle size={20} className="spin" /> Loading your questions…</div> : <AskCards asks={visibleAsks} answerDrafts={answerDrafts} setAnswerDrafts={setAnswerDrafts} saveAnswer={(id, answer) => void saveAnswer(id, answer)} busy={busy} />}
    <div className="shelf-book-bottom-nav"><Link to="/shelf"><ArrowLeft size={14} /> Back to library</Link><span>{books.filter((book) => book.available).length} books indexed</span><Link to="/shelf/graph">View book graph <ArrowUpRight size={13} /></Link></div>
  </div>;
}
