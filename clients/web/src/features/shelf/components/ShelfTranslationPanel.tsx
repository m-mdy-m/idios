import { useState, type ReactNode } from 'react';
import {
  BookOpen, ChevronDown, GraduationCap, Languages, Lightbulb,
  LoaderCircle, Settings2, Sparkles, X,
} from 'lucide-react';
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

const modes: Array<{ value: ShelfTranslationTab; label: string; description: string }> = [
  { value: 'translate', label: 'Translate', description: 'Full page translation' },
  { value: 'vocabulary', label: 'Word', description: 'Meaning in this book and sentence' },
  { value: 'grammar', label: 'Grammar', description: 'Grammar patterns from this page' },
  { value: 'ielts', label: 'IELTS', description: 'Academic language and useful expressions' },
];

function renderMarkdownInline(value: string): ReactNode[] {
  return value.split(/(\\*\\*[^*]+\\*\\*|\\*[^*]+\\*|\x60[^\x60]+\x60)/g).filter(Boolean).map((part, index) => {
    if (part.startsWith('**') && part.endsWith('**')) return <strong key={index}>{part.slice(2, -2)}</strong>;
    if (part.startsWith('*') && part.endsWith('*')) return <em key={index}>{part.slice(1, -1)}</em>;
    if (part.startsWith('`') && part.endsWith('`')) return <code key={index}>{part.slice(1, -1)}</code>;
    return <span key={index}>{part}</span>;
  });
}

