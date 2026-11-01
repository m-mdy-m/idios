export type ReaderTranslationKind = 'page' | 'chunk' | 'selection' | 'word' | 'grammar' | 'ielts';

export type ReaderTranslationCompletion = { content: string; providerID: string; model: string; durationMS: number };
export type ReaderTranslationModelOption = { value: string; label: string; availability: 'available' | 'unknown'; local: boolean; timeoutSeconds: number };

export type ReaderTranslationRequest = {
  text: string;
  pageText: string;
  pageNumber: number;
  targetLanguage: string;
  modelRef: string;
  timeoutSeconds?: number;
  signal?: AbortSignal;
  kind: ReaderTranslationKind;
  documentContext: string;
  priorGlossary: Array<{ source: string; translation: string; meaning: string }>;
  onStream?: (rawContent: string) => void;
};

export type ReaderTranslationWordAlignment = { source: string; translation: string };
export type ReaderTranslationSegment = { source: string; translation: string; words?: ReaderTranslationWordAlignment[] };
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

/** Lossless identity for exact extracted source text. */
export function translationSourceIdentity(value: string): string {
  return value.length + ':' + encodeURIComponent(value);
}

export function normalizeTranslationMatch(value: string): string {
  return value.normalize('NFKC').toLocaleLowerCase().replace(/[’‘]/g, "'").replace(/\s+/g, ' ').trim();
}

/**
 * Rebuild PDF.js text items using their baseline and advance width when available.
 * The fallback preserves stream-order behavior for PDFs without usable geometry.
 */
