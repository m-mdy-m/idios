import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fitShelfRuntimeMessages, splitUtf8Text, utf8ByteLength } from '../src/features/shelf/model/study-request-budget.ts';
import {
  isBookMutationRequest,
  isExplicitBookNoteRequest,
  isExplicitHighlightRequest,
  isExplicitQuoteRequest,
  parseBookActionResponse,
} from '../src/features/shelf/model/study-actions.ts';

assert.equal(isExplicitHighlightRequest('Highlight this passage'), true);
assert.equal(isExplicitHighlightRequest('این عبارت رو هایلایت کن'), true);
assert.equal(isExplicitQuoteRequest('Please quote this exact passage'), true);
assert.equal(isExplicitQuoteRequest('لطفاً یک نقل قول از این بخش استخراج کن'), true);
assert.equal(isExplicitQuoteRequest('لطفاً این بخش رو کوت کن'), true);
assert.equal(isExplicitQuoteRequest('Give me a quote from this page'), true);
assert.equal(isBookMutationRequest('Please quote this exact passage'), true);
assert.equal(isBookMutationRequest('لطفاً یک نقل قول از این بخش استخراج کن'), true);
assert.equal(isExplicitHighlightRequest('Please quote this exact passage'), false);
assert.equal(isExplicitBookNoteRequest('Please save a note about this page'), true);
assert.equal(isExplicitBookNoteRequest('برای این بخش یک یادداشت بنویس'), true);
assert.equal(isBookMutationRequest('What does this highlighted sentence mean?'), false);

const highlight = parseBookActionResponse(
  '<idios_book_actions>{"actions":[{"type":"highlight","page":4,"quote":"small exact phrase","color":"green"}]}</idios_book_actions>',
  4,
);
assert.equal(highlight.actions.length, 1);
assert.equal(highlight.actions[0].type, 'highlight');
assert.equal(highlight.actions[0].quote, 'small exact phrase');
assert.equal(highlight.actions[0].page, 4);
assert.equal(highlight.visible.trim(), '');

const quoted = parseBookActionResponse(
  '<idios_book_actions>{"actions":[{"type":"quote","page":4,"quote":"an exact sentence from this page"}]}</idios_book_actions>',
  4,
);
assert.equal(quoted.actions.length, 1);
assert.equal(quoted.actions[0].type, 'quote', 'a model quote action must become a Quotes-library record');
assert.equal(quoted.actions[0].text, 'an exact sentence from this page');

const legacyQuote = parseBookActionResponse('QUOTE: “A line worth keeping.”', 4);
assert.equal(legacyQuote.actions[0]?.type, 'quote', 'legacy QUOTE labels must not be misread as PDF highlights');
assert.equal(legacyQuote.actions[0]?.text, 'A line worth keeping.');

const note = parseBookActionResponse(
  'I made a useful note.\n<idios_book_actions>{"actions":[{"type":"note","title":"Key point","content":"A saved learning note.","labels":["reading"]}]}</idios_book_actions>',
  8,
);
assert.equal(note.actions.length, 1);
assert.equal(note.actions[0].type, 'note');
assert.equal(note.actions[0].content, 'A saved learning note.');
assert.match(note.visible, /useful note/);

const combinedPrompt = 'Mimi, please write a concise standalone note about this passage and highlight “order and system” on page 21.';
assert.equal(isBookMutationRequest(combinedPrompt), true);
assert.equal(isExplicitBookNoteRequest(combinedPrompt), true);
assert.equal(isExplicitHighlightRequest(combinedPrompt), true);

// A complete book context may span multiple API messages without losing PDF pages.
const longBookContext = Array.from({ length: 15 }, (_, page) => `===== PDF PAGE ${page + 1} =====\n${'Rawls page text and prior argument. '.repeat(220)}`).join('\n\n');
const chunks = splitUtf8Text(longBookContext);
assert.ok(chunks.length > 1, 'long book context should be split into multiple messages');
assert.equal(chunks.join(''), longBookContext, 'splitting must preserve every source character');
assert.ok(chunks.every((chunk) => utf8ByteLength(chunk) <= 48000), 'each context message must stay below the runtime per-message cap');