function TranslationMarkdown({ value }: { value: string }) {
  const blocks = value.replace(/\\r/g, '').trim().split(/\\n\\s*\\n/).filter(Boolean);
  return <div className="shelf-translation-panel__markdown">
    {blocks.map((block, index) => {
      const lines = block.split('\\n').map((line) => line.trim()).filter(Boolean);
      const heading = lines[0]?.match(/^#{1,3}\\s+(.+)$/);
      if (heading && lines.length === 1) return <h3 key={index}>{renderMarkdownInline(heading[1])}</h3>;
      if (lines.length && lines.every((line) => /^[-*]\\s+/.test(line))) {
        return <ul key={index}>{lines.map((line, itemIndex) => <li key={itemIndex}>{renderMarkdownInline(line.replace(/^[-*]\\s+/, ''))}</li>)}</ul>;
      }
      if (lines.length && lines.every((line) => /^\\d+[.)]\\s+/.test(line))) {
        return <ol key={index}>{lines.map((line, itemIndex) => <li key={itemIndex}>{renderMarkdownInline(line.replace(/^\\d+[.)]\\s+/, ''))}</li>)}</ol>;
      }
      return <p key={index}>{lines.map((line, lineIndex) => <span key={lineIndex}>{lineIndex > 0 && <br />}{renderMarkdownInline(line)}</span>)}</p>;
    })}
  </div>;
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
  pageData, selectionData, wordData, selectedText, hoveredWord,
  pageBusy, analysisBusy, selectionBusy, wordBusy, error,
  onClose, onRetryTranslation, onTranslateSelection, onExplainWord, onSelectTerm, onAnalyzePage,
  translationModel, modelOptions, modelsLoading, modelError, lastUsedModel, onModelChange,
  timeoutSeconds, onTimeoutChange, progress,
}: {
  pageNumber: number;
  targetLanguage: string;
  onTargetLanguageChange: (language: string) => void;
  tab: ShelfTranslationTab;
  onTabChange: (tab: ShelfTranslationTab) => void;
  pageData: ReaderTranslationData | null;
  selectionData: ReaderTranslationData | null;
  wordData: ReaderTranslationData | null;
  selectedText: string;
  hoveredWord: string;
  pageBusy: boolean;
  analysisBusy: boolean;
  selectionBusy: boolean;
  wordBusy: boolean;
  error: string;
  onClose: () => void;
  onRetryTranslation: () => void;
  onTranslateSelection: () => void;
  onExplainWord: () => void;
  onSelectTerm: (term: string) => void;
  onAnalyzePage: (kind: 'grammar' | 'ielts') => void;
  translationModel: string;
  modelOptions: Array<{ value: string; label: string; availability: 'available' | 'unknown'; local: boolean }>;
  modelsLoading: boolean;
  modelError: string;
  lastUsedModel: string;
  onModelChange: (model: string) => void;
  timeoutSeconds: number;
  onTimeoutChange: (seconds: number) => void;
  progress: { done: number; total: number };
}) {
  const [modeMenuOpen, setModeMenuOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const activeMode = modes.find((mode) => mode.value === tab) ?? { value: 'translate' as const, label: 'Translate', description: 'Full page translation' };
  const pageTerm = pageData?.terms.find((term) => termMatches(term, hoveredWord));
  const currentTerm = pageTerm || wordData?.terms[0];
  const selectedWord = /^\p{L}[\p{L}\p{N}_’'-]*$/u.test(selectedText.trim());

  return <aside className="shelf-translation-panel" aria-label="Reading translation companion">
    <header className="shelf-translation-panel__header">
      <div className="shelf-translation-panel__brand">
        <span className="shelf-translation-panel__brand-icon"><Languages size={16} /></span>
        <div><strong>Reading companion</strong><small>Page {pageNumber}</small></div>
      </div>
      <div className="shelf-translation-panel__actions">
        <div className="shelf-translation-panel__menu-anchor">
          <button
            type="button"
            className="shelf-translation-panel__mode-trigger"
            aria-haspopup="menu"
            aria-expanded={modeMenuOpen}
            onClick={() => { setModeMenuOpen((open) => !open); setSettingsOpen(false); }}
          >
            {tab === 'translate' ? <Languages size={14} /> : tab === 'vocabulary' ? <BookOpen size={14} /> : tab === 'grammar' ? <Lightbulb size={14} /> : <GraduationCap size={14} />}
            <span>{activeMode.label}</span><ChevronDown size={13} />
          </button>
          {modeMenuOpen && <div className="shelf-translation-panel__mode-menu" role="menu" aria-label="Reading tool">
            {modes.map((mode) => <button
              key={mode.value}
              type="button"
              role="menuitemradio"
              aria-checked={tab === mode.value}
              className={tab === mode.value ? 'is-active' : ''}
              onClick={() => { onTabChange(mode.value); setModeMenuOpen(false); }}
            >
              <span className="shelf-translation-panel__mode-menu-label">{mode.label}</span>
              <small>{mode.description}</small>
            </button>)}
          </div>}
        </div>
        <button
          type="button"
          className={`shelf-translation-panel__icon-button${settingsOpen ? ' is-active' : ''}`}
          aria-label="Translation settings"
          title="Translation settings"
          aria-expanded={settingsOpen}
          onClick={() => { setSettingsOpen((open) => !open); setModeMenuOpen(false); }}
        ><Settings2 size={15} /></button>
        <button type="button" className="shelf-translation-panel__icon-button" onClick={onClose} aria-label="Close translation panel" title="Close panel"><X size={16} /></button>
      </div>
      {settingsOpen && <section className="shelf-translation-panel__settings" aria-label="Translation settings">
        <label>Translate into
          <select value={targetLanguage} onChange={(event) => onTargetLanguageChange(event.target.value)}>
            {languages.map((language) => <option key={language.value} value={language.value}>{language.label}</option>)}
          </select>
        </label>
        <label>AI model
          <select value={translationModel} onChange={(event) => onModelChange(event.target.value)} disabled={modelsLoading || modelOptions.length === 0}>
            {!translationModel && <option value="">Select a model…</option>}
            {modelOptions.map((model) => <option key={model.value} value={model.value}>{model.label}{model.availability === 'unknown' ? ' · unchecked' : ''}</option>)}
          </select>
        </label>
        <label>Request timeout
          <select value={String(timeoutSeconds)} onChange={(event) => onTimeoutChange(Number(event.target.value))}>
            <option value="120">2 minutes</option>
            <option value="300">5 minutes</option>
            <option value="600">10 minutes</option>
          </select>
        </label>
        <div className="shelf-translation-panel__settings-meta">
          <span>{modelsLoading ? 'Loading model catalogue…' : lastUsedModel || modelOptions.find((model) => model.value === translationModel)?.label || modelError || 'No model selected'}</span>
          <a href="/settings/ai">Manage AI</a>
        </div>
        {modelError && !modelsLoading && <p className="shelf-translation-panel__settings-error">{modelError}</p>}
      </section>}
    </header>

    <div className="shelf-translation-panel__content">
      {error && <div className="shelf-translation-panel__error" role="alert">
        <p>{error}</p>
        {tab === 'translate' && <><span>Completed parts are kept in this reader session. Change the model or timeout in settings, then retry.</span><button type="button" onClick={onRetryTranslation} disabled={pageBusy}>Resume translation</button></>}
      </div>}

      {tab === 'translate' && <>
        {pageBusy && <div className="shelf-translation-panel__progress" role="status" aria-live="polite">
          <span className="shelf-translation-panel__progress-spinner"><LoaderCircle size={14} className="is-spinning" /></span>
          <div><strong>{progress.total > 1 ? `Translating part ${Math.min(progress.done + 1, progress.total)} of ${progress.total}` : 'Translating this page'}</strong><small>{progress.total ? `${progress.done} of ${progress.total} parts complete` : 'Preparing page text…'}</small></div>
          <div className="shelf-translation-panel__progress-track"><i style={{ width: progress.total ? `${Math.min(100, (progress.done / progress.total) * 100)}%` : '5%' }} /></div>
        </div>}
        {selectionData && selectedText.trim() && <article className="shelf-translation-panel__selection-result">
          <small>SELECTED PASSAGE</small><p>{selectionData.translation}</p>
        </article>}
        {selectedText.trim() && !selectedWord && !selectionData && <button type="button" className="shelf-translation-panel__selection-action" onClick={onTranslateSelection} disabled={selectionBusy}>
          {selectionBusy ? <LoaderCircle size={13} className="is-spinning" /> : <Sparkles size={13} />} Translate selected passage
        </button>}
        {pageData?.translation ? <article className="shelf-translation-panel__translation-document" aria-live="polite">
          {pageData.segments.length > 1 ? <p>{pageData.segments.map((segment, index) => {
            const source = normalizeTranslationMatch(segment.source);
            const active = Boolean(hoveredWord && (source === normalizeTranslationMatch(hoveredWord) || source.split(' ').includes(normalizeTranslationMatch(hoveredWord))));
            return <span
              key={`${index}:${segment.source}`}
              className={`shelf-translation-panel__linked-segment${active ? ' is-linked' : ''}`}
              onMouseEnter={() => onSelectTerm(segment.source)}
              title={segment.source}
            >{segment.translation}{index < pageData.segments.length - 1 ? ' ' : ''}</span>;
          })}</p>  : <TranslationMarkdown value={pageData.translation} />}
        </article> : !pageBusy && <div className="shelf-translation-panel__empty">
          <Languages size={21} />
          <strong>{translationModel ? 'No translation available' : 'Choose an AI model'}</strong>
          <p>{translationModel ? 'This page may not contain extractable text. Select a passage in the PDF to translate it.' : 'Open the settings icon to choose a model and target language.'}</p>
        </div>}
        {pageData?.translatorNote && <p className="shelf-translation-panel__note"><strong>Translator’s note</strong>{pageData.translatorNote}</p>}
      </>}

      {tab === 'vocabulary' && <>
        {hoveredWord && <div className="shelf-translation-panel__word-heading"><small>IN THIS SENTENCE</small><strong>{hoveredWord}</strong></div>}
        {selectedText.trim() && !selectedWord && <div className="shelf-translation-panel__selection-hint"><span>Selected passage</span><button type="button" onClick={onTranslateSelection} disabled={selectionBusy}>{selectionBusy ? 'Translating…' : 'Translate passage'}</button></div>}
        {currentTerm ? <article className="shelf-translation-panel__word-card">
          <div><strong>{currentTerm.source}</strong><span>{currentTerm.translation}</span></div>
          {currentTerm.partOfSpeech && <small>{currentTerm.partOfSpeech}</small>}
          <p>{currentTerm.meaning}</p>
          {currentTerm.example && <blockquote>{currentTerm.example}</blockquote>}
        </article> : wordData?.translation ? <article className="shelf-translation-panel__word-card"><p>{wordData.translation}</p></article> : wordBusy ? <div className="shelf-translation-panel__empty"><LoaderCircle size={18} className="is-spinning" /><span>Looking up the word in this book…</span></div> : <div className="shelf-translation-panel__empty"><BookOpen size={21} /><strong>Explore a word</strong><p>Move over a word in the PDF or select one. Its meaning and usage will appear here.</p>{hoveredWord && <button type="button" onClick={onExplainWord}>Explain word</button>}</div>}
        {(pageData?.terms.length ?? 0) > 0 && <div className="shelf-translation-panel__term-list"><small>WORDS FROM THIS PAGE</small>{pageData!.terms.map((term, index) => <button key={index} type="button" onClick={() => onSelectTerm(term.source)}><span>{term.source}</span><strong>{term.translation}</strong></button>)}</div>}
      </>}

      {tab === 'grammar' && <>
        {(pageData?.grammar.length ?? 0) > 0 ? pageData!.grammar.map((note, index) => <GrammarCard key={index} note={note} />) : <div className="shelf-translation-panel__empty">
          <Lightbulb size={21} /><strong>Grammar in context</strong><p>Request a focused analysis of this page. It runs separately from translation and does not slow down the translated text.</p>
          <button type="button" onClick={() => onAnalyzePage('grammar')} disabled={analysisBusy || pageBusy || !pageData?.translation}>{analysisBusy ? <LoaderCircle size={13} className="is-spinning" /> : null} Analyse grammar</button>
        </div>}
      </>}

      {tab === 'ielts' && <>
        {(pageData?.ielts.length ?? 0) > 0 ? pageData!.ielts.map((tip, index) => <IELTSCard key={index} tip={tip} />) : <div className="shelf-translation-panel__empty">
          <GraduationCap size={21} /><strong>IELTS & academic English</strong><p>Find useful academic expressions and collocations in the source page. This analysis is separate from translation.</p>
          <button type="button" onClick={() => onAnalyzePage('ielts')} disabled={analysisBusy || pageBusy || !pageData?.translation}>{analysisBusy ? <LoaderCircle size={13} className="is-spinning" /> : null} Analyse IELTS language</button>
        </div>}
      </>}
    </div>
  </aside>;
}