export function extractReaderPDFText(items: readonly unknown[]): string {
  type PositionedItem = {
    text: string; x: number; y: number; width: number; height: number;
    direction: 'ltr' | 'rtl'; hasEOL: boolean; positioned: boolean;
  };
  const parsed: PositionedItem[] = items.map((item) => {
    const record = asRecord(item);
    const transform = Array.isArray(record.transform) ? record.transform.map(Number) : [];
    const x = Number(transform[4]);
    const y = Number(transform[5]);
    const width = Number(record.width);
    const explicitHeight = Number(record.height);
    const transformHeight = Math.hypot(Number(transform[2]) || 0, Number(transform[3]) || 0);
    const height = Math.max(
      Number.isFinite(explicitHeight) ? Math.abs(explicitHeight) : 0,
      Number.isFinite(transformHeight) ? transformHeight : 0,
      1,
    );
    return {
      text: typeof record.str === 'string' ? record.str.replace(/\u00ad/g, '') : '',
      x: Number.isFinite(x) ? x : 0,
      y: Number.isFinite(y) ? y : 0,
      width: Number.isFinite(width) ? Math.max(0, Math.abs(width)) : 0,
      height,
      direction: record.dir === 'rtl' ? ('rtl' as const) : ('ltr' as const),
      hasEOL: record.hasEOL === true,
      positioned: transform.length >= 6 && Number.isFinite(x) && Number.isFinite(y) && Number.isFinite(width),
    };
  }).filter((item) => item.text.length > 0);

  const geometryAvailable = parsed.length > 1 && parsed.every((item) => item.positioned);
  let output = '';
  let previous: PositionedItem | null = null;
  const openingPunctuation = /^[\s([{«“‘‹]/u;
  const closingPunctuation = /^[,.;:!?%)\]}»”’›،؛؟]/u;

  for (const item of parsed) {
    if (previous) {
      if (geometryAvailable) {
        const fontHeight = Math.max(1, Math.min(previous.height, item.height));
        const verticalDistance = Math.abs(previous.y - item.y);
        if (previous.hasEOL || verticalDistance > fontHeight * 0.58) {
          output += verticalDistance > fontHeight * 1.48 ? '\n\n' : '\n';
        } else {
          const horizontalGap = item.direction === 'rtl'
            ? previous.x - (item.x + item.width)
            : item.x - (previous.x + previous.width);
          const needsSpace = horizontalGap > Math.max(1, fontHeight * 0.18);
          if (needsSpace && !closingPunctuation.test(item.text) && !openingPunctuation.test(previous.text.slice(-1))) {
            output += ' ';
          }
        }
      } else if (previous.hasEOL) {
        output += '\n';
      } else if (!closingPunctuation.test(item.text) && !openingPunctuation.test(previous.text.slice(-1))) {
        output += ' ';
      }
    }
    output += item.text;
    previous = item;
  }

  return output
    .replace(/\u00ad/g, '')
    .replace(/-\s*\r?\n\s*(?=\p{Ll})/gu, '')
    .replace(/[ \t]*\r?\n[ \t]*/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .replace(/[ \t]{2,}/g, ' ')
    .trim();
}

/** Normalize PDF selection text for AI while preserving the original quote for saving. */
export function normalizeReaderSelectionText(value: string): string {
  return value
    .normalize('NFKC')
    .replace(/[\u00ad\u200b\ufeff]/g, '')
    .replace(/[-‐‑‒–—][ \t]*\r?\n[ \t]*(?=\p{Ll})/gu, '')
    .replace(/\r?\n+/g, ' ')
    .replace(/[ \t]+/g, ' ')
    .trim();
}

/**
 * Read the translation JSON string while it is still arriving over SSE.
 * Returns null until the translation key starts; incomplete escape sequences
 * are held back so the visible preview never displays broken JSON syntax.
 */
export function extractPartialReaderTranslation(raw: string): string | null {
  const key = /"translation"\s*:\s*"/g;
  const match = key.exec(raw);
  if (!match) return null;
  let output = '';
  for (let index = match.index + match[0].length; index < raw.length; index += 1) {
    const character = raw[index];
    if (character === '"') return output;
    if (character !== '\\') {
      output += character;
      continue;
    }
    const escaped = raw[index + 1];
    if (escaped === undefined) break;
    const simple: Record<string, string> = {
      '"': '"', '\\': '\\', '/': '/', b: '\b', f: '\f', n: '\n', r: '\r', t: '\t',
    };
    if (Object.prototype.hasOwnProperty.call(simple, escaped)) {
      output += simple[escaped];
      index += 1;
      continue;
    }
    if (escaped === 'u') {
      const hex = raw.slice(index + 2, index + 6);
      if (!/^[\da-f]{4}$/i.test(hex)) break;
      output += String.fromCharCode(Number.parseInt(hex, 16));
      index += 5;
      continue;
    }
    output += escaped;
    index += 1;
  }
  return output;
}

function containsAlignedPhrase(container: string, candidate: string): boolean {
  const normalizedContainer = normalizeTranslationMatch(container);
  const normalizedCandidate = normalizeTranslationMatch(candidate);
  if (!normalizedContainer || !normalizedCandidate) return false;
  let position = normalizedContainer.indexOf(normalizedCandidate);
  while (position >= 0) {
    const before = position > 0 ? normalizedContainer[position - 1] : '';
    const after = normalizedContainer[position + normalizedCandidate.length] ?? '';
    const isWordCharacter = (character: string) => Boolean(character && /[\p{L}\p{N}]/u.test(character));
    if (!isWordCharacter(before) && !isWordCharacter(after)) return true;
    position = normalizedContainer.indexOf(normalizedCandidate, position + 1);
  }
  return false;
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
export function mergeReaderTranslations(parts: ReaderTranslationData[], separators: string[] = []): ReaderTranslationData {
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
    translation: (() => {
      let combined = '';
      let previousIndex = -1;
      for (const [index, part] of parts.entries()) {
        const next = part.translation.trim();
        if (!next) continue;
        if (previousIndex >= 0) {
          const interveningSeparators = separators.slice(previousIndex, index);
          const separator = interveningSeparators.find((value) => value.includes('\n\n'))
            ?? interveningSeparators.find((value) => value.includes('\n'))
            ?? ' ';
          combined += separator;
        }
        combined += next;
        previousIndex = index;
      }
      return combined;
    })(),
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
  if (start < 0 || end <= start) {
    const partial = extractPartialReaderTranslation(cleaned)?.trim();
    return partial ? { ...fallback, translation: partial, segments: [{ source: sourceText.trim(), translation: partial }] } : fallback;
  }

  try {
    const parsed = asRecord(JSON.parse(cleaned.slice(start, end + 1)));
    const hasTranslationField = Object.prototype.hasOwnProperty.call(parsed, 'translation');
    const translation = readString(parsed.translation) || (hasTranslationField ? '' : fallbackText);
    const sourceNormalized = normalizeTranslationMatch(sourceText);
    const rawSegments = Array.isArray(parsed.segments) ? parsed.segments : [];
    const candidateSegments = rawSegments.map((item) => {
      const segment = asRecord(item);
      const segmentSource = readString(segment.source);
      const segmentTranslation = readString(segment.translation);
      const seenWords = new Set<string>();
      const words = (Array.isArray(segment.words) ? segment.words : [])
        .map((wordItem) => {
          const word = asRecord(wordItem);
          return { source: readString(word.source), translation: readString(word.translation) };
        })
        .filter((word) => {
          const sourceKey = normalizeTranslationMatch(word.source);
          const translationKey = normalizeTranslationMatch(word.translation);
          if (!sourceKey || !translationKey || seenWords.has(sourceKey)) return false;
          if (!containsAlignedPhrase(segmentSource, word.source) || !containsAlignedPhrase(segmentTranslation, word.translation)) return false;
          seenWords.add(sourceKey);
          return true;
        })
        .slice(0, 100);
      return {
        source: segmentSource,
        translation: segmentTranslation,
        ...(words.length ? { words } : {}),
      };
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
    const partial = extractPartialReaderTranslation(cleaned)?.trim();
    return partial ? { ...fallback, translation: partial, segments: [{ source: sourceText.trim(), translation: partial }] } : fallback;
  }
}

export type MixedDirectionPart = {
  text: string;
  direction: 'ltr' | 'inherit';
};

/** Split embedded Latin words and Western numerals from RTL prose for bidi isolation. */
export function splitMixedDirectionText(value: string): MixedDirectionPart[] {
  const parts = value.split(/([A-Za-z][A-Za-z0-9_’'–—-]*(?:[ \t]+[A-Za-z][A-Za-z0-9_’'–—-]*)*|[0-9]+(?:[.,:/-][0-9]+)*)/g).filter(Boolean);
  return parts.map((text) => ({ text, direction: /^[A-Za-z0-9]/.test(text) ? 'ltr' : 'inherit' }));
}
