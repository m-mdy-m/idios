export type ReaderTranslationKind = 'page' | 'chunk' | 'selection' | 'word';

export type ReaderTranslationCompletion = { content: string; providerID: string; model: string; durationMS: number };
export type ReaderTranslationModelOption = { value: string; label: string; availability: 'available' | 'unknown'; local: boolean; timeoutSeconds: number };

export type ReaderTranslationRequest = {
  text: string;
  pageText: string;
  pageNumber: number;
  targetLanguage: string;
  modelRef: string;
  kind: ReaderTranslationKind;
  documentContext: string;
  priorGlossary: Array<{ source: string; translation: string; meaning: string }>;
};

export type ReaderTranslationSegment = { source: string; translation: string };
export type ReaderTranslationTerm = {
  source: string;
  translation: string;
  meaning: string;
  partOfSpeech?: string;
  example?: string;
};
export type ReaderGrammarNote = { pattern: string; explanation: string; example?: string };
export type ReaderIELTSTip = { expression: string; academicUse: string; example?: string };
export type ReaderTranslationData = {
  translation: string;
  segments: ReaderTranslationSegment[];
  terms: ReaderTranslationTerm[];
  grammar: ReaderGrammarNote[];
  ielts: ReaderIELTSTip[];
  translatorNote?: string;
};

function asRecord(value: unknown): Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}

