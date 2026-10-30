import assert from 'node:assert/strict';
import {
  buildDiscussionHistory,
  MAX_DISCUSSION_HISTORY_BYTES,
  MAX_DISCUSSION_HISTORY_TURN_BYTES,
  MAX_DISCUSSION_HISTORY_TURNS,
} from '../src/features/shelf/model/discussion-history.ts';

function turn(index, overrides = {}) {
  return {
    round: 1,
    agent_id: 'agent-' + index,
    agent_name: 'Agent ' + index,
    local: true,
    status: 'completed',
    content: 'Agent ' + index + ' made a distinct observation. [source:page-' + index + ']',
    source_ids: ['page-' + index],
    ...overrides,
  };
}

const ordinary = Array.from({ length: 16 }, (_, index) => turn(index));
const preserved = buildDiscussionHistory(ordinary);
assert.equal(preserved.contributions.length, 16, 'all ordinary turns should survive the carry-over');
assert.equal(preserved.omittedCount, 0);
assert.equal(preserved.truncatedCount, 0);
assert.equal(preserved.contributions[0].content, ordinary[0].content, 'ordinary contributions must remain complete');
assert.match(preserved.contributions[0].content, /\[source:page-0\]/, 'source markers must remain attached');

const failed = [...ordinary, turn(16, { status: 'failed', content: 'This failed response must not be treated as evidence.' })];
assert.equal(buildDiscussionHistory(failed).contributions.length, 16, 'failed turns must not be passed as peer evidence');

const overlong = turn(17, { content: 'الف '.repeat(3000), source_ids: ['page-17'] });
const clipped = buildDiscussionHistory([overlong]);
assert.equal(clipped.contributions.length, 1);
assert.ok(new TextEncoder().encode(clipped.contributions[0].content ?? '').byteLength <= MAX_DISCUSSION_HISTORY_TURN_BYTES);
assert.match(clipped.contributions[0].content ?? '', /\[source:page-17\]/, 'truncation must preserve the source marker');
assert.equal(clipped.truncatedCount, 1);
assert.ok(clipped.contributions[0].warnings?.some((warning) => warning.includes('shortened')));

const bounded = buildDiscussionHistory(Array.from({ length: MAX_DISCUSSION_HISTORY_TURNS + 1 }, (_, index) => turn(index, { content: 'turn-' + index + ' ' + 'evidence '.repeat(100) + ' [source:page-' + index + ']' })));
assert.ok(bounded.contributions.length <= MAX_DISCUSSION_HISTORY_TURNS);
assert.equal(bounded.omittedCount, 1, 'old turns outside the supported window must be reported');
assert.ok(bounded.contributions.at(-1)?.content?.includes('turn-' + MAX_DISCUSSION_HISTORY_TURNS), 'the newest turn must be retained');
assert.ok(MAX_DISCUSSION_HISTORY_BYTES > MAX_DISCUSSION_HISTORY_TURN_BYTES);

const aggregate = buildDiscussionHistory(Array.from({ length: MAX_DISCUSSION_HISTORY_TURNS }, (_, index) => turn(index, { content: 'x'.repeat(4800) + ' [source:page-' + index + ']' })));
const aggregateBytes = aggregate.contributions.reduce((sum, item) => sum + new TextEncoder().encode(item.content ?? '').byteLength, 0);
assert.ok(aggregateBytes <= MAX_DISCUSSION_HISTORY_BYTES, 'aggregate history must stay inside the server byte limit');
assert.ok(aggregate.omittedCount > 0 || aggregate.truncatedCount > 0, 'aggregate clipping must be reported');

console.log('OK agent discussion history: ordered carry-over, failed-turn filtering, UTF-8 byte bounds, citation preservation, and truncation reporting.');
