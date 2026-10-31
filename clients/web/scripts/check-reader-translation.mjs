import assert from 'node:assert/strict';
import { extractPartialReaderTranslation, extractReaderPDFText, mergeReaderTranslations, normalizeTranslationMatch, parseReaderTranslation, splitReaderTranslationText } from '../src/features/shelf/model/reader-translation.ts';

const source = 'A careful reader follows the argument. The result is not guaranteed.';
const rawlsTitlePage = [
  'A THEORY OF JUSTICE',
  'Revised Edition',
  'JOHN RAWLS',
  'the belknap press of',
  'harvard university press',
  'cambridge, massachusetts',
].join('\n');
const titlePageChunks = splitReaderTranslationText(rawlsTitlePage, 1800);
assert.equal(titlePageChunks.length, 1, 'a short title page must use one lightweight translation request');
assert.equal(titlePageChunks[0].replace(/\s+/g, ' '), rawlsTitlePage.replace(/\s+/g, ' '), 'the title page must retain its title, author, publisher and location');


const rawPDFItems = [
  { str: 'The section headings, the remarks that preface each chapter, and the', hasEOL: true },
  { str: 'index will guide the reader to the contents of the book. It seems superflu-', hasEOL: true },
  { str: 'ous to comment on this except to say that I have avoided extensive', hasEOL: true },
  { str: 'methodological discussions. There is a brief consideration of the nature', hasEOL: true },
  { str: 'of moral theory in §9, and of justification in §4 and §87.', hasEOL: true },
  { str: 'A short digression on the meaning of “good” is found in §62.', hasEOL: true },
];
const extractedRawlsText = extractReaderPDFText(rawPDFItems);
const rawlsPage21Source = [
  "The section headings, the remarks that preface each chapter, and the",
  "index will guide the reader to the contents of the book. It seems superflu-",
  "ous to comment on this except to say that I have avoided extensive",
  "methodological discussions. There is a brief consideration of the nature",
  "of moral theory in §9, and of justification in §4 and §87. A short digres-",
  "sion on the meaning of “good” is found in §62. Occasionally there are",
  "methodological comments and asides, but for the most part I try to work",
  "out a substantive theory of justice. Comparisons and contrasts with other",
  "theories, and criticisms thereof now and then, especially of utilitarianism,",
  "are viewed as means to this end.",
  "",
  "By not including most of Chapters IV–VIII in the more basic parts of",
  "the book, I do not mean to suggest that these chapters are peripheral, or",
  "merely applications. Rather, I believe that an important test of a theory of",
  "justice is how well it introduces order and system into our considered",
  "judgments over a wide range of questions. Therefore the topics of these",
  "chapters need to be taken up, and the conclusions reached modify in turn",
  "the view proposed. But in this regard the reader is more free to follow his",
  "preferences and to look at the problems which most concern him.",
  "",
  "In writing this book I have acquired many debts in addition to those",
  "indicated in the text. Some of these I should like to acknowledge here.",
  "Three different versions of the manuscript have passed among students",
  "and colleagues, and I have benefited beyond estimation from the innu-",
  "merable suggestions and criticisms that I have received. I am grateful to",
  "Allan Gibbard for his criticism of the first version (1964–1965). To meet",
  "his objections to the veil of ignorance as then presented, it seemed neces-",
  "sary to include a theory of the good. The notion of primary goods based",
  "on the conception discussed in Chapter VII is the result. I also owe him",
  "thanks, along with Norman Daniels, for pointing out difficulties with my",
  "account of utilitarianism as a basis for individual duties and obligations.",
  "Their objections led me to eliminate much of this topic and to simplify",
  "the treatment of this part of the theory. David Diamond objected force-",
  "fully to my discussion of equality, particularly to its failure to consider",
  "the relevance of status. I eventually included an account of self-respect as",
  "a primary good to try to deal with this and other questions, including",
  "those of society as a social union of social unions and the priority of",
  "liberty. I had profitable discussions with David Richards on the problems",
  "of political duty and obligation. Although supererogation is not a central",
  "topic of the book, I have been helped in my comments on it by Barry",
].join('\n');
const fullPageItems = rawlsPage21Source.split('\n').map((str) => ({ str, hasEOL: true }));
const extractedFullPage = extractReaderPDFText(fullPageItems);
for (const repaired of ['superfluous', 'digression', 'innumerable', 'necessary', 'forcefully']) {
  assert.ok(extractedFullPage.includes(repaired), 'PDF extraction must repair the split word: ' + repaired);
}
assert.ok(extractedFullPage.includes('by Barry'), 'the last visible line on page 21 must not be dropped');
const fullPageChunks = splitReaderTranslationText(extractedFullPage, 1800);
assert.ok(fullPageChunks.length >= 2, 'the complete Rawls page should be split into sequential requests');
assert.ok(fullPageChunks.every((chunk) => chunk.length <= 1800), 'no Rawls page request may exceed 1,800 source characters');
assert.equal(fullPageChunks.join(' ').replace(/\s+/g, ' '), extractedFullPage.replace(/\s+/g, ' '), 'all page 21 content must survive chunking');
assert.equal(parseReaderTranslation('{"translation":"a streamed but incomplete translation', source).translation, 'a streamed but incomplete translation', 'a provider that ends after a useful translation but before closing JSON must not display raw JSON');

assert.ok(extractedRawlsText.includes('superfluous to comment'), 'PDF line-end hyphenation must be rejoined');
assert.ok(!extractedRawlsText.includes('superflu- ous'), 'line-end hyphenation must not leak into the model prompt');
assert.ok(extractedRawlsText.includes('methodological discussions.'), 'PDF extraction must retain the complete passage');

const page21Chunks = splitReaderTranslationText(extractedRawlsText, 1800);
assert.ok(page21Chunks.length >= 1, 'a normal Rawls page should produce at least one bounded request');
assert.ok(page21Chunks.every((chunk) => chunk.length <= 1800), 'page translation chunks must stay within the 1,800-character budget');
assert.equal(page21Chunks.join(' ').replace(/\s+/g, ' '), extractedRawlsText.replace(/\s+/g, ' '), 'page chunking must preserve all extracted words');

assert.equal(
  extractPartialReaderTranslation('{"translation":"The first sentence is already arriving'),
  'The first sentence is already arriving',
  'the UI should be able to preview the translation before the JSON response is complete',
);
assert.equal(
  extractPartialReaderTranslation('{"translation":"او گفت: \\"نه\\".\\nسپس رفت.'),
  'او گفت: "نه".\nسپس رفت.',
  'the streaming preview must decode JSON quotes and paragraph newlines',
);
assert.equal(
  extractPartialReaderTranslation('{"translation":"عدالت بر اساس ' + '\\u06'),
  'عدالت بر اساس ',
  'incomplete Unicode escapes must not appear as broken text',
);
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
