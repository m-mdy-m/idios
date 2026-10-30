import { BookOpen, GraduationCap, Languages, Lightbulb, LoaderCircle, RefreshCw, Sparkles, X } from 'lucide-react';
import type { ReaderGrammarNote, ReaderIELTSTip, ReaderTranslationData, ReaderTranslationTerm } from '../model/reader-translation';
import { normalizeTranslationMatch } from '../model/reader-translation';
import './ShelfTranslationPanel.css';

export type ShelfTranslationTab = 'translate' | 'vocabulary' | 'grammar' | 'ielts';

const languages = [
  { value: 'fa', label: 'فارسی' },
  { value: 'en', label: 'English' },
  { value: 'de', label: 'Deutsch' },
  { value: 'ar', label: 'العربية' },
  { value: 'fr', label: 'Français' },
  { value: 'es', label: 'Español' },
  { value: 'tr', label: 'Türkçe' },
];

function highlightPhrase(value: string, phrase: string) {
  const needle = normalizeTranslationMatch(phrase);
  if (!needle) return value;
  const originalNeedle = phrase.trim();
  const originalIndex = value.toLocaleLowerCase().indexOf(originalNeedle.toLocaleLowerCase());
  if (originalIndex < 0) return value;
  return <>{value.slice(0, originalIndex)}<mark>{value.slice(originalIndex, originalIndex + originalNeedle.length)}</mark>{value.slice(originalIndex + originalNeedle.length)}</>;
}

function termMatches(term: ReaderTranslationTerm, word: string): boolean {
  const needle = normalizeTranslationMatch(word);
  const source = normalizeTranslationMatch(term.source);
  return Boolean(needle && (source === needle || source.includes(needle) || needle.includes(source)));
}

function GrammarCard({ note }: { note: ReaderGrammarNote }) {
  return <article className="shelf-translation-panel__learning-card">
    <strong>{note.pattern}</strong>
    <p>{note.explanation}</p>
    {note.example && <small>Example: {note.example}</small>}
  </article>;
}

function IELTSCard({ tip }: { tip: ReaderIELTSTip }) {
  return <article className="shelf-translation-panel__learning-card">
    <strong>{tip.expression}</strong>
    <p>{tip.academicUse}</p>
    {tip.example && <small>Example: {tip.example}</small>}
  </article>;
}

