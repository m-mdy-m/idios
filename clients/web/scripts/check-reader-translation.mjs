import { readFile } from 'node:fs/promises';
import assert from 'node:assert/strict';
import { extractPartialReaderTranslation, extractReaderPDFText, normalizeReaderSelectionText, mergeReaderTranslations, normalizeTranslationMatch, parseReaderTranslation, splitReaderTranslationText, splitMixedDirectionText, translationSourceIdentity } from '../src/features/shelf/model/reader-translation.ts';
import { buildFaithfulTranslationInstructions, buildTranslationContext } from '../src/features/shelf/model/translation/context.ts';
import { validateRawTranslationAlignment, validateTranslationIntegrity } from '../src/features/shelf/model/translation/source-integrity.ts';
import { translationJobs } from '../src/features/shelf/model/translation/job-manager.ts';
import { translationBenchmarkFixtures, translationBenchmarkRubric } from '../src/features/shelf/model/translation/benchmarks.ts';

const source = 'A careful reader follows the argument. The result is not guaranteed.';
const readerPageSource = await readFile(new URL('../src/features/shelf/ShelfPDFReaderPage.tsx', import.meta.url), 'utf8');
assert.ok(!/task:\s*'translation',[\s\S]{0,100}reasoning_effort\s*:/.test(readerPageSource), 'Shelf translation must not force a reasoning/thinking request on the selected model.');
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
assert.equal(
  normalizeReaderSelectionText('  superflu-' + String.fromCharCode(10) + 'ous' + String.fromCharCode(0x00ad) + '  argument  '),
  'superfluous argument',
  'AI-facing selections must join line-end hyphenation and remove soft hyphens',
);
assert.equal(normalizeReaderSelectionText('می‌روم'), 'می‌روم', 'Persian ZWNJ must be preserved in selected text');
const positionedItems = [
  { str: 'A careful', transform: [10, 0, 0, 10, 50, 700], width: 44, height: 10, hasEOL: false },
  { str: 'reader follows', transform: [10, 0, 0, 10, 98, 700], width: 72, height: 10, hasEOL: true },
  { str: 'the argument', transform: [10, 0, 0, 10, 50, 688], width: 66, height: 10, hasEOL: false },
  { str: ',', transform: [10, 0, 0, 10, 117, 688], width: 2, height: 10, hasEOL: false },
  { str: ' clearly.', transform: [10, 0, 0, 10, 120, 688], width: 40, height: 10, hasEOL: true },
  { str: 'Next paragraph starts here.', transform: [10, 0, 0, 10, 50, 650], width: 160, height: 10, hasEOL: true },
];
assert.equal(
  extractReaderPDFText(positionedItems),
  ['A careful reader follows', 'the argument, clearly.', '', 'Next paragraph starts here.'].join(String.fromCharCode(10)),
  'geometric extraction must reconstruct spaces, lines, punctuation and paragraph gaps',
);
const rtlItems = [
  { str: 'کتاب', dir: 'rtl', transform: [10, 0, 0, 10, 150, 700], width: 25, height: 10, hasEOL: false },
  { str: 'متن', dir: 'rtl', transform: [10, 0, 0, 10, 90, 700], width: 25, height: 10, hasEOL: true },
];
assert.equal(extractReaderPDFText(rtlItems), 'کتاب متن', 'PDF extraction must restore word gaps for right-to-left text');

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


const mixedRTL = splitMixedDirectionText('عدالت (Justice)، چاپ 2026، صفحهٔ 21.');
assert.deepEqual(mixedRTL.filter((part) => part.direction === 'ltr').map((part) => part.text), ['Justice', '2026', '21'], 'Latin terminology and Western page numerals must be isolated inside RTL prose');
assert.ok(mixedRTL.some((part) => part.text.includes('،')), 'Arabic punctuation must remain in the surrounding RTL run');
assert.ok(mixedRTL.some((part) => part.text.includes('(') || part.text.includes(')')), 'paired punctuation must not be dropped during bidi segmentation');

