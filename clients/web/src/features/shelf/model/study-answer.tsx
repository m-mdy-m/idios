import type { ReactNode } from 'react';

function inlineText(value: string): ReactNode[] {
  return value.split(/(\*\*[^*]+\*\*|`[^`]+`)/g).map((part, index) => {
    if (part.startsWith('**') && part.endsWith('**')) return <strong key={index}>{part.slice(2, -2)}</strong>;
    if (part.startsWith('`') && part.endsWith('`')) return <code key={index}>{part.slice(1, -1)}</code>;
    return <span key={index}>{part}</span>;
  });
}

export function renderAnswer(content: string): ReactNode[] {
  return content.split(/\n{2,}/).map((block, index) => {
    const trimmed = block.trim();
    if (!trimmed) return null;
    const heading = trimmed.match(/^#{1,3}\s+(.+)$/);
    if (heading) return <h4 key={index}>{inlineText(heading[1])}</h4>;
    const lines = trimmed.split('\n');
    if (lines.every((line) => /^\s*(?:[-*•]|\d+[.)])\s+/.test(line))) {
      const numbered = /^\s*\d+[.)]\s+/.test(lines[0]);
      const Tag = numbered ? 'ol' : 'ul';
      return <Tag key={index}>{lines.map((line, itemIndex) => <li key={itemIndex}>{inlineText(line.replace(/^\s*(?:[-*•]|\d+[.)])\s+/, ''))}</li>)}</Tag>;
    }
    return <p key={index}>{lines.map((line, lineIndex) => <span key={lineIndex}>{lineIndex > 0 && <br />}{inlineText(line)}</span>)}</p>;
  }).filter(Boolean);
}

