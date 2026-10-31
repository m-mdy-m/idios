import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
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

// Keep the browser's round-table instructions aligned with the runtime's staged protocol.
const discussionUI = await readFile(new URL('../src/features/shelf/components/ShelfAgentDiscussion.tsx', import.meta.url), 'utf8');
const settingsUI = await readFile(new URL('../src/features/ai-settings/AISettingsPage.tsx', import.meta.url), 'utf8');
assert.match(discussionUI, /This is a fresh discussion: in Round 1[\s\S]*?independently perform their configured role/, 'fresh opening specialists must be told to work independently');
assert.match(discussionUI, /From Round 2 onward[\s\S]*?actual completed peer claim/, 'later rounds must explicitly require real peer review');
assert.match(discussionUI, /Execution check/, 'the UI must expose a run-level execution summary');
assert.match(discussionUI, /not run/, 'terminal discussions must distinguish agents that never started from agents that failed');
assert.match(discussionUI, /runNotStartedCount/, 'the run summary must count queued/waiting agents that did not execute');
assert.match(discussionUI, /runWaitingCount/, 'a deliberate reader pause must be counted separately from agents that never ran');
assert.match(discussionUI, /waiting for you/, 'reader-waiting turns must remain distinguishable from not-run turns');
assert.match(discussionUI, /not whether an interpretation is true/, 'the UI must distinguish runtime validation from semantic correctness');
assert.match(discussionUI, /Use a guided page-analysis prompt/, 'the UI must offer a useful guided discussion prompt');
assert.match(settingsUI, /max_agents: 6/, 'new Web settings must allow the six-role discussion by default');
assert.match(settingsUI, /defaultDiscussionRoster = \['reader', 'tutor', 'steelman', 'concept-connector', 'evidence-auditor', 'discussion-chair'\]/, 'the default Shelf roster must include all six requested roles');
assert.doesNotMatch(discussionUI, /Every agent after the first must address one actual earlier claim/, 'the old contradictory first-round peer instruction must not return');

console.log('OK agent discussion UI: staged opening/peer-review contract, visible execution diagnostics, and guided prompt.');
