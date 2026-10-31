import type { ReaderTranslationTerm } from '../reader-translation.ts';

export type TranslationContextInput = {
  bookTitle: string;
  bookSummary?: string;
  subjectTags?: string[];
  pageNumber: number;
  previousContext?: string;
  glossary?: ReaderTranslationTerm[];
};

const MAX_CONTEXT = 5200;

/** Build bounded, explicit context. Context disambiguates the source; it is never translatable input. */
export function buildTranslationContext(input: TranslationContextInput): string {
  const glossary = (input.glossary ?? [])
    .filter((term) => term.source.trim() && term.translation.trim() && term.meaning.trim())
    .slice(-24)
    .map((term) => `- ${term.source} → ${term.translation} (${term.meaning})`)
    .join('\n');
  return [
    `Book: ${input.bookTitle.trim()}`,
    input.bookSummary?.trim() ? `Subject context: ${input.bookSummary.trim().slice(0, 900)}` : '',
    input.subjectTags?.length ? `Subject tags: ${input.subjectTags.join(', ').slice(0, 500)}` : '',
    `Current PDF page: ${Math.max(1, Math.floor(input.pageNumber))}`,
    input.previousContext?.trim() ? `Previous reading context (interpretation only; do not translate):\n${input.previousContext.trim().slice(0, 3000)}` : '',
    glossary ? `Established terminology (reuse only when the local sense matches):\n${glossary}` : '',
  ].filter(Boolean).join('\n\n').slice(0, MAX_CONTEXT);
}

export function buildFaithfulTranslationInstructions(targetLanguage: string): string {
  const languageStyle = targetLanguage === 'fa'
    ? 'Write idiomatic, readable Persian with correct RTL-language phrasing. Avoid English calques while preserving every distinction.'
    : targetLanguage === 'ar'
      ? 'Write fluent Modern Standard Arabic with precise terminology.'
      : 'Write fluent, natural English.';
  return [
    'Translate only the exact source passage supplied separately by the user.',
    languageStyle,
    'Preserve every proposition, negation, qualifier, condition, comparison, actor, tense, uncertainty, number, citation and paragraph boundary.',
    'Never summarize, omit, invent, explain inside the translation, or use book context as source text.',
    'Keep technical and philosophical terminology consistent with the supplied glossary when the meaning is identical.',
    'For terms without a stable target-language equivalent, use a clear translation and retain the original term in parentheses on first use when it materially aids comprehension.',
    'Return valid JSON with translation as the first property, followed by segments, terms and translatorNote. Every segment source must be copied verbatim and in original order.',
  ].join('\n');
}
