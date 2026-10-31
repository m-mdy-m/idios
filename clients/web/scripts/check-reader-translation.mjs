import assert from 'node:assert/strict';
import { mergeReaderTranslations, normalizeTranslationMatch, parseReaderTranslation, splitReaderTranslationText } from '../src/features/shelf/model/reader-translation.ts';

const source = 'A careful reader follows the argument. The result is not guaranteed.';
const valid = parseReaderTranslation(JSON.stringify({
  translation: 'یک خواننده دقیق استدلال را دنبال می‌کند. نتیجه تضمین‌شده نیست.',
  segments: [
    { source: 'A careful reader follows the argument.', translation: 'یک خواننده دقیق استدلال را دنبال می‌کند.' },
    { source: 'The result is not guaranteed.', translation: 'نتیجه تضمین‌شده نیست.' },
    { source: 'A sentence that is not in the source.', translation: 'متن ساختگی' },
  ],
  terms: [{ source: 'argument', translation: 'استدلال', meaning: 'روند منطقی ارائه‌شده در متن', partOfSpeech: 'noun', example: 'The argument is clear.' }, { source: 'fabricated-term', translation: 'ساختگی', meaning: 'not in source' }],
  grammar: [{ pattern: 'is not guaranteed', explanation: 'برای بیان واقعیت یا روند کلی استفاده شده است.' }],
  ielts: [{ expression: 'not guaranteed', academicUse: 'برای بیان محدودیت یا عدم قطعیت در نوشتار رسمی.' }],
}), source);

assert.equal(valid.translation, 'یک خواننده دقیق استدلال را دنبال می‌کند. نتیجه تضمین‌شده نیست.');
assert.equal(valid.segments.length, 2, 'only source-aligned segments should be kept');
assert.equal(valid.terms.length, 1, 'terms must come from the exact source text');
assert.equal(valid.terms[0]?.translation, 'استدلال');
assert.equal(valid.grammar.length, 1);
assert.equal(valid.ielts.length, 1);
const aligned = parseReaderTranslation(JSON.stringify({
  translation: 'او پرده را کنار زد و وارد اتاق شد.',
  segments: [{
    source: 'She lifted the veil and entered the room.',
    translation: 'او پرده را کنار زد و وارد اتاق شد.',
    words: [
      { source: 'veil', translation: 'پرده' },
      { source: 'entered', translation: 'وارد' },
      { source: 'invented', translation: 'ساختگی' },
      { source: 'veil', translation: 'پرده' },
      { source: 'the', translation: 'غیرمرتبط' },
    ],
  }],
}), 'She lifted the veil and entered the room.');
assert.deepEqual(aligned.segments[0]?.words, [
  { source: 'veil', translation: 'پرده' },
  { source: 'entered', translation: 'وارد' },
], 'word alignments must map exact source and target phrases and discard invalid or duplicate entries');

const wrongTarget = parseReaderTranslation(JSON.stringify({
  translation: 'او پرده را کنار زد.',
  segments: [{
    source: 'She lifted the veil.',
    translation: 'او پرده را کنار زد.',
    words: [{ source: 'veil', translation: 'دریا' }],
  }],
}), 'She lifted the veil.');
assert.equal(wrongTarget.segments[0]?.words, undefined, 'a target phrase absent from the translated sentence must be rejected');

assert.equal(normalizeTranslationMatch('  ＡＲＧＵＭＥＮＴ  '), 'argument');

const grammarOnly = parseReaderTranslation(JSON.stringify({
  translation: '',
  grammar: [{ pattern: 'is not guaranteed', explanation: 'A focused grammar note from the source.' }],
}), 'The outcome is not guaranteed.');
assert.equal(grammarOnly.translation, '', 'analysis-only responses must not be mistaken for translated page text');
assert.equal(grammarOnly.grammar.length, 1, 'analysis-only grammar should be retained');

const fallback = parseReaderTranslation('یک ترجمه ساده بدون JSON', 'A sentence.');
assert.equal(fallback.translation, 'یک ترجمه ساده بدون JSON');
assert.equal(fallback.segments[0]?.source, 'A sentence.');
assert.equal(fallback.terms.length, 0);


const longSource = Array.from({ length: 48 }, (_, index) =>
  `Sentence ${index + 1} explains a different part of the same chapter and its surrounding argument.`,
).join(' ');
const chunks = splitReaderTranslationText(longSource, 380);
assert.ok(chunks.length > 1, 'long page text must be split into multiple requests');
assert.ok(chunks.every((chunk) => chunk.length <= 380), 'each request must remain within the configured size');
assert.equal(chunks.join(' '), longSource, 'splitting must preserve every source character apart from repeated whitespace');

const paragraphMerge = mergeReaderTranslations([
  { translation: 'First paragraph.', segments: [], terms: [], grammar: [], ielts: [] },
  { translation: 'Second paragraph.', segments: [], terms: [], grammar: [], ielts: [] },
], ['\n\n']);
assert.equal(paragraphMerge.translation, 'First paragraph.\n\nSecond paragraph.', 'chunk merging must preserve PDF paragraph breaks');

const sparseParagraphMerge = mergeReaderTranslations([
  { translation: '', segments: [], terms: [], grammar: [], ielts: [] },
  { translation: 'First paragraph.', segments: [], terms: [], grammar: [], ielts: [] },
  { translation: '', segments: [], terms: [], grammar: [], ielts: [] },
  { translation: 'Second paragraph.', segments: [], terms: [], grammar: [], ielts: [] },
], [' ', '\n\n', '\n\n']);
assert.equal(sparseParagraphMerge.translation, 'First paragraph.\n\nSecond paragraph.', 'empty chunks must not shift paragraph separators');

const merged = mergeReaderTranslations([
  { translation: 'ترجمه یک.', segments: [{ source: 'Sentence one.', translation: 'ترجمه یک.' }], terms: [{ source: 'chapter', translation: 'فصل', meaning: 'بخش کتاب' }], grammar: [], ielts: [] },
  { translation: 'ترجمه دو.', segments: [{ source: 'Sentence two.', translation: 'ترجمه دو.' }], terms: [{ source: 'chapter', translation: 'فصل', meaning: 'بخش کتاب' }, { source: 'argument', translation: 'استدلال', meaning: 'روند منطقی متن' }], grammar: [], ielts: [] },
]);
assert.equal(merged.translation, 'ترجمه یک. ترجمه دو.');
assert.equal(merged.segments.length, 2);
assert.equal(merged.terms.length, 2, 'merged glossary entries should be deduplicated');

console.log('Reader translation checks passed.');
