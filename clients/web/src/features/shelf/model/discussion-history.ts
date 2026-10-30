export type DiscussionHistoryTurn = {
  round?: number;
  agent_id: string;
  agent_name: string;
  provider_id?: string;
  model?: string;
  local: boolean;
  status: string;
  content?: string;
  source_ids?: string[];
  warnings?: string[];
  error?: { code: string; message: string };
};

export type DiscussionHistoryResult<T> = {
  contributions: T[];
  omittedCount: number;
  truncatedCount: number;
};

export const MAX_DISCUSSION_HISTORY_TURNS = 40;
export const MAX_DISCUSSION_HISTORY_TURN_BYTES = 5000;
export const MAX_DISCUSSION_HISTORY_BYTES = 48000;

const encoder = new TextEncoder();
const sourceMarkerPattern = /\[source:[^\]\s]+\]/g;

function byteLength(value: string): number {
  return encoder.encode(value).byteLength;
}

function utf8Prefix(value: string, limit: number): string {
  let result = '';
  let used = 0;
  for (const character of value) {
    const size = byteLength(character);
    if (used + size > limit) break;
    result += character;
    used += size;
  }
  return result;
}

function sourceMarkers(turn: DiscussionHistoryTurn): string[] {
  const markers = [
    ...(turn.content?.match(sourceMarkerPattern) ?? []),
    ...(turn.source_ids ?? []).map((id) => '[source:' + id + ']'),
  ];
  return [...new Set(markers)];
}

function fitContent(turn: DiscussionHistoryTurn, maxBytes: number): { content: string; truncated: boolean } {
  const content = (turn.content ?? '').trim();
  if (byteLength(content) <= maxBytes) return { content, truncated: false };

  const markers = sourceMarkers(turn);
  const suffix = markers.join(' ');
  const separator = suffix ? ' … ' : '…';
  const fixedBytes = byteLength(suffix) + byteLength(separator);
  if (suffix && fixedBytes > maxBytes) return { content: '', truncated: true };
  const prefixBudget = Math.max(0, maxBytes - fixedBytes);
  const prefix = utf8Prefix(content, prefixBudget).trimEnd();
  const fitted = suffix ? prefix + (prefix ? separator : '') + suffix : prefix + '…';
  return byteLength(fitted) <= maxBytes ? { content: fitted, truncated: true } : { content: '', truncated: true };
}

/**
 * Carries the most recent completed discussion turns forward without silently
 * cutting every contribution to a tiny excerpt. The byte limits mirror the Go
 * request validator, and citations survive any necessary truncation.
 */
export function buildDiscussionHistory<T extends DiscussionHistoryTurn>(
  turns: readonly T[],
): DiscussionHistoryResult<T> {
  const completed = turns.filter((turn) => turn.status === 'completed' && Boolean(turn.content?.trim()));
  const candidates = completed.slice(-MAX_DISCUSSION_HISTORY_TURNS);
  const selected: T[] = [];
  let remainingBytes = MAX_DISCUSSION_HISTORY_BYTES;
  let truncatedCount = 0;

  for (const turn of [...candidates].reverse()) {
    const budget = Math.min(MAX_DISCUSSION_HISTORY_TURN_BYTES, remainingBytes);
    if (budget <= 0) break;
    const fitted = fitContent(turn, budget);
    if (!fitted.content) break;
    const warnings = fitted.truncated
      ? [...new Set([...(turn.warnings ?? []), 'This earlier contribution was shortened to fit the bounded discussion history; its source markers were preserved.'])]
      : turn.warnings;
    selected.push({
      ...turn,
      content: fitted.content,
      ...(warnings ? { warnings } : {}),
    });
    remainingBytes -= byteLength(fitted.content);
    if (fitted.truncated) truncatedCount += 1;
  }

  return {
    contributions: selected.reverse(),
    omittedCount: completed.length - selected.length,
    truncatedCount,
  };
}
