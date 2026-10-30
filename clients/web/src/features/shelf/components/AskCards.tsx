import { Link } from 'react-router';
import { ArrowUpRight, BookOpen, Check, CheckCircle2, CircleHelp, MessageCircleQuestion } from 'lucide-react';
import type { ShelfAsk } from '@/types/idios';
import { Button, EmptyState } from '@/design-system';
import { formatShelfDate } from '@/features/shelf/components/ShelfCommon';

export function AskCards({ asks, answerDrafts, setAnswerDrafts, saveAnswer, busy, showBook = true }: {
  asks: ShelfAsk[];
  answerDrafts: Record<string, string>;
  setAnswerDrafts: (update: (current: Record<string, string>) => Record<string, string>) => void;
  saveAnswer: (id: string, answer?: string) => void;
  busy: boolean;
  showBook?: boolean;
}) {
  if (asks.length === 0) return <EmptyState icon={<MessageCircleQuestion size={22} />} title="No questions here" body="Questions are saved as you read. Answered questions stay in the archive instead of disappearing." />;
  return <div className="shelf-asks-list">{asks.map((ask) => <article className={`panel shelf-ask-card${ask.status === 'answered' ? ' shelf-ask-card--answered' : ''}`} key={ask.id}>
    <div className="shelf-ask-card__top"><span className={`shelf-ask-card__status ${ask.status === 'answered' ? 'is-answered' : ''}`}>{ask.status === 'answered' ? <CheckCircle2 size={13} /> : <CircleHelp size={13} />}{ask.status === 'answered' ? 'Answered · archived' : 'Open question'}</span><time>{formatShelfDate(ask.created_at)}</time></div>
    <h3>{ask.question}</h3>
    {(ask.context || ask.reason) && <div className="shelf-ask-card__context">{ask.context && <p><span>Location</span>{ask.context}</p>}{ask.reason && <p><span>Why it came up</span>{ask.reason}</p>}</div>}
    {showBook && <Link className="shelf-ask-card__book" to={`/shelf/${encodeURIComponent(ask.book_id)}`}><BookOpen size={13} /> {ask.book_title} <ArrowUpRight size={12} /></Link>}
    {ask.status === 'answered' ? <div className="shelf-ask-card__answer"><span><CheckCircle2 size={13} /> FOUND ANSWER</span><p>{ask.answer}</p><button type="button" onClick={() => saveAnswer(ask.id, '')}>Reopen question</button></div> : <form className="shelf-ask-card__answer-form" onSubmit={(event) => { event.preventDefault(); saveAnswer(ask.id, answerDrafts[ask.id] ?? ''); }}>
      <label><span>Your answer (leave blank to keep open)</span><textarea className="input" value={answerDrafts[ask.id] ?? ''} onChange={(event) => setAnswerDrafts((current) => ({ ...current, [ask.id]: event.target.value }))} placeholder="Answer, experiment result, reference, or a link to your implementation…" rows={2} /></label>
      <div><span>When you find it, save it to archive this question.</span><Button type="submit" variant="secondary" loading={busy} disabled={!answerDrafts[ask.id]?.trim()}><Check size={13} /> Save answer</Button></div>
    </form>}
  </article>)}</div>;
}