const conversation = [
  { role: 'system', content: 'MIMI instructions' },
  { role: 'user', content: 'Earlier question: what does considered judgment mean?', context: 'DUPLICATED WHOLE BOOK PDF CONTEXT '.repeat(2000) },
  { role: 'assistant', content: 'We established that these are reflective judgments.' },
  { role: 'user', content: 'Why does Rawls need a systematic theory?', context: 'DUPLICATED WHOLE BOOK PDF CONTEXT '.repeat(2000) },
  { role: 'assistant', content: 'Because separate judgments can conflict.' },
  { role: 'user', content: 'Mimi, write a note and highlight order and system.\n' + longBookContext },
];
const fitted = fitShelfRuntimeMessages(conversation);
assert.ok(fitted.every((message) => utf8ByteLength(message.content) <= 48000), 'fitted request messages must satisfy the per-message cap');
assert.ok(fitted.reduce((sum, message) => sum + utf8ByteLength(message.content), 0) <= 175000, 'fitted request must stay below the total runtime cap');
assert.ok(fitted.some((message) => message.content.includes('Earlier question: what does considered judgment mean?')), 'conversation dialogue should be retained');
assert.ok(fitted.some((message) => message.content.includes('===== PDF PAGE 15 =====')), 'the end of the current book context should be preserved');
assert.ok(!fitted.some((message) => message.content.includes('DUPLICATED WHOLE BOOK PDF CONTEXT')), 'old embedded document contexts must not be duplicated');
assert.equal(isBookMutationRequest('Mimi, please write a concise standalone note about this page.'), true);
assert.equal(isExplicitBookNoteRequest('Mimi, please write a concise standalone note about this page.'), true);

const combined = parseBookActionResponse(
  '<idios_book_actions>{"actions":[{"type":"highlight","page":21,"quote":"order and system","note":"This is the practical test of the theory.","color":"purple"},{"type":"note","title":"A theory must organize judgments","content":"Rawls treats a theory of justice as something that must bring order to considered judgments across many questions, not merely explain a few basic principles.","labels":["reading-companion"]}]}</idios_book_actions>',
  21,
);
assert.deepEqual(combined.actions.map((action) => action.type), ['highlight', 'note']);
const fixture = readFileSync(new URL('../../../tests/fixtures/rawls-a-theory-of-justice-page-21.txt', import.meta.url), 'utf8');
assert.equal(fixture.split('order and system').length - 1, 1, 'the requested highlight anchor must be exact and unique on page 21');
assert.equal(fixture.includes(combined.actions[0].quote), true, 'the parsed highlight must be copied verbatim from the source page');

// Guard the actual Shelf action dispatch: a mixed request must persist the annotation
// and the standalone note separately, rather than collapsing one into the other.
const assistantSource = readFileSync(new URL('../src/features/shelf/components/ShelfStudyAssistant.tsx', import.meta.url), 'utf8');
assert.equal(assistantSource.includes('onApplyBookHighlights(highlightActions)'), true);
assert.equal(assistantSource.includes('api.createShelfNote(bookID'), true);
assert.equal(assistantSource.includes('Keep a separately requested note as its own shelf-note action'), true);
assert.equal(assistantSource.includes('content: message.content'), true, 'conversation dialogue must be preserved in request history');
assert.equal(assistantSource.includes('fitShelfRuntimeMessages(['), true, 'the runtime payload must be byte-budgeted');
assert.equal(assistantSource.includes('if (separateNote && promptAskedForNote) annotationNote = separateNote.content;'), false, 'the standalone note must not overwrite the margin explanation');
assert.equal(assistantSource.includes('A highlight persistence failure must not prevent a separately requested'), true);
assert.equal(assistantSource.includes('needsNoteAction'), true, 'partial action envelopes must retry missing book notes');
assert.equal(assistantSource.includes('needsQuoteAction'), true, 'partial action envelopes must retry missing Quotes actions');
assert.equal(assistantSource.includes('api.createQuote'), true, 'quotes must persist through the Quotes feature API');
assert.equal(assistantSource.includes('focused retry'), true, 'missing notes must be reported rather than claimed saved');
assert.equal(assistantSource.indexOf('for (const action of noteActions)') > assistantSource.indexOf('A highlight persistence failure must not prevent a separately requested'), true);

const malformed = parseBookActionResponse('<idios_book_actions>{bad json}</idios_book_actions>', 1);
assert.equal(malformed.actions.length, 0);
assert.equal(malformed.visible.trim(), '');

console.log('Shelf action checks passed: English/Farsi intents, exact unique page anchors, mixed highlight + standalone note parsing, separate persistence dispatch, and malformed-action safety.');
