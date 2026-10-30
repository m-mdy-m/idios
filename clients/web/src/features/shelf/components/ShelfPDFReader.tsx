import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent as ReactMouseEvent } from 'react';
import {
  ArrowLeft, Bookmark, Check, ChevronLeft, ChevronRight, ExternalLink, FileText, Focus, Highlighter, Languages,
  LoaderCircle, Maximize2, MessageCircle, MessagesSquare, Search, Sparkles, StickyNote, Trash2, X, ZoomIn, ZoomOut,
} from 'lucide-react';
import { getDocument, GlobalWorkerOptions, TextLayer, type PDFDocumentProxy, type PDFPageProxy, type PageViewport } from 'pdfjs-dist/legacy/build/pdf.mjs';
import pdfWorkerUrl from 'pdfjs-dist/legacy/build/pdf.worker.min.mjs?url';
import 'pdfjs-dist/web/pdf_viewer.css';
import { api } from '@/lib/api';
import type { ShelfAnnotation, ShelfAnnotationRect } from '@/types/idios';
import { ShelfStudyAssistant } from './ShelfStudyAssistant';
import type { ApplyBookHighlightsResult, RequestedBookHighlight } from '../model/study-types';
import { mergeReaderTranslations, parseReaderTranslation, splitReaderTranslationText, normalizeTranslationMatch, type ReaderTranslationCompletion, type ReaderTranslationData, type ReaderTranslationKind, type ReaderTranslationModelOption, type ReaderTranslationRequest } from '../model/reader-translation';
import { ShelfTranslationPanel, type ShelfTranslationTab } from './ShelfTranslationPanel';
import { ShelfAgentDiscussion } from './ShelfAgentDiscussion';
import './ShelfPDFReader.css';

GlobalWorkerOptions.workerSrc = pdfWorkerUrl;
// Runtime assets (image decoders, fonts, CMaps) copied to /pdfjs/ by scripts/copy-pdfjs-assets.mjs.
const pdfAssetBase = '/pdfjs/';

type HighlightColor = 'yellow' | 'green' | 'blue' | 'pink' | 'purple';
const highlightColors: Array<{ value: HighlightColor; label: string }> = [
  { value: 'yellow', label: 'Yellow' }, { value: 'green', label: 'Green' },
  { value: 'blue', label: 'Blue' }, { value: 'pink', label: 'Pink' }, { value: 'purple', label: 'Purple' },
];

type PendingSelection = { quote: string; rectangles: ShelfAnnotationRect[] };
type CompanionMark = { quote: string; note: string; kind: 'important' | 'unclear' };
type CompanionPin = CompanionMark & { left: number; top: number };

type PDFRequestEntry = {
  controller: AbortController;
  promise: Promise<Uint8Array>;
  consumers: number;
  settled: boolean;
  abortTimer: ReturnType<typeof setTimeout> | null;
};
// React StrictMode re-runs effects in development. Share an in-flight request across the
// remount, then cancel it only if no Reader is using it after a short handoff window.
const inFlightPDFRequests = new Map<string, PDFRequestEntry>();

function describeResponse(response: Response): string {
  const names: string[] = [];
  response.headers.forEach((_value, name) => { names.push(name); });
  return `[status ${response.status}, type ${response.type}, url ${response.url || '?'}, content-type ${response.headers.get('content-type') || 'none'}, headers: ${names.join(', ') || 'none'}]`;
}

function hasPDFSignature(bytes: Uint8Array): boolean {
  return bytes.byteLength >= 8 && new TextDecoder('ascii').decode(bytes.subarray(0, Math.min(1024, bytes.byteLength))).includes('%PDF-');
}

// Primary transport: a plain binary GET of the PDF.
async function downloadPDFBinary(url: string, signal: AbortSignal): Promise<Uint8Array> {
  const response = await fetch(url, { method: 'GET', cache: 'no-store', credentials: 'same-origin', headers: { Accept: 'application/pdf' }, signal });
  if (response.status !== 200) {
    let detail = '';
    try { const payload = await response.clone().json() as { error?: string }; detail = payload.error || ''; } catch { /* not JSON */ }
    throw new Error(`${detail || `HTTP ${response.status}, expected 200 with PDF bytes`} ${describeResponse(response)}`);
  }
  const bytes = new Uint8Array(await response.arrayBuffer());
  if (!hasPDFSignature(bytes)) throw new Error(`The response was not a PDF ${describeResponse(response)}`);
  return bytes;
}

// Fallback transport: the same PDF as base64 inside a normal JSON API response. Browser
// extensions, download managers and local proxies that swallow PDF-looking responses (an
// empty HTTP 204) do not touch ordinary JSON.
async function downloadPDFAsJSON(url: string, signal: AbortSignal): Promise<Uint8Array> {
  const response = await fetch(url.replace(/\/pdf$/, '/pdf-data'), { method: 'GET', cache: 'no-store', credentials: 'same-origin', headers: { Accept: 'application/json' }, signal });
  if (response.status !== 200) {
    let detail = '';
    try { const payload = await response.clone().json() as { error?: string }; detail = payload.error || ''; } catch { /* not JSON */ }
    throw new Error(`${detail || `HTTP ${response.status}, expected 200 with JSON`} ${describeResponse(response)}`);
  }
  const payload = await response.json() as { data?: string };
  if (!payload.data) throw new Error(`The JSON response contained no PDF data ${describeResponse(response)}`);
  const binary = atob(payload.data);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  if (!hasPDFSignature(bytes)) throw new Error('The decoded JSON data was not a PDF.');
  return bytes;
}

function createPDFRequest(url: string): PDFRequestEntry {
  const controller = new AbortController();
  const entry: PDFRequestEntry = {
    controller,
    promise: Promise.resolve(new Uint8Array()),
    consumers: 0,
    settled: false,
    abortTimer: null,
  };
  entry.promise = (async () => {
    let first: string;
    try {
      return await downloadPDFBinary(url, controller.signal);
    } catch (cause) {
      if (controller.signal.aborted) throw cause;
      first = cause instanceof Error ? cause.message : String(cause);
    }
    try {
      return await downloadPDFAsJSON(url, controller.signal);
    } catch (cause) {
      if (controller.signal.aborted) throw cause;
      const second = cause instanceof Error ? cause.message : String(cause);
      throw new Error(`Binary download failed: ${first}. JSON fallback failed: ${second}`, { cause });
    }
  })();
  void entry.promise.then(() => { entry.settled = true; }, () => { entry.settled = true; });
  return entry;
}

function acquirePDFBytes(url: string): { promise: Promise<Uint8Array>; release: () => void } {
  let entry = inFlightPDFRequests.get(url);
  if (!entry) {
    entry = createPDFRequest(url);
    inFlightPDFRequests.set(url, entry);
    const requestEntry = entry;
    void entry.promise.then(() => {
      if (inFlightPDFRequests.get(url) === requestEntry) inFlightPDFRequests.delete(url);
    }, () => {
      if (inFlightPDFRequests.get(url) === requestEntry) inFlightPDFRequests.delete(url);
    });
  }
  if (entry.abortTimer !== null) {
    clearTimeout(entry.abortTimer);
    entry.abortTimer = null;
  }
  entry.consumers += 1;
  let released = false;
  return {
    // PDF.js may transfer/detach the ArrayBuffer to its worker, so each consumer receives
    // its own copy even when StrictMode shares the underlying HTTP request.
    promise: entry.promise.then((bytes) => bytes.slice()),
    release: () => {
      if (released) return;
      released = true;
      entry!.consumers = Math.max(0, entry!.consumers - 1);
      if (entry!.consumers === 0 && !entry!.settled) {
        const requestEntry = entry!;
        requestEntry.abortTimer = setTimeout(() => {
          requestEntry.abortTimer = null;
          if (requestEntry.consumers === 0 && !requestEntry.settled) requestEntry.controller.abort();
        }, 180);
      }
    },
  };
}

type RenderedPage = { canvas: HTMLCanvasElement; page: PDFPageProxy; viewport: PageViewport; parseMs: number; drawMs: number };
type PageRender = { promise: Promise<RenderedPage>; cancel: () => void; readonly ready: boolean };
// Very large scanned pages are drawn at a capped pixel count; the page is still shown at its
// normal CSS size, so only the invisible oversampling is reduced.
const maxCanvasPixels = 8_000_000;

// Renders one page into an off-screen canvas. Results are cached and neighbouring pages are
// rendered ahead of time, so slow pages (large scans, JBIG2/JPEG 2000) are ready when the
// reader turns to them.
function startPageRender(doc: PDFDocumentProxy, pageNumber: number, scale: number): PageRender {
  let cancelled = false;
  let ready = false;
  let task: { cancel: () => void } | null = null;
  const promise = (async (): Promise<RenderedPage> => {
    const started = performance.now();
    const page = await doc.getPage(pageNumber);
    const parsed = performance.now();
    if (cancelled) throw new Error('Page render cancelled.');
    const viewport = page.getViewport({ scale });
    let ratio = Math.max(1, Math.min(2, window.devicePixelRatio || 1));
    const pixels = viewport.width * viewport.height * ratio * ratio;
    if (pixels > maxCanvasPixels) ratio = Math.sqrt(maxCanvasPixels / (viewport.width * viewport.height));
    const canvas = document.createElement('canvas');
    canvas.width = Math.max(1, Math.ceil(viewport.width * ratio));
    canvas.height = Math.max(1, Math.ceil(viewport.height * ratio));
    const context = canvas.getContext('2d');
    if (!context) throw new Error('Canvas rendering is unavailable in this browser.');
    const renderTask = page.render({ canvasContext: context, viewport, canvas, transform: ratio !== 1 ? [ratio, 0, 0, ratio, 0, 0] : undefined });
    task = renderTask;
    await renderTask.promise;
    ready = true;
    return { canvas, page, viewport, parseMs: parsed - started, drawMs: performance.now() - parsed };
  })();
  return { promise, cancel: () => { cancelled = true; task?.cancel(); }, get ready() { return ready; } };
}

