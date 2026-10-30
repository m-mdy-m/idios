import assert from 'node:assert/strict';
import { normalizeTranslationMatch, parseReaderTranslation } from '../src/features/shelf/model/reader-translation.ts';

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
assert.equal(normalizeTranslationMatch('  ＡＲＧＵＭＥＮＴ  '), 'argument');

const fallback = parseReaderTranslation('یک ترجمه ساده بدون JSON', 'A sentence.');
assert.equal(fallback.translation, 'یک ترجمه ساده بدون JSON');
assert.equal(fallback.segments[0]?.source, 'A sentence.');
assert.equal(fallback.terms.length, 0);

console.log('Reader translation checks passed.');