const architectureContext = buildTranslationContext({
  bookTitle: 'A Theory of Justice',
  pageNumber: 21,
  previousContext: 'The author distinguishes justice from utility.',
  glossary: [{ source: 'primary goods', translation: 'خیرهای اولیه', meaning: 'وسایلی که برای تحقق طرح‌های زندگی لازم‌اند.' }],
});
assert.ok(architectureContext.includes('خیرهای اولیه'));
assert.ok(architectureContext.includes('interpretation only'));
assert.ok(buildFaithfulTranslationInstructions('fa').includes('negation'));
assert.ok(buildFaithfulTranslationInstructions('fa').includes('paragraph boundary'));
assert.equal(validateTranslationIntegrity(source, valid).valid, true);
assert.equal(validateTranslationIntegrity(source, { ...valid, segments: [valid.segments[0], valid.segments[0]] }).reason, 'duplicate-segment', 'cached/legacy duplicate source anchors must also fail closed');
const rawValid = validateRawTranslationAlignment(JSON.stringify({ translation: valid.translation, segments: valid.segments }), source);
assert.equal(rawValid.valid, true);
assert.equal(rawValid.verified, true);
assert.equal(validateRawTranslationAlignment(JSON.stringify({ translation: 'x', segments: [{ source: 'A sentence that belongs to another page.', translation: 'x' }] }), source).reason, 'source-mismatch');
assert.equal(validateRawTranslationAlignment(JSON.stringify({ translation: 'x', segments: [{ source: 'The result is not guaranteed.', translation: 'x' }, { source: 'A careful reader follows the argument.', translation: 'x' }] }), source).reason, 'segment-order');
assert.equal(validateRawTranslationAlignment(JSON.stringify({ translation: 'x', segments: [{ source: 'A careful reader follows the argument.', translation: 'x' }] }), source).reason, 'segment-coverage');
assert.equal(validateRawTranslationAlignment('{"translation":"broken", "segments":[', source).reason, 'malformed-structured-response');
assert.equal(validateRawTranslationAlignment('Model response: {\"translation\":\"partial\", \"segments\":[', source).reason, 'malformed-structured-response', 'a preamble must not downgrade malformed structured output to plain text');
assert.equal(validateRawTranslationAlignment('Here is JSON: ' + JSON.stringify({ translation: valid.translation, segments: valid.segments }), source).reason, 'malformed-structured-response', 'structured output with an unexpected preamble must fail closed');
assert.equal(validateRawTranslationAlignment(JSON.stringify({ translation: 'x', segments: [] }), source).reason, 'missing-segments');
assert.equal(validateRawTranslationAlignment(JSON.stringify({ translation: 'x', segments: [{ source: 'A careful reader follows the argument.', translation: 'x' }, { source: 'A careful reader follows the argument.', translation: 'x' }] }), source).reason, 'duplicate-segment', 'duplicate source anchors must fail closed');
const plain = validateRawTranslationAlignment('یک ترجمهٔ متنی ساده', source);
assert.equal(plain.valid, true);
assert.equal(plain.verified, false);
assert.notEqual(translationSourceIdentity('same page'), translationSourceIdentity('same page '));
assert.notEqual(translationSourceIdentity('a:b'), translationSourceIdentity('a%3Ab'));
assert.equal(translationSourceIdentity(source), translationSourceIdentity(source));
const manager = translationJobs;
const runningSignal = manager.start({ id: 'translation-check-job', bookID: 'book-check', pageNumber: 21, totalParts: 2 });
assert.equal(runningSignal.aborted, false);
manager.progress('translation-check-job', 1);
assert.equal(manager.get('translation-check-job')?.completedParts, 1);
manager.complete('translation-check-job');
assert.equal(manager.get('translation-check-job')?.status, 'completed');
const cancelSignal = manager.start({ id: 'translation-check-cancel', bookID: 'book-check', pageNumber: 22, totalParts: 2 });
manager.cancel('translation-check-cancel');
assert.equal(cancelSignal.aborted, true);
manager.progress('translation-check-cancel', 2);
manager.complete('translation-check-cancel');
assert.equal(manager.get('translation-check-cancel')?.status, 'cancelled');
assert.equal(manager.get('translation-check-cancel')?.completedParts, 0);
for (let index = 0; index < 60; index += 1) {
  const id = 'translation-history-' + index;
  manager.start({ id, bookID: 'book-check', pageNumber: index + 1, totalParts: 1 });
  manager.complete(id);
}
assert.ok(manager.list().length <= 40);
assert.equal(translationBenchmarkFixtures.length, 3);
assert.equal(translationBenchmarkRubric.length, 6);
for (const fixture of translationBenchmarkFixtures) {
  assert.equal(fixture.proposedPersianReference.split(/\n\s*\n/).length, fixture.source.split(/\n\s*\n/).length);
  assert.equal(fixture.reviewStatus, 'maintainer-review-required');
  for (const marker of fixture.requiredMeaningChecks) assert.ok(fixture.source.includes(marker));
  for (const term of fixture.terminology) {
    assert.ok(fixture.source.toLocaleLowerCase().includes(term.source.toLocaleLowerCase()));
    assert.ok(fixture.proposedPersianReference.includes(term.preferred));
  }
}
const readerSource = await readFile(new URL('../src/features/shelf/components/ShelfPDFReader.tsx', import.meta.url), 'utf8');
const companionSource = await readFile(new URL('../src/features/shelf/components/ShelfTranslationPanel.tsx', import.meta.url), 'utf8');
assert.ok(readerSource.includes('translationJobs.subscribe(setTranslationJobSnapshots)'));
assert.ok(readerSource.includes("requestTranslation(currentPageText, 'page', signal, jobID)"));
const pageTranslationEffect = readerSource.match(/useEffect\(\(\) => \{\s*if \(!translationOpen \|\| !currentPageText\.trim\(\) \|\| !pdf \|\| !translationModel\) return;[\s\S]*?\n\s*\}, \[([^\]]+)\]\);/);
assert.ok(pageTranslationEffect, 'the page translation effect must remain discoverable as a focused lifecycle block');
assert.match(pageTranslationEffect[1], /\btranslationOpen\b/, 'opening the companion after mount must re-run the page effect');
assert.match(pageTranslationEffect[1], /\bcurrentPageTranslationKey\b/, 'page changes and exact-source changes must re-run the page effect');
assert.ok(readerSource.includes('translationSourceIdentity(currentPageText)'));
assert.ok(readerSource.includes('const pageKey = currentPageTranslationKey'), 'learning-analysis cache must share the exact page/source identity');
assert.ok(readerSource.includes('currentPageTranslationKeyRef.current !== pageKey'), 'late learning-analysis results must not update a different page');
assert.ok(readerSource.includes('learningAnalysisSequenceRef.current === requestSequence'), 'late learning-analysis results must not clear a newer request state');
assert.ok(readerSource.includes('pageLearningCache.current.delete(`${currentPageTranslationKey}:grammar`)'), 'refresh must invalidate grammar analysis for the current source');
assert.ok(readerSource.includes('translationJobs.cancel(job.id)'), 'refresh must cancel every stale in-flight page translation');
assert.ok(readerSource.includes('normalizeReaderSelectionText'));
assert.ok(companionSource.includes('dir={targetLanguage'));
assert.ok(companionSource.includes('lang={targetLanguage}'), 'the companion must expose the selected language semantically');
assert.ok(companionSource.includes('splitMixedDirectionText(value)'), 'embedded LTR words and Western numerals must use the tested bidi splitter');
assert.ok(companionSource.includes('onStopTranslation'));
const panelStyles = await readFile(new URL('../src/features/shelf/components/ShelfTranslationPanel.css', import.meta.url), 'utf8');
const readerStyles = await readFile(new URL('../src/features/shelf/components/ShelfPDFReader.css', import.meta.url), 'utf8');
assert.ok(panelStyles.includes('@media (prefers-reduced-motion: reduce)'), 'the companion must respect reduced-motion preferences');
assert.ok(panelStyles.includes('@media (max-width: 520px)'), 'the companion must have a narrow viewport layout');
assert.ok(panelStyles.includes('.shelf-translation-panel[lang="fa"]') && panelStyles.includes('.shelf-translation-panel[lang="ar"]'), 'Persian and Arabic typography must be explicit');
assert.ok(panelStyles.includes('unicode-bidi: isolate'), 'embedded LTR runs must be isolated from surrounding RTL prose');
const hoverRules = [...panelStyles.matchAll(/\.shelf-translation-panel__linked-segment:hover\s*\{([^}]*)\}/g)];
assert.ok(hoverRules.length > 0 && !/box-shadow\s*:\s*0\s+0\s+0\s+2px/i.test(hoverRules.at(-1)[1]), 'linked-term hover must not add the previous blue glow');
assert.ok(readerStyles.includes('background:rgb(251 191 36 / 36%)'), 'PDF word hover must use a restrained warm highlight');
assert.ok(readerStyles.includes('background:rgb(58 118 238 / 38%)'), 'native text selection must remain visually distinct from word hover');

console.log('Reader translation checks passed.');
