export type SeparatedModelOutput = { answer: string; thinking: string };

/** Split model-reported thinking blocks from the user-facing answer. */
export function separateModelThinking(content: string, providerThinking = ''): SeparatedModelOutput {
  let answer = content;
  const thoughts: string[] = [];
  const addThought = (value: string) => {
    const cleaned = value.replace(/^\s+|\s+$/g, '');
    if (!cleaned) return;
    const key = cleaned.replace(/\s+/g, ' ').toLocaleLowerCase();
    if (!thoughts.some((item) => item.replace(/\s+/g, ' ').toLocaleLowerCase() === key)) thoughts.push(cleaned);
  };

  // Common model formats: <think>…</think>, <analysis>…</analysis>, and <reasoning>…</reasoning>.
  answer = answer.replace(/<(think|thinking|analysis|reasoning)\b[^>]*>([\s\S]*?)<\/\1\s*>/gi, (_match, _tag: string, body: string) => {
    addThought(body);
    return '\n';
  });

  // Some gateways/models omit the opening token but still terminate reasoning with </think>.
  const strayClose = /<\/(?:think|thinking|analysis|reasoning)\s*>/i.exec(answer);
  if (strayClose) {
    const prefix = answer.slice(0, strayClose.index).trim();
    const suffix = answer.slice(strayClose.index + strayClose[0].length).trim();
    if (prefix) addThought(prefix);
    answer = suffix;
  }

  // Fail closed for an unclosed reasoning prefix: don't show/copy it as the answer.
  const unclosedOpen = /<(think|thinking|analysis|reasoning)\b[^>]*>/i.exec(answer);
  if (unclosedOpen) {
    const before = answer.slice(0, unclosedOpen.index).trim();
    const body = answer.slice(unclosedOpen.index + unclosedOpen[0].length).trim();
    addThought(body);
    answer = before;
  }

  addThought(providerThinking);
  answer = answer
    .replace(/<\/?(?:think|thinking|analysis|reasoning)\b[^>]*>/gi, '')
    .replace(/\n{3,}/g, '\n\n')
    .trim();
  return { answer, thinking: thoughts.join('\n\n') };
}
