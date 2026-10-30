import { useMemo, type ReactNode } from 'react';

type InlinePart = { text: string; kind: 'text' | 'bold' | 'italic' | 'code' | 'link'; href?: string };

function inlineParts(value: string): InlinePart[] {
  const result: InlinePart[] = [];
  const pattern = /(\*\*[^*]+\*\*|\*[^*]+\*|`[^`]+`|\[[^\]]+\]\([^)]+\))/g;
  let offset = 0;
  for (const match of value.matchAll(pattern)) {
    const index = match.index ?? 0;
    if (index > offset) result.push({ text: value.slice(offset, index), kind: 'text' });
    const token = match[0];
    if (token.startsWith('**')) result.push({ text: token.slice(2, -2), kind: 'bold' });
    else if (token.startsWith('*')) result.push({ text: token.slice(1, -1), kind: 'italic' });
    else if (token.startsWith('`')) result.push({ text: token.slice(1, -1), kind: 'code' });
    else {
      const link = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(token);
      if (link) result.push({ text: link[1], href: link[2], kind: 'link' });
    }
    offset = index + token.length;
  }
  if (offset < value.length) result.push({ text: value.slice(offset), kind: 'text' });
  return result;
}

function InlineMarkdown({ value }: { value: string }) {
  return <>{inlineParts(value).map((part, index) => {
    if (part.kind === 'bold') return <strong key={index}>{part.text}</strong>;
    if (part.kind === 'italic') return <em key={index}>{part.text}</em>;
    if (part.kind === 'code') return <code key={index}>{part.text}</code>;
    if (part.kind === 'link') {
      let safeHref = '';
      try {
        const url = new URL(part.href ?? '', window.location.href);
        if (url.protocol === 'https:' || url.protocol === 'http:' || url.protocol === 'mailto:') safeHref = url.href;
      } catch { /* Render unsupported/relative file links as plain text. */ }
      return safeHref ? <a href={safeHref} key={index} target="_blank" rel="noreferrer">{part.text}</a> : <span key={index}>{part.text}</span>;
    }
    return <span key={index}>{part.text}</span>;
  })}</>;
}

function startsBlock(lines: string[], index: number) {
  const line = lines[index] ?? '';
  return /^\s*$/.test(line) || /^#{1,6}\s/.test(line) || /^```/.test(line) || /^>\s?/.test(line) || /^\s*[-*+]\s+/.test(line) || /^\s*\d+\.\s+/.test(line) || /^\s*(---+|\*\*\*|___)\s*$/.test(line);
}

function blocks(markdown: string): ReactNode[] {
  const lines = markdown.replaceAll('\r\n', '\n').split('\n');
  const result: ReactNode[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (!line.trim()) { i += 1; continue; }
    if (/^```/.test(line.trim())) {
      const code: string[] = [];
      i += 1;
      while (i < lines.length && !/^```/.test(lines[i].trim())) { code.push(lines[i]); i += 1; }
      if (i < lines.length) i += 1;
      result.push(<pre key={`code-${i}`}><code>{code.join('\n')}</code></pre>);
      continue;
    }
    const heading = /^(#{1,6})\s+(.*)$/.exec(line.trim());
    if (heading) {
      const level = heading[1].length;
      const content = <InlineMarkdown value={heading[2]} />;
      if (level === 1) result.push(<h1 key={i}>{content}</h1>);
      else if (level === 2) result.push(<h2 key={i}>{content}</h2>);
      else if (level === 3) result.push(<h3 key={i}>{content}</h3>);
      else if (level === 4) result.push(<h4 key={i}>{content}</h4>);
      else if (level === 5) result.push(<h5 key={i}>{content}</h5>);
      else result.push(<h6 key={i}>{content}</h6>);
      i += 1;
      continue;
    }
    if (/^\s*(---+|\*\*\*|___)\s*$/.test(line)) { result.push(<hr key={i} />); i += 1; continue; }
    if (/^>\s?/.test(line)) {
      const quote: string[] = [];
      while (i < lines.length && /^>\s?/.test(lines[i])) { quote.push(lines[i].replace(/^>\s?/, '')); i += 1; }
      result.push(<blockquote key={`quote-${i}`}>{quote.map((part, index) => <p key={index}><InlineMarkdown value={part} /></p>)}</blockquote>);
      continue;
    }
    if (/^\s*[-*+]\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\s*[-*+]\s+/.test(lines[i])) { items.push(lines[i].replace(/^\s*[-*+]\s+/, '')); i += 1; }
      result.push(<ul key={`ul-${i}`}>{items.map((item, index) => <li key={index}><InlineMarkdown value={item} /></li>)}</ul>);
      continue;
    }
    if (/^\s*\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) { items.push(lines[i].replace(/^\s*\d+\.\s+/, '')); i += 1; }
      result.push(<ol key={`ol-${i}`}>{items.map((item, index) => <li key={index}><InlineMarkdown value={item} /></li>)}</ol>);
      continue;
    }
    const paragraph = [line.trim()];
    i += 1;
    while (i < lines.length && !startsBlock(lines, i)) { paragraph.push(lines[i].trim()); i += 1; }
    result.push(<p key={`p-${i}`}><InlineMarkdown value={paragraph.join(' ')} /></p>);
  }
  return result;
}

export function MarkdownPreview({ markdown, className = '' }: { markdown: string; className?: string }) {
  const rendered = useMemo(() => blocks(markdown), [markdown]);
  if (!markdown.trim()) return <div className={`shelf-markdown shelf-markdown--empty ${className}`}>This book doesn’t have a README yet.</div>;
  return <article className={`shelf-markdown ${className}`}>{rendered}</article>;
}
