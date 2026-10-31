import { normalizeTranslationMatch, type ReaderTranslationData } from '../reader-translation.ts';

export type TranslationIntegrity = {
  valid: boolean;
  verified: boolean;
  reason?: 'empty-source' | 'empty-translation' | 'malformed-structured-response' | 'missing-segments' | 'source-mismatch' | 'segment-order' | 'segment-coverage';
  coverage: number;
};

function stripFence(value: string): string {
  return value.trim().replace(/^```(?:json)?\s*/i, '').replace(/\s*```$/i, '').trim();
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

/**
 * Validate model-provided source anchors before parseReaderTranslation can fall back
 * to a single whole-passage segment. Plain-text model responses remain usable but
 * are explicitly marked unverified because they contain no source alignment metadata.
 */
export function validateRawTranslationAlignment(raw: string, sourceText: string): TranslationIntegrity {
  const source = normalizeTranslationMatch(sourceText);
  if (!source) return { valid: false, verified: false, reason: 'empty-source', coverage: 0 };

  const cleaned = stripFence(raw);
  if (!cleaned.startsWith('{')) {
    return { valid: Boolean(cleaned), verified: false, ...(cleaned ? {} : { reason: 'empty-translation' as const }), coverage: 0 };
  }

  let parsed: Record<string, unknown>;
  try {
    const value: unknown = JSON.parse(cleaned);
    if (!isRecord(value)) throw new Error('Structured response must be an object.');
    parsed = value;
  } catch {
    return { valid: false, verified: false, reason: 'malformed-structured-response', coverage: 0 };
  }

  if (typeof parsed.translation !== 'string' || !parsed.translation.trim()) {
    return { valid: false, verified: true, reason: 'empty-translation', coverage: 0 };
  }
  if (!Array.isArray(parsed.segments) || parsed.segments.length === 0) {
    return { valid: false, verified: true, reason: 'missing-segments', coverage: 0 };
  }

  let cursor = 0;
  let covered = 0;
  for (const entry of parsed.segments) {
    if (!isRecord(entry) || typeof entry.source !== 'string' || typeof entry.translation !== 'string' || !entry.translation.trim()) {
      return { valid: false, verified: true, reason: 'source-mismatch', coverage: covered / Math.max(1, source.length) };
    }
    const fragment = normalizeTranslationMatch(entry.source);
    if (!fragment) return { valid: false, verified: true, reason: 'source-mismatch', coverage: covered / Math.max(1, source.length) };
    const position = source.indexOf(fragment, cursor);
    if (position < 0) {
      const anywhere = source.indexOf(fragment);
      return {
        valid: false,
        verified: true,
        reason: anywhere >= 0 ? 'segment-order' : 'source-mismatch',
        coverage: covered / Math.max(1, source.length),
      };
    }
    cursor = position + fragment.length;
    covered += fragment.length;
  }

  const coverage = covered / Math.max(1, source.length);
  if (coverage < 0.72) return { valid: false, verified: true, reason: 'segment-coverage', coverage };
  return { valid: true, verified: true, coverage };
}

/** Validate parsed translation data as a secondary guard for cached/legacy values. */
export function validateTranslationIntegrity(sourceText: string, data: ReaderTranslationData): TranslationIntegrity {
  const source = normalizeTranslationMatch(sourceText);
  const translation = data.translation.trim();
  if (!source) return { valid: false, verified: false, reason: 'empty-source', coverage: 0 };
  if (!translation) return { valid: false, verified: false, reason: 'empty-translation', coverage: 0 };

  const segments = data.segments.filter((segment) => segment.source.trim() && segment.translation.trim());
  if (!segments.length) return { valid: true, verified: false, coverage: 0 };

  let cursor = 0;
  let covered = 0;
  for (const segment of segments) {
    const fragment = normalizeTranslationMatch(segment.source);
    const position = source.indexOf(fragment, cursor);
    if (!fragment || position < 0) {
      return { valid: false, verified: true, reason: 'source-mismatch', coverage: covered / Math.max(1, source.length) };
    }
    cursor = position + fragment.length;
    covered += fragment.length;
  }
  const coverage = covered / Math.max(1, source.length);
  if (coverage < 0.72) return { valid: false, verified: true, reason: 'segment-coverage', coverage };
  return { valid: true, verified: true, coverage };
}
