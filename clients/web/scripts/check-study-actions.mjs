import assert from 'node:assert/strict';
import {
  isBookMutationRequest,
  isExplicitBookNoteRequest,
  isExplicitHighlightRequest,
  parseBookActionResponse,
} from '../src/features/shelf/model/study-actions.ts';

assert.equal(isExplicitHighlightRequest('Highlight this passage'), true);
assert.equal(isExplicitHighlightRequest('این عبارت رو هایلایت کن'), true);
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

const note = parseBookActionResponse(
  'I made a useful note.\n<idios_book_actions>{"actions":[{"type":"note","title":"Key point","content":"A saved learning note.","labels":["reading"]}]}</idios_book_actions>',
  8,
);
assert.equal(note.actions.length, 1);
assert.equal(note.actions[0].type, 'note');
assert.equal(note.actions[0].content, 'A saved learning note.');
assert.match(note.visible, /useful note/);

const malformed = parseBookActionResponse('<idios_book_actions>{bad json}</idios_book_actions>', 1);
assert.equal(malformed.actions.length, 0);
assert.equal(malformed.visible.trim(), '');

console.log('Shelf action checks passed: English/Farsi intents, highlight/note action parsing, and malformed-action safety.');
