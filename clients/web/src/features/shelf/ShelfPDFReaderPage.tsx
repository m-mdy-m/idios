import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react';
import { useNavigate, useParams } from 'react-router';
import { ArrowLeft, FileWarning, Languages, LoaderCircle, Search, X } from 'lucide-react';
import { ShelfPDFReader } from '@/features/shelf/components/ShelfPDFReader';
import type { ReaderTranslationRequest } from './model/reader-translation';
import { api } from '@/lib/api';
import type { ShelfAnnotation, Note, ShelfBook } from '@/types/idios';
import { CORE_RELIABILITY_CONTRACT, CONTEXTUAL_TRANSLATION_SKILL, READING_SKILLS, loadSelectedReadingSkills, selectedReadingSkillInstructions } from '@/features/reading-skills';
import { newReadingSessionID, queueShelfReadingSession, syncPendingShelfReadingSessions } from '@/lib/reading-sessions';
import './ShelfPDFReaderPage.css';

type FocusTool = 'search' | 'translate' | null;
type AISettings = { model?: string };
type ActiveFocusSession = { id: string; bookID: string; startedAt: number; startPage: number; currentPage: number };
type ReaderContext = { bookID: string; pageNumber: number; pageText: string; selectedText: string };
const activeFocusStorageKey = 'idios.shelf.focus-session.v1';

const targetLanguages = [
  { value: 'fa', label: 'Persian (فارسی)' },
  { value: 'en', label: 'English' },
  { value: 'de', label: 'German (Deutsch)' },
  { value: 'ar', label: 'Arabic (العربية)' },
  { value: 'fr', label: 'French (Français)' },
  { value: 'es', label: 'Spanish (Español)' },
  { value: 'tr', label: 'Turkish (Türkçe)' },
];

function readStudyAISettings(): AISettings {
  try {
    const raw = localStorage.getItem('idios.shelf.study-ai.v2');
    if (!raw) return {};
    const settings = JSON.parse(raw) as AISettings;
    return { model: settings.model?.trim() };
  } catch { return {}; }
}

function splitRuntimeModel(modelRef: string): { providerID: string; modelID: string } | null {
  const separator = modelRef.indexOf('/');
  if (separator < 1 || separator === modelRef.length - 1) return null;
  return { providerID: modelRef.slice(0, separator), modelID: modelRef.slice(separator + 1) };
}


