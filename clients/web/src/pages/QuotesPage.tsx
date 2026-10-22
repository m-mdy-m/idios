import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';
import { Copy, Dices, ExternalLink, ListPlus, Plus, Quote as QuoteIcon, Search, Trash2 } from 'lucide-react';
import { api } from '@/lib/api';
import type { ProtocolCommand, Quote } from '@/types/idios';
import { Badge, Button, Card, EmptyState, PageHeading } from '@/components/ui/primitives';
import { CommandRunner } from '@/components/commands/CommandRunner';
import { Modal } from '@/components/ui/Modal';
import { formatRelative } from '@/components/ui/IssueTable';
import { useWorkspaceStore } from '@/store/workspace-store';

export function QuotesPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [items, setItems] = useState<Quote[]>([]);
  const [next, setNext] = useState<Quote | null>(null);
  const [commands, setCommands] = useState<ProtocolCommand[]>([]);
  const [selectedCommand, setSelectedCommand] = useState<ProtocolCommand | null>(null);
  const [selectedQuote, setSelectedQuote] = useState<Quote | null>(null);
  const [query, setQuery] = useState('');
  const [visibleCount, setVisibleCount] = useState(6);
  const [initialValues, setInitialValues] = useState<Record<string, unknown>>({});
  const [error, setError] = useState('');
  const [bulkCreateOpen, setBulkCreateOpen] = useState(false);
  const { load } = useWorkspaceStore();

  async function refresh() {
    try {
      const [quotes, catalog] = await Promise.all([api.quotes(), api.protocol()]);
      setItems(quotes);
      setCommands(catalog.commands);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to load quotes.');
    }
  }
  useEffect(() => { void refresh(); }, []);
  useEffect(() => {
    if (searchParams.get('bulk') !== '1' || !commands.some((item) => item.id === 'quote.bulk_create')) return;
    setBulkCreateOpen(true);
    const nextParams = new URLSearchParams(searchParams);
    nextParams.delete('bulk');
    setSearchParams(nextParams, { replace: true });
  }, [searchParams, commands, setSearchParams]);

  const filtered = useMemo(() => {
    const search = query.trim().toLowerCase();
    return items.filter((quote) => !search || `${quote.text} ${quote.author ?? ''} ${quote.source ?? ''} ${quote.category ?? ''}`.toLowerCase().includes(search));
  }, [items, query]);

  const visibleQuotes = filtered.slice(0, visibleCount);

  function run(id: string, values: Record<string, unknown> = {}) {
    const command = commands.find((item) => item.id === id);
    if (!command) return;
    setInitialValues(values);
    setSelectedCommand(command);
    setSelectedQuote(null);
  }
  async function complete() { setSelectedQuote(null); await Promise.all([refresh(), load(true)]); }
  async function pickNext() {
    setError('');
    try { setNext(await api.nextQuote()); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to select a quote.'); }
  }
  async function copyQuote(quote: Quote) {
    try {
      await navigator.clipboard.writeText(`“${quote.text}”${quote.author ? ` — ${quote.author}` : ''}`);
    } catch { setError('Clipboard access is unavailable in this browser context.'); }
  }

  return <div className="page-stack">
    <PageHeading eyebrow="PERSONAL LIBRARY" title="Quotes" description="Save lines worth remembering, then find them quickly when you need them." actions={<><Button variant="secondary" onClick={() => setBulkCreateOpen(true)} disabled={!commands.some((item) => item.id === 'quote.bulk_create')}><ListPlus size={16} /> Save many</Button><Button onClick={() => run('quote.create')}><Plus size={16} /> Save quote</Button></>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}

    <Card className="quote-feature-card quote-feature-card--compact">
      <span className="quote-feature-mark"><QuoteIcon size={19} /></span>
      <div className="quote-feature-copy"><p className="eyebrow">A LINE TO KEEP</p>{next ? <><blockquote>{next.text}</blockquote><p className="quote-attribution">{next.author || 'Unknown author'}{next.source ? ` · ${next.source}` : ''}</p></> : <><h2>Need a different perspective?</h2><p>Pick a random quote from your collection.</p></>}<div className="quote-feature-actions"><Button onClick={() => void pickNext()}><Dices size={15} /> Pick a quote</Button>{next && <Button variant="secondary" onClick={() => void copyQuote(next)}><Copy size={14} /> Copy</Button>}</div></div>
    </Card>

    <section className="quote-library" aria-label="Saved quotes">
      <div className="quote-library__toolbar"><div><h2>Saved quotes</h2><span>{filtered.length ? `Showing ${visibleQuotes.length} of ${filtered.length}` : `0 of ${items.length}`}</span></div><label className="inline-search"><Search size={16} /><input value={query} onChange={(event) => { setQuery(event.target.value); setVisibleCount(6); }} placeholder="Search quotes, authors, sources…" aria-label="Search quotes" /></label></div>
      {visibleQuotes.length ? <div className="quote-tiles">{visibleQuotes.map((quote) => <button className="quote-tile" type="button" key={quote.id} onClick={() => setSelectedQuote(quote)}><span className="quote-tile__mark"><QuoteIcon size={15} /></span><span className="quote-tile__text">{quote.text}</span><span className="quote-tile__author">{quote.author || 'Unknown author'}</span><span className="quote-tile__source">{quote.source || quote.category || 'Source not added'}</span><span className="quote-tile__footer"><time>{formatRelative(quote.created_at)}</time><span>Details <ExternalLink size={12} /></span></span></button>)}</div> : <EmptyState title={query ? 'No quotes found' : 'Your quote library is empty'} body={query ? 'Try another search phrase.' : 'Save a useful line with an author or source to build your library.'} icon={<QuoteIcon size={21} />} />}
      {filtered.length > 6 && <div className="quote-library__more">{visibleCount < filtered.length ? <Button variant="secondary" onClick={() => setVisibleCount((count) => Math.min(count + 6, filtered.length))}>Show more quotes ({filtered.length - visibleCount} left)</Button> : <Button variant="ghost" onClick={() => setVisibleCount(6)}>Show fewer</Button>}</div>}
    </section>

    <Modal open={Boolean(selectedQuote)} title="Quote details" description="The full quote and its source information." onClose={() => setSelectedQuote(null)}>
      {selectedQuote && <div className="quote-detail"><blockquote>{selectedQuote.text}</blockquote><div className="quote-detail__meta"><strong>{selectedQuote.author || 'Unknown author'}</strong><span>Saved {formatRelative(selectedQuote.created_at)}</span>{selectedQuote.category && <Badge>{selectedQuote.category}</Badge>}</div>{(selectedQuote.source || selectedQuote.source_url) && <p className="quote-detail__source">Source: {selectedQuote.source || 'Source link'}{selectedQuote.source_url && <> · <a href={selectedQuote.source_url} target="_blank" rel="noreferrer">Open source <ExternalLink size={12} /></a></>}</p>}<div className="quote-detail__actions"><Button type="button" variant="secondary" onClick={() => void copyQuote(selectedQuote)}><Copy size={15} /> Copy quote</Button><Button type="button" variant="danger" onClick={() => run('quote.delete', { id: selectedQuote.id })}><Trash2 size={15} /> Delete</Button></div></div>}
    </Modal>
    <CommandRunner command={commands.find((item) => item.id === 'quote.bulk_create') ?? null} open={bulkCreateOpen} onClose={() => setBulkCreateOpen(false)} onSuccess={complete} />
    <CommandRunner command={selectedCommand} open={Boolean(selectedCommand)} initialValues={initialValues} onClose={() => setSelectedCommand(null)} onSuccess={complete} />
  </div>;
}
