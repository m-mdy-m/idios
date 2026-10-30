import type { RequestedBookAction, RequestedBookHighlight } from './study-types';

export function isBookMutationRequest(prompt: string): boolean {
  const text = prompt.normalize('NFKC').toLocaleLowerCase();
  const explicitHighlight = /^\s*(?:please\s+)?(?:highlight|mark|annotate|underline|flag)\s+\S/i.test(text)
    || /\b(?:please|can you|could you|would you|i want you to|i'd like you to)\s+(?:please\s+)?(?:highlight|mark|annotate|underline|flag)\b/i.test(text)
    || /\b(?:highlight|mark|annotate|underline|flag)\b.{0,70}\b(?:this|that|these|those|the|important|key|main|phrase|word|text|passage|sentence|idea|concept|term|line|part|page|above|selected)\b/i.test(text)
    || /\b(?:highlight|mark|annotate|underline|flag)\s+(?:it|them|these|those)\b/i.test(text)
    || /\b(?:highlight|mark|annotate|underline|flag)\b.{0,22}[“"'`]/i.test(text)
    || /\b(?:add|create|save|make|put|leave|write)\b.{0,70}\b(?:highlight|highlights|annotation|annotations|marked passage)\b/i.test(text);
  const explicitNoteAction = /^\s*(?:please\s+)?(?:write|add|create|save|make|put|leave)\s+(?:me\s+)?(?:a\s+)?(?:book\s+|margin\s+|marginal\s+)?notes?\b/i.test(text)
    || /\b(?:please|can you|could you|would you|i want you to|i'd like you to)\s+(?:please\s+)?(?:write|add|create|save|make|put|leave)\b.{0,60}\b(?:notes?|marginalia)\b/i.test(text)
    || /\b(?:add|create|save|write|leave|put)\s+(?:a\s+)?(?:margin\s+|marginal\s+)?note\b.{0,70}\b(?:on|about|for|next to|beside|in|this|that|the)\b/i.test(text);
  const farsiIntent = /(?:هایلایت|علامت\s*(?:گذاری)?).{0,70}(?:کن|بزن|بنویس|بذار|بزار|ثبت|اضافه|روی|برا)/u.test(text)
    || /(?:یادداشت|نوت).{0,70}(?:بنویس|بذار|بزار|ثبت|اضافه|کن|روی|برا)/u.test(text)
    || /(?:بنویس|بذار|بزار|ثبت کن|اضافه کن).{0,70}(?:نوت|یادداشت|هایلایت)/u.test(text);
  const beginsAsQuestion = /^\s*(?:what does|what is|what's|explain|tell me about|why does|why is|what do you mean by|معنی|توضیح بده)/i.test(text);
  // Mentioning an existing note/highlight while asking what it means is not permission to edit the book.
  if (beginsAsQuestion && !explicitHighlight && !explicitNoteAction && !farsiIntent) return false;
  return explicitHighlight || explicitNoteAction || farsiIntent || isBroadPageAnnotationRequest(prompt);
}

export function isExplicitHighlightRequest(prompt: string): boolean {
  const text = prompt.normalize('NFKC').toLocaleLowerCase();
  return /^\s*(?:please\s+)?(?:highlight|mark|annotate|underline|flag)\s+\S/i.test(text)
    || /\b(?:please|can you|could you|would you|i want you to|i'd like you to)\s+(?:please\s+)?(?:highlight|mark|annotate|underline|flag)\b/i.test(text)
    || /\b(?:highlight|mark|annotate|underline|flag)\b.{0,70}\b(?:this|that|these|those|the|important|key|main|phrase|word|text|passage|sentence|idea|concept|term|line|part|page|above|selected)\b/i.test(text)
    || /\b(?:highlight|mark|annotate|underline|flag)\s+(?:it|them|these|those)\b/i.test(text)
    || /(?:هایلایت|علامت\s*(?:گذاری)?).{0,50}(?:کن|بزن|ثبت|اضافه)/u.test(text);
}

export function isExplicitBookNoteRequest(prompt: string): boolean {
  const text = prompt.normalize('NFKC').toLocaleLowerCase();
  return /^\s*(?:please\s+)?(?:write|add|create|save|make|put|leave)\s+(?:me\s+)?(?:a\s+)?(?:book\s+|margin\s+|marginal\s+)?notes?\b/i.test(text)
    || /\b(?:please|can you|could you|would you|i want you to|i'd like you to)\s+(?:please\s+)?(?:write|add|create|save|make|put|leave)\b.{0,60}\b(?:notes?|marginalia)\b/i.test(text)
    || /\b(?:add|create|save|write|leave|put)\s+(?:a\s+)?(?:margin\s+|marginal\s+)?note\b.{0,70}\b(?:on|about|for|next to|beside|in|this|that|the)\b/i.test(text)
    || /(?:یادداشت|نوت).{0,70}(?:بنویس|بذار|بزار|ثبت|اضافه|کن|روی|برا)/u.test(text)
    || /(?:بنویس|بذار|بزار|ثبت کن|اضافه کن).{0,70}(?:نوت|یادداشت)/u.test(text);
}

export function isBroadPageAnnotationRequest(prompt: string): boolean {
  const text = prompt.normalize('NFKC').toLocaleLowerCase();
  const action = /\b(?:highlight|mark|annotate|underline|note|notes|write|add|save)\b/i.test(text)
    || /(?:هایلایت|علامت|یادداشت|نوت|بنویس|اضافه کن|ثبت کن)/u.test(text);
  const pageWideTarget = /\b(?:this page|current page|on the page|main ideas?|key ideas?|important (?:parts?|phrases?|words?|terms?|concepts?|ideas?|points?)|key (?:parts?|phrases?|words?|terms?|concepts?|points?)|main (?:concepts?|terms?|points?))\b/i.test(text)
    || /(?:این صفحه|صفحه|نکته(?:های)? مهم|مفهوم(?:های)? اصلی|عبارت(?:های)? مهم|کلمات مهم|نکات کلیدی)/u.test(text);
  return action && pageWideTarget;
}

export function normalizeRequestedActions(value: unknown, page: number): RequestedBookAction[] {
  if (!value || typeof value !== 'object') return [];
  const record = value as Record<string, unknown>;
  const rawActions = Array.isArray(record.actions) ? record.actions : (Array.isArray(value) ? value : []);
  const actions: RequestedBookAction[] = [];
  for (const raw of rawActions.slice(0, 8)) {
    if (!raw || typeof raw !== 'object') continue;
    const item = raw as Record<string, unknown>;
    const type = String(item.type ?? item.action ?? '').toLowerCase();
    const quote = String(item.quote ?? item.highlight ?? item.text ?? '').trim().replace(/^[“”"'`]+|[“”"'`]+$/g, '');
    const note = String(item.note ?? item.comment ?? item.explanation ?? (String(item.type ?? item.action ?? '').toLowerCase() === 'note' ? item.content : '') ?? '').trim();
    const itemPage = Number.isFinite(Number(item.page)) ? Number(item.page) : page;
    const colorValue = String(item.color ?? 'purple').toLowerCase();
    const color = (['yellow', 'green', 'blue', 'pink', 'purple'].includes(colorValue) ? colorValue : 'purple') as RequestedBookHighlight['color'];
    if ((type.includes('highlight') || type.includes('mark') || type.includes('annotat') || (type === 'note' && quote)) && quote) {
      actions.push({ type: 'highlight', quote, note, page: itemPage, color });
      continue;
    }
    if (type === 'note' || type === 'save_note' || type === 'book_note') {
      const content = String(item.content ?? item.body ?? note).trim();
      if (content) actions.push({ type: 'note', title: String(item.title ?? `Reading note · page ${itemPage}`).trim(), content, labels: Array.isArray(item.labels) ? item.labels.map(String).slice(0, 8) : ['reading-companion'] });
    }
  }
  const seen = new Set<string>();
  return actions.filter((action) => {
    const key = action.type === 'highlight' ? `h:${action.page}:${action.quote.toLocaleLowerCase()}` : `n:${action.title}:${action.content}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

export function parseBookActionResponse(raw: string, page: number): { visible: string; actions: RequestedBookAction[] } {
  let source = raw;
  const actions: RequestedBookAction[] = [];
  const blockPattern = /<idios_book_actions\s*>([\s\S]*?)<\/idios_book_actions\s*>/ig;
  let blockMatch: RegExpExecArray | null;
  while ((blockMatch = blockPattern.exec(raw)) !== null) {
    try { actions.push(...normalizeRequestedActions(JSON.parse(blockMatch[1].trim()) as unknown, page)); } catch { /* Legacy label parsing below can still recover useful actions. */ }
  }
  source = source.replace(blockPattern, '');
  const standaloneJSON = source.trim().match(/^```(?:json)?\s*([\s\S]*?)\s*```$/i)?.[1] ?? source.trim();
  if (!actions.length && /^\{[\s\S]*\}$/.test(standaloneJSON) && /["']actions["']\s*:/.test(standaloneJSON)) {
    try {
      actions.push(...normalizeRequestedActions(JSON.parse(standaloneJSON) as unknown, page));
      source = '';
    } catch { /* The legacy HIGHLIGHT:/NOTE: parser below remains the fallback. */ }
  }
  // Some smaller models ignore the JSON envelope but still return the older HIGHLIGHT:/NOTE: labels.
  // Recover those only in the client; never show them as if they were a normal answer.
  const lines = source.split(/\r?\n/);
  const markLines: Array<{ index: number; value: string }> = [];
  const noteLines: Array<{ index: number; value: string }> = [];
  lines.forEach((line, index) => {
    const mark = line.match(/^\s*(?:[-*•]\s*)?(?:HIGHLIGHT(?:\s+(?:QUOTE|TEXT))?|MARK|QUOTE)\s*[:=—-]\s*(.+?)\s*$/i);
    const note = line.match(/^\s*(?:[-*•]\s*)?(?:MARGIN\s+)?NOTE(?:\s+\d+)?\s*[:=—-]\s*(.+?)\s*$/i);
    if (mark) {
      const value = mark[1].trim();
      const quoted = value.match(/[“"]([^”"]+)[”"]/);
      markLines.push({ index, value: (quoted?.[1] || value).replace(/^[“”"'`]+|[“”"'`]+$/g, '').trim() });
    }
    if (note) noteLines.push({ index, value: note[1].replace(/^[“”"'`]+|[“”"'`]+$/g, '').trim() });
  });
  if (markLines.length) {
    markLines.slice(0, 3).forEach((mark, index) => {
      if (mark.value) actions.push({ type: 'highlight', quote: mark.value, note: noteLines[index]?.value || noteLines[0]?.value || '', page, color: 'purple' });
    });
  } else if (noteLines.length) {
    const content = noteLines.map((item) => item.value).filter(Boolean).join('\n');
    if (content) actions.push({ type: 'note', title: `Reading note · page ${page}`, content, labels: ['reading-companion'] });
  }
  const removeIndexes = new Set([...markLines.map((item) => item.index), ...noteLines.map((item) => item.index)]);
  const visible = lines.filter((_, index) => !removeIndexes.has(index)).join('\n')
    .replace(/```(?:json)?\s*[\s\S]*?```/gi, (fence) => /idios_book_actions|\"actions\"|HIGHLIGHT\s*:/i.test(fence) ? '' : fence)
    .replace(/^\s*(?:BOOK\s+)?ACTIONS?\s*:\s*.*$/gim, '')
    .replace(/\n{3,}/g, '\n\n').trim();
  const normalized = normalizeRequestedActions({ actions }, page);
  return { visible, actions: normalized };
}