export function ShelfPDFReader({ bookID, bookTitle, initialPage, pdfURL, onClose, onProgress, focusMode = false, focusElapsedSeconds = 0, focusBusy = false, onToggleFocus, onOpenFocusTool, onTranslateText, translationModel, translationModelOptions, translationModelsLoading, translationModelError, lastUsedTranslationModel, onTranslationModelChange }: {
  bookID: string; bookTitle: string; initialPage: number; pdfURL: string; onClose: () => void; onProgress: (page: number, total: number) => void; focusMode?: boolean; focusElapsedSeconds?: number; focusBusy?: boolean; onToggleFocus: () => void; onOpenFocusTool: (tool: 'search', initialText?: string) => void;
  onTranslateText: (request: ReaderTranslationRequest) => Promise<ReaderTranslationCompletion>;
  translationModel: string; translationModelOptions: ReaderTranslationModelOption[]; translationModelsLoading: boolean; translationModelError: string; lastUsedTranslationModel: string; onTranslationModelChange: (model: string) => void;
}) {
  const readerRef = useRef<HTMLDivElement>(null);
  const pageSheetRef = useRef<HTMLDivElement>(null);
  const textLayerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const pageTextCache = useRef<Map<number, string>>(new Map());
  const [pdf, setPdf] = useState<PDFDocumentProxy | null>(null);
  const [pageNumber, setPageNumber] = useState(Math.max(1, initialPage || 1));
  const [pageCount, setPageCount] = useState(0);
  const [scale, setScale] = useState(1.05);
  const [pageSize, setPageSize] = useState({ width: 0, height: 0 });
  const [loading, setLoading] = useState(true);
  const [rendering, setRendering] = useState(false);
  const [textLayerLoading, setTextLayerLoading] = useState(false);
  const [showRenderOverlay, setShowRenderOverlay] = useState(false);
  const [elapsedSeconds, setElapsedSeconds] = useState(0);
  const [currentPageText, setCurrentPageText] = useState('');
  // Keep the reader's internal page in sync if Focus Mode restores a same-tab session.
  // Normal page changes already match initialPage, so they do not trigger an extra turn.
  useEffect(() => {
    const nextPage = Math.max(1, Math.floor(Number(initialPage) || 1));
    setPageNumber((current) => current === nextPage ? current : nextPage);
  }, [initialPage]);
  const [pageTextWarning, setPageTextWarning] = useState('');
  const [error, setError] = useState('');
  const [annotations, setAnnotations] = useState<ShelfAnnotation[]>([]);
  const [loadingAnnotations, setLoadingAnnotations] = useState(true);
  const [pending, setPending] = useState<PendingSelection | null>(null);
  const [pendingColor, setPendingColor] = useState<HighlightColor>('yellow');
  const [pendingNote, setPendingNote] = useState('');
  const [searchText, setSearchText] = useState('');
  const [searchPages, setSearchPages] = useState<number[]>([]);
  const [searching, setSearching] = useState(false);
  const [savingAnnotation, setSavingAnnotation] = useState(false);
  const [activeAnnotationID, setActiveAnnotationID] = useState('');
  const pdfBytesRef = useRef<Uint8Array | null>(null);
  const pageRenderCache = useRef<{ pdf: PDFDocumentProxy | null; entries: Map<string, PageRender> }>({ pdf: null, entries: new Map() });
  const [renderInfo, setRenderInfo] = useState('');
  const [pdfReady, setPdfReady] = useState(false);
  const [notePopup, setNotePopup] = useState<{ id: string; x: number; y: number } | null>(null);
  const [sidePanelOpen, setSidePanelOpen] = useState(false);
  const [sidePanelMode, setSidePanelMode] = useState<'notes' | 'discussion'>('notes');
  const [translationOpen, setTranslationOpen] = useState(false);
  const [translationTab, setTranslationTab] = useState<ShelfTranslationTab>('translate');
  const [translationTargetLanguage, setTranslationTargetLanguage] = useState('fa');
  const [pageTranslation, setPageTranslation] = useState<ReaderTranslationData | null>(null);
  const [partialPageTranslation, setPartialPageTranslation] = useState<ReaderTranslationData | null>(null);
  const [selectionTranslation, setSelectionTranslation] = useState<ReaderTranslationData | null>(null);
  const [wordTranslation, setWordTranslation] = useState<ReaderTranslationData | null>(null);
  const [wordTranslationKey, setWordTranslationKey] = useState('');
  const [wordTranslationLoadingKey, setWordTranslationLoadingKey] = useState('');
  const [translationPageKey, setTranslationPageKey] = useState('');
  const [translationRevision, setTranslationRevision] = useState(0);
  const [pageTranslationBusy, setPageTranslationBusy] = useState(false);
  const [translationProgress, setTranslationProgress] = useState({ done: 0, total: 0 });
  const [selectionTranslationBusy, setSelectionTranslationBusy] = useState(false);
  const [translationError, setTranslationError] = useState('');
  const wordTranslationCache = useRef<Map<string, ReaderTranslationData>>(new Map());
  const pageTranslationCache = useRef<Map<string, ReaderTranslationData>>(new Map());
  const translationChunkCache = useRef<Map<string, ReaderTranslationData>>(new Map());
  const wordTranslationInFlight = useRef<Set<string>>(new Set());
  const translationPageInFlight = useRef('');
  const translationRequestID = useRef(0);
  const [selectedTextForAI, setSelectedTextForAI] = useState('');
  const [inlineChatOpen, setInlineChatOpen] = useState(false);
  const [inlineChatPrompt, setInlineChatPrompt] = useState('');
  const [inlineChatSettingsOpen, setInlineChatSettingsOpen] = useState(false);
  const [companionHighlights, setCompanionHighlights] = useState<{ pageNumber: number; marks: CompanionMark[] }>({ pageNumber: 0, marks: [] });
  const [companionPins, setCompanionPins] = useState<CompanionPin[]>([]);
  const [activeCompanionPin, setActiveCompanionPin] = useState<number | null>(null);
  const [hoveredTextForAI, setHoveredTextForAI] = useState('');
  const [progressMessage, setProgressMessage] = useState('');

  useEffect(() => {
    let cancelled = false;
    setLoading(true); setError(''); setElapsedSeconds(0); setPdf(null); setPageCount(0); setPageSize({ width: 0, height: 0 });
    setCurrentPageText(''); setPageTextWarning(''); setSelectedTextForAI(''); setHoveredTextForAI(''); setPending(null);
    pageTextCache.current.clear(); pdfBytesRef.current = null; setPdfReady(false);
    let task: ReturnType<typeof getDocument> | null = null;
    const pdfRequest = acquirePDFBytes(pdfURL);
    void (async () => {
      try {
        // Share the request across StrictMode's effect replay; release it on cleanup and
        // abort only if no Reader remounts to adopt it during the short handoff window.
        const data = await pdfRequest.promise;
        if (cancelled) return;
        // PDF.js may detach the buffer it receives, so keep our own copy for "Open original".
        pdfBytesRef.current = data.slice(); setPdfReady(true);
        task = getDocument({ data, wasmUrl: pdfAssetBase + 'wasm/', cMapUrl: pdfAssetBase + 'cmaps/', cMapPacked: true, standardFontDataUrl: pdfAssetBase + 'standard_fonts/', iccUrl: pdfAssetBase + 'iccs/', enableHWA: true });
        const document = await task.promise;
        if (cancelled) { await document.destroy(); return; }
        setPdf(document); setPageCount(document.numPages);
        setPageNumber((page) => Math.min(Math.max(1, page), document.numPages));
        setLoading(false);
      } catch (cause: unknown) {
        if (!cancelled) {
          setError(cause instanceof Error ? cause.message : 'The PDF could not be opened.');
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
      pdfRequest.release();
      if (task) void task.destroy();
    };
  }, [pdfURL]);

  // Debounce the overlay so cached page turns do not flash a spinner. Slow renders and text
  // extraction still show progress after a short grace period.
  useEffect(() => {
    if (!rendering && !textLayerLoading) { setShowRenderOverlay(false); return; }
    const timer = window.setTimeout(() => setShowRenderOverlay(true), 220);
    return () => window.clearTimeout(timer);
  }, [rendering, textLayerLoading, pageNumber]);

  // An elapsed-time ticker keeps slow document parsing, page rasterization and text-layer
  // preparation visibly alive without pretending PDF.js exposes an accurate render percentage.
  useEffect(() => {
    if (!loading && !rendering && !textLayerLoading) return;
    const timer = window.setInterval(() => setElapsedSeconds((seconds) => seconds + 1), 1000);
    return () => window.clearInterval(timer);
  }, [loading, rendering, textLayerLoading]);

  useEffect(() => {
    let cancelled = false;
    setLoadingAnnotations(true);
    api.shelfAnnotations(bookID).then((items) => { if (!cancelled) setAnnotations(items); })
      .catch((cause: unknown) => { if (!cancelled) setProgressMessage(cause instanceof Error ? cause.message : 'Could not load saved highlights.'); })
      .finally(() => { if (!cancelled) setLoadingAnnotations(false); });
    return () => { cancelled = true; };
  }, [bookID]);

  useEffect(() => {
    if (!pdf) return;
    let cancelled = false;
    let textLayerTask: TextLayer | null = null;
    setTextLayerLoading(false); setError(''); setPending(null); setRenderInfo(''); setElapsedSeconds(0); setPageTextWarning('');
    const canvas = canvasRef.current;
    const textLayer = textLayerRef.current;
    const sheet = pageSheetRef.current;
    if (!canvas || !textLayer || !sheet) { setRendering(false); return; }
    // Keep the previous page visible until the next raster is ready. This avoids the white
    // flash that used to look like a full refresh on every page turn. The old text layer is
    // temporarily non-interactive so it cannot be selected as if it belonged to the new page.
    textLayer.style.pointerEvents = 'none';
    setCurrentPageText(pageTextCache.current.get(pageNumber) ?? '');

    const cache = pageRenderCache.current;
    if (cache.pdf !== pdf) { cache.entries.forEach((entry) => entry.cancel()); cache.entries.clear(); cache.pdf = pdf; }
    const keyOf = (number: number) => `${scale}:${number}`;
    // Keep only the current page and its close neighbours at the current zoom.
    cache.entries.forEach((entry, key) => {
      const [entryScale, entryPage] = key.split(':');
      if (entryScale !== String(scale) || Math.abs(Number(entryPage) - pageNumber) > 2) { entry.cancel(); cache.entries.delete(key); }
    });
    const ensure = (number: number): PageRender | null => {
      if (number < 1 || number > pdf.numPages) return null;
      let entry = cache.entries.get(keyOf(number));
      if (!entry) {
        const created = startPageRender(pdf, number, scale);
        entry = created;
        cache.entries.set(keyOf(number), created);
        created.promise.catch(() => { if (cache.entries.get(keyOf(number)) === created) cache.entries.delete(keyOf(number)); });
      }
      return entry;
    };

    const wasCachedReady = cache.entries.get(keyOf(pageNumber))?.ready ?? false;
    // Only show the render state immediately for a cold page. A cached page swap should feel
    // instantaneous; slow cached/in-flight work still gets the delayed progress overlay.
    setRendering(!wasCachedReady);
    // Extract text concurrently with canvas rendering, so a heavy scan/image page doesn't force
    // an additional full parse wait before its selectable layer can appear.
    const textContentPromise = pdf.getPage(pageNumber).then((page) => page.getTextContent()).catch((cause: unknown) => {
      if (!cancelled) setPageTextWarning(cause instanceof Error ? cause.message : 'Could not extract page text.');
      return null;
    });
    const current = ensure(pageNumber);
    if (!current) { setRendering(false); setTextLayerLoading(false); return; }
    const requested = performance.now();
    current.promise.then((rendered) => {
      if (cancelled) return;
      const { canvas: source, viewport } = rendered;
      setPageSize({ width: viewport.width, height: viewport.height });
      canvas.width = source.width;
      canvas.height = source.height;
      canvas.style.width = `${viewport.width}px`;
      canvas.style.height = `${viewport.height}px`;
      sheet.style.width = `${viewport.width}px`;
      sheet.style.height = `${viewport.height}px`;
      // PDF.js 5.x uses these viewer CSS variables to size its selectable text spans.
      sheet.style.setProperty('--scale-factor', String(viewport.scale));
      sheet.style.setProperty('--user-unit', String(viewport.userUnit));
      textLayer.style.setProperty('--scale-factor', String(viewport.scale));
      textLayer.style.setProperty('--total-scale-factor', String(viewport.scale * viewport.userUnit));
      textLayer.style.setProperty('--user-unit', String(viewport.userUnit));
      textLayer.style.setProperty('--min-font-size', '1');
      canvas.getContext('2d')?.drawImage(source, 0, 0);
      // Raster and text must transition together. Keep the old page visible while drawing;
      // now that the new pixels are in place, replace the old page's invisible text spans.
      textLayer.replaceChildren();
      setRendering(false); setTextLayerLoading(true);
      const waited = performance.now() - requested;
      setRenderInfo(wasCachedReady && waited < 50 ? 'ready instantly (pre-rendered)' : `parsed ${(rendered.parseMs / 1000).toFixed(1)}s · drawn ${(rendered.drawMs / 1000).toFixed(1)}s`);
      console.info(`[IDIOS Reader] page ${pageNumber}: parse ${Math.round(rendered.parseMs)} ms, draw ${Math.round(rendered.drawMs)} ms, canvas ${source.width}x${source.height}`);

      void (async () => {
        try {
          const textContent = await textContentPromise;
          if (cancelled) return;
          if (!textContent) { textLayer.style.pointerEvents = 'auto'; setTextLayerLoading(false); return; }
          const pageText = textContent.items.map((item) => 'str' in item ? item.str : '').join(' ');
          pageTextCache.current.set(pageNumber, pageText);
          setCurrentPageText(pageText);
          if (pageText.replace(/\s/g, '').length === 0) setPageTextWarning('This page appears to be image-only; there is no embedded text to select (OCR is not enabled).');
          const layer = new TextLayer({ textContentSource: textContent, container: textLayer, viewport });
          textLayerTask = layer;
          await layer.render();
          if (cancelled) return;
          if (searchText.trim()) markSearchSpans(textLayer, searchText.trim());
          textLayer.style.pointerEvents = 'auto';
          setTextLayerLoading(false);
        } catch {
          if (!cancelled) { textLayer.style.pointerEvents = 'auto'; setPageTextWarning('This page has no selectable text layer.'); setTextLayerLoading(false); }
        }
      })();

      // Pre-render the next page, then the previous one, one after another.
      const next = ensure(pageNumber + 1);
      void (next ? next.promise.catch(() => undefined) : Promise.resolve()).then(() => { if (!cancelled) ensure(pageNumber - 1); });
    }).catch((cause: unknown) => {
      if (!cancelled) { textLayer.style.pointerEvents = 'auto'; setError(cause instanceof Error ? cause.message : 'Unable to render this page.'); setRendering(false); setTextLayerLoading(false); }
    });
    return () => { cancelled = true; textLayerTask?.cancel(); };
  }, [pdf, pageNumber, scale]);

  useEffect(() => {
    const cache = pageRenderCache.current;
    return () => { cache.entries.forEach((entry) => entry.cancel()); cache.entries.clear(); };
  }, []);

  useEffect(() => {
    if (!pdf || !pageCount) return;
    const timer = window.setTimeout(() => {
      api.setShelfBookProgress(bookID, { status: 'reading', current_page: pageNumber, total_pages: pageCount })
        .then(() => { setProgressMessage('Progress saved'); onProgress(pageNumber, pageCount); })
        .catch(() => setProgressMessage('Progress could not be saved'));
    }, 700);
    return () => window.clearTimeout(timer);
  }, [bookID, pageNumber, pageCount, pdf, onProgress]);

  useEffect(() => {
    const layer = textLayerRef.current;
    if (!layer) return;
    if (searchText.trim()) markSearchSpans(layer, searchText.trim());
    else layer.querySelectorAll('.shelf-pdf-reader__search-match').forEach((span) => span.classList.remove('shelf-pdf-reader__search-match'));
  }, [searchText, pageNumber, rendering]);

  const pageAnnotations = useMemo(() => annotations.filter((annotation) => annotation.page === pageNumber), [annotations, pageNumber]);
  const popupAnnotation = useMemo(() => notePopup ? pageAnnotations.find((annotation) => annotation.id === notePopup.id) ?? null : null, [notePopup, pageAnnotations]);
  const notePopupAbove = notePopup ? notePopup.y > window.innerHeight * 0.55 : false;
  const notePopupTop = notePopup ? (notePopupAbove ? notePopup.y - 14 : Math.min(notePopup.y + 22, Math.max(8, window.innerHeight * 0.45 - 8))) : 0;
  const notePopupLeft = notePopup ? Math.min(Math.max(notePopup.x, 180), Math.max(180, window.innerWidth - 180)) : 0;
  // Saved notes open on click, not hover. The text layer remains selectable; the click is
  // hit-tested against stored page rectangles, and the popup stays open until close/outside click.
  const handleSheetClick = useCallback((event: ReactMouseEvent<HTMLDivElement>) => {
    const target = event.target;
    if (target instanceof Element && target.closest('[data-shelf-note-popup], .shelf-pdf-reader__companion-pin, .shelf-pdf-reader__companion-note')) return;
    const sheet = pageSheetRef.current;
    if (!sheet) return;
    const box = sheet.getBoundingClientRect();
    if (!box.width || !box.height) return;
    const px = (event.clientX - box.left) / box.width;
    const py = (event.clientY - box.top) / box.height;
    const hit = pageAnnotations.find((annotation) => annotation.rectangles.some((rect) => px >= rect.x && px <= rect.x + rect.width && py >= rect.y && py <= rect.y + rect.height));
    if (hit) setNotePopup({ id: hit.id, x: event.clientX, y: event.clientY });
    else { setNotePopup(null); setActiveCompanionPin(null); }
  }, [pageAnnotations]);

  useEffect(() => {
    const closePopupsOutside = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Element && target.closest('[data-shelf-note-popup], .shelf-pdf-reader__companion-pin, .shelf-pdf-reader__companion-note')) return;
      setNotePopup(null);
      setActiveCompanionPin(null);
    };
    document.addEventListener('pointerdown', closePopupsOutside);
    return () => document.removeEventListener('pointerdown', closePopupsOutside);
  }, []);

  useEffect(() => {
    setNotePopup(null);
    setActiveCompanionPin(null);
  }, [pageNumber, bookID]);

  // Build complete document context for Tutor: every page before the current page,
  // the current page, and the next three pages. Reuse extracted text when available.
  // This is intentionally not based on the reader's visited-page history.
  const getDocumentContext = useCallback(async (targetPage: number): Promise<string> => {
    const document = pdf;
    if (!document) throw new Error('The PDF is still loading. Wait for the book to finish opening, then ask Mimi again.');
    const current = Math.min(Math.max(1, Math.floor(targetPage || pageNumber)), document.numPages);
    const lastPage = Math.min(document.numPages, current + 3);
    const pageTexts = new Map<number, string>();
    const pages = Array.from({ length: lastPage }, (_, index) => index + 1);
    // Small batches avoid overwhelming PDF.js on large books while still loading every prior page.
    for (let offset = 0; offset < pages.length; offset += 8) {
      const batch = pages.slice(offset, offset + 8);
      await Promise.all(batch.map(async (pageNo) => {
        const cached = pageTextCache.current.get(pageNo);
        if (cached !== undefined) { pageTexts.set(pageNo, cached); return; }
        try {
          const page = await document.getPage(pageNo);
          const textContent = await page.getTextContent();
          const text = textContent.items.map((item) => 'str' in item ? item.str : '').join(' ').replace(/\s+/g, ' ').trim();
          pageTextCache.current.set(pageNo, text);
          pageTexts.set(pageNo, text);
        } catch {
          pageTexts.set(pageNo, '[Text extraction failed for this PDF page.]');
        }
      }));
    }
    return pages.map((pageNo) => {
      const label = pageNo < current ? 'PREVIOUS PAGE' : pageNo === current ? 'CURRENT PAGE — PRIMARY SOURCE' : 'FOLLOWING PAGE';
      const text = pageTexts.get(pageNo)?.trim() || '[No extractable text on this page; it may be scanned or image-only.]';
      return `===== PDF PAGE ${pageNo} · ${label} =====\n${text}`;
    }).join('\n\n');
  }, [pdf, pageNumber]);

  // Keep the context window small enough for compact/local models while still
  // carrying one page on either side for references and terminology.
  const getTranslationContext = useCallback(async (targetPage: number): Promise<string> => {
    const document = pdf;
    if (!document) throw new Error('The PDF is still loading. Wait for the page to finish opening.');
    const current = Math.min(Math.max(1, Math.floor(targetPage || pageNumber)), document.numPages);
    const firstPage = Math.max(1, current - 1);
    const lastPage = Math.min(document.numPages, current + 1);
    const pageNumbers = Array.from({ length: lastPage - firstPage + 1 }, (_, index) => firstPage + index);
    const entries = await Promise.all(pageNumbers.map(async (pageNo) => {
      const cached = pageTextCache.current.get(pageNo);
      if (cached !== undefined) return { pageNo, text: cached };
      try {
        const page = await document.getPage(pageNo);
        const content = await page.getTextContent();
        const text = content.items.map((item) => 'str' in item ? item.str : '').join(' ').replace(/\s+/g, ' ').trim();
        pageTextCache.current.set(pageNo, text);
        return { pageNo, text };
      } catch {
        return { pageNo, text: '[Text extraction failed for this PDF page.]' };
      }
    }));
    return entries.map(({ pageNo, text }) => {
      const label = pageNo < current ? 'PREVIOUS PAGE' : pageNo === current ? 'CURRENT PAGE — PRIMARY SOURCE' : 'FOLLOWING PAGE';
      return `===== PDF PAGE ${pageNo} · ${label} =====\n${(text || '[No extractable text on this PDF page.]').slice(0, 1400)}`;
    }).join('\n\n').slice(0, 4700);
  }, [pdf, pageNumber]);

  const requestTranslation = useCallback(async (text: string, kind: ReaderTranslationKind): Promise<ReaderTranslationData> => {
    const source = text.trim();
    if (!source) throw new Error('Select some readable text first.');
    if (!translationModel) throw new Error('Choose an AI model in the Reading Companion panel before translating.');
    const documentContext = await getTranslationContext(pageNumber);
    const glossary = new Map<string, { source: string; translation: string; meaning: string }>();
    const cachedPages = Array.from(pageTranslationCache.current.entries()).sort((left, right) => Number(left[0].split(':')[1]) - Number(right[0].split(':')[1]));
    for (const [cacheKey, data] of cachedPages) {
      const [cachedBook, cachedPage, cachedLanguage, ...cachedModelParts] = cacheKey.split(':');
      const cachedModel = cachedModelParts.join(':');
      if (cachedBook !== bookID || cachedLanguage !== translationTargetLanguage || cachedModel !== translationModel || Number(cachedPage) >= pageNumber) continue;
      for (const term of data.terms) {
        const termKey = normalizeTranslationMatch(term.source);
        if (!termKey) continue;
        if (glossary.has(termKey)) glossary.delete(termKey);
        glossary.set(termKey, { source: term.source, translation: term.translation, meaning: term.meaning });
      }
    }
    const priorGlossary = Array.from(glossary.values()).slice(-30);
    const contextRequest: ReaderTranslationRequest = {
      text: source,
      pageText: currentPageText,
      pageNumber,
      targetLanguage: translationTargetLanguage,
      kind,
      documentContext,
      priorGlossary,
    };
    const chunks = kind === 'word' ? [source] : splitReaderTranslationText(source, 1050);
    if (!chunks.length) throw new Error('No readable text was extracted from this passage.');
    const results: ReaderTranslationData[] = [];
    const cachePrefix = `${bookID}:${pageNumber}:${translationTargetLanguage}:${translationModel}:${kind}`;
    setTranslationProgress({ done: 0, total: chunks.length });

    for (let index = 0; index < chunks.length; index += 1) {
      const chunk = chunks[index];
      const cacheKey = `${cachePrefix}:${index}:${chunk}`;
      const cached = translationChunkCache.current.get(cacheKey);
      if (cached) {
        results.push(cached);
        if (kind === 'page') setPartialPageTranslation(mergeReaderTranslations(results));
        for (const term of cached.terms) {
          const key = normalizeTranslationMatch(term.source);
          if (key) glossary.set(key, { source: term.source, translation: term.translation, meaning: term.meaning });
        }
        setTranslationProgress({ done: index + 1, total: chunks.length });
        continue;
      }

      const chunkRequest: ReaderTranslationRequest = {
        ...contextRequest,
        text: chunk,
        kind: chunks.length > 1 ? 'chunk' : kind,
        priorGlossary: Array.from(glossary.values()).slice(-30),
      };
      const completion = await onTranslateText(chunkRequest);
      const parsed = parseReaderTranslation(completion.content, chunk);
      if (!parsed.translation) throw new Error('The selected model returned an empty translation. Try again or select another model.');
      translationChunkCache.current.set(cacheKey, parsed);
      results.push(parsed);
      if (kind === 'page') setPartialPageTranslation(mergeReaderTranslations(results));
      for (const term of parsed.terms) {
        const key = normalizeTranslationMatch(term.source);
        if (key) glossary.set(key, { source: term.source, translation: term.translation, meaning: term.meaning });
      }
      setTranslationProgress({ done: index + 1, total: chunks.length });
    }
    setTranslationProgress({ done: chunks.length, total: chunks.length });
    const combined = results.length === 1 ? results[0] : mergeReaderTranslations(results);
    if (kind === 'page') setPartialPageTranslation(null);
    return combined;
  }, [bookID, currentPageText, getTranslationContext, onTranslateText, pageNumber, translationModel, translationTargetLanguage]);

  const refreshPageTranslation = useCallback(() => {
    pageTranslationCache.current.delete(`${bookID}:${pageNumber}:${translationTargetLanguage}:${translationModel}`);
    setPageTranslation(null);
    setTranslationPageKey('');
    translationPageInFlight.current = '';
    setPartialPageTranslation(null);
    setTranslationRevision((revision) => revision + 1);
    setSelectionTranslation(null);
    setTranslationError('');
  }, [bookID, pageNumber, translationModel, translationTargetLanguage]);

  const translateSelection = useCallback(async () => {
    const selected = selectedTextForAI.trim();
    if (!selected) return;
    setSelectionTranslationBusy(true);
    setTranslationError('');
    try {
      const translated = await requestTranslation(selected, 'selection');
      setSelectionTranslation(translated);
      setTranslationTab('translate');
    } catch (cause) {
      setTranslationError(cause instanceof Error ? cause.message : 'The selected passage could not be translated.');
    } finally {
      setSelectionTranslationBusy(false);
    }
  }, [requestTranslation, selectedTextForAI]);

  const explainHoveredWord = useCallback(async () => {
    const word = hoveredTextForAI.trim();
    if (!word || !translationOpen || !currentPageText.trim()) return;
    const key = `${bookID}:${pageNumber}:${translationTargetLanguage}:${translationModel}:${normalizeTranslationMatch(word)}`;
    const cached = wordTranslationCache.current.get(key);
    if (cached) { setWordTranslation(cached); setWordTranslationKey(key); return; }
    if (wordTranslationInFlight.current.has(key)) return;
    wordTranslationInFlight.current.add(key);
    setWordTranslationLoadingKey(key);
    setTranslationError('');
    try {
      const translated = await requestTranslation(word, 'word');
      wordTranslationCache.current.set(key, translated);
      setWordTranslation(translated);
      setWordTranslationKey(key);
    } catch (cause) {
      setTranslationError(cause instanceof Error ? cause.message : 'The word could not be explained in context.');
    } finally {
      wordTranslationInFlight.current.delete(key);
      setWordTranslationLoadingKey((current) => current === key ? '' : current);
    }
  }, [bookID, currentPageText, hoveredTextForAI, pageNumber, requestTranslation, translationModel, translationOpen, translationTargetLanguage]);

  useEffect(() => {
    if (!translationOpen || !currentPageText.trim() || !pdf || !translationModel) return;
    const key = `${bookID}:${pageNumber}:${translationTargetLanguage}:${translationModel}`;
    if ((translationPageKey === key && pageTranslation) || translationPageInFlight.current === key) return;
    const cachedTranslation = pageTranslationCache.current.get(key);
    if (cachedTranslation) {
      setPageTranslation(cachedTranslation);
      setTranslationPageKey(key);
      setPageTranslationBusy(false);
      return;
    }
    translationPageInFlight.current = key;
    let active = true;
    const requestID = ++translationRequestID.current;
    setPageTranslation(null);
    setPartialPageTranslation(null);
    setSelectionTranslation(null);
    setPageTranslationBusy(true);
    setTranslationError('');
    void requestTranslation(currentPageText, 'page').then((translated) => {
      if (!active || requestID !== translationRequestID.current) return;
      setPageTranslation(translated);
      setPartialPageTranslation(null);
      pageTranslationCache.current.set(key, translated);
      if (pageTranslationCache.current.size > 40) {
        const oldestKey = pageTranslationCache.current.keys().next().value;
        if (oldestKey) pageTranslationCache.current.delete(oldestKey);
      }
      setTranslationPageKey(key);
    }).catch((cause) => {
      if (active && requestID === translationRequestID.current) {
        translationPageInFlight.current = '';
        setTranslationError(cause instanceof Error ? cause.message : 'This page could not be translated.');
      }
    }).finally(() => {
      if (active && requestID === translationRequestID.current) setPageTranslationBusy(false);
    });
    return () => { active = false; if (translationPageInFlight.current === key) translationPageInFlight.current = ''; };
  }, [bookID, currentPageText, pageNumber, pageTranslation, pdf, requestTranslation, translationModel, translationOpen, translationPageKey, translationTargetLanguage, translationRevision]);

  useEffect(() => {
    if (!translationOpen || !pageTranslation || !hoveredTextForAI.trim()) return;
    const normalizedWord = normalizeTranslationMatch(hoveredTextForAI);
    const supplied = pageTranslation.terms.some((term) => normalizeTranslationMatch(term.source) === normalizedWord);
    if (supplied) return;
    const timer = window.setTimeout(() => { void explainHoveredWord(); }, 650);
    return () => window.clearTimeout(timer);
  }, [explainHoveredWord, hoveredTextForAI, pageTranslation, pageNumber, translationOpen]);

  const applyAIHighlights = useCallback(async (marks: RequestedBookHighlight[]): Promise<ApplyBookHighlightsResult> => {
    const result: ApplyBookHighlightsResult = { saved: [], failed: [] };
    for (const mark of marks.slice(0, 3)) {
      const targetPage = Number(mark.page ?? pageNumber);
      const quote = mark.quote.trim();
      if (!quote) { result.failed.push({ quote: '(empty quote)', reason: 'the model did not provide exact text' }); continue; }
      if (targetPage !== pageNumber) {
        result.failed.push({ quote, reason: `page ${targetPage} is not the page currently open; open that page and ask again` });
        continue;
      }
      const layer = textLayerRef.current;
      const sheet = pageSheetRef.current;
      if (!layer || !sheet || textLayerLoading || rendering) {
        result.failed.push({ quote, reason: 'the page text layer is still loading; retry after the page is ready' });
        continue;
      }
      const located = findUniqueQuoteRectangles(layer, sheet, quote);
      if (located.occurrences !== 1 || !located.rectangles.length) {
        result.failed.push({ quote, reason: located.occurrences > 1 ? 'that phrase appears more than once on this page, so I avoided marking the wrong one' : 'that exact phrase was not found in the selectable text of this page' });
        continue;
      }
      const color = mark.color || 'purple';
      const existing = annotations.find((item) => item.page === targetPage && item.quote.normalize('NFKC').replace(/\s+/g, ' ').trim().toLocaleLowerCase() === quote.normalize('NFKC').replace(/\s+/g, ' ').trim().toLocaleLowerCase());
      try {
        let saved: ShelfAnnotation;
        if (existing) {
          saved = await api.updateShelfAnnotation(existing.id, { note: mark.note.trim(), color });
          setAnnotations((items) => items.map((item) => item.id === existing.id ? { ...item, ...saved, rectangles: existing.rectangles } : item));
        } else {
          saved = await api.createShelfAnnotation(bookID, { page: targetPage, quote, note: mark.note.trim(), color, rectangles: located.rectangles });
          setAnnotations((items) => [...items.filter((item) => item.id !== saved.id), saved]);
        }
        result.saved.push(`“${quote}” on page ${targetPage}${mark.note.trim() ? ` — ${mark.note.trim()}` : ''}`);
      } catch (cause) {
        result.failed.push({ quote, reason: cause instanceof Error ? cause.message : 'the annotation could not be saved' });
      }
    }
    return result;
  }, [annotations, bookID, pageNumber, textLayerLoading, rendering]);
  const handleTextHover = useCallback((event: ReactMouseEvent<HTMLDivElement>) => {
    const layer = textLayerRef.current;
    if (!layer) return;
    // Chromium and Firefox expose different caret hit-testing APIs. Resolve the character
    // beneath the pointer, then expand to the complete word instead of guessing from a span.
    const textDocument = document as Document & {
      caretRangeFromPoint?: (x: number, y: number) => Range | null;
      caretPositionFromPoint?: (x: number, y: number) => { offsetNode: Node; offset: number } | null;
    };
    const range = textDocument.caretRangeFromPoint?.(event.clientX, event.clientY) ?? null;
    const position = range
      ? { node: range.startContainer, offset: range.startOffset }
      : (() => {
          const caret = textDocument.caretPositionFromPoint?.(event.clientX, event.clientY);
          return caret ? { node: caret.offsetNode, offset: caret.offset } : null;
        })();
    // Preserve the last word while the pointer travels from the PDF to the assistant panel.
    // The word is cleared when the reader changes pages; a new word replaces it immediately.
    if (!position || !layer.contains(position.node) || position.node.nodeType !== Node.TEXT_NODE) return;
    const value = position.node.textContent ?? '';
    const offset = Math.max(0, Math.min(value.length, position.offset));
    const isWordCharacter = (character: string) => /[\p{L}\p{N}_’'-]/u.test(character);
    let start = offset;
    let end = offset;
    if (start === value.length || (start < value.length && !isWordCharacter(value[start]))) start = Math.max(0, start - 1);
    while (start > 0 && isWordCharacter(value[start - 1])) start -= 1;
    while (end < value.length && isWordCharacter(value[end])) end += 1;
    const word = value.slice(start, end).trim();
    if (word.length >= 2) setHoveredTextForAI(word);
  }, []);

  useEffect(() => {
    setHoveredTextForAI('');
    setSelectedTextForAI('');
  }, [pageNumber, pdfURL]);
  const visiblePageLabel = pageCount ? `${pageNumber} / ${pageCount}` : '—';

  const captureSelection = useCallback(() => {
    const selection = window.getSelection();
    const textLayer = textLayerRef.current;
    const sheet = pageSheetRef.current;
    const quote = selection?.toString().trim() ?? '';
    if (!selection || !textLayer || !sheet || !quote || selection.rangeCount === 0) { setPending(null); setSelectedTextForAI(''); return; }
    const range = selection.getRangeAt(0);
    const anchor = range.commonAncestorContainer;
    if (!textLayer.contains(anchor.nodeType === Node.ELEMENT_NODE ? anchor as Element : anchor.parentElement)) { setPending(null); setSelectedTextForAI(''); return; }
    const pageRect = sheet.getBoundingClientRect();
    const rectangles = Array.from(range.getClientRects()).map((rect) => {
      const x = Math.max(0, (rect.left - pageRect.left) / pageRect.width);
      const y = Math.max(0, (rect.top - pageRect.top) / pageRect.height);
      const right = Math.min(1, (rect.right - pageRect.left) / pageRect.width);
      const bottom = Math.min(1, (rect.bottom - pageRect.top) / pageRect.height);
      return { x, y, width: Math.max(0, right - x), height: Math.max(0, bottom - y) };
    }).filter((rect) => rect.width > 0.001 && rect.height > 0.001).slice(0, 300);
    if (!rectangles.length) { setPending(null); setSelectedTextForAI(''); return; }
    setSelectionTranslation(null);
    const selectedWord = /^\p{L}[\p{L}\p{N}_’'-]*$/u.test(quote);
    setHoveredTextForAI(selectedWord ? quote : '');
    setSelectedTextForAI(quote.slice(0, 10000));
    setPending({ quote: quote.slice(0, 8000), rectangles });
  }, []);

  async function saveHighlight() {
    if (!pending) return;
    setSavingAnnotation(true); setProgressMessage('');
    try {
      const item = await api.createShelfAnnotation(bookID, { page: pageNumber, quote: pending.quote, note: pendingNote.trim(), color: pendingColor, rectangles: pending.rectangles });
      setAnnotations((items) => [...items, item]);
      setPending(null); setPendingNote(''); window.getSelection()?.removeAllRanges(); setProgressMessage(pendingNote.trim() ? 'Note saved' : 'Highlight saved');
    } catch (cause) { setProgressMessage(cause instanceof Error ? cause.message : 'Could not save this highlight'); }
    finally { setSavingAnnotation(false); }
  }

  // Opening the raw /pdf URL in a tab makes download managers such as IDM grab it. A blob URL
  // built from the bytes already loaded is shown by the browser's own PDF viewer instead.
  function openOriginalPDF() {
    const bytes = pdfBytesRef.current;
    if (!bytes) return;
    const url = URL.createObjectURL(new Blob([bytes.slice()], { type: 'application/pdf' }));
    window.open(url, '_blank');
    window.setTimeout(() => URL.revokeObjectURL(url), 5 * 60 * 1000);
  }

  async function deleteAnnotation(item: ShelfAnnotation) {
    try {
      await api.deleteShelfAnnotation(item.id);
      setAnnotations((items) => items.filter((annotation) => annotation.id !== item.id));
      if (activeAnnotationID === item.id) setActiveAnnotationID('');
    } catch (cause) { setProgressMessage(cause instanceof Error ? cause.message : 'Could not delete this highlight'); }
  }

  async function findText(direction: 1 | -1 = 1) {
    const query = searchText.trim().toLocaleLowerCase();
    if (!pdf || !query) { setSearchPages([]); return; }
    setSearching(true); setProgressMessage('');
    try {
      const matches: number[] = [];
      for (let page = 1; page <= pdf.numPages; page += 1) {
        let text = pageTextCache.current.get(page);
        if (text === undefined) {
          const pageProxy = await pdf.getPage(page);
          const content = await pageProxy.getTextContent();
          text = content.items.map((item) => 'str' in item ? item.str : '').join(' ');
          pageTextCache.current.set(page, text);
        }
        if (text.toLocaleLowerCase().includes(query)) matches.push(page);
      }
      setSearchPages(matches);
      if (!matches.length) { setProgressMessage('No matches found'); return; }
      const currentIndex = matches.findIndex((page) => direction > 0 ? page > pageNumber : page < pageNumber);
      const index = currentIndex >= 0 ? currentIndex : (direction > 0 ? 0 : matches.length - 1);
      setPageNumber(matches[index]);
    } catch (cause) { setProgressMessage(cause instanceof Error ? cause.message : 'Search failed'); }
    finally { setSearching(false); }
  }

  async function goToSearchMatch(direction: 1 | -1) {
    if (!searchPages.length) { await findText(direction); return; }
    const index = searchPages.findIndex((page) => page === pageNumber);
    const next = index < 0 ? 0 : (index + direction + searchPages.length) % searchPages.length;
    setPageNumber(searchPages[next]);
  }

  async function toggleFullscreen() {
    if (!readerRef.current) return;
    try {
      if (document.fullscreenElement === readerRef.current) await document.exitFullscreen();
      else await readerRef.current.requestFullscreen();
    } catch { setProgressMessage('Fullscreen is not available in this window'); }
  }

  useEffect(() => {
    const handleCompanionHighlights = (event: Event) => {
      const detail = (event as CustomEvent<CompanionHighlightDetail>).detail;
      if (!detail || detail.bookID !== bookID) return;
      const incoming = Array.isArray(detail.marks) ? detail.marks : (typeof detail.quote === 'string' && detail.quote.trim() ? [{ quote: detail.quote, note: '', kind: 'important' as const }] : []);
      const marks = incoming.filter((item): item is CompanionMark => Boolean(item && typeof item.quote === 'string' && item.quote.trim() && (item.kind === 'important' || item.kind === 'unclear')))
        .slice(0, 3).map((item) => ({ quote: item.quote.slice(0, 120), note: String(item.note || '').slice(0, 240), kind: item.kind }));
      setCompanionHighlights({ pageNumber: detail.pageNumber, marks });
    };
    window.addEventListener('idios:shelf-companion-highlights', handleCompanionHighlights);
    return () => window.removeEventListener('idios:shelf-companion-highlights', handleCompanionHighlights);
  }, [bookID]);

  useEffect(() => {
    clearCompanionHighlight();
    const layer = textLayerRef.current;
    const sheet = pageSheetRef.current;
    if (!layer || !sheet) { setCompanionPins([]); return; }
    const marks = companionHighlights.pageNumber === pageNumber ? companionHighlights.marks : [];
    const pins = markCompanionHighlights(layer, sheet, marks);
    setCompanionPins(pins);
    setActiveCompanionPin((active) => active !== null && active < pins.length ? active : null);
    return () => clearCompanionHighlight();
  }, [companionHighlights, pageNumber, textLayerLoading, rendering]);

  const openAIChat = useCallback((prompt?: string, openSettings = false) => {
    setInlineChatPrompt(prompt || '');
    setInlineChatSettingsOpen(openSettings);
    setInlineChatOpen(true);
  }, []);

  const closeInlineChat = useCallback(() => {
    setInlineChatOpen(false);
    setInlineChatPrompt('');
    setInlineChatSettingsOpen(false);
  }, []);

  useEffect(() => {
    if (focusMode) setSidePanelOpen(false);
  }, [focusMode]);

  useEffect(() => {
    const openTranslation = () => {
      setSidePanelOpen(false);
      setInlineChatOpen(false);
      setTranslationTab('translate');
      setTranslationOpen(true);
    };
    window.addEventListener('idios:shelf-open-translation', openTranslation);
    return () => window.removeEventListener('idios:shelf-open-translation', openTranslation);
  }, []);

  useEffect(() => {
    const onOpenMimi = (event: Event) => {
      const detail = (event as CustomEvent<{ bookID?: string; prompt?: string; openSettings?: boolean }>).detail;
      if (!detail || detail.bookID !== bookID) return;
      setSidePanelOpen(false);
      openAIChat(detail.prompt || `Help me study “${bookTitle}”, page ${pageNumber}.`, detail.openSettings === true);
    };
    window.addEventListener('idios:shelf-open-mimi', onOpenMimi);
    return () => window.removeEventListener('idios:shelf-open-mimi', onOpenMimi);
  }, [bookID, bookTitle, openAIChat, pageNumber]);

  useEffect(() => {
    if (!selectedTextForAI.trim()) return;
    window.dispatchEvent(new CustomEvent('idios:shelf-reader-selection', {
      detail: { bookID, text: selectedTextForAI, pageNumber },
    }));
  }, [bookID, pageNumber, selectedTextForAI]);

  useEffect(() => {
    window.dispatchEvent(new CustomEvent('idios:shelf-reader-context', {
      detail: { bookID, pageNumber, pageText: currentPageText, selectedText: selectedTextForAI },
    }));
  }, [bookID, currentPageText, pageNumber, selectedTextForAI]);

  return <div className={`shelf-pdf-reader shelf-v3-pdf-reader${focusMode ? ' shelf-pdf-reader--focus-session' : ''}`} ref={readerRef}>
    <div className="shelf-pdf-reader__toolbar shelf-v3-reader-toolbar">
      <div className="shelf-pdf-reader__brand shelf-v3-reader-brand">
        <span className="shelf-pdf-reader__brand-icon"><FileText size={16} /></span>
        <div><strong title={bookTitle}>{bookTitle}</strong><span>PDF reader · IDIOS Shelf</span></div>
      </div>
      <div className="shelf-pdf-reader__controls shelf-v3-reader-controls">
        <div className="shelf-v3-reader-control-group shelf-v3-reader-pagination" aria-label="Page navigation">
          <button type="button" className="shelf-pdf-reader__icon" onClick={() => setPageNumber((page) => Math.max(1, page - 1))} disabled={loading || pageNumber <= 1} aria-label="Previous page" title="Previous page"><ChevronLeft size={17} /></button>
          <label className="shelf-pdf-reader__page-input"><input aria-label="Page number" type="number" min={1} max={pageCount || undefined} value={pageNumber} onChange={(event) => { const value = Number(event.target.value); if (Number.isFinite(value) && value >= 1 && value <= pageCount) setPageNumber(value); }} onKeyDown={(event) => { if (event.key === 'Enter') (event.currentTarget as HTMLInputElement).blur(); }} /><span>/ {pageCount || '—'}</span></label>
          <button type="button" className="shelf-pdf-reader__icon" onClick={() => setPageNumber((page) => Math.min(pageCount || page, page + 1))} disabled={loading || pageNumber >= pageCount} aria-label="Next page" title="Next page"><ChevronRight size={17} /></button>
        </div>
        <div className="shelf-v3-reader-control-group shelf-v3-reader-zoom" aria-label="Zoom controls">
          <button type="button" className="shelf-pdf-reader__icon" onClick={() => setScale((value) => Math.max(0.55, Math.round((value - 0.1) * 100) / 100))} disabled={scale <= 0.55} aria-label="Zoom out" title="Zoom out"><ZoomOut size={16} /></button>
          <span className="shelf-pdf-reader__zoom">{Math.round(scale * 100)}%</span>
          <button type="button" className="shelf-pdf-reader__icon" onClick={() => setScale((value) => Math.min(2.2, Math.round((value + 0.1) * 100) / 100))} disabled={scale >= 2.2} aria-label="Zoom in" title="Zoom in"><ZoomIn size={16} /></button>
        </div>
        <div className="shelf-v3-reader-control-group shelf-v3-reader-actions" aria-label="Reading tools">
          <button type="button" className={`shelf-pdf-reader__icon shelf-pdf-reader__focus-toggle${focusMode ? ' is-active' : ''}`} onClick={onToggleFocus} aria-label={focusMode ? 'End focus session' : 'Start focus session'} aria-pressed={focusMode} disabled={focusBusy} title={focusBusy ? 'Saving focus session…' : focusMode ? `End focus session · ${Math.floor(focusElapsedSeconds / 3600).toString().padStart(2, '0')}:${Math.floor((focusElapsedSeconds % 3600) / 60).toString().padStart(2, '0')}:${(focusElapsedSeconds % 60).toString().padStart(2, '0')}` : 'Focus mode'}><Focus size={15} /></button>
          {focusMode && <span className="shelf-pdf-reader__focus-timer" aria-label="Focus session time">{Math.floor(focusElapsedSeconds / 3600).toString().padStart(2, '0')}:{Math.floor((focusElapsedSeconds % 3600) / 60).toString().padStart(2, '0')}:{(focusElapsedSeconds % 60).toString().padStart(2, '0')}</span>}
          {focusMode && <>
            <button type="button" className="shelf-pdf-reader__icon" onClick={() => onOpenFocusTool('search')} aria-label="Search the web" title="Quick web search"><Search size={15} /></button>
          </>}
          <button type="button" className={`shelf-pdf-reader__icon ${translationOpen ? 'is-active' : ''}`} onClick={() => { setTranslationOpen((open) => !open); setTranslationTab('translate'); setSidePanelOpen(false); setInlineChatOpen(false); }} aria-label="Toggle AI translation and learning" title="Translate & learn"><Languages size={16} /></button>
          <button type="button" className={`shelf-pdf-reader__icon ${sidePanelOpen && sidePanelMode === 'notes' ? 'is-active' : ''}`} onClick={() => { if (sidePanelOpen && sidePanelMode === 'notes') setSidePanelOpen(false); else { setSidePanelMode('notes'); setSidePanelOpen(true); } }} aria-label="Show highlights panel" title="Highlights & notes"><Bookmark size={16} /></button>
          <button type="button" className={`shelf-pdf-reader__icon ${sidePanelOpen && sidePanelMode === 'discussion' ? 'is-active' : ''}`} onClick={() => { setTranslationOpen(false); if (sidePanelOpen && sidePanelMode === 'discussion') setSidePanelOpen(false); else { setInlineChatOpen(false); setSidePanelMode('discussion'); setSidePanelOpen(true); } }} aria-label="Toggle agent discussion" title="Agent discussion"><MessagesSquare size={16} /></button>
          <button type="button" className={`shelf-pdf-reader__icon ${inlineChatOpen ? 'is-active' : ''}`} onClick={() => { setTranslationOpen(false); setSidePanelOpen(false); if (inlineChatOpen) closeInlineChat(); else openAIChat(); }} aria-label="Toggle AI reading companion" title="AI reading companion"><MessageCircle size={16} /></button>
        </div>
        <div className="shelf-v3-reader-control-group shelf-v3-reader-utilities" aria-label="Reader options">
          <button type="button" className="shelf-pdf-reader__icon" onClick={() => void toggleFullscreen()} aria-label="Toggle fullscreen" title="Fullscreen"><Maximize2 size={15} /></button>
          <button type="button" className="shelf-pdf-reader__icon" onClick={openOriginalPDF} disabled={!pdfReady} aria-label="Open original PDF in a new tab" title="Open original PDF"><ExternalLink size={15} /></button>
          {!focusMode && <button type="button" className="shelf-pdf-reader__close" onClick={onClose} aria-label="Back to book"><ArrowLeft size={15} /><span>Back to book</span></button>}
        </div>
      </div>
    </div>

    <div className={`shelf-pdf-reader__body ${translationOpen ? 'shelf-pdf-reader__body--translation' : sidePanelOpen && sidePanelMode === 'notes' ? '' : sidePanelOpen && sidePanelMode === 'discussion' ? 'shelf-pdf-reader__body--discussion' : 'shelf-pdf-reader__body--wide'}`}>
      {translationOpen && <button type="button" className="shelf-translation-panel__scrim" aria-label="Close translation panel" onClick={() => setTranslationOpen(false)} />}
      {translationOpen && <ShelfTranslationPanel
        pageNumber={pageNumber}
        translationModel={translationModel}
        modelOptions={translationModelOptions}
        modelsLoading={translationModelsLoading}
        modelError={translationModelError}
        lastUsedModel={lastUsedTranslationModel}
        onModelChange={(model) => { onTranslationModelChange(model); setTranslationPageKey(''); setPageTranslation(null); setPartialPageTranslation(null); setSelectionTranslation(null); setTranslationError(''); }}
        targetLanguage={translationTargetLanguage}
        onTargetLanguageChange={(language) => { setTranslationTargetLanguage(language); setTranslationPageKey(''); setPageTranslation(null); setPartialPageTranslation(null); setSelectionTranslation(null); }}
        tab={translationTab}
        onTabChange={setTranslationTab}
        pageData={pageTranslation ?? partialPageTranslation}
        selectionData={selectionTranslation}
        wordData={wordTranslationKey === `${bookID}:${pageNumber}:${translationTargetLanguage}:${translationModel}:${normalizeTranslationMatch(hoveredTextForAI)}` ? wordTranslation : null}
        activeText={selectedTextForAI || hoveredTextForAI}
        selectedText={selectedTextForAI}
        hoveredWord={hoveredTextForAI}
        pageBusy={pageTranslationBusy}
        progress={translationProgress}
        selectionBusy={selectionTranslationBusy}
        wordBusy={wordTranslationLoadingKey === `${bookID}:${pageNumber}:${translationTargetLanguage}:${translationModel}:${normalizeTranslationMatch(hoveredTextForAI)}`}
        error={translationError}
        onClose={() => setTranslationOpen(false)}
        onTranslatePage={refreshPageTranslation}
        onTranslateSelection={() => void translateSelection()}
        onExplainWord={() => void explainHoveredWord()}
        onSelectTerm={(term) => { setSelectedTextForAI(''); setHoveredTextForAI(term); }}
      />}
      <section className="shelf-pdf-reader__workspace">
        <div className="shelf-pdf-reader__searchbar"><form onSubmit={(event) => { event.preventDefault(); void findText(1); }}><Search size={15} /><input value={searchText} onChange={(event) => setSearchText(event.target.value)} placeholder="Find in this book…" aria-label="Find text in PDF" /><button type="submit" disabled={searching || !searchText.trim()}>{searching ? <LoaderCircle size={14} className="spin" /> : 'Find'}</button></form>{searchPages.length > 0 && <div className="shelf-pdf-reader__search-nav"><span>{searchPages.length} matching {searchPages.length === 1 ? 'page' : 'pages'}</span><button type="button" onClick={() => void goToSearchMatch(-1)} aria-label="Previous search match"><ChevronLeft size={14} /></button><button type="button" onClick={() => void goToSearchMatch(1)} aria-label="Next search match"><ChevronRight size={14} /></button></div>}</div>
        {error ? <div className="shelf-pdf-reader__error"><FileText size={22} /><strong>Could not open this PDF</strong><p>{error}</p></div> : loading ? <div className="shelf-pdf-reader__loading" role="status" aria-live="polite"><div className="shelf-pdf-reader__loading-art"><div className="shelf-pdf-reader__loading-ring" /><FileText size={24} /></div><div className="shelf-pdf-reader__loading-copy"><strong>{elapsedSeconds < 3 ? 'Opening your book…' : elapsedSeconds < 10 ? 'Reading the PDF structure…' : 'This is a large PDF — still working…'}</strong><span>Downloading, parsing and preparing the first page</span><div className="shelf-pdf-reader__progress-track"><i /></div><small>{elapsedSeconds}s elapsed · large scans and embedded fonts can take longer</small></div></div> : <>
          <div className="shelf-pdf-reader__page-stage" onMouseUp={() => window.requestAnimationFrame(captureSelection)} onKeyUp={(event) => { if (event.key === 'Shift' || event.key === 'ArrowLeft' || event.key === 'ArrowRight') captureSelection(); }}>
            {showRenderOverlay && (rendering || textLayerLoading) && <div className="shelf-pdf-reader__rendering-overlay" role="status" aria-live="polite"><div className="shelf-pdf-reader__rendering-card"><LoaderCircle size={20} className="spin" /><div><strong>{rendering ? `Drawing page ${pageNumber}…` : 'Making the text selectable…'}</strong><span>{rendering ? 'The PDF is loaded; a complex page can take a little while to draw.' : 'Preparing accurate text positions for selection and highlighting.'}<span className="shelf-pdf-reader__progress-track"><i /></span></span></div><small>{elapsedSeconds}s</small></div></div>}
            <div className="shelf-pdf-reader__page-sheet" ref={pageSheetRef} onClick={handleSheetClick} style={{ width: pageSize.width || undefined, height: pageSize.height || undefined }}>
              <canvas ref={canvasRef} className="shelf-pdf-reader__canvas" aria-label={`PDF page ${pageNumber}`} />
              <div className="shelf-pdf-reader__saved-marks" aria-hidden="true">{pageAnnotations.map((annotation) => annotation.rectangles.map((rect, index) => <span key={`${annotation.id}-${index}`} className={`shelf-pdf-reader__mark shelf-pdf-reader__mark--${annotation.color} ${annotation.note?.trim() ? 'shelf-pdf-reader__mark--underline' : ''} ${activeAnnotationID === annotation.id ? 'is-active' : ''}`} style={{ left: `${rect.x * 100}%`, top: `${rect.y * 100}%`, width: `${rect.width * 100}%`, height: `${rect.height * 100}%` }} />))}</div>
              <div className="textLayer shelf-pdf-reader__text-layer" ref={textLayerRef} onMouseMove={handleTextHover} />
              {companionPins.length > 0 && <div className="shelf-pdf-reader__companion-pins" aria-label="Mimi's page notes">
                {companionPins.map((pin, index) => <button key={`${pin.quote}-${index}`} type="button" className={`shelf-pdf-reader__companion-pin shelf-pdf-reader__companion-pin--${pin.kind} ${activeCompanionPin === index ? 'is-active' : ''}`} style={{ left: `${pin.left}%`, top: `${pin.top}%` }} onClick={(event) => { event.stopPropagation(); setActiveCompanionPin((active) => active === index ? null : index); }} title={`${pin.quote}: ${pin.note}`} aria-label={`Mimi note for ${pin.quote}`}><span>{pin.kind === 'unclear' ? '?' : '✦'}</span></button>)}
                {activeCompanionPin !== null && companionPins[activeCompanionPin] && <aside className={`shelf-pdf-reader__companion-note shelf-pdf-reader__companion-note--${companionPins[activeCompanionPin].kind}`} style={{ left: `${Math.max(3, Math.min(29, companionPins[activeCompanionPin].left))}%`, top: `${Math.max(3, Math.min(80, companionPins[activeCompanionPin].top + 2))}%` }} role="note">
                  <button type="button" className="shelf-pdf-reader__companion-note-close" onClick={() => setActiveCompanionPin(null)} aria-label="Close Mimi note"><X size={12} /></button>
                  <small>{companionPins[activeCompanionPin].kind === 'unclear' ? 'MIMI · WORTH UNPACKING' : 'MIMI · KEY IDEA'}</small><strong>{companionPins[activeCompanionPin].quote}</strong><p>{companionPins[activeCompanionPin].note || 'Mimi marked this as worth noticing in context.'}</p>
                  <button type="button" className="shelf-pdf-reader__companion-note-discuss" onClick={() => openAIChat(`Can we talk about “${companionPins[activeCompanionPin!].quote}” on page ${pageNumber}? Mimi's note: ${companionPins[activeCompanionPin!].note || 'This phrase is worth noticing.'} Explain it simply using this page and our reading context.`)}>Discuss with Tutor <MessageCircle size={12} /></button>
                </aside>}
              </div>}
            </div>
            {notePopup && popupAnnotation && <aside data-shelf-note-popup role="dialog" aria-label={`${popupAnnotation.note?.trim() ? 'Saved note' : 'Saved highlight'} on page ${popupAnnotation.page}`} className={`shelf-pdf-reader__note-bubble shelf-pdf-reader__note-bubble--${popupAnnotation.color} ${notePopupAbove ? 'is-above' : 'is-below'}`} style={{ left: notePopupLeft, top: notePopupTop }}><div className="shelf-pdf-reader__note-bubble-head"><span className="shelf-pdf-reader__note-bubble-label">{popupAnnotation.note?.trim() ? <StickyNote size={11} /> : <Highlighter size={11} />} {popupAnnotation.note?.trim() ? 'NOTE' : 'HIGHLIGHT'} · PAGE {popupAnnotation.page}</span><button type="button" onClick={() => setNotePopup(null)} aria-label="Close note" title="Close note"><X size={13} /></button></div><p>{popupAnnotation.note?.trim() || 'No note saved for this highlight yet. Ask Tutor to explain this passage or add a note.'}</p><small>{popupAnnotation.quote}</small><button type="button" className="shelf-pdf-reader__note-bubble-action" onClick={() => { const annotation = popupAnnotation; if (!annotation) return; setNotePopup(null); openAIChat(`Help me understand this saved highlight on page ${annotation.page}: “${annotation.quote}”. ${annotation.note?.trim() ? `My saved note says: ${annotation.note}. ` : ''}Explain it simply using the current page and our reading context. I may also want to add or improve a note on this highlight.`); }}><MessageCircle size={12} /> Discuss with Tutor</button></aside>}
          </div>
          <div className="shelf-pdf-reader__page-footer"><span>Page {visiblePageLabel}{renderInfo ? ` · ${renderInfo}` : ''}{textLayerLoading ? ' · preparing selectable text…' : ''}</span><span>Select text to highlight it. Click any saved highlight or note to open it; close it or click outside to dismiss.</span><span>{pageTextWarning || progressMessage || (loadingAnnotations ? 'Loading highlights…' : `${annotations.length} saved highlights`)}</span></div>
          {pending && <div className="shelf-pdf-reader__selection-card"><div className="shelf-pdf-reader__selection-title"><Highlighter size={15} /><strong>Highlight or add a note</strong><button type="button" onClick={() => setPending(null)} aria-label="Discard selection"><X size={14} /></button></div><blockquote>{pending.quote}</blockquote><div className="shelf-pdf-reader__color-row" aria-label="Mark color">{highlightColors.map((color) => <button key={color.value} type="button" className={`shelf-pdf-reader__color-dot shelf-pdf-reader__color-dot--${color.value} ${pendingColor === color.value ? 'is-selected' : ''}`} onClick={() => setPendingColor(color.value)} aria-label={`${color.label} highlight`} title={color.label} />)}</div><label className="shelf-pdf-reader__note-field"><StickyNote size={14} /><input value={pendingNote} onChange={(event) => setPendingNote(event.target.value)} placeholder="Write a note — this text will be underlined with a hover bubble…" maxLength={20000} /></label><div className="shelf-pdf-reader__selection-actions"><span>{pending.quote.length} characters selected</span><button type="button" className="button button--ai" onClick={() => { setSidePanelOpen(false); openAIChat(pending.quote ? `Help me understand this highlighted passage in the context of the book: “${pending.quote}”` : undefined); }}><Sparkles size={13} /> Ask AI</button><button type="button" className="button button--primary" onClick={() => void saveHighlight()} disabled={savingAnnotation}>{savingAnnotation ? <LoaderCircle size={14} className="spin" /> : <Check size={14} />} {pendingNote.trim() ? 'Save note' : 'Save highlight'}</button></div></div>}
        </>}
      </section>

      {sidePanelOpen && sidePanelMode === 'notes' && <aside className="shelf-pdf-reader__side-panel"><div className="shelf-pdf-reader__side-heading"><div><span className="eyebrow">YOUR MARGINALIA</span><h3><Highlighter size={16} /> Highlights & notes</h3></div><span>{annotations.length}</span></div>{annotations.length === 0 ? <div className="shelf-pdf-reader__empty"><Bookmark size={24} /><strong>Nothing highlighted yet</strong><p>Select text on a PDF page, choose a color, and save it here with an optional note.</p></div> : <div className="shelf-pdf-reader__annotation-list">{annotations.map((item) => <article key={item.id} className={`shelf-pdf-reader__annotation ${activeAnnotationID === item.id ? 'is-active' : ''}`}><button type="button" className={`shelf-pdf-reader__annotation-jump shelf-pdf-reader__annotation-jump--${item.color}`} onClick={() => { setPageNumber(item.page); setActiveAnnotationID(item.id); }}><span>PAGE {item.page}</span><p>{item.quote}</p>{item.note && <small><StickyNote size={12} /> {item.note}</small>}</button><button type="button" className="shelf-pdf-reader__annotation-delete" onClick={() => void deleteAnnotation(item)} aria-label="Delete highlight" title="Delete highlight"><Trash2 size={13} /></button></article>)}</div>}
        <div className="shelf-pdf-reader__side-footer"><span><Bookmark size={12} /> Saved to IDIOS</span><span>Page {visiblePageLabel}</span></div></aside>}
      <ShelfAgentDiscussion open={sidePanelOpen && sidePanelMode === 'discussion'} bookID={bookID} bookTitle={bookTitle} pageNumber={pageNumber} pageText={currentPageText} selectedText={selectedTextForAI} annotations={annotations} onClose={() => setSidePanelOpen(false)} onOpenWebSearch={(query) => onOpenFocusTool('search', query)} />
    </div>
    <ShelfStudyAssistant key={`${bookID}:reader-companion`} bookID={bookID} bookTitle={bookTitle} pageNumber={pageNumber} pageText={currentPageText} selectedText={selectedTextForAI} hoveredText={hoveredTextForAI} displayMode="reader" quietMode={focusMode} onOpenChat={openAIChat} onApplyBookHighlights={applyAIHighlights} onGetDocumentContext={getDocumentContext} />
    {inlineChatOpen && <ShelfStudyAssistant key={`${bookID}:inline-tutor`} bookID={bookID} bookTitle={bookTitle} pageNumber={pageNumber} pageText={currentPageText} selectedText={selectedTextForAI} hoveredText={hoveredTextForAI} displayMode="inline-chat" initialPrompt={inlineChatPrompt} initialSettingsOpen={inlineChatSettingsOpen} onApplyBookHighlights={applyAIHighlights} onGetDocumentContext={getDocumentContext} onCloseInlineChat={closeInlineChat} onOpenModels={() => setInlineChatOpen(false)} />}
  </div>;
}

function markSearchSpans(layer: HTMLDivElement, query: string) {
  layer.querySelectorAll('.shelf-pdf-reader__search-match').forEach((span) => span.classList.remove('shelf-pdf-reader__search-match'));
  const normalized = query.toLocaleLowerCase();
  layer.querySelectorAll('span').forEach((span) => {
    if ((span.textContent ?? '').toLocaleLowerCase().includes(normalized)) span.classList.add('shelf-pdf-reader__search-match');
  });
}

type CompanionHighlightDetail = { bookID: string; pageNumber: number; marks?: CompanionMark[]; quote?: string };
const companionHighlightImportantName = 'idios-shelf-companion-anchor';
const companionHighlightUnclearName = 'idios-shelf-companion-unclear';
type HighlightRegistryLike = { set: (name: string, highlight: unknown) => void; delete: (name: string) => boolean };
type WindowWithTextHighlight = Window & { Highlight?: new (...ranges: Range[]) => unknown };
type TextWordRange = { word: string; node: Text; start: number; end: number };

function normalizeHighlightWord(value: string): string {
  return value.normalize('NFKC').toLocaleLowerCase().replace(/[’‘]/g, "'").replace(/[‐‑‒–—]/g, '-');
}

type ExactQuoteRectangleResult = { occurrences: number; rectangles: ShelfAnnotationRect[] };
function findUniqueQuoteRectangles(layer: HTMLDivElement, sheet: HTMLDivElement, quote: string): ExactQuoteRectangleResult {
  const toWords = (value: string) => value.match(/[\p{L}\p{N}][\p{L}\p{N}'’-]*/gu)?.map(normalizeHighlightWord) ?? [];
  const query = toWords(quote);
  if (!query.length || query.length > 32) return { occurrences: 0, rectangles: [] };
  const words: TextWordRange[] = [];
  const walker = document.createTreeWalker(layer, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      if (!node.textContent?.trim() || node.parentElement?.closest('.endOfContent')) return NodeFilter.FILTER_REJECT;
      return NodeFilter.FILTER_ACCEPT;
    },
  });
  let current: Node | null;
  while ((current = walker.nextNode())) {
    if (current.nodeType !== Node.TEXT_NODE) continue;
    const node = current as Text;
    for (const match of node.data.matchAll(/[\p{L}\p{N}][\p{L}\p{N}'’-]*/gu)) {
      const start = match.index ?? 0;
      words.push({ word: normalizeHighlightWord(match[0]), node, start, end: start + match[0].length });
    }
  }
  const matches: number[] = [];
  for (let start = 0; start <= words.length - query.length; start += 1) {
    if (query.every((word, offset) => words[start + offset]?.word === word)) matches.push(start);
    if (matches.length > 1) return { occurrences: matches.length, rectangles: [] };
  }
  if (matches.length !== 1) return { occurrences: matches.length, rectangles: [] };
  const first = words[matches[0]];
  const last = words[matches[0] + query.length - 1];
  const range = document.createRange();
  range.setStart(first.node, first.start);
  range.setEnd(last.node, last.end);
  if (toWords(range.toString()).join(' ') !== query.join(' ')) return { occurrences: 0, rectangles: [] };
  const box = sheet.getBoundingClientRect();
  if (!box.width || !box.height) return { occurrences: 0, rectangles: [] };
  const rectangles = Array.from(range.getClientRects()).map((rect) => {
    const left = Math.max(box.left, rect.left);
    const top = Math.max(box.top, rect.top);
    const right = Math.min(box.right, rect.right);
    const bottom = Math.min(box.bottom, rect.bottom);
    return { x: (left - box.left) / box.width, y: (top - box.top) / box.height, width: (right - left) / box.width, height: (bottom - top) / box.height };
  }).filter((rect) => rect.width > 0.001 && rect.height > 0.001).slice(0, 300);
  return { occurrences: rectangles.length ? 1 : 0, rectangles };
}

function clearCompanionHighlight() {
  const registry = (window.CSS as typeof CSS & { highlights?: HighlightRegistryLike }).highlights;
  registry?.delete(companionHighlightImportantName);
  registry?.delete(companionHighlightUnclearName);
}

function markCompanionHighlights(layer: HTMLDivElement, sheet: HTMLDivElement, marks: CompanionMark[]): CompanionPin[] {
  clearCompanionHighlight();
  // CSS Custom Highlights keep the marks inside the exact text range; never colour whole PDF.js spans.
  if (!marks.length) return [];
  const registry = (window.CSS as typeof CSS & { highlights?: HighlightRegistryLike }).highlights;
  const HighlightConstructor = (window as WindowWithTextHighlight).Highlight;
  if (!registry || !HighlightConstructor) return [];

  const toWords = (value: string) => value.match(/[\p{L}\p{N}][\p{L}\p{N}'’-]*/gu)?.map(normalizeHighlightWord) ?? [];
  const words: TextWordRange[] = [];
  const walker = document.createTreeWalker(layer, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      if (!node.textContent?.trim()) return NodeFilter.FILTER_REJECT;
      if (node.parentElement?.closest('.endOfContent')) return NodeFilter.FILTER_REJECT;
      return NodeFilter.FILTER_ACCEPT;
    },
  });
  let current: Node | null;
  while ((current = walker.nextNode())) {
    if (current.nodeType !== Node.TEXT_NODE) continue;
    const node = current as Text;
    const pattern = /[\p{L}\p{N}][\p{L}\p{N}'’-]*/gu;
    for (const match of node.data.matchAll(pattern)) {
      const start = match.index ?? 0;
      words.push({ word: normalizeHighlightWord(match[0]), node, start, end: start + match[0].length });
    }
  }
  const sheetRect = sheet.getBoundingClientRect();
  const safePins: CompanionPin[] = [];
  const importantRanges: Range[] = [];
  const unclearRanges: Range[] = [];
  const seen = new Set<string>();
  for (const mark of marks.slice(0, 3)) {
    const query = toWords(mark.quote);
    if (query.length < 1 || query.length > 8 || seen.has(query.join(' '))) continue;
    seen.add(query.join(' '));
    const matches: number[] = [];
    for (let index = 0; index <= words.length - query.length; index += 1) {
      if (query.every((word, offset) => words[index + offset]?.word === word)) matches.push(index);
      if (matches.length > 1) break;
    }
    // Ambiguous phrases are deliberately not marked; guessing the wrong occurrence is worse than no mark.
    if (matches.length !== 1) continue;
    const first = words[matches[0]];
    const last = words[matches[0] + query.length - 1];
    try {
      const range = document.createRange();
      range.setStart(first.node, first.start);
      range.setEnd(last.node, last.end);
      if (toWords(range.toString()).join(' ') !== query.join(' ')) continue;
      const rects = Array.from(range.getClientRects());
      const rect = rects[0] ?? range.getBoundingClientRect();
      if (!rect.width && !rect.height) continue;
      (mark.kind === 'unclear' ? unclearRanges : importantRanges).push(range);
      safePins.push({
        ...mark,
        left: Math.max(2, Math.min(98, ((rect.right - sheetRect.left) / Math.max(1, sheetRect.width)) * 100)),
        top: Math.max(2, Math.min(98, ((rect.top - sheetRect.top) / Math.max(1, sheetRect.height)) * 100)),
      });
    } catch {
      // Fail closed: do not replace precise marking with broad span styling.
    }
  }
  try {
    if (importantRanges.length) registry.set(companionHighlightImportantName, new HighlightConstructor(...importantRanges));
    if (unclearRanges.length) registry.set(companionHighlightUnclearName, new HighlightConstructor(...unclearRanges));
  } catch {
    clearCompanionHighlight();
    return [];
  }
  return safePins;
}