export function ShelfTranslationPanel({
  pageNumber, targetLanguage, onTargetLanguageChange, tab, onTabChange,
  pageData, selectionData, wordData, activeText, selectedText, hoveredWord,
  pageBusy, selectionBusy, wordBusy, error,
  onClose, onTranslatePage, onTranslateSelection, onExplainWord,
}: {
  pageNumber: number;
  targetLanguage: string;
  onTargetLanguageChange: (language: string) => void;
  tab: ShelfTranslationTab;
  onTabChange: (tab: ShelfTranslationTab) => void;
  pageData: ReaderTranslationData | null;
  selectionData: ReaderTranslationData | null;
  wordData: ReaderTranslationData | null;
  activeText: string;
  selectedText: string;
  hoveredWord: string;
  pageBusy: boolean;
  selectionBusy: boolean;
  wordBusy: boolean;
  error: string;
  onClose: () => void;
  onTranslatePage: () => void;
  onTranslateSelection: () => void;
  onExplainWord: () => void;
}) {
  const activeNeedle = normalizeTranslationMatch(activeText);
  const activeSegment = pageData?.segments.find((segment) => {
    const source = normalizeTranslationMatch(segment.source);
    return Boolean(activeNeedle && (source.includes(activeNeedle) || activeNeedle.includes(source)));
  });
  const pageTerm = pageData?.terms.find((term) => termMatches(term, hoveredWord));
  const wordTerm = wordData?.terms[0];
  const currentTerm = pageTerm || wordTerm;
  const selectedCovered = !selectedText.trim() || Boolean(pageData?.segments.some((segment) =>
    normalizeTranslationMatch(segment.source).includes(normalizeTranslationMatch(selectedText)),
  ));

  return <aside className="shelf-translation-panel" aria-label="AI translation and language learning">
    <header className="shelf-translation-panel__header">
      <div className="shelf-translation-panel__identity">
        <span><Languages size={16} /></span>
        <div><strong>Reading companion</strong><small>Translate · understand · learn</small></div>
      </div>
      <button type="button" className="shelf-translation-panel__close" onClick={onClose} aria-label="Close translation panel" title="Close panel"><X size={16} /></button>
    </header>

    <div className="shelf-translation-panel__controls">
      <label>Translate into
        <select value={targetLanguage} onChange={(event) => onTargetLanguageChange(event.target.value)}>
          {languages.map((language) => <option key={language.value} value={language.value}>{language.label}</option>)}
        </select>
      </label>
      <button type="button" onClick={onTranslatePage} disabled={pageBusy} aria-label="Translate this page again" title="Translate this page again">
        {pageBusy ? <LoaderCircle size={14} className="is-spinning" /> : <RefreshCw size={14} />}
      </button>
    </div>

    <nav className="shelf-translation-panel__tabs" aria-label="Learning tools">
      <button type="button" className={tab === 'translate' ? 'is-active' : ''} onClick={() => onTabChange('translate')}><Languages size={13} /> Translate</button>
      <button type="button" className={tab === 'vocabulary' ? 'is-active' : ''} onClick={() => onTabChange('vocabulary')}><BookOpen size={13} /> Words</button>
      <button type="button" className={tab === 'grammar' ? 'is-active' : ''} onClick={() => onTabChange('grammar')}><Lightbulb size={13} /> Grammar</button>
      <button type="button" className={tab === 'ielts' ? 'is-active' : ''} onClick={() => onTabChange('ielts')}><GraduationCap size={13} /> IELTS</button>
    </nav>

    <div className="shelf-translation-panel__content">
      {error && <p className="shelf-translation-panel__error" role="alert">{error}</p>}
      {tab === 'translate' && <>
        <div className="shelf-translation-panel__section-title"><span>PAGE {pageNumber} · FULL TRANSLATION</span>{pageData && <span>{pageData.segments.length} aligned parts</span>}</div>
        {selectionData && selectedText.trim() && <section className="shelf-translation-panel__selection-result">
          <small>SELECTED PASSAGE</small>
          <blockquote>{selectedText}</blockquote>
          <p>{selectionData.translation}</p>
        </section>}
        {selectedText.trim() && !selectedCovered && !selectionData && <button type="button" className="shelf-translation-panel__action" onClick={onTranslateSelection} disabled={selectionBusy}>
          {selectionBusy ? <LoaderCircle size={14} className="is-spinning" /> : <Sparkles size={14} />} Translate selected passage
        </button>}
        {pageBusy && !pageData && <div className="shelf-translation-panel__loading" role="status"><LoaderCircle size={19} className="is-spinning" /><strong>Translating this page…</strong><span>Using nearby pages and book terminology for context.</span></div>}
        {!pageBusy && !pageData && <div className="shelf-translation-panel__empty"><Languages size={22} /><strong>Translation is not ready</strong><p>Start a page translation to read the full passage beside the PDF.</p><button type="button" onClick={onTranslatePage}>Translate page</button></div>}
        {hoveredWord && currentTerm && <article className="shelf-translation-panel__inline-word">
          <small>{currentTerm.source} · {currentTerm.partOfSpeech || 'IN THIS CONTEXT'}</small>
          <strong>{currentTerm.translation}</strong>
          <p>{currentTerm.meaning}</p>
        </article>}
        {hoveredWord && !currentTerm && wordData?.translation && <article className="shelf-translation-panel__inline-word">
          <small>{hoveredWord} · CONTEXTUAL EXPLANATION</small>
          <p>{wordData.translation}</p>
        </article>}
        {pageData && <div className="shelf-translation-panel__segments" aria-live="polite">
          {(pageData.segments.length ? pageData.segments : [{ source: '', translation: pageData.translation }]).map((segment, index) => {
            const active = Boolean(activeSegment && activeSegment === segment);
            return <article key={index} className={active ? 'is-active' : ''}>
              {segment.source && <small>{segment.source}</small>}
              <p>{active && hoveredWord && currentTerm ? highlightPhrase(segment.translation, currentTerm.translation) : segment.translation}</p>
            </article>;
          })}
        </div>}
        {pageData?.translatorNote && <p className="shelf-translation-panel__note"><strong>Translator's note</strong>{pageData.translatorNote}</p>}
      </>}

      {tab === 'vocabulary' && <>
        <div className="shelf-translation-panel__section-title"><span>CONTEXTUAL VOCABULARY</span><span>PAGE {pageNumber}</span></div>
        {hoveredWord && <div className="shelf-translation-panel__hovered"><small>UNDER YOUR POINTER</small><strong>{hoveredWord}</strong></div>}
        {currentTerm ? <article className="shelf-translation-panel__word-card">
          <div><strong>{currentTerm.source}</strong><span>{currentTerm.translation}</span></div>
          {currentTerm.partOfSpeech && <small>{currentTerm.partOfSpeech}</small>}
          <p>{currentTerm.meaning}</p>
          {currentTerm.example && <blockquote>{currentTerm.example}</blockquote>}
        </article> : wordData?.translation ? <article className="shelf-translation-panel__word-card"><strong>{hoveredWord}</strong><p>{wordData.translation}</p></article> : wordBusy ? <div className="shelf-translation-panel__loading" role="status"><LoaderCircle size={18} className="is-spinning" /><span>Looking up this word in the book's context…</span></div> : hoveredWord ? <div className="shelf-translation-panel__empty"><BookOpen size={22} /><strong>{hoveredWord}</strong><p>No contextual note is available yet.</p><button type="button" onClick={onExplainWord}>Explain this word</button></div> : <div className="shelf-translation-panel__empty"><BookOpen size={22} /><strong>Explore words as you read</strong><p>Move over a word in the PDF to see its contextual meaning, translation, and a natural example here.</p></div>}
        {(pageData?.terms.length ?? 0) > 0 && <div className="shelf-translation-panel__term-list"><small>WORDS FROM THIS PAGE</small>{pageData!.terms.map((term, index) => <button key={index} type="button" onClick={() => onTabChange('vocabulary')}><span>{term.source}</span><strong>{term.translation}</strong></button>)}</div>}
      </>}

      {tab === 'grammar' && <>
        <div className="shelf-translation-panel__section-title"><span>GRAMMAR IN CONTEXT</span><span>PAGE {pageNumber}</span></div>
        {pageBusy && !pageData ? <div className="shelf-translation-panel__loading"><LoaderCircle size={18} className="is-spinning" /><span>Reading the page for useful grammar patterns…</span></div> : (pageData?.grammar.length ?? 0) > 0 ? pageData!.grammar.map((note, index) => <GrammarCard key={index} note={note} />) : <div className="shelf-translation-panel__empty"><Lightbulb size={22} /><strong>No grammar notes yet</strong><p>Translate the page first. The companion will point out useful structures without turning the reading into a lesson.</p><button type="button" onClick={onTranslatePage} disabled={pageBusy}>Analyse this page</button></div>}
      </>}

      {tab === 'ielts' && <>
        <div className="shelf-translation-panel__section-title"><span>IELTS & ACADEMIC ENGLISH</span><span>PAGE {pageNumber}</span></div>
        {pageBusy && !pageData ? <div className="shelf-translation-panel__loading"><LoaderCircle size={18} className="is-spinning" /><span>Finding useful academic language…</span></div> : (pageData?.ielts.length ?? 0) > 0 ? pageData!.ielts.map((tip, index) => <IELTSCard key={index} tip={tip} />) : <div className="shelf-translation-panel__empty"><GraduationCap size={22} /><strong>Build academic English from this book</strong><p>Translate the page to identify useful academic expressions, collocations, and natural examples. No band score is guessed.</p><button type="button" onClick={onTranslatePage} disabled={pageBusy}>Analyse this page</button></div>}
      </>}
    </div>
    <footer className="shelf-translation-panel__footer"><span>AI-assisted · grounded in this book</span><span>Page {pageNumber}</span></footer>
  </aside>;
}
