// The runtime counts UTF-8 bytes per message and across the full request.
// Preserve the current book context intact by splitting it into labelled user
// messages instead of truncating it. Historical turns keep their actual dialogue
// text, but not the full PDF context that was already attached to those turns.
export function utf8ByteLength(value: string): number {
  return new TextEncoder().encode(value).byteLength;
}

export function splitUtf8Text(value: string, maxBytes = 48000): string[] {
  if (!value) return [''];
  const chunks: string[] = [];
  let remaining = value;
  while (remaining) {
    let chunk = '';
    let bytes = 0;
    let cutAt = 0;
    for (const character of remaining) {
      const size = utf8ByteLength(character);
      if (bytes + size > maxBytes) break;
      chunk += character;
      bytes += size;
      cutAt += character.length;
    }
    if (!cutAt) throw new Error('A single character exceeds the AI request message limit.');
    if (cutAt < remaining.length) {
      const preferred = Math.max(chunk.lastIndexOf('\n\n'), chunk.lastIndexOf('\n'));
      if (preferred > maxBytes * 0.55) {
        chunk = chunk.slice(0, preferred);
        cutAt = chunk.length;
      }
    }
    chunks.push(chunk);
    remaining = remaining.slice(cutAt);
  }
  return chunks;
}

export type ShelfAIMessage = { role: 'system' | 'user' | 'assistant'; content: string };

export function fitShelfRuntimeMessages(messages: ShelfAIMessage[]): ShelfAIMessage[] {
  if (messages.length < 2) return messages;
  const system = messages[0];
  const current = messages[messages.length - 1];
  const history = messages.slice(1, -1);
  const systemChunks = splitUtf8Text(system.content, 48000);
  if (systemChunks.length > 1) {
    throw new Error('MIMI system instructions exceed the AI request budget; shorten the selected reading skills.');
  }
  const currentChunks = splitUtf8Text(current.content, 47000).map((content, index, all) => ({
    role: current.role,
    content: all.length === 1
      ? content
      : '[CURRENT READING CONTEXT — PART ' + (index + 1) + '/' + all.length + ']\n' + content,
  }));
  const maxTotalBytes = 175000;
  let used = utf8ByteLength(systemChunks[0]) + currentChunks.reduce((sum, item) => sum + utf8ByteLength(item.content), 0);
  if (used > maxTotalBytes) {
    throw new Error('The current book context alone exceeds the AI runtime request budget. The previous conversation was not discarded; narrow the PDF reading window or choose a smaller source range.');
  }
  const keptHistory: ShelfAIMessage[] = [];
  // Keep the recent dialogue so MIMI understands what the reader and assistant
  // have been discussing. Do not resend each old turn's duplicate PDF/context.
  for (let index = history.length - 1; index >= 0 && keptHistory.length < 12; index -= 1) {
    const message = history[index];
    const content = splitUtf8Text(message.content, 3500)[0];
    const size = utf8ByteLength(content);
    if (!content.trim() || used + size > maxTotalBytes) continue;
    keptHistory.unshift({ ...message, content });
    used += size;
  }
  return [{ ...system, content: systemChunks[0] }, ...keptHistory, ...currentChunks];
}