function readString(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

export function normalizeTranslationMatch(value: string): string {
  return value.normalize('NFKC').toLocaleLowerCase().replace(/[’‘]/g, "'").replace(/\s+/g, ' ').trim();
}

function stripCodeFence(value: string): string {
  return value.trim().replace(/^\x60{3}(?:json)?\s*/i, '').replace(/\s*\x60{3}$/i, '').trim();
}

/**
 * Parse the structured translation response. Keep a plain-text fallback for smaller
 * models that do not follow the JSON contract; never discard a non-empty translation.
 */

/** Split long PDF text at sentence/paragraph boundaries without dropping source characters. */
export function splitReaderTranslationText(value: string, maxLength = 1100): string[] {
  const source = value.trim();
  if (!source) return [];
  const limit = Math.max(350, maxLength);
  const chunks: string[] = [];
  let start = 0;
  while (start < source.length) {
    if (source.length - start <= limit) {
      chunks.push(source.slice(start).trim());
      break;
    }
    const hardEnd = start + limit;
    const searchStart = start + Math.floor(limit * 0.55);
    let boundary = -1;
    for (let index = hardEnd; index >= searchStart; index -= 1) {
      const character = source[index - 1];
      if (character === '\n' || character === '.' || character === '?' || character === '!' || character === ';') {
        boundary = index;
        break;
      }
    }
    if (boundary < 0) {
      for (let index = hardEnd; index >= searchStart; index -= 1) {
        if (/\s/.test(source[index - 1] ?? '')) { boundary = index; break; }
      }
    }
    if (boundary <= start) boundary = hardEnd;
    const chunk = source.slice(start, boundary).trim();
    if (chunk) chunks.push(chunk);
    start = boundary;
    while (start < source.length && /\s/.test(source[start])) start += 1;
  }
  return chunks;
}
export function mergeReaderTranslations(parts: ReaderTranslationData[]): ReaderTranslationData {
  const uniqueBy = <T,>(items: T[], keyOf: (item: T) => string, limit: number): T[] => {
    const seen = new Set<string>();
    const result: T[] = [];
    for (const item of items) {
      const key = keyOf(item).toLocaleLowerCase();
      if (!key || seen.has(key)) continue;
      seen.add(key);
      result.push(item);
      if (result.length >= limit) break;
    }
    return result;
  };
  const notes = uniqueBy(parts.map((part) => part.translatorNote ?? '').filter(Boolean), (item) => item, 6);
  return {
    translation: parts.map((part) => part.translation.trim()).filter(Boolean).join(' '),
    segments: parts.flatMap((part) => part.segments),
    terms: uniqueBy(parts.flatMap((part) => part.terms), (item) => item.source, 24),
    grammar: uniqueBy(parts.flatMap((part) => part.grammar), (item) => item.pattern, 10),
    ielts: uniqueBy(parts.flatMap((part) => part.ielts), (item) => item.expression, 10),
    ...(notes.length ? { translatorNote: notes.join(' · ') } : {}),
  };
}

export function parseReaderTranslation(raw: string, sourceText: string): ReaderTranslationData {
  const fallbackText = raw.trim();
  const fallback: ReaderTranslationData = {
    translation: fallbackText,
    segments: fallbackText ? [{ source: sourceText.trim(), translation: fallbackText }] : [],
    terms: [],
    grammar: [],
    ielts: [],
  };
  if (!fallbackText) return fallback;

  const cleaned = stripCodeFence(fallbackText);
  const start = cleaned.indexOf('{');
  const end = cleaned.lastIndexOf('}');
  if (start < 0 || end <= start) return fallback;

  try {
    const parsed = asRecord(JSON.parse(cleaned.slice(start, end + 1)));
    const translation = readString(parsed.translation) || fallbackText;
    const sourceNormalized = normalizeTranslationMatch(sourceText);
    const rawSegments = Array.isArray(parsed.segments) ? parsed.segments : [];
    const candidateSegments = rawSegments.map((item) => {
      const segment = asRecord(item);
      return { source: readString(segment.source), translation: readString(segment.translation) };
    }).filter((item) => item.source && item.translation);
    const segments: ReaderTranslationSegment[] = [];
    let sourceCursor = 0;
    let coveredCharacters = 0;
    for (const segment of candidateSegments) {
      const normalizedSegment = normalizeTranslationMatch(segment.source);
      const position = sourceNormalized.indexOf(normalizedSegment, sourceCursor);
      if (!normalizedSegment || position < 0) continue;
      segments.push(segment);
      sourceCursor = position + normalizedSegment.length;
      coveredCharacters += normalizedSegment.length;
    }
    // A partial or out-of-order list must not masquerade as a complete aligned page.
    // Fall back to the model's complete translation as one block in that case.
    const segmentCoverage = coveredCharacters / Math.max(1, sourceNormalized.length);
    const terms = (Array.isArray(parsed.terms) ? parsed.terms : []).map((item) => {
      const term = asRecord(item);
      return {
        source: readString(term.source),
        translation: readString(term.translation),
        meaning: readString(term.meaning),
        ...(readString(term.partOfSpeech) ? { partOfSpeech: readString(term.partOfSpeech) } : {}),
        ...(readString(term.example) ? { example: readString(term.example) } : {}),
      };
    }).filter((item) => item.source && item.translation && item.meaning && sourceNormalized.includes(normalizeTranslationMatch(item.source))).slice(0, 24);
    const grammar = (Array.isArray(parsed.grammar) ? parsed.grammar : []).map((item) => {
      const note = asRecord(item);
      return {
        pattern: readString(note.pattern),
        explanation: readString(note.explanation),
        ...(readString(note.example) ? { example: readString(note.example) } : {}),
      };
    }).filter((item) => item.pattern && item.explanation && sourceNormalized.includes(normalizeTranslationMatch(item.pattern))).slice(0, 10);
    const ielts = (Array.isArray(parsed.ielts) ? parsed.ielts : []).map((item) => {
      const tip = asRecord(item);
      return {
        expression: readString(tip.expression),
        academicUse: readString(tip.academicUse),
        ...(readString(tip.example) ? { example: readString(tip.example) } : {}),
      };
    }).filter((item) => item.expression && item.academicUse && sourceNormalized.includes(normalizeTranslationMatch(item.expression))).slice(0, 10);
    const validSegments = segments.length && segmentCoverage >= 0.72 ? segments : (translation ? [{ source: sourceText.trim(), translation }] : []);
    return {
      translation,
      segments: validSegments,
      terms,
      grammar,
      ielts,
      ...(readString(parsed.translatorNote) ? { translatorNote: readString(parsed.translatorNote) } : {}),
    };
  } catch {
    return fallback;
  }
}