export function ShelfPDFReaderPage() {
  const { slug = '' } = useParams();
  const navigate = useNavigate();
  const [book, setBook] = useState<ShelfBook | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [focused, setFocused] = useState(false);
  const [elapsed, setElapsed] = useState(0);
  const [tool, setTool] = useState<FocusTool>(null);
  const [toolText, setToolText] = useState('');
  const [readerContext, setReaderContext] = useState<ReaderContext | null>(null);
  const [toolResult, setToolResult] = useState('');
  const [searchURL, setSearchURL] = useState('');
  const [toolError, setToolError] = useState('');
  const [toolBusy, setToolBusy] = useState(false);
  const [targetLanguage, setTargetLanguage] = useState('fa');
  const [sessionStartedAt, setSessionStartedAt] = useState<number | null>(null);
  const [sessionID, setSessionID] = useState<string | null>(null);
  const [sessionStartPage, setSessionStartPage] = useState(1);
  const [currentPage, setCurrentPage] = useState(1);
  const [savingSession, setSavingSession] = useState(false);
  const [focusNotice, setFocusNotice] = useState('');
  const endSessionInFlight = useRef(false);
  const historyGuardActive = useRef(false);
  const focusURL = useRef('');
  const pageRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!focused || sessionStartedAt === null) return;
    const tick = () => setElapsed(Math.max(0, Math.floor((Date.now() - sessionStartedAt) / 1000)));
    tick();
    const id = window.setInterval(tick, 1000);
    return () => window.clearInterval(id);
  }, [focused, sessionStartedAt]);

  // Refreshing the same tab should not silently discard an active session.
  useEffect(() => {
    if (!book || focused) return;
    try {
      const raw = sessionStorage.getItem(activeFocusStorageKey);
      if (!raw) return;
      const active = JSON.parse(raw) as Partial<ActiveFocusSession>;
      if (active.bookID !== book.id || typeof active.startedAt !== 'number' || !Number.isFinite(active.startedAt) || active.startedAt > Date.now() || Date.now() - active.startedAt > 24 * 60 * 60 * 1000) return;
      const startPage = Math.max(1, Math.floor(Number(active.startPage) || book.current_page || 1));
      const recoveredSessionID = typeof active.id === 'string' && /^[a-zA-Z0-9_-]{1,128}$/.test(active.id) ? active.id : newReadingSessionID();
      const recoveredPage = Math.max(1, Math.floor(Number(active.currentPage) || book.current_page || startPage));
      setSessionID(recoveredSessionID);
      setSessionStartedAt(active.startedAt);
      setSessionStartPage(startPage);
      setCurrentPage(recoveredPage);
      setElapsed(Math.floor((Date.now() - active.startedAt) / 1000));
      setFocusNotice('Your previous focus session was restored. End session when you are ready to stop.');
      setFocused(true);
    } catch { /* A malformed recovery record must never prevent reading. */ }
  }, [book, focused]);

  useEffect(() => {
    if (!focused || !book || sessionStartedAt === null || !sessionID) return;
    try {
      sessionStorage.setItem(activeFocusStorageKey, JSON.stringify({
        id: sessionID, bookID: book.id, startedAt: sessionStartedAt,
        startPage: sessionStartPage, currentPage: Math.max(1, currentPage || sessionStartPage),
      } satisfies ActiveFocusSession));
    } catch { /* Focus remains usable without tab-session storage. */ }
  }, [book, currentPage, focused, sessionID, sessionStartPage, sessionStartedAt]);

  const enterFocus = useCallback(() => {
    if (!book || focused) return;
    const now = Date.now();
    const page = Math.max(1, book.current_page || currentPage || 1);
    const id = newReadingSessionID();
    setSessionID(id);
    setSessionStartedAt(now);
    setSessionStartPage(page);
    try { sessionStorage.setItem(activeFocusStorageKey, JSON.stringify({ id, bookID: book.id, startedAt: now, startPage: page, currentPage: page } satisfies ActiveFocusSession)); } catch { /* Focus still works when browser storage is disabled. */ }
    setCurrentPage(page);
    setElapsed(0);
    const root = pageRef.current;
    if (root && document.fullscreenElement !== root && typeof root.requestFullscreen === 'function') {
      void root.requestFullscreen().catch(() => {
        setFocusNotice('Browser fullscreen was unavailable. Focus Mode is still active in the full reading window.');
      });
    }
    setTool(null);
    setToolText('');
    setToolResult('');
    setSearchURL('');
    setToolError('');
    setFocusNotice('');
    endSessionInFlight.current = false;
    setFocused(true);
  }, [book, currentPage, focused]);

  const exitFocus = useCallback(async () => {
    if (endSessionInFlight.current || !book) return;
    endSessionInFlight.current = true;
    setSavingSession(true);
    const endedAt = Date.now();
    const startedAt = sessionStartedAt;
    const seconds = startedAt === null ? 0 : Math.max(0, Math.floor((endedAt - startedAt) / 1000));

    if (startedAt !== null && seconds >= 5) {
      const session = {
        id: sessionID || newReadingSessionID(),
        book_id: book.id,
        started_at: new Date(startedAt).toISOString(),
        ended_at: new Date(endedAt).toISOString(),
        start_page: sessionStartPage,
        end_page: Math.max(1, currentPage || book.current_page || 1),
      };
      try {
        await api.createShelfReadingSession(session);
      } catch {
        const queued = queueShelfReadingSession(session);
        if (!queued) {
          const warning = 'IDIOS could not save this session, and browser storage is unavailable. Your timer is still running; reconnect to the server and press End session again.';
          setFocusNotice(warning);
          setSavingSession(false);
          endSessionInFlight.current = false;
          return;
        }
      }
    }

    if (document.fullscreenElement === pageRef.current && typeof document.exitFullscreen === 'function') {
      try { await document.exitFullscreen(); } catch { /* The session can still end if browser fullscreen is unavailable. */ }
    }
    try { sessionStorage.removeItem(activeFocusStorageKey); } catch { /* Optional recovery storage. */ }
    setFocused(false);
    setSessionStartedAt(null);
    setSessionID(null);
    setTool(null);
    setToolText('');
    setToolResult('');
    setToolError('');
    setSavingSession(false);
    endSessionInFlight.current = false;
  }, [book, currentPage, sessionID, sessionStartPage, sessionStartedAt]);

  // Focus is deliberately opt-in; closing a tab or following an in-app link should not silently
  // terminate the timer. Only the explicit End session action commits a completed session.
  useEffect(() => {
    if (!focused) {
      if (historyGuardActive.current) {
        historyGuardActive.current = false;
        window.history.back(); // Remove the same-URL guard entry added on focus entry.
      }
      return;
    }
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    const guardInAppLinks = (event: MouseEvent) => {
      const target = event.target instanceof Element ? event.target : null;
      const link = target?.closest('a[href]') as HTMLAnchorElement | null;
      if (!link || link.target === '_blank' || link.hasAttribute('download')) return;
      const href = link.getAttribute('href') ?? '';
      if (href.startsWith('#') || href.startsWith('javascript:')) return;
      event.preventDefault();
      event.stopPropagation();
      setFocusNotice('Your reading session is still active. Use End session before leaving Focus Mode.');
    };
    if (!historyGuardActive.current) {
      focusURL.current = window.location.href;
      window.history.pushState({ ...(window.history.state ?? {}), idiosFocusGuard: true }, '', focusURL.current);
      historyGuardActive.current = true;
    }
    const guardBrowserBack = () => {
      if (!historyGuardActive.current) return;
      window.history.pushState({ ...(window.history.state ?? {}), idiosFocusGuard: true }, '', focusURL.current || window.location.href);
      setFocusNotice('Focus Mode is still active. End the session before leaving this page.');
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        // Escape closes tool panels only. It never ends the reading session.
        if (tool) { event.preventDefault(); event.stopPropagation(); setTool(null); }
      }
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault(); setTool('search'); setToolResult(''); setSearchURL(''); setToolError('');
      }
      if ((event.ctrlKey || event.metaKey) && event.shiftKey && event.key.toLowerCase() === 't') {
        event.preventDefault(); window.dispatchEvent(new CustomEvent('idios:shelf-open-translation'));
      }
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'j') {
        event.preventDefault();
        window.dispatchEvent(new CustomEvent('idios:shelf-open-mimi', { detail: { bookID: book?.id, prompt: `Help me understand page ${currentPage} of ${book?.title}. Ask me a question if you need more context.` } }));
      }
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    document.addEventListener('click', guardInAppLinks, true);
    window.addEventListener('popstate', guardBrowserBack, true);
    window.addEventListener('keydown', onKeyDown, true);
    document.body.classList.add('shelf-focus-active');
    return () => {
      window.removeEventListener('beforeunload', onBeforeUnload);
      document.removeEventListener('click', guardInAppLinks, true);
      window.removeEventListener('popstate', guardBrowserBack, true);
      window.removeEventListener('keydown', onKeyDown, true);
      document.body.classList.remove('shelf-focus-active');
    };
  }, [book?.id, book?.title, currentPage, focused, tool]);

  useEffect(() => {
    const onReaderSelection = (event: Event) => {
      const detail = (event as CustomEvent<{ bookID?: string; text?: string; pageNumber?: number }>).detail;
      if (!detail || detail.bookID !== book?.id || tool !== 'translate' || !detail.text?.trim()) return;
      setToolText(detail.text.slice(0, 8000));
    };
    window.addEventListener('idios:shelf-reader-selection', onReaderSelection);
    return () => window.removeEventListener('idios:shelf-reader-selection', onReaderSelection);
  }, [book?.id, tool]);

  useEffect(() => {
    const onReaderContext = (event: Event) => {
      const detail = (event as CustomEvent<ReaderContext>).detail;
      if (!detail || detail.bookID !== book?.id) return;
      setReaderContext(detail);
    };
    window.addEventListener('idios:shelf-reader-context', onReaderContext);
    return () => window.removeEventListener('idios:shelf-reader-context', onReaderContext);
  }, [book?.id]);

  useEffect(() => {
    let active = true;
    const loadBook = async () => {
      setLoading(true);
      setError('');
      try {
        await syncPendingShelfReadingSessions();
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
        if (!resolved.pdf_file) throw new Error('This book does not have a PDF file.');
        if (active) { setBook(resolved); setCurrentPage(Math.max(1, resolved.current_page || 1)); }
      } catch (cause) {
        if (active) setError(cause instanceof Error ? cause.message : 'Could not open this book.');
      } finally {
        if (active) setLoading(false);
      }
    };
    void loadBook();
    return () => { active = false; };
  }, [slug]);

  const closeReader = useCallback(() => {
    if (focused) return;
    navigate(`/shelf/${encodeURIComponent(book?.id || slug)}`);
  }, [book?.id, focused, navigate, slug]);

  const syncProgress = useCallback((page: number, total: number) => {
    const safePage = Math.max(1, Math.floor(page || 1));
    const safeTotal = Math.max(0, Math.floor(total || 0));
    setCurrentPage(safePage);
    setBook((current) => current ? { ...current, reading_status: 'reading', current_page: safePage, total_pages: safeTotal } : current);
    if (book) {
      void api.setShelfBookProgress(book.id, { status: 'reading', current_page: safePage, total_pages: safeTotal }).catch(() => undefined);
    }
  }, [book]);

  const runTool = useCallback(async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const text = toolText.trim();
    if (!text || !tool) return;
    setToolError('');
    setToolResult('');
    if (tool === 'search') {
      setSearchURL(`https://www.google.com/search?q=${encodeURIComponent(text)}`);
      setToolResult('Your search is ready. Open the results below; the book and focus timer will stay here.');
      return;
    }

    // The page can render before the book request resolves; callbacks do not
    // inherit the component's render-time null check, so guard this path explicitly.
    const currentBook = book;
    if (!currentBook) {
      setToolError('Book details are still loading. Please try again in a moment.');
      return;
    }

    const settings = readStudyAISettings();
    const selectedModel = settings.model ? splitRuntimeModel(settings.model) : null;
    if (!selectedModel) {
      setToolError('Choose a model in AI Runtime settings before translating.');
      return;
    }
    setToolBusy(true);
    try {
      const target = targetLanguages.find((language) => language.value === targetLanguage)?.label ?? targetLanguage;
      const selectedSkills = loadSelectedReadingSkills();
      const translationRelevantSkills = READING_SKILLS.filter((skill) => selectedSkills.includes(skill.id) && (skill.category === 'subject' || skill.id === 'english-learning')).map((skill) => skill.id);
      const skillInstructions = selectedReadingSkillInstructions(translationRelevantSkills);
      const [annotationsResult, notesResult] = await Promise.allSettled([
        api.shelfAnnotations(currentBook.id),
        api.shelfBookNotes(currentBook.id),
      ]);
      const annotations: ShelfAnnotation[] = annotationsResult.status === 'fulfilled' ? annotationsResult.value : [];
      const notes: Note[] = notesResult.status === 'fulfilled' ? notesResult.value : [];
      const bookContext = [
        `Book title: ${currentBook.title}`,
        currentBook.summary?.trim() ? `Book summary / description:\n${currentBook.summary.trim().slice(0, 4500)}` : '',
        currentBook.tags?.length ? `Book subject tags: ${currentBook.tags.join(', ')}` : '',
        currentBook.repository_name ? `Related repository / technical context: ${currentBook.repository_name}` : '',
        `Current page: ${readerContext?.pageNumber ?? currentPage}`,
        readerContext?.pageText?.trim() ? `Current page text for context (not all of this text needs translating):\n${readerContext.pageText.trim().slice(0, 10000)}` : '',
        annotations.length ? `Reader's saved highlights and margin notes (use only to disambiguate terms; never translate these instead of the supplied passage):\n${annotations.slice(-12).map((item) => `Page ${item.page}: “${item.quote.slice(0, 280)}”${item.note?.trim() ? ` — ${item.note.trim().slice(0, 280)}` : ''}`).join('\n').slice(0, 4500)}` : '',
        notes.length ? `Reader's saved book notes (fallible supporting context):\n${notes.slice(-8).map((item) => `- ${item.title}: ${item.content.slice(0, 400)}`).join('\n').slice(0, 2500)}` : '',
      ].filter(Boolean).join('\n\n');
      const response = await api.aiChat({
        provider_id: selectedModel.providerID,
        model: selectedModel.modelID,
        task: 'translation',
        reasoning_effort: 'low',
        messages: [
          { role: 'system', content: `${CORE_RELIABILITY_CONTRACT}\n\n${CONTEXTUAL_TRANSLATION_SKILL}\n\n${skillInstructions}\n\n## Task for this request\nTranslate the source passage into ${target}. The source passage supplied in the user message is the only text to translate. Use book/page context to interpret terminology and maintain consistency; never insert contextual facts into the translation. Preserve structure, formulas, identifiers, code, citations, numbers, negation, uncertainty, and the author's tone. Do not summarize or replace translation with explanation. Return the full translation first. Add a short translator note only if a genuine ambiguity materially affects meaning. If context is insufficient, choose the most likely meaning without pretending certainty.` },
          { role: 'user', content: `## Book and reading context\n${bookContext || 'No additional book context is available.'}\n\n## Exact passage to translate\n${text}` },
        ],
      });
      const translated = response.content?.trim();
      if (!translated) throw new Error('The selected model returned an empty translation. Check provider health and try again.');
      setToolResult(translated);
    } catch (cause) {
      setToolError(cause instanceof Error ? cause.message : 'Translation failed. Check your AI provider and retry.');
    } finally {
      setToolBusy(false);
    }
  }, [book, currentPage, readerContext, targetLanguage, tool, toolText]);

  const translateReaderText = useCallback(async (request: ReaderTranslationRequest): Promise<string> => {
    const currentBook = book;
    if (!currentBook) throw new Error('Book details are still loading. Please try again.');
    const settings = readStudyAISettings();
    const selectedModel = settings.model ? splitRuntimeModel(settings.model) : null;
    if (!selectedModel) throw new Error('Choose a model in AI Runtime settings before translating.');
    const language = targetLanguages.find((item) => item.value === request.targetLanguage)?.label ?? request.targetLanguage;
    const selectedSkills = loadSelectedReadingSkills();
    const relevantSkills = READING_SKILLS
      .filter((skill) => selectedSkills.includes(skill.id) && (skill.category === 'subject' || skill.id === 'english-learning'))
      .map((skill) => skill.id);
    // English-learning guidance is deliberately active for this reader feature, even when
    // the user has not manually enabled the optional skill in settings.
    const skillInstructions = selectedReadingSkillInstructions([...new Set([...relevantSkills, 'english-learning'])]);
    const [annotationsResult, notesResult] = await Promise.allSettled([
      api.shelfAnnotations(currentBook.id),
      api.shelfBookNotes(currentBook.id),
    ]);
    const annotations: ShelfAnnotation[] = annotationsResult.status === 'fulfilled' ? annotationsResult.value : [];
    const notes: Note[] = notesResult.status === 'fulfilled' ? notesResult.value : [];
    const nearbyAnnotations = annotations
      .filter((item) => Math.abs(item.page - request.pageNumber) <= 4)
      .slice(-20);
    const bookContext = [
      `Book title: ${currentBook.title}`,
      currentBook.summary?.trim() ? `Book description: ${currentBook.summary.trim().slice(0, 4000)}` : '',
      currentBook.tags?.length ? `Book subject tags: ${currentBook.tags.join(', ')}` : '',
      currentBook.repository_name ? `Related technical context: ${currentBook.repository_name}` : '',
      `Requested page: PDF page ${request.pageNumber}`,
      request.documentContext.trim() ? `Neighbouring pages actually extracted from this same book:\n${request.documentContext.slice(0, 22000)}` : '',
      nearbyAnnotations.length ? `Reader's saved highlights and notes near this page (fallible context only):\n${nearbyAnnotations.map((item) => `- Page ${item.page}: “${item.quote.slice(0, 260)}”${item.note?.trim() ? ` — ${item.note.trim().slice(0, 260)}` : ''}`).join('\n').slice(0, 5000)` : '',
      notes.length ? `Reader's saved book notes (supporting context only):\n${notes.slice(-12).map((item) => `- ${item.title}: ${item.content.slice(0, 420)}`).join('\n').slice(0, 5000)}` : '',
    ].filter(Boolean).join('\n\n');

    const kindInstructions = request.kind === 'page'
      ? 'Translate the entire exact current-page source, preserving paragraph boundaries. Split segments by sentence or original paragraph so the source and translation align closely. Extract only useful words, grammar, and academic expressions actually present on this page.'
      : request.kind === 'selection'
        ? 'Translate only the exact selected passage. Keep all selected sentences and clauses; do not silently expand to the full page. Align segments to the selected passage.'
        : 'Explain and translate only this exact word in its sentence and book context. Give its part of speech, contextual sense, a concise explanation in the reader’s language, and one original natural example. Do not return a made-up full-page translation.';
    const systemPrompt = [
      CORE_RELIABILITY_CONTRACT,
      CONTEXTUAL_TRANSLATION_SKILL,
      skillInstructions,
      '## Reader translation task',
      `Target language: ${language}.`,
      `Passage type: ${request.kind}.`,
      kindInstructions,
      'The exact source text in the user message is the only material to translate. Use prior/current/following pages, book metadata, saved highlights and notes only to resolve references, domain sense and consistent terminology; never insert contextual facts into the translated passage.',
      'Use the author’s subject, tone and terminology. Preserve negation, tense, modality, conditions, quantifiers, comparisons, numbers, units, citations, code, identifiers, formulas and list structure. Do not summarize. Do not omit difficult sentences.',
      'For the page task, create a source/translation segment for every sentence or original paragraph in order. The source field must be copied exactly and contiguously from the supplied exact text. Do not invent source text.',
      'Vocabulary terms must be taken from the supplied page/selection, with the meaning of the word in this book—not a generic dictionary dump. Explain part of speech and one natural example where useful.',
      'Grammar notes must explain real structures appearing in this source, in simple learner-friendly language. IELTS items must use expressions actually present in the source and explain a legitimate academic use; never invent IELTS band scores or guarantee a score. Keep teaching brief and relevant.',
      'Before responding, compare every translated segment against the source. Make sure every proposition is translated once, with no additions or omissions. If OCR/text extraction appears incomplete or materially ambiguous, mention it in translatorNote instead of guessing.',
      'Return ONLY a valid JSON object, with no Markdown fences, in this exact shape:',
      '{"translation":"complete faithful translation or contextual word explanation","segments":[{"source":"exact source sentence or paragraph","translation":"corresponding target text"}],"terms":[{"source":"source word or phrase","translation":"target equivalent","meaning":"contextual meaning explained simply in the reader language","partOfSpeech":"grammatical role","example":"one original natural example"}],"grammar":[{"pattern":"exact grammar pattern from source","explanation":"simple explanation in reader language","example":"one original example"}],"ielts":[{"expression":"expression actually present in source","academicUse":"accurate academic/IELTS use","example":"one original example"}],"translatorNote":"only a material ambiguity or extraction warning, otherwise an empty string"}',
      'For page requests return no more than 12 useful terms, 5 grammar notes and 5 IELTS items. For selected passages return at most 8 terms, 3 grammar notes and 3 IELTS items. For a single word return one term and no more than 2 concise notes in the other arrays.',
    ].join('\n\n');
    const response = await api.aiChat({
      provider_id: selectedModel.providerID,
      model: selectedModel.modelID,
      task: 'translation',
      reasoning_effort: 'low',
      messages: [
        { role: 'system', content: systemPrompt },
        { role: 'user', content: `## Book and reading context\n${bookContext || 'No additional book context is available.'}\n\n## Exact source text — translate only this text\n${request.text}` },
      ],
    });
    const content = response.content?.trim();
    if (!content) throw new Error('The selected model returned an empty response. Check provider health or choose another model.');
    return content;
  }, [book]);

  if (loading) return <main className="shelf-pdf-reader-page shelf-pdf-reader-page--state"><LoaderCircle size={22} className="spin" /><span>Opening your reading room…</span></main>;
  if (!book) return <main className="shelf-pdf-reader-page shelf-pdf-reader-page--state"><section className="shelf-pdf-reader-page__error"><FileWarning size={28} /><h1>Could not open this book</h1><p>{error || 'The book could not be loaded.'}</p><button type="button" onClick={() => navigate(`/shelf/${encodeURIComponent(slug)}`)}><ArrowLeft size={15} /> Back to book</button></section></main>;

  return <main ref={pageRef} className={`shelf-pdf-reader-page shelf-v3-reader-page${focused ? ' shelf-pdf-reader-page--focused shelf-pdf-reader-page--focus-session' : ''}`} aria-label={`PDF reader for ${book.title}`}>
    {focused && focusNotice && <div className="shelf-focus-notice" role="status">{focusNotice}</div>}

    {tool && (focused || tool === 'search') && <section className={`shelf-focus-tool shelf-focus-tool--${tool}`} aria-label={tool === 'search' ? 'Quick web search' : 'Translate selection'}>
      <div className="shelf-focus-tool__heading"><div><span className="shelf-focus-tool__eyebrow">{tool === 'search' ? 'LOOK SOMETHING UP' : 'TRANSLATE WITHOUT LEAVING THE BOOK'}</span><strong>{tool === 'search' ? 'Quick search' : 'Quick translation'}</strong></div><button type="button" onClick={() => setTool(null)} aria-label="Close tool panel"><X size={15}/></button></div>
      <form onSubmit={(event) => void runTool(event)}>
        {tool === 'translate' && <label className="shelf-focus-tool__language">Translate into<select value={targetLanguage} onChange={(event) => setTargetLanguage(event.target.value)}>{targetLanguages.map((language) => <option key={language.value} value={language.value}>{language.label}</option>)}</select></label>}
        <textarea value={toolText} onChange={(event) => setToolText(event.target.value)} placeholder={tool === 'search' ? 'Search any topic, concept, or question…' : 'Select text in the PDF or paste a passage here…' } rows={3} autoFocus aria-label={tool === 'search' ? 'Search query' : 'Text to translate'} />
        <div className="shelf-focus-tool__actions"><span>{tool === 'search' ? 'Open results from this panel; your book stays in place.' : 'Uses this book, the current page, saved notes and subject-specific terminology.'}</span><button type="submit" disabled={!toolText.trim() || toolBusy}>{toolBusy ? <LoaderCircle size={14} className="spin"/> : tool === 'search' ? <Search size={14}/> : <Languages size={14}/>} {toolBusy ? 'Working…' : tool === 'search' ? 'Search web' : 'Translate'}</button></div>
      </form>
      {toolError && <p className="shelf-focus-tool__error" role="alert">{toolError}{tool === 'translate' && <button type="button" onClick={() => navigate('/settings/ai')}>Open AI Runtime settings</button>}</p>}
      {toolResult && <div className={`shelf-focus-tool__result${tool === 'translate' ? ' shelf-focus-tool__result--translation' : ''}`} aria-live="polite"><div className="shelf-focus-tool__result-heading"><strong>{tool === 'search' ? 'Search status' : 'Translation'}</strong>{tool === 'translate' && <button type="button" onClick={() => void navigator.clipboard?.writeText(toolResult)} title="Copy translation">Copy</button>}</div><p>{toolResult}</p>{tool === 'search' && searchURL && <a className="shelf-focus-search-link" href={searchURL} target="_blank" rel="noopener noreferrer"><Search size={14}/> Open search results <span aria-hidden="true">↗</span></a>}</div>}
    </section>}

    <ShelfPDFReader
      bookID={book.id}
      bookTitle={book.title}
      initialPage={Math.max(1, currentPage || book.current_page || 1)}
      pdfURL={api.shelfPDFURL(book.id)}
      onClose={closeReader}
      onProgress={syncProgress}
      focusMode={focused}
      focusElapsedSeconds={elapsed}
      focusBusy={savingSession}
      onToggleFocus={() => { if (focused) void exitFocus(); else enterFocus(); }}
      onOpenFocusTool={(nextTool, initialText) => { setTool(nextTool); setToolResult(''); setSearchURL(''); setToolError(''); setToolText(initialText?.trim() || ''); }}
      onTranslateText={translateReaderText}
    />
  </main>;
}
