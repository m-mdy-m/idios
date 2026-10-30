import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router';
import { FileWarning, LoaderCircle } from 'lucide-react';
import { ShelfStudyAssistant } from '@/features/shelf/components/ShelfStudyAssistant';
import { api } from '@/lib/api';
import type { ShelfBook } from '@/types/idios';
import './ShelfAIChatPage.css';

type ReadPage = { page: number; text: string; visitedAt?: string };
type Handoff = { pageNumber?: number; pageText?: string; selectedText?: string; hoveredText?: string; prompt?: string };

function readHandoff(bookID: string): Handoff {
  try {
    const value = JSON.parse(sessionStorage.getItem(`idios.shelf.ai-handoff.v1.${bookID}`) ?? '{}') as Handoff;
    return {
      pageNumber: Number.isFinite(value.pageNumber) && Number(value.pageNumber) > 0 ? Number(value.pageNumber) : undefined,
      pageText: typeof value.pageText === 'string' ? value.pageText.slice(0, 16000) : '',
      selectedText: typeof value.selectedText === 'string' ? value.selectedText : '',
      hoveredText: typeof value.hoveredText === 'string' ? value.hoveredText : '',
      prompt: typeof value.prompt === 'string' ? value.prompt : '',
    };
  } catch { return {}; }
}

function readHistory(bookID: string): ReadPage[] {
  try {
    const value = JSON.parse(localStorage.getItem(`idios.shelf.read-history.v1.${bookID}`) ?? '[]') as ReadPage[];
    return Array.isArray(value) ? value.filter((item) => Number.isFinite(item.page) && typeof item.text === 'string') : [];
  } catch { return []; }
}

export function ShelfAIChatPage({ modelsOnly = false }: { modelsOnly?: boolean }) {
  const { slug = '' } = useParams();
  const navigate = useNavigate();
  const [book, setBook] = useState<ShelfBook | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const handoff = useMemo(() => readHandoff(slug), [slug, modelsOnly]);
  const [history, setHistory] = useState<ReadPage[]>(() => readHistory(slug));

  // A prompt handed off from the reader should prefill the chat once, not reappear
  // every time the reader visits Model Settings and returns to the discussion.
  useEffect(() => {
    try {
      const key = `idios.shelf.ai-handoff.v1.${slug}`;
      const stored = JSON.parse(sessionStorage.getItem(key) ?? '{}') as Handoff;
      if (stored.prompt) {
        stored.prompt = '';
        sessionStorage.setItem(key, JSON.stringify(stored));
      }
    } catch { /* The in-memory handoff still seeds this first page visit. */ }
  }, [slug]);

  useEffect(() => {
    let active = true;
    const loadBook = async () => {
      setLoading(true);
      setError('');
      try {
        let resolved: ShelfBook | undefined;
        try {
          resolved = await api.shelfBook(slug);
        } catch {
          let shelf = await api.shelf();
          resolved = shelf.books.find((item) => item.id === slug);
          if (!resolved) {
            shelf = await api.scanShelf();
            resolved = shelf.books.find((item) => item.id === slug);
          }
        }
        if (!resolved) throw new Error('This book is not available in the connected Shelf library.');
        if (active) {
          setBook(resolved);
          setHistory(readHistory(resolved.id));
        }
      } catch (cause) {
        if (active) setError(cause instanceof Error ? cause.message : 'Could not open the AI reading room.');
      } finally {
        if (active) setLoading(false);
      }
    };
    void loadBook();
    return () => { active = false; };
  }, [slug]);

  const pageNumber = handoff.pageNumber || history[history.length - 1]?.page || book?.current_page || 1;
  const pageText = handoff.pageText || (handoff.pageNumber ? history.find((item) => item.page === pageNumber)?.text : history[history.length - 1]?.text) || '';
  const selectedText = handoff.selectedText || '';
  const hoveredText = handoff.hoveredText || '';
  const backToReader = () => navigate(`/shelf/${encodeURIComponent(book?.id || slug)}/read`);
  const backToChat = () => navigate(`/shelf/${encodeURIComponent(book?.id || slug)}/ai`);
  const openModels = () => navigate(`/shelf/${encodeURIComponent(book?.id || slug)}/ai/models`);

  if (loading) return <main className="shelf-ai-route-state"><LoaderCircle size={23} className="spin" /><span>Opening your book discussion…</span></main>;
  if (!book) return <main className="shelf-ai-route-state"><section><FileWarning size={28} /><h1>Could not open this book</h1><p>{error || 'The selected book is not available.'}</p><button type="button" onClick={() => navigate(`/shelf/${encodeURIComponent(slug)}`)}>Back to book</button></section></main>;

  return <ShelfStudyAssistant
    key={`${book.id}:${modelsOnly ? 'models' : 'chat'}`}
    bookID={book.id}
    bookTitle={book.title}
    pageNumber={pageNumber}
    pageText={pageText}
    selectedText={selectedText}
    hoveredText={hoveredText}
    displayMode={modelsOnly ? 'models' : 'chat'}
    initialPrompt={handoff.prompt || ''}
    onBackToReading={backToReader}
    onOpenModels={openModels}
    onBackToChat={backToChat}
  />;
}
