import { api, type AIProvider } from '@/lib/api';
import type { AISettings, ArchivedThread, ChatMessage, CompanionMark, LegacyAISettings, MascotPosition, PageInsight, ReadPage, ReasoningEffort, StudyContext, ShelfRuntimeMessage, ShelfRuntimeResponse } from './study-types';
import { separateModelThinking } from './modelThinking';

export const settingsKey = 'idios.shelf.study-ai.v2';
const legacySettingsMigrationKey = 'idios.shelf.study-ai.legacy-migration.v1';
export const threadKey = (bookID: string) => `idios.shelf.study-thread.v2.${bookID}`;
export const archivedThreadsKey = (bookID: string) => `idios.shelf.study-threads.v1.${bookID}`;
export const historyKey = (bookID: string) => `idios.shelf.read-history.v1.${bookID}`;
export const overviewKey = (bookID: string) => `idios.shelf.reading-overview.v1.${bookID}`;
// v4 stores multiple verified on-page marks with reader-friendly marginal notes.
// v5 invalidates essay-shaped cached notes so Mimi regenerates conversational notes and exact margin marks.
export const pageInsightKey = (bookID: string, page: number) => `idios.shelf.page-insight.v5.${bookID}.${page}`;
export const mascotPositionKey = (bookID: string) => `idios.shelf.mimi-position.v1.${bookID}`;
export const mascotBubbleKey = (bookID: string) => `idios.shelf.mimi-bubble-open.v1.${bookID}`;

export function loadMascotBubbleOpen(bookID: string): boolean {
  try { return localStorage.getItem(mascotBubbleKey(bookID)) !== 'closed'; }
  catch { return true; }
}

export function loadMascotPosition(bookID: string): MascotPosition | null {
  try {
    const value = JSON.parse(localStorage.getItem(mascotPositionKey(bookID)) ?? 'null') as Partial<MascotPosition> | null;
    if (value && Number.isFinite(value.left) && Number.isFinite(value.top)) return { left: Number(value.left), top: Number(value.top) };
  } catch { /* The mascot can use its default position. */ }
  return null;
}

const highlightFunctionWords = new Set((
  'a an the and or but if then than as at by for from in into of on onto to with within without about above after against along among around before behind below beneath beside between beyond despite down during except inside near off out outside over past per since through throughout till under until up upon via versus while although because therefore however moreover also very quite rather merely comfortably perhaps often just only even such same this that these those which who whom whose what when where how all any each few more most much some another other own its their his her our your my is are was were be been being do does did doing have has had having can could may might must shall should will would found said say make made get gets got seem seems appeared appears'.split(/\s+/)
))

