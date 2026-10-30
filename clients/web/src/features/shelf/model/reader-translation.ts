export type ReaderTranslationKind = 'page' | 'selection' | 'word';

export type ReaderTranslationRequest = {
  text: string;
  pageText: string;
  pageNumber: number;
  targetLanguage: string;
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
