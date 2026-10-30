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
  onClose, onTranslatePage, onTranslateSelection, onExplainWord, onSelectTerm,
  translationModel, modelOptions, modelsLoading, modelError, lastUsedModel, onModelChange, progress,
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
  onSelectTerm: (term: string) => void;
  translationModel: string;
  modelOptions: Array<{ value: string; label: string; availability: 'available' | 'unknown'; local: boolean }>;
  modelsLoading: boolean;
  modelError: string;
  lastUsedModel: string;
  onModelChange: (model: string) => void;
  progress: { done: number; total: number };
}) {
  const activeNeedle = normalizeTranslationMatch(activeText);
  const activeSegment = pageData?.segments.find((segment) => {
    const source = normalizeTranslationMatch(segment.source);
    return Boolean(activeNeedle && (source.includes(activeNeedle) || activeNeedle.includes(source)));
  });
  const pageTerm = pageData?.terms.find((term) => termMatches(term, hoveredWord));
  const wordTerm = wordData?.terms[0];
  const currentTerm = pageTerm || wordTerm;
  const selectedWord = /^\p{L}[\p{L}\p{N}_’'-]*$/u.test(selectedText.trim());
  const selectedCovered = !selectedText.trim() || selectedWord || Boolean(pageData?.segments.some((segment) =>
    normalizeTranslationMatch(segment.source) === normalizeTranslationMatch(selectedText),
  ));

  return <aside className="shelf-translation-panel" aria-label="AI translation and language learning">
    <header className="shelf-translation-panel__header">
      <div className="shelf-translation-panel__identity">
        <span><Languages size={16} /></span>
        <div><strong>Reading companion</strong><small>Translate · understand · learn</small></div>
      </div>
      <button type="button" className="shelf-translation-panel__close" onClick={onClose} aria-label="Close translation panel" title="Close panel"><X size={16} /></button>
    </header>

    <section className="shelf-translation-panel__model-card" aria-label="AI translation model">
      <label className="shelf-translation-panel__model-label" htmlFor="shelf-reader-model">AI MODEL</label>
      <div className="shelf-translation-panel__model-select">
        <select id="shelf-reader-model" value={translationModel} onChange={(event) => onModelChange(event.target.value)} disabled={modelsLoading || modelOptions.length === 0}>
          {!translationModel && <option value="">Select a model…</option>}
          {modelOptions.map((model) => <option key={model.value} value={model.value}>{model.label}{model.availability === 'unknown' ? ' · not checked' : ''}</option>)}
        </select>
        {modelsLoading ? <LoaderCircle size={14} className="is-spinning" /> : <span className="shelf-translation-panel__model-dot" />}
      </div>
      <div className="shelf-translation-panel__model-meta">
        <span>{translationModel ? (lastUsedModel || modelOptions.find((model) => model.value === translationModel)?.label || translationModel) : modelError || 'Choose a model to begin'}</span>
        <a href="/settings/ai">Manage models</a>
      </div>
      {modelError && !modelsLoading && <p className="shelf-translation-panel__model-error">{modelError}</p>}
    </section>

    <div className="shelf-translation-panel__controls">
      <label>Translate into
        <select value={targetLanguage} onChange={(event) => onTargetLanguageChange(event.target.value)}>
          {languages.map((language) => <option key={language.value} value={language.value}>{language.label}</option>)}
        </select>
      </label>
      <button type="button" onClick={onTranslatePage} disabled={pageBusy || !translationModel} aria-label="Translate this page again" title="Translate this page again">
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
      {error && <div className="shelf-translation-panel__error" role="alert">
        <p>{error}</p>
        {tab === 'translate' && <button type="button" onClick={onTranslatePage} disabled={!translationModel || pageBusy}>
          <RefreshCw size={12} /> {pageData ? 'Resume translation' : 'Retry translation'}
        </button>}
      </div>}
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
        {pageBusy && <div className="shelf-translation-panel__progress" role="status" aria-live="polite">
          <div><LoaderCircle size={16} className="is-spinning" /><strong>{progress.total > 1 ? `Translating part ${Math.max(1, progress.done + 1)} of ${progress.total}` : 'Translating this page'}</strong><span>{progress.total > 1 ? 'Completed parts are cached; retries continue from the interrupted part.' : 'Using the selected model and nearby book context.'}</span></div>
          <div className="shelf-translation-panel__progress-track"><i style={{ width: progress.total ? `${Math.min(100, (progress.done / progress.total) * 100)}%` : '8%' }} /></div>
          <small>{progress.total ? `${progress.done}/${progress.total} parts complete` : 'Preparing passage…'} · {lastUsedModel || 'Connecting to model'}</small>
        </div>}
        {!pageBusy && !pageData && <div className="shelf-translation-panel__empty"><Languages size={22} /><strong>{translationModel ? 'Ready to translate' : 'Choose your translation model'}</strong><p>{translationModel ? 'Translate this page in short parts. Completed parts are kept so a retry does not start from zero.' : 'Select a model above, or open AI Settings to configure and refresh your model catalogue.'}</p><button type="button" onClick={onTranslatePage} disabled={!translationModel}>Translate page</button></div>}
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
        {(pageData?.terms.length ?? 0) > 0 && <div className="shelf-translation-panel__term-list"><small>WORDS FROM THIS PAGE</small>{pageData!.terms.map((term, index) => <button key={index} type="button" onClick={() => { onTabChange('vocabulary'); onSelectTerm(term.source); }}><span>{term.source}</span><strong>{term.translation}</strong></button>)}</div>}
      </>}

      {tab === 'grammar' && <>
        <div className="shelf-translation-panel__section-title"><span>GRAMMAR IN CONTEXT</span><span>PAGE {pageNumber}</span></div>
        {pageBusy && !pageData ? <div className="shelf-translation-panel__loading"><LoaderCircle size={18} className="is-spinning" /><span>Reading the page for useful grammar patterns…</span></div> : (pageData?.grammar.length ?? 0) > 0 ? pageData!.grammar.map((note, index) => <GrammarCard key={index} note={note} />) : <div className="shelf-translation-panel__empty"><Lightbulb size={22} /><strong>No grammar notes yet</strong><p>Translate the page first. The companion will point out useful structures without turning the reading into a lesson.</p><button type="button" onClick={onTranslatePage} disabled={pageBusy || !translationModel}>Analyse this page</button></div>}
      </>}

      {tab === 'ielts' && <>
        <div className="shelf-translation-panel__section-title"><span>IELTS & ACADEMIC ENGLISH</span><span>PAGE {pageNumber}</span></div>
        {pageBusy && !pageData ? <div className="shelf-translation-panel__loading"><LoaderCircle size={18} className="is-spinning" /><span>Finding useful academic language…</span></div> : (pageData?.ielts.length ?? 0) > 0 ? pageData!.ielts.map((tip, index) => <IELTSCard key={index} tip={tip} />) : <div className="shelf-translation-panel__empty"><GraduationCap size={22} /><strong>Build academic English from this book</strong><p>Translate the page to identify useful academic expressions, collocations, and natural examples. No band score is guessed.</p><button type="button" onClick={onTranslatePage} disabled={pageBusy}>Analyse this page</button></div>}
      </>}
    </div>
    <footer className="shelf-translation-panel__footer"><span>AI-assisted · grounded in this book</span><span>Page {pageNumber}</span></footer>
  </aside>;
}
