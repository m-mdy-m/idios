import type { PageInsight, StudyContext } from './study-types';
import { loadPageInsight, loadReadHistory } from './study-storage';

export function makeStudyContext(context: StudyContext, bookTitle: string, bookID: string, pageNumber: number, pageText: string, selectedText: string, hoveredText: string, documentContext = ''): string {
  const parts = [`Book: ${bookTitle} (library ID: ${bookID})`, `Current page: ${pageNumber}`];
  if (context.book?.summary?.trim()) parts.push(`Book summary from its library entry:\n${context.book.summary.trim().slice(0, 5000)}`);
  if (context.book?.tags?.length) parts.push(`Book tags: ${context.book.tags.join(', ')}`);
  if (selectedText.trim()) parts.push(`Passage selected by the reader:\n${selectedText.trim().slice(0, 12000)}`);
  else if (hoveredText.trim()) parts.push(`Word the reader asked about:\n${hoveredText.trim().slice(0, 500)}`);
  if (pageText.trim()) parts.push(`Text extracted from the current PDF page ${pageNumber}:\n${pageText.trim()}`);
  if (documentContext.trim()) parts.push(`COMPLETE PDF READING WINDOW — use this actual extracted text, not the visited-page history. Every page from PDF page 1 through the current page and the next three pages is included when available. Read all supplied pages before answering.\n${documentContext}`);

  const visitedPages = loadReadHistory(bookID).filter((item) => item.page !== pageNumber).slice(-10);
  if (visitedPages.length) {
    parts.push(`Recent pages the reader has actually visited (use as continuity, not as a complete transcript):\n${visitedPages.map((item) => `- Page ${item.page}: ${item.text.slice(0, 850)}`).join('\n').slice(0, 9000)}`);
  }
  const priorPageInsights = visitedPages.slice(-6).map((item) => loadPageInsight(bookID, item.page)).filter((item): item is PageInsight => Boolean(item));
  if (priorPageInsights.length) {
    parts.push(`Earlier page-side learning notes generated while reading this book (use only as continuity; the source passage remains authoritative):\n${priorPageInsights.map((item) => `- Page ${item.page} (${item.topic}): ${item.companion}${item.tutorReply ? ` Tutor added: ${item.tutorReply}` : ''}`).join('\n').slice(0, 4200)}`);
  }
  if (context.notes.length) {
    parts.push(`Reader's saved notes for this book or explicitly linked to it (the reader's own knowledge, not necessarily claims by the author):\n${context.notes.slice(0, 24).map((note) => `- ${note.title || 'Untitled note'}${note.labels?.length ? ` [${note.labels.map((label) => label.name).join(', ')}]` : ''}: ${note.content.slice(0, 1600)}`).join('\n').slice(0, 14000)}`);
  }
  const annotated = context.annotations.filter((item) => item.quote.trim() || item.note?.trim()).slice(-22);
  if (annotated.length) parts.push(`Saved highlights and marginal notes from this book:\n${annotated.map((item) => `- Page ${item.page}: “${item.quote.slice(0, 700)}”${item.note?.trim() ? ` — Reader's note: ${item.note.trim().slice(0, 700)}` : ''}`).join('\n').slice(0, 12000)}`);
  const questions = context.previousQuestions.filter((item) => item.question.trim() || item.answer?.trim()).slice(-10);
  if (questions.length) parts.push(`Earlier questions the reader saved for this book:\n${questions.map((item) => `- Q: ${item.question.slice(0, 700)}${item.context?.trim() ? ` (context: ${item.context.slice(0, 350)})` : ''}${item.answer?.trim() ? `\n  A/notes: ${item.answer.slice(0, 900)}` : ''}`).join('\n').slice(0, 8000)}`);
  if (context.relations.length) {
    const related = context.relations.map((relation) => {
      const otherID = relation.source_book_id === bookID ? relation.target_book_id : relation.source_book_id;
      const other = context.relatedBooks.find((book) => book.id === otherID);
      const title = other?.title || (relation.source_book_id === bookID ? relation.target_book_title : relation.source_book_title);
      return `- ${title}: ${relation.relation_type.replaceAll('_', ' ')}`;
    });
    parts.push(`Reader-defined connections to other books (titles only; do not assume their contents):\n${related.join('\n').slice(0, 3500)}`);
  }
  return parts.join('\n\n---\n\n');
}