export function quoteWords(value: string): string[] {
  return value.normalize('NFKC').match(/[\p{L}\p{N}][\p{L}\p{N}'’-]*/gu) ?? [];
}

function normalizeQuoteWord(value: string): string {
  return value.normalize('NFKC').toLocaleLowerCase().replace(/[’‘]/g, "'").replace(/[‐‑‒–—]/g, '-');
}

function verbatimOccurrenceCount(phrase: string, sourceText: string): number {
  const needle = quoteWords(phrase).map(normalizeQuoteWord);
  const source = quoteWords(sourceText).map(normalizeQuoteWord);
  if (needle.length < 1 || needle.length > 8 || !source.length) return 0;
  let count = 0;
  for (let start = 0; start <= source.length - needle.length; start += 1) {
    if (needle.every((word, offset) => source[start + offset] === word)) count += 1;
    if (count > 1) return count;
  }
  return count;
}

function cleanMarkQuote(value: string): string {
  return value.trim()
    .replace(/^(?:MARK\s*\d*\s*)?(?:QUOTE|HIGHLIGHT(?:\s+QUOTE)?)\s*[:—-]?\s*/i, '')
    .replace(/^[\s"'“”‘’`]+|[\s"'“”‘’`]+$/g, '')
    .replace(/^[^\p{L}\p{N}]+|[^\p{L}\p{N}]+$/gu, '')
    .trim();
}

export function validatedCompanionMarks(insight: PageInsight | null, pageText: string): CompanionMark[] {
  if (!insight || !pageText.trim()) return [];
  const candidates: CompanionMark[] = insight.marks?.length ? insight.marks : insight.highlightQuote?.trim()
    ? [{ quote: insight.highlightQuote, note: insight.companion, kind: 'important' }]
    : [];
  const accepted: CompanionMark[] = [];
  const seen = new Set<string>();
  for (const candidate of candidates.slice(0, 3)) {
    const quote = cleanMarkQuote(candidate.quote || '');
    if (!quote || /^(?:none|n\/a|no highlight|not applicable)$/i.test(quote) || quote.length > 86) continue;
    const words = quoteWords(quote);
    if (words.length < 1 || words.length > 8) continue;
    const first = normalizeQuoteWord(words[0]).replace(/^'+|'+$/g, '');
    const last = normalizeQuoteWord(words[words.length - 1]).replace(/^'+|'+$/g, '');
    if (words.length > 1 && (highlightFunctionWords.has(first) || highlightFunctionWords.has(last))) continue;
    if (words.length === 1 && (words[0].length < 4 || highlightFunctionWords.has(first))) continue;
    if (!words.some((word) => word.length >= 3 && !highlightFunctionWords.has(normalizeQuoteWord(word)))) continue;
    if (verbatimOccurrenceCount(quote, pageText) !== 1) continue;
    const identity = words.map(normalizeQuoteWord).join(' ');
    if (seen.has(identity)) continue;
    seen.add(identity);
    accepted.push({ quote, note: String(candidate.note || '').trim().slice(0, 240), kind: candidate.kind === 'unclear' ? 'unclear' : 'important' });
  }
  return accepted;
}

export function fallbackTopicMark(insight: PageInsight, pageText: string): CompanionMark | null {
  // If a model ignores the mark schema twice, only reuse its own topic when that title
  // is an exact, unique, compact phrase in the current page. This is a last-resort anchor,
  // never a guessed location or broad text-span highlight.
  const title = cleanMarkQuote(insight.topic || '');
  const candidates = [title.split(/\s+(?:vs\.?|versus)\s+/i)[0], title]
    .map((candidate) => cleanMarkQuote(candidate || ''))
    .filter((candidate, index, all) => candidate && all.indexOf(candidate) === index);
  for (const quote of candidates) {
    const words = quoteWords(quote);
    if (words.length < 2 || words.length > 6 || verbatimOccurrenceCount(quote, pageText) !== 1) continue;
    const first = normalizeQuoteWord(words[0]);
    const last = normalizeQuoteWord(words[words.length - 1]);
    if (highlightFunctionWords.has(first) || highlightFunctionWords.has(last)) continue;
    if (!words.some((word) => word.length >= 4 && !highlightFunctionWords.has(normalizeQuoteWord(word)))) continue;
    return { quote, kind: 'important', note: 'This looks like the main idea here. Notice how the author uses it to explain the rest of the page.' };
  }
  return null;
}

export function publishCompanionHighlights(bookID: string, page: number, marks: CompanionMark[]) {
  window.dispatchEvent(new CustomEvent('idios:shelf-companion-highlights', { detail: { bookID, pageNumber: page, marks } }));
}
const defaultSettings: AISettings = { model: '' };

function normalizeProviderURL(value: string): string {
  return value.trim().replace(/\/+$/, '');
}

function readLegacySettings(): LegacyAISettings | null {
  try {
    const migratedSnapshot = localStorage.getItem(legacySettingsMigrationKey);
    if (migratedSnapshot) return JSON.parse(migratedSnapshot) as LegacyAISettings;
    // Consider both keys independently: a prior run may already have rewritten v2
    // to model-only while an older v1 connection still needs importing.
    for (const key of [settingsKey, 'idios.shelf.study-ai.v1']) {
      const current = localStorage.getItem(key);
      if (!current) continue;
      const value = JSON.parse(current) as Partial<AISettings> & LegacyAISettings;
      if (typeof value.baseURL === 'string' || typeof value.apiKey === 'string') {
        return { baseURL: value.baseURL, apiKey: value.apiKey, model: value.model };
      }
    }
    return null;
  } catch { return null; }
}

export function loadSettings(): AISettings {
  try {
    const stored = localStorage.getItem(settingsKey) ?? localStorage.getItem('idios.shelf.study-ai.v1');
    if (stored) {
      const value = JSON.parse(stored) as Partial<AISettings> & LegacyAISettings;
      if (typeof value.baseURL === 'string' || typeof value.apiKey === 'string') {
        // Preserve old connection details in a migration-only key before removing them
        // from browser storage. Provider credentials are moved to the server-side runtime.
        localStorage.setItem(legacySettingsMigrationKey, JSON.stringify({ baseURL: value.baseURL, apiKey: value.apiKey, model: value.model }));
        localStorage.setItem(settingsKey, JSON.stringify({ model: value.model ?? '' }));
        localStorage.removeItem('idios.shelf.study-ai.v1');
      }
      return { model: typeof value.model === 'string' ? value.model : '' };
    }
  } catch { /* Browser storage can be disabled by policy. */ }
  return defaultSettings;
}

export async function migrateLegacyShelfProvider(existingProviders: AIProvider[]): Promise<boolean> {
  const legacy = readLegacySettings();
  const baseURL = normalizeProviderURL(legacy?.baseURL ?? '');
  const apiKey = legacy?.apiKey?.trim() ?? '';
  const oldModel = legacy?.model?.trim() ?? '';
  // The former Tutor default was 9Router at loopback:20128/v1. Migrate it even
  // without a saved API key/model so Refresh can discover keyless local routers.
  if (!baseURL) return false;

  let parsed: URL;
  try { parsed = new URL(baseURL); } catch { throw new Error('The saved Tutor endpoint is not a valid URL. Open AI Runtime settings to configure a provider.'); }
  const type: AIProvider['type'] = parsed.hostname.toLowerCase() === 'openrouter.ai'
    ? 'openrouter'
    : parsed.port === '11434' && !parsed.pathname.replace(/\/+$/, '').endsWith('/v1')
      ? 'ollama'
      : 'openai-compatible';
  const matching = existingProviders.find((provider) => normalizeProviderURL(provider.base_url) === baseURL);
  if (matching) {
    if ((apiKey && !matching.has_api_key) || (oldModel && !matching.default_model)) {
      await api.updateAIProvider(matching.id, {
        id: matching.id,
        name: matching.name,
        type: matching.type,
        base_url: matching.base_url,
        api_key: apiKey,
        default_model: matching.default_model || oldModel,
        enabled: matching.enabled,
        timeout_seconds: matching.timeout_seconds,
        max_concurrent: matching.max_concurrent,
        trusted_local: matching.trusted_local,
      });
    }
  } else {
    await api.createAIProvider({
      name: type === 'openrouter' ? 'OpenRouter (migrated from Tutor)' : type === 'ollama' ? 'Ollama (migrated from Tutor)' : /:20128(?:\/|$)/.test(`${parsed.host}${parsed.pathname}`) ? '9Router (migrated from Tutor)' : 'Shelf AI provider (migrated)',
      type,
      base_url: baseURL,
      api_key: apiKey,
      default_model: oldModel,
      enabled: true,
      timeout_seconds: 120,
      max_concurrent: 2,
      trusted_local: false,
    });
    // Loopback is already treated as local by the backend; do not mark arbitrary
    // network endpoints trusted merely because they were present in old settings.
  }
  try { localStorage.removeItem(legacySettingsMigrationKey); localStorage.removeItem('idios.shelf.study-ai.v1'); } catch { /* The server has the migrated config; ignore storage cleanup failures. */ }
  return true;
}

export async function runShelfAIThroughRuntime(modelRef: string, messages: ShelfRuntimeMessage[], reasoningEffort?: ReasoningEffort): Promise<ShelfRuntimeResponse> {
  const separator = modelRef.indexOf('/');
  if (separator < 1 || separator === modelRef.length - 1) throw new Error('Choose a model from AI Runtime settings before sending a message.');
  const providerID = modelRef.slice(0, separator);
  const modelID = modelRef.slice(separator + 1);
  const result = await api.aiChat({
    provider_id: providerID,
    model: modelID,
    task: 'reading_analysis',
    ...(reasoningEffort ? { reasoning_effort: reasoningEffort } : {}),
    messages,
  });
  return { model: `${result.provider_id}/${result.model}`, choices: [{ message: { content: result.content } }] };
}
export const emptyContext: StudyContext = { book: null, notes: [], annotations: [], previousQuestions: [], relations: [], relatedBooks: [], loading: true };
export const overviewPrompt = `We're opening this book together for the first time. Talk to me like a thoughtful friend who has just looked at the title, description, and available opening pages. In 3–5 short, natural paragraphs, tell me what this book seems to be about, what question or idea makes it interesting, and what I might get out of reading it. Mention just 2–3 useful things I may learn and one practical tip for reading it. Finish with one small, intriguing thing to watch for as we read. No numbered report, no academic headings, no generic book-review template. Use only supplied evidence; be honest if the available context is thin, and never invent an author biography or table of contents.`;


export function loadThread(bookID: string): ChatMessage[] {
  try {
    const stored = localStorage.getItem(threadKey(bookID)) ?? localStorage.getItem(`idios.shelf.study-thread.v1.${bookID}`);
    if (!stored) return [];
    const value = JSON.parse(stored) as ChatMessage[];
    return Array.isArray(value) ? normalizeStoredChatMessages(value).slice(-60) : [];
  } catch { return []; }
}

export function loadArchivedThreads(bookID: string): ArchivedThread[] {
  try {
    const stored = JSON.parse(localStorage.getItem(archivedThreadsKey(bookID)) ?? '[]') as ArchivedThread[];
    return Array.isArray(stored) ? stored.filter((item) => typeof item.id === 'string' && Array.isArray(item.messages)).map((item) => ({ ...item, messages: normalizeStoredChatMessages(item.messages) })).slice(0, 15) : [];
  } catch { return []; }
}

export function archivedThreadSnapshot(messages: ChatMessage[]): ArchivedThread | null {
  if (!messages.length) return null;
  const firstPrompt = messages.find((message) => message.role === 'user' && !message.hidden)?.content?.trim();
  return {
    id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    title: firstPrompt ? firstPrompt.replace(/\s+/g, ' ').slice(0, 68) : 'Reading discussion',
    messages: messages.slice(-60),
    updatedAt: new Date().toISOString(),
  };
}

export function loadReadHistory(bookID: string): ReadPage[] {
  try {
    const value = JSON.parse(localStorage.getItem(historyKey(bookID)) ?? '[]') as ReadPage[];
    return Array.isArray(value) ? value.filter((item) => Number.isFinite(item.page) && typeof item.text === 'string').slice(-16) : [];
  } catch { return []; }
}

export function rememberReadPage(bookID: string, page: number, text: string) {
  const excerpt = text.trim().slice(0, 1400);
  if (!excerpt || page < 1) return;
  try {
    const pages = loadReadHistory(bookID).filter((item) => item.page !== page);
    pages.push({ page, text: excerpt, visitedAt: new Date().toISOString() });
    localStorage.setItem(historyKey(bookID), JSON.stringify(pages.slice(-16)));
  } catch { /* Reading remains usable if storage is full. */ }
}

export function getContent(value: string | Array<{ type?: string; text?: string }> | undefined): string {
  if (typeof value === 'string') return value;
  return Array.isArray(value) ? value.map((part) => part.text ?? '').join('') : '';
}

export function normalizeStoredChatMessages(value: unknown[]): ChatMessage[] {
  return value.filter((item): item is ChatMessage => Boolean(item && typeof item === 'object'
    && ((item as ChatMessage).role === 'user' || (item as ChatMessage).role === 'assistant')
    && typeof (item as ChatMessage).content === 'string')).map((item) => {
      if (item.role !== 'assistant') return item;
      const separated = separateModelThinking(item.content, item.reasoning || '');
      return { ...item, content: separated.answer, ...(separated.thinking ? { reasoning: separated.thinking } : { reasoning: undefined }) };
    });
}

export function loadPageInsight(bookID: string, page: number): PageInsight | null {
  try {
    const raw = localStorage.getItem(pageInsightKey(bookID, page));
    if (!raw) return null;
    const value = JSON.parse(raw) as PageInsight;
    return value.page === page && typeof value.companion === 'string' ? value : null;
  } catch { return null; }
}

export function parsePageInsight(raw: string, page: number): PageInsight {
  const normalized = raw
    .replace(/^#{1,4}\s*/gm, '')
    .replace(/\*\*(MARK\s+\d+\s+(?:KIND|QUOTE|NOTE)|TOPIC|SUBJECT|COMPANION|TUTOR(?:\s+ADDS|\s+SAYS|\s+ANGLE)?|LOOK FOR|NOTICE|NEXT LENS|GENTLE CHALLENGE|CHALLENGE|CURIOUS QUESTION|HIGHLIGHT QUOTE):\*\*/gi, '$1:')
    .replace(/\*\*(MARK\s+\d+\s+(?:KIND|QUOTE|NOTE)|TOPIC|SUBJECT|COMPANION|TUTOR(?:\s+ADDS|\s+SAYS|\s+ANGLE)?|LOOK FOR|NOTICE|NEXT LENS|GENTLE CHALLENGE|CHALLENGE|CURIOUS QUESTION|HIGHLIGHT QUOTE)\*\*\s*:/gi, '$1:')
    .replace(/^\s*[-*]\s*(?=(?:MARK\s+\d+\s+(?:KIND|QUOTE|NOTE)|TOPIC|SUBJECT|COMPANION|TUTOR(?:\s+ADDS)?|LOOK FOR|NOTICE|GENTLE CHALLENGE|CHALLENGE|CURIOUS QUESTION|HIGHLIGHT QUOTE)\s*[:—-])/gim, '');
  const section = (label: string, nextLabels: string[]) => {
    const next = nextLabels.map((item) => item.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|');
    const match = normalized.match(new RegExp(`(?:^|\\n)\\s*(?:${label})\\s*[:—-]\\s*([\\s\\S]*?)(?=\\n\\s*(?:${next})\\s*[:—-]|$)`, 'i'));
    return match?.[1]?.trim().replace(/^\s*[-*]\s*/, '') ?? '';
  };
  const allMarkLabels = ['MARK 1 KIND', 'MARK 1 QUOTE', 'MARK 1 NOTE', 'MARK 2 KIND', 'MARK 2 QUOTE', 'MARK 2 NOTE', 'MARK 3 KIND', 'MARK 3 QUOTE', 'MARK 3 NOTE'];
  const topicSection = section('(?:TOPIC|SUBJECT)', ['HIGHLIGHT QUOTE', 'COMPANION', 'TUTOR ADDS', 'TUTOR SAYS', 'TUTOR ANGLE', 'LOOK FOR', 'NOTICE', 'GENTLE CHALLENGE', 'CHALLENGE', 'CURIOUS QUESTION', ...allMarkLabels]);
  const highlightQuote = section('HIGHLIGHT QUOTE', ['TOPIC', 'SUBJECT', 'COMPANION', 'TUTOR ADDS', 'TUTOR SAYS', 'TUTOR ANGLE', 'LOOK FOR', 'NOTICE', 'GENTLE CHALLENGE', 'CHALLENGE', 'CURIOUS QUESTION', ...allMarkLabels]);
  const companionSection = section('COMPANION', ['TUTOR ADDS', 'TUTOR SAYS', 'TUTOR ANGLE', 'LOOK FOR', 'NOTICE', 'TOPIC', 'SUBJECT', 'GENTLE CHALLENGE', 'CHALLENGE', 'CURIOUS QUESTION', 'HIGHLIGHT QUOTE', ...allMarkLabels]);
  const tutorReply = section('(?:TUTOR ADDS|TUTOR SAYS|TUTOR ANGLE|TUTOR)', ['LOOK FOR', 'NOTICE', 'TOPIC', 'SUBJECT', 'COMPANION', 'GENTLE CHALLENGE', 'CHALLENGE', 'CURIOUS QUESTION', 'HIGHLIGHT QUOTE', ...allMarkLabels]);
  const lookFor = section('(?:LOOK FOR|NOTICE|NEXT LENS)', ['TOPIC', 'SUBJECT', 'COMPANION', 'TUTOR ADDS', 'GENTLE CHALLENGE', 'CHALLENGE', 'CURIOUS QUESTION', 'HIGHLIGHT QUOTE', ...allMarkLabels]);
  const challenge = section('(?:GENTLE CHALLENGE|CHALLENGE)', ['CURIOUS QUESTION', 'TOPIC', 'SUBJECT', 'COMPANION', 'TUTOR ADDS', 'LOOK FOR', 'HIGHLIGHT QUOTE', ...allMarkLabels]);
  const curiousQuestion = section('CURIOUS QUESTION', ['TOPIC', 'SUBJECT', 'COMPANION', 'TUTOR ADDS', 'LOOK FOR', 'GENTLE CHALLENGE', 'CHALLENGE', 'HIGHLIGHT QUOTE', ...allMarkLabels]);
  const cleaned = (value: string) => value.replace(/^\*{1,3}|\*{1,3}$/g, '').trim();
  const marks: CompanionMark[] = [1, 2, 3].flatMap((index) => {
    const next = [...allMarkLabels, 'TOPIC', 'SUBJECT', 'COMPANION', 'TUTOR ADDS', 'LOOK FOR', 'GENTLE CHALLENGE', 'CHALLENGE', 'CURIOUS QUESTION', 'HIGHLIGHT QUOTE'];
    const kind = section(`MARK\\s+${index}\\s+KIND`, next);
    const quote = section(`MARK\\s+${index}\\s+QUOTE`, next);
    const note = section(`MARK\\s+${index}\\s+NOTE`, next);
    if (!quote || /^(?:none|n\/a)$/i.test(quote.trim())) return [];
    return [{ quote: cleanMarkQuote(quote), note: cleaned(note), kind: /^unclear/i.test(kind) ? 'unclear' as const : 'important' as const }];
  });
  if (!marks.length && highlightQuote && !/^(?:none|n\/a)$/i.test(highlightQuote.trim())) marks.push({ quote: cleanMarkQuote(highlightQuote), note: cleaned(companionSection), kind: 'important' });

  // Older/weaker models may return a normal essay instead of the field contract.
  // Recover only the title and first useful paragraph; never display the entire essay as one bubble.
  const plain = raw.replace(/\r/g, '').trim();
  const plainLines = plain.split('\n').map((line) => line.trim()).filter(Boolean);
  const stripMarkdown = (value: string) => value.replace(/^#{1,4}\s*/, '').replace(/^\*{1,3}|\*{1,3}$/g, '').replace(/[`*_]/g, '').trim();
  const firstLine = stripMarkdown(plainLines[0] ?? '');
  const looksLikeHeading = Boolean(firstLine && firstLine.length <= 96 && quoteWords(firstLine).length <= 10 && !/[.!?]$/.test(firstLine) && !/^(?:TOPIC|SUBJECT|COMPANION|MARK\s+\d+|CURIOUS QUESTION|LOOK FOR)\s*:/i.test(firstLine));
  const fallbackTopic = looksLikeHeading ? firstLine : '';
  const compact = (value: string) => {
    const text = stripMarkdown(value).replace(/\s+/g, ' ');
    if (!text) return '';
    const sentences = text.match(/[^.!?]+[.!?]+|[^.!?]+$/g) ?? [text];
    const short = sentences.slice(0, 2).join(' ').trim();
    const words = short.split(/\s+/);
    return words.length > 38 ? `${words.slice(0, 38).join(' ').replace(/[,:;—-]+$/, '')}…` : short;
  };
  const topic = cleaned(topicSection || fallbackTopic || 'A thought from this page');
  const companion = cleaned(companionSection ? compact(companionSection) : '');
  return { page, topic, companion: companion || 'I’d pause on this idea for a moment—it may help connect the points on this page.', tutorReply: cleaned(tutorReply), lookFor: cleaned(lookFor), challenge: cleaned(challenge), curiousQuestion: cleaned(curiousQuestion), highlightQuote: cleaned(highlightQuote), marks, createdAt: new Date().toISOString() };
}

export function userFacingAIError(cause: unknown): string {
  const message = cause instanceof Error ? cause.message : 'The AI request failed.';
  if (/missing api key|api key required|unauthorized|invalid api key|authentication|invalid token|\b401\b|\b403\b/i.test(message)) return 'The gateway rejected this API key. Check the key and try again.';
  if (/could not reach|failed to fetch|connection refused|networkerror|\b502\b|\b504\b/i.test(message)) return 'The router could not be reached. Check that the endpoint is running, then refresh and test the models again.';
  if (/model.*(not found|unknown|unavailable)|no such model|\b404\b/i.test(message)) return 'This model is no longer reachable. Refresh the provider catalogue, then select a model to verify it.';
  if (/rate limit|\b429\b|quota|insufficient credits/i.test(message)) return 'This provider is rate-limited or out of quota right now. Try a different available model.';
  return message;
}

