import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type PointerEvent as ReactPointerEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import {
  ArrowLeft, ArrowRight, BookMarked, BookOpenText, Check, ChevronDown, ChevronUp, CircleHelp, Copy,
  GraduationCap, History, Layers3, Lightbulb, Link2, LoaderCircle, MessageCircle, Pencil, RefreshCw,
  RotateCcw, Search, Send, Settings2, Sparkles, X, Minimize2, Trash2, MoreHorizontal,
} from 'lucide-react';
import { api, type AIModel, type AIStatus } from '@/lib/api';
import type { Note } from '@/types/idios';
import readingCompanionSkill from '../model/reading-companion.skill.md?raw';
import coreReliabilitySkill from '../../reading-skills/skills/core-reliability/SKILL.md?raw';
import { READING_SKILLS, SKILL_SELECTION_STORAGE_KEY, loadSelectedReadingSkills, selectedReadingSkillInstructions } from '@/features/reading-skills';
import { separateModelThinking } from '../model/modelThinking';
import './ShelfStudyAssistant.css';
import type {
  AISettings, ArchivedThread, AskOptions, ChatMessage, CompanionMark, MascotDrag, MascotPosition,
  ModelAvailability, ModelOption, PageInsight, Props, PromptMode, ReasoningEffort, RequestedBookAction, StudyContext,
} from '../model/study-types';
import {
  isBookMutationRequest, isBroadPageAnnotationRequest, isExplicitBookNoteRequest, isExplicitHighlightRequest,
  parseBookActionResponse,
} from '../model/study-actions';
import {
  archivedThreadSnapshot, archivedThreadsKey, emptyContext, fallbackTopicMark, getContent,
  loadArchivedThreads, loadMascotBubbleOpen, loadMascotPosition, loadPageInsight, loadReadHistory,
  loadSettings, loadThread, mascotBubbleKey, mascotPositionKey, migrateLegacyShelfProvider,
  overviewKey, overviewPrompt, pageInsightKey, parsePageInsight, publishCompanionHighlights,
  quoteWords, rememberReadPage, runShelfAIThroughRuntime, settingsKey, threadKey,
  userFacingAIError, validatedCompanionMarks,
} from '../model/study-storage';
import { makeStudyContext } from '../model/study-context';
import { fitShelfRuntimeMessages } from '../model/study-request-budget';
import { renderAnswer } from '../model/study-answer';

const promptModes: PromptMode[] = [
  { label: 'Unpack this page', icon: Lightbulb, prompt: 'Can you help me get what matters on this page? Tell me the main point in plain language, explain one tricky term or step, and give me one everyday example if it helps. Keep it natural and fairly short; I can ask you to go deeper.' },
  { label: 'Think together', icon: MessageCircle, prompt: 'Let’s think about this page together. Point out one detail or assumption that is easy to miss, then ask me one natural question that helps me look at it differently. Don’t make it feel like a test.' },
  { label: 'Connect my notes', icon: Link2, prompt: 'Compare the current passage with my saved notes, highlights, prior visited pages, and earlier discussion. Identify a meaningful link, tension, repeated pattern, or possible gap. Refer to page numbers and note titles only when supplied. Explain the connection simply.' },
  { label: 'Remember it', icon: BookMarked, prompt: 'Start active recall using the current page and what we have already discussed. Ask one focused question at a time and wait for my answer before explaining it. Adapt difficulty based on my earlier responses.' },
];

export function ShelfStudyAssistant({ bookID, bookTitle, pageNumber, pageText, selectedText, hoveredText, displayMode, onOpenChat, onBackToReading, onOpenModels, onBackToChat, onCloseInlineChat, onApplyBookHighlights, onGetDocumentContext, initialPrompt = '', initialSettingsOpen = false, quietMode = false }: Props) {
  const [settings, setSettings] = useState<AISettings>(loadSettings);
  const [models, setModels] = useState<ModelOption[]>([]);
  const [modelSearch, setModelSearch] = useState('');
  const [loadingModels, setLoadingModels] = useState(false);
  const [modelsChecked, setModelsChecked] = useState(false);
  const [modelProbeProgress, setModelProbeProgress] = useState({ done: 0, total: 0 });
  const [testingModelID, setTestingModelID] = useState('');
  const [busy, setBusy] = useState(false);
  const [question, setQuestion] = useState('');
  const [editingMessageIndex, setEditingMessageIndex] = useState<number | null>(null);
  const [openTryModelIndex, setOpenTryModelIndex] = useState<number | null>(null);
  const [tryModelSelections, setTryModelSelections] = useState<Record<number, string>>({});
  const [reasoningEffort, setReasoningEffort] = useState<ReasoningEffort>('medium');
  const [openThinkingControl, setOpenThinkingControl] = useState<string | null>(null);
  const [collapsedModelThinking, setCollapsedModelThinking] = useState<Record<string, boolean>>({});
  const [messages, setMessages] = useState<ChatMessage[]>(() => loadThread(bookID));
  const [archivedThreads, setArchivedThreads] = useState<ArchivedThread[]>(() => loadArchivedThreads(bookID));
  const [showThreadHistory, setShowThreadHistory] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [lastFailedPrompt, setLastFailedPrompt] = useState('');
  const [savingNote, setSavingNote] = useState(false);
  const [context, setContext] = useState<StudyContext>(emptyContext);
  const [pageInsight, setPageInsight] = useState<PageInsight | null>(() => loadPageInsight(bookID, pageNumber));
  const [insightBusy, setInsightBusy] = useState(false);
  const [insightError, setInsightError] = useState('');
  const [insightErrorsByPage, setInsightErrorsByPage] = useState<Record<string, string>>({});
  const [inlineSettingsOpen, setInlineSettingsOpen] = useState(initialSettingsOpen);
  const [inlineSkillsOpen, setInlineSkillsOpen] = useState(false);
  const [selectedSkillIDs, setSelectedSkillIDs] = useState<string[]>(loadSelectedReadingSkills);
  const [inlineHeaderMenuOpen, setInlineHeaderMenuOpen] = useState(false);
  const [mascotPosition, setMascotPosition] = useState<MascotPosition | null>(() => loadMascotPosition(bookID));
  const [mascotBubbleOpen, setMascotBubbleOpen] = useState(() => loadMascotBubbleOpen(bookID));
  const [thoughtExpanded, setThoughtExpanded] = useState(true);
  const [mascotQuestion, setMascotQuestion] = useState('');
  const mascotDragRef = useRef<MascotDrag | null>(null);
  const suppressMascotClickRef = useRef(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const thinkingControlRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const didAutoLoadModels = useRef(false);
  const didAutoOverview = useRef(false);
  const insightInFlightRef = useRef('');
  const insightLastTriedRef = useRef('');
  const activePageKeyRef = useRef(`${bookID}:${pageNumber}`);
  const configured = Boolean(settings.model.trim() && models.some((model) => model.id === settings.model));
  const currentModel = models.find((model) => model.id === settings.model);
  const selectedModelAvailable = currentModel?.availability === 'available';
  const supportsThinking = currentModel?.reasoning === true;
  const availableModels = models.filter((model) => model.availability === 'available');
  const contextCount = context.notes.length + context.annotations.length + context.previousQuestions.length;
  const availableCount = models.filter((model) => model.availability === 'available').length;
  const selectedOrHovered = selectedText.trim() || hoveredText.trim();

  useEffect(() => {
    try { localStorage.setItem(settingsKey, JSON.stringify({ model: settings.model } satisfies AISettings)); }
    catch { setNotice('Browser storage is unavailable. Your model selection will not persist after refresh.'); }
  }, [settings.model]);
  useEffect(() => {
    if (displayMode === 'inline-chat') setInlineSettingsOpen(initialSettingsOpen);
  }, [displayMode, initialSettingsOpen]);
  useEffect(() => {
    try { localStorage.setItem(SKILL_SELECTION_STORAGE_KEY, JSON.stringify(selectedSkillIDs)); } catch { /* Skill choices remain active for this session. */ }
  }, [selectedSkillIDs]);
  const activeSkillInstructions = useMemo(() => selectedReadingSkillInstructions(selectedSkillIDs), [selectedSkillIDs]);

  useEffect(() => {
    if (!openThinkingControl) return;
    const handlePointerDown = (event: PointerEvent) => {
      const root = thinkingControlRefs.current[openThinkingControl];
      if (root && event.target instanceof Node && !root.contains(event.target)) setOpenThinkingControl(null);
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      setOpenThinkingControl(null);
      thinkingControlRefs.current[openThinkingControl]?.querySelector<HTMLButtonElement>('.shelf-ai-thinking-control__trigger')?.focus();
    };
    document.addEventListener('pointerdown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('pointerdown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [openThinkingControl]);

  useEffect(() => {
    try { localStorage.setItem(threadKey(bookID), JSON.stringify(messages.slice(-60))); }
    catch { /* The live thread still works even without browser storage. */ }
  }, [bookID, messages]);

  useEffect(() => {
    try { localStorage.setItem(archivedThreadsKey(bookID), JSON.stringify(archivedThreads.slice(0, 15))); }
    catch { /* Previous chats remain in memory for this session. */ }
  }, [bookID, archivedThreads]);

  useEffect(() => {
    if (pageText.trim()) rememberReadPage(bookID, pageNumber, pageText);
  }, [bookID, pageNumber, pageText]);

  useEffect(() => {
    let cancelled = false;
    setContext({ ...emptyContext, loading: true });
    Promise.allSettled([
      api.shelfBook(bookID), api.shelfBookNotes(bookID), api.shelfAnnotations(bookID),
      api.shelfBookAsks(bookID), api.shelfRelations(bookID), api.shelfBooks(), api.notes(),
    ]).then((results) => {
      if (cancelled) return;
      const [bookResult, bookNotesResult, annotationsResult, asksResult, relationsResult, booksResult, notesResult] = results;
      const book = bookResult.status === 'fulfilled' ? bookResult.value : null;
      const bookNotes = bookNotesResult.status === 'fulfilled' ? bookNotesResult.value : [];
      const globalNotes = notesResult.status === 'fulfilled' ? notesResult.value : [];
      const linkedNotes = globalNotes.filter((note) => note.linked_entity_type === 'shelf_book' && note.linked_entity_id === bookID);
      const notesByID = new Map<string, Note>();
      [...bookNotes, ...linkedNotes].forEach((note) => notesByID.set(note.id, note));
      setContext({
        book,
        notes: [...notesByID.values()].sort((a, b) => b.updated_at.localeCompare(a.updated_at)),
        annotations: annotationsResult.status === 'fulfilled' ? annotationsResult.value : [],
        previousQuestions: asksResult.status === 'fulfilled' ? asksResult.value : [],
        relations: relationsResult.status === 'fulfilled' ? relationsResult.value : [],
        relatedBooks: booksResult.status === 'fulfilled' ? booksResult.value : [],
        loading: false,
      });
    });
    return () => { cancelled = true; };
  }, [bookID]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: busy ? 'smooth' : 'auto', block: 'end' });
  }, [messages, busy]);

  useEffect(() => {
    if ((displayMode === 'chat' || displayMode === 'inline-chat') && initialPrompt.trim()) setQuestion(initialPrompt.trim());
  }, [displayMode, initialPrompt]);

  useEffect(() => {
    if (displayMode !== 'inline-chat') return;
    document.body.classList.add('idios-reading-companion-chat-open');
    return () => document.body.classList.remove('idios-reading-companion-chat-open');
  }, [displayMode]);

  useEffect(() => {
    if (!inlineHeaderMenuOpen) return;
    const closeOutside = (event: PointerEvent) => {
      if (event.target instanceof Element && !event.target.closest('.shelf-study-ai-inline__menu-wrap')) setInlineHeaderMenuOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setInlineHeaderMenuOpen(false);
    };
    document.addEventListener('pointerdown', closeOutside);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('pointerdown', closeOutside);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [inlineHeaderMenuOpen]);

  useEffect(() => {
    if (displayMode !== 'reader' || quietMode) return;
    const insight = pageInsight?.page === pageNumber ? pageInsight : loadPageInsight(bookID, pageNumber);
    publishCompanionHighlights(bookID, pageNumber, validatedCompanionMarks(insight, pageText));
  }, [displayMode, quietMode, bookID, pageNumber, pageText, pageInsight]);

  useEffect(() => {
    try {
      if (mascotPosition) localStorage.setItem(mascotPositionKey(bookID), JSON.stringify(mascotPosition));
    } catch { /* Keep the custom position for this session even if storage is unavailable. */ }
  }, [bookID, mascotPosition]);

  useEffect(() => {
    try { localStorage.setItem(mascotBubbleKey(bookID), mascotBubbleOpen ? 'open' : 'closed'); }
    catch { /* The bubble can still be opened and closed for this session. */ }
  }, [bookID, mascotBubbleOpen]);

  const loadModels = useCallback(async (forceRefresh = false) => {
    const previousModels = models;
    setLoadingModels(true);
    setError('');
    setNotice('');
    setModelProbeProgress({ done: 0, total: 0 });
    try {
      let migrationWarning = '';
      try {
        const currentProviders = await api.aiProviders();
        // Migrate the legacy Shelf connection, but do not silently refresh every provider
        // as a side effect of opening the reader.
        await migrateLegacyShelfProvider(Array.isArray(currentProviders) ? currentProviders : []);
      } catch (cause) {
        migrationWarning = `The saved Tutor connection could not be migrated: ${cause instanceof Error ? cause.message : 'unknown error'}.`;
      }

      // Opening Shelf reads the cached catalogue only. An explicit refresh below is
      // scoped to the preferred model's provider (or the sole eligible provider).
      let catalogue = await api.aiModels();
      let status: AIStatus = await api.aiStatus();
      let providers = Array.isArray(status.providers) ? status.providers : [];
      let cachedModels: AIModel[] = Array.isArray(catalogue?.models) ? catalogue.models : [];
      const allowedProviders = providers.filter((provider) => provider.enabled
        && !(status.mode === 'offline-only' && !provider.local));
      if (forceRefresh) {
        const exactCachedModel = cachedModels.find((model) => `${model.provider_id}/${model.id}` === settings.model);
        const legacyCachedModels = cachedModels.filter((model) => model.id === settings.model);
        const selectedCachedModel = exactCachedModel || (legacyCachedModels.length === 1 ? legacyCachedModels[0] : undefined);
        const targetProviderID = selectedCachedModel?.provider_id || (allowedProviders.length === 1 ? allowedProviders[0].id : '');
        if (!targetProviderID) {
          throw new Error('Choose a provider in Settings → AI → Model catalogue before refreshing. Shelf only reads cached models and never refreshes every provider automatically.');
        }
        catalogue = await api.refreshAIModels(targetProviderID);
        status = await api.aiStatus();
        providers = Array.isArray(status.providers) ? status.providers : [];
        cachedModels = Array.isArray(catalogue?.models) ? catalogue.models : [];
      }

      const providersByID = new Map(providers.map((provider) => [provider.id, provider]));
      let listed: ModelOption[] = cachedModels.flatMap((model) => {
        const provider = providersByID.get(model.provider_id);
        if (!provider) return [];
        // Cached cloud/disabled models may remain visible as gray references, but they are
        // never probed or selectable when the current privacy mode forbids their use.
        const policyBlocked = (status.mode === 'offline-only' && !provider.local)
          || (status.mode === 'local-first' && !status.allow_cloud_fallback && !provider.local);
        const health = provider.health?.state ?? 'unknown';
        let availability: ModelAvailability;
        if (!provider.enabled || policyBlocked || model.availability === 'blocked') availability = 'blocked';
        else if (health === 'unavailable' || health === 'unauthorized' || health === 'unsupported') availability = 'unavailable';
        else if (model.availability === 'available' || model.availability === 'unavailable') availability = model.availability;
        else availability = 'unknown';
        const modelID = String(model.id ?? '').trim();
        if (!modelID) return [];
        const capabilities = model.capabilities ?? {};
        const reasoning = model.reasoning === true || capabilities.reasoning === true || capabilities.thinking === true
          || (model.supported_parameters ?? []).some((parameter) => /^(reasoning_effort|thinking)$/i.test(parameter));
        return [{
          id: `${provider.id}/${modelID}`,
          providerID: provider.id,
          modelID,
          providerName: provider.name,
          local: provider.local,
          name: model.name || modelID,
          availability,
          error: model.availability_message || (availability === 'available' ? undefined : !provider.enabled ? 'Provider is disabled' : policyBlocked ? `Blocked by ${status.mode} privacy policy` : provider.health?.message || (availability === 'blocked' ? 'Blocked by execution policy' : availability === 'unknown' ? 'Model has not been tested yet' : `Provider status: ${health}`)),
          reasoning,
          loaded: model.loaded ?? undefined,
          contextSize: model.context_size || undefined,
        }];
      });

      // Verify only the preferred/current model. All other models remain "unknown"
      // until the reader selects one; selecting an unknown model triggers its one-off test.
      const canProbe = (model: ModelOption) => {
        const provider = providersByID.get(model.providerID);
        return Boolean(provider?.enabled && provider.health?.state === 'healthy')
          && !(status.mode === 'offline-only' && !model.local)
          && !(status.mode === 'local-first' && !status.allow_cloud_fallback && !model.local)
          && model.availability !== 'blocked';
      };
      const previousSelection = settings.model.trim();
      const exactSelection = listed.find((model) => model.id === previousSelection);
      const legacySelections = listed.filter((model) => model.modelID === previousSelection);
      const runtimeDefault = String(status.default_model ?? '').trim();
      const runtimeDefaultSelection = listed.find((model) => model.id === runtimeDefault || `${model.providerID}/${model.modelID}` === runtimeDefault || model.modelID === runtimeDefault);
      const providerDefaultSelection = listed.find((model) => {
        const configured = providersByID.get(model.providerID)?.default_model?.trim();
        return Boolean(configured && (configured === model.modelID || configured === model.id || configured === `${model.providerID}/${model.modelID}`));
      });
      const candidates = [
        exactSelection,
        legacySelections.length === 1 ? legacySelections[0] : undefined,
        runtimeDefaultSelection,
        providerDefaultSelection,
        listed.find((model) => model.availability === 'available'),
        listed.find((model) => model.availability === 'unknown'),
      ].filter((model): model is ModelOption => Boolean(model));
      const candidate = candidates.find((model) => canProbe(model)
        && (forceRefresh || model.availability === 'unknown' || model.availability === 'available'));

      setModels(listed);
      setModelsChecked(true);
      if (candidate && (forceRefresh || candidate.availability === 'unknown')) {
        setModelProbeProgress({ done: 0, total: 1 });
        listed = listed.map((model) => model.id === candidate.id ? { ...model, availability: 'checking', error: undefined } : model);
        setModels(listed.slice());
        try {
          const result = await api.testAIModel(candidate.providerID, candidate.modelID);
          const availability: ModelAvailability = result.availability === 'available' ? 'available'
            : result.availability === 'blocked' ? 'blocked' : 'unavailable';
          listed = listed.map((model) => model.id === candidate.id ? {
            ...model,
            availability,
            error: result.availability_message || (availability === 'available' ? undefined : 'Model did not pass its availability test'),
            loaded: result.loaded ?? model.loaded,
            contextSize: result.context_size || model.contextSize,
          } : model);
          if (availability !== 'available') {
            setError(`${candidate.modelID} did not pass its model-level test. Other models were not tested; choose one to verify it individually.`);
          }
        } catch (cause) {
          const message = cause instanceof Error ? cause.message : 'Model availability check failed';
          listed = listed.map((model) => model.id === candidate.id ? { ...model, availability: 'unavailable', error: message } : model);
          setError(`${candidate.modelID} could not be verified: ${message}. Other models were not tested.`);
        } finally {
          setModelProbeProgress({ done: 1, total: 1 });
        }
      }
      setModels(listed);

      const selectedExact = listed.find((model) => model.id === previousSelection && model.availability === 'available');
      const selectedLegacy = listed.filter((model) => model.modelID === previousSelection && model.availability === 'available');
      const selection = selectedExact?.id
        || (selectedLegacy.length === 1 ? selectedLegacy[0].id : '')
        || listed.find((model) => model.availability === 'available')?.id
        || '';
      setSettings((current) => current.model === selection ? current : { ...current, model: selection });

      const eligibleProviders = providers.filter((provider) => provider.enabled
        && !(status.mode === 'offline-only' && !provider.local)
        && !(status.mode === 'local-first' && !status.allow_cloud_fallback && !provider.local));
      const unavailableProviders = eligibleProviders.filter((provider) => provider.health?.state !== 'healthy');
      const availableCount = listed.filter((model) => model.availability === 'available').length;
      const unavailableCount = listed.filter((model) => model.availability === 'unavailable' || model.availability === 'blocked').length;
      if (listed.length > 0) {
        setNotice(`${listed.length} model(s) discovered · ${availableCount} verified · ${unavailableCount} offline or blocked. Only the preferred model was tested; other models are verified on selection.`);
        if (unavailableProviders.length) {
          const detail = unavailableProviders.slice(0, 3).map((provider) => `${provider.name}: ${provider.health?.message || provider.health?.state || 'unavailable'}`).join(' · ');
          setError(`Some providers are unavailable. Their last-known models remain visible in gray. ${detail}`);
        } else if (migrationWarning) setError(migrationWarning);
      } else if (eligibleProviders.length === 0) {
        setError(`${migrationWarning ? `${migrationWarning} ` : ''}No enabled provider is available for the current execution mode. Open AI Runtime settings to configure OpenRouter, 9Router/OpenAI-compatible, or a local Ollama provider.`);
      } else if (unavailableProviders.length) {
        const detail = unavailableProviders.map((provider) => `${provider.name}: ${provider.health?.message || provider.health?.state || 'unavailable'}`).join(' · ');
        setError(`${migrationWarning ? `${migrationWarning} ` : ''}No models were discovered because provider checks failed. ${detail}`);
      } else {
        setError(`${migrationWarning ? `${migrationWarning} ` : ''}The configured provider(s) responded but returned no models. For 9Router, use its OpenAI-compatible /v1 endpoint; for Ollama, use http://127.0.0.1:11434 and ensure at least one model is installed.`);
      }
    } catch (cause) {
      setModels(previousModels);
      setModelsChecked(true);
      setError(userFacingAIError(cause));
      setNotice('The existing model catalogue was kept. Use Refresh models to query configured providers again.');
    } finally {
      setLoadingModels(false);
    }
  }, [settings.model, models]);

  const chooseModel = async (model: ModelOption) => {
    if (loadingModels || testingModelID) return;
    const commitSelection = (selected: ModelOption) => {
      const next = { ...settings, model: selected.id };
      try { localStorage.setItem(settingsKey, JSON.stringify(next)); } catch { /* The model remains selected for this session. */ }
      setSettings(next);
      setError('');
      setNotice(`${selected.id} selected.`);
      if (displayMode === 'models') onBackToChat?.();
    };
    if (model.availability === 'available') {
      commitSelection(model);
      return;
    }
    if (model.availability !== 'unknown') return;

    setTestingModelID(model.id);
    setModelProbeProgress({ done: 0, total: 1 });
    setModels((current) => current.map((item) => item.id === model.id ? { ...item, availability: 'checking', error: undefined } : item));
    setError('');
    setNotice(`Checking only ${model.modelID}…`);
    try {
      const result = await api.testAIModel(model.providerID, model.modelID);
      const availability: ModelAvailability = result.availability === 'available' ? 'available'
        : result.availability === 'blocked' ? 'blocked' : 'unavailable';
      const updated: ModelOption = {
        ...model,
        availability,
        error: result.availability_message || (availability === 'available' ? undefined : 'Model did not pass its availability test'),
        loaded: result.loaded ?? model.loaded,
        contextSize: result.context_size || model.contextSize,
      };
      setModels((current) => current.map((item) => item.id === model.id ? updated : item));
      if (availability === 'available') commitSelection(updated);
      else setError(`${model.modelID} is ${availability}. Choose another model or check the provider configuration.`);
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : 'Model availability check failed';
      setModels((current) => current.map((item) => item.id === model.id ? { ...item, availability: 'unavailable', error: message } : item));
      setError(`${model.modelID} could not be verified: ${message}`);
    } finally {
      setTestingModelID('');
      setModelProbeProgress({ done: 1, total: 1 });
    }
  };

  useEffect(() => {
    if (didAutoLoadModels.current) return;
    didAutoLoadModels.current = true;
    void loadModels(false);
  }, [loadModels]);

  const ask = useCallback(async (prompt: string, retry = false, autoOverview = false, options: AskOptions = {}) => {
    const userPrompt = prompt.trim();
    if (!userPrompt || busy) return;

    // A selected-phrase highlight is a local document operation, not an AI generation.
    // Save it directly so a missing/blocked model can never make the reader's explicit
    // annotation request silently fail. Explanations, notes, retries and edited turns still
    // go through the normal tutor pipeline.
    const selectedQuoteForDirectAction = selectedText.trim();
    const asksForExplanation = /\b(?:explain|why|what does|what is|tell me|meaning|describe)\b|(?:توضیح|معنی|چرا|شرح بده|بگو چرا)/iu.test(userPrompt);
    const directHighlightAllowed = !retry
      && options.replaceFromIndex === undefined
      && !options.hiddenUser
      && !options.contextOverride
      && !options.modelOverride
      && !asksForExplanation
      && !isExplicitBookNoteRequest(userPrompt)
      && isExplicitHighlightRequest(userPrompt)
      && selectedQuoteForDirectAction.length > 0
      && quoteWords(selectedQuoteForDirectAction).length > 0
      && quoteWords(selectedQuoteForDirectAction).length <= 32
      && Boolean(onApplyBookHighlights);
    if (directHighlightAllowed && onApplyBookHighlights) {
      const userMessage: ChatMessage = { role: 'user', content: userPrompt, pageNumber };
      setBusy(true);
      setError('');
      setNotice('');
      setQuestion('');
      setEditingMessageIndex(null);
      setOpenTryModelIndex(null);
      try {
        const outcome = await onApplyBookHighlights([{
          quote: selectedQuoteForDirectAction,
          note: '',
          page: pageNumber,
          color: 'purple',
        }]);
        const details = [
          ...outcome.saved.map((item) => `Saved in the book: ${item}`),
          ...outcome.failed.map((item) => `I couldn’t mark “${item.quote}”: ${item.reason}`),
        ];
        const confirmation = details.length
          ? details.join('\n\n')
          : 'No highlight was saved. The PDF reader did not confirm a matching text range.';
        setMessages([...messages, userMessage, { role: 'assistant', content: confirmation, pageNumber }]);
        if (outcome.failed.length || !outcome.saved.length) setError('The selected text was not highlighted. Check the message above and try selecting the phrase again.');
        setLastFailedPrompt('');
      } catch (cause) {
        const message = cause instanceof Error ? cause.message : 'the annotation could not be saved';
        setMessages([...messages, userMessage, { role: 'assistant', content: `I couldn’t save the highlight: ${message}`, pageNumber }]);
        setError(`I couldn’t save the highlight: ${message}`);
      } finally {
        setBusy(false);
      }
      return;
    }

    const targetModel = (options.modelOverride || settings.model).trim();
    const targetModelOption = models.find((model) => model.id === targetModel);
    const targetModelAvailable = targetModelOption?.availability === 'available';
    // A note is a durable user action. If inference is not available, at least preserve
    // the selected passage as a clearly-labelled note instead of dropping the request.
    // When a model is usable we keep the normal AI path, which can write a richer note.
    const saveSelectedPassageAsNote = async (includePrompt: boolean, reason: string): Promise<boolean> => {
      if (!isExplicitBookNoteRequest(userPrompt) || !selectedQuoteForDirectAction) return false;
      const excerptTitle = selectedQuoteForDirectAction.replace(/\s+/g, ' ').trim().split(' ').slice(0, 7).join(' ');
      setBusy(true); setError(''); setNotice('');
      try {
        const saved = await api.createShelfNote(bookID, {
          title: `Reading note · ${excerptTitle.slice(0, 72) || `page ${pageNumber}`}`,
          content: `Book: ${bookTitle}\nPage: ${pageNumber}\n\nSelected passage:\n“${selectedQuoteForDirectAction}”\n\n${reason}`,
          labels: ['reading-companion', 'selected-passage'],
        });
        setContext((current) => ({ ...current, notes: [saved, ...current.notes.filter((note) => note.id !== saved.id)] }));
        setMessages((current) => [
          ...current,
          ...(includePrompt ? [{ role: 'user' as const, content: userPrompt, pageNumber }] : []),
          { role: 'assistant', content: `Saved a note from your selected passage: “${saved.title}”. The passage is preserved; connect an available model to generate an AI-written explanation too.`, pageNumber },
        ]);
        setLastFailedPrompt('');
        setNotice('Selected passage saved as a note. No AI-generated text was claimed.');
      } catch (cause) {
        const message = cause instanceof Error ? cause.message : 'the notes service returned an error';
        setError(`I couldn’t save the selected passage as a note: ${message}`);
        setMessages((current) => [
          ...current,
          ...(includePrompt ? [{ role: 'user' as const, content: userPrompt, pageNumber }] : []),
          { role: 'assistant', content: `I couldn’t save the note: ${message}`, pageNumber },
        ]);
      } finally { setBusy(false); }
      return true;
    };
    if (!targetModel) {
      if (await saveSelectedPassageAsNote(true, 'Saved directly from the selected passage because no usable AI model was configured.')) return;
      if (displayMode === 'inline-chat') setInlineSettingsOpen(true); else onOpenModels?.();
      setError('Choose a model in the shared AI Runtime. Open Settings → AI Runtime to configure a provider, then refresh the model list.');
      return;
    }
    if (!targetModelAvailable) {
      if (await saveSelectedPassageAsNote(true, `Saved directly because the selected model “${targetModel}” is not currently available.`)) return;
      if (displayMode === 'inline-chat') setInlineSettingsOpen(true); else onOpenModels?.();
      setError(`“${targetModel}” has not passed the availability test. Select it in the model list to run one direct check, or choose a verified model.`);
      return;
    }
    const effectiveEffort: ReasoningEffort | undefined = targetModelOption.reasoning
      ? (options.reasoningEffort || reasoningEffort)
      : undefined;
    setBusy(true); setError(''); setNotice('');
    let completeDocumentContext = '';
    if (!options.contextOverride?.trim() && onGetDocumentContext) {
      try {
        completeDocumentContext = await onGetDocumentContext(pageNumber);
      } catch (cause) {
        // Do not silently pretend that full-document context was loaded. Current page and saved context remain usable.
        setNotice(cause instanceof Error ? `Using available page context only: ${cause.message}` : 'Using available page context only; the complete PDF context could not be loaded.');
      }
    }
    const baseReadingContext = makeStudyContext(context, bookTitle, bookID, pageNumber, pageText, selectedText, hoveredText, completeDocumentContext);
    const companionContext = pageInsight?.page === pageNumber ? `\n\nMimi's short note for this page (keep the same friendly, plain-spoken tone): ${pageInsight.companion}\nExact phrases Mimi marked on this page: ${(pageInsight.marks ?? []).map((mark) => `${mark.kind} — “${mark.quote}”: ${mark.note}`).join(' | ') || 'No verified marks were saved for this page.'}\nOptional curiosity: ${pageInsight.curiousQuestion || 'none'}\nSomething to notice next: ${pageInsight.lookFor || 'none'}` : '';
    // Do not truncate the actual PDF window here: page 19 must include pages 1–18 and 20–22.
    const currentReadingContext = options.contextOverride?.trim() || `${baseReadingContext}${companionContext}`;
    const replaceIndex = options.replaceFromIndex;
    const existingMessage = replaceIndex !== undefined ? messages[replaceIndex] : undefined;
    const turnContext = existingMessage?.context || currentReadingContext;
    const turnPage = existingMessage?.pageNumber ?? options.pageOverride ?? pageNumber;
    let nextMessages: ChatMessage[];
    if (replaceIndex !== undefined) {
      const replacement: ChatMessage = {
        role: 'user', content: userPrompt, context: turnContext, pageNumber: turnPage,
        ...(options.hiddenUser ? { hidden: true } : {}),
      };
      nextMessages = [...messages.slice(0, replaceIndex), replacement];
      setMessages(nextMessages);
    } else if (retry) {
      nextMessages = messages;
    } else {
      nextMessages = [...messages, {
        role: 'user', content: userPrompt, context: currentReadingContext, pageNumber: turnPage,
        ...(options.hiddenUser ? { hidden: true } : {}),
      }];
      setMessages(nextMessages);
    }
    setQuestion('');
    setEditingMessageIndex(null);
    setOpenTryModelIndex(null);
    const bookMutationRequested = isBookMutationRequest(userPrompt);
    const bookActionContract = bookMutationRequested
      ? `\n\n## REQUIRED BOOK EDIT — THIS IS AN ACTION, NOT A SUGGESTION\nThe reader explicitly asked you to change the book. Do not merely print HIGHLIGHT: or NOTE: labels or describe what you would do. Return one short, natural confirmation sentence followed by exactly one machine block in this form, with valid JSON and no markdown fence:\n<idios_book_actions>{"actions":[{"type":"highlight","page":${turnPage},"quote":"exact contiguous words copied from the current page","note":"short friendly note attached to that phrase","color":"purple"}]}</idios_book_actions>\nUse only exact contiguous quotes present in the labelled PDF text for the target page. For a current-page highlight, the quote must occur exactly once in the current page text. Keep each quote short (1–8 words) and choose a unique occurrence. Never fabricate or paraphrase an anchor. For general book notes with no text anchor, use {"type":"note","title":"short note title","content":"the saved note","labels":["reading-companion"]}. A note about a specific word/phrase may also be attached to its highlight. When the reader asks for both a highlight and a note, return two distinct actions: one exact anchored highlight and one standalone note. The highlight must carry only a short margin explanation. The standalone note must separately explain the page's central argument; do not copy it into the margin field. If you cannot find a valid quote, return no highlight action rather than inventing one; a valid standalone note may still be saved. Up to 3 actions. The app will execute each action independently and report its actual save result. Do not claim that it was saved before the app confirms it.`
      : `\n\n## NO BOOK EDITS REQUESTED\nDo not output HIGHLIGHT:, NOTE:, MARK:, <idios_book_actions>, JSON action blocks, or machine-readable note labels. Only create a highlight or note when the reader explicitly asks you to change/save something in the book.`;
    const systemPrompt = `${readingCompanionSkill}${activeSkillInstructions}\n\n## MUST-FOLLOW VOICE RULES FOR THIS CHAT\nThese style rules take priority over generic assistant habits. You are the same friendly reading companion as Mimi, just in conversation mode. Speak like a perceptive friend sitting beside the reader, not a lecturer. Lead with the useful answer in plain everyday language. Usually use 2–5 short sentences unless the reader explicitly asks for a deep dive. Do not default to academic prose, book-report summaries, or headings such as “Another way to see it” and “A gentle challenge”. Do not force a critique, question, or analogy into every answer. Explain a necessary technical term immediately in simple language. Answer the exact question and use page/notes/history context only when it helps. Never output Mimi’s machine-readable page-note labels in chat. Keep the visible final answer separate from analysis/planning: if this provider format uses a dedicated reasoning field or <think> block, never repeat that material in the user-facing answer.\n\n## Your role: MIMI — the primary Shelf reading companion\nYou are MIMI, the same reading companion the reader meets in the PDF margin. In chat, use your conversational tutoring mode: answer directly and naturally, explain the central idea clearly, and keep the same voice and standards across page notes and discussion. Your saved page notes are optional context, not sections you must repeat. Do not introduce yourself as a separate Tutor persona, stage a debate, or imitate a round-table agent.${bookActionContract}\n\n## Current reading state\nBook: ${bookTitle}\nCurrent page: ${turnPage}\n\n## REQUIRED PDF CONTEXT RULE\nWhen a COMPLETE PDF READING WINDOW is supplied, inspect the provided page-labelled text before answering. It contains every extractable page from page 1 through the current page and up to three following pages. Never claim a page is missing if its labelled text is present in this context. Never ask the reader to resend or paste pages already included here. Use earlier pages to understand definitions, prior arguments, and the author’s setup; use the current page as the primary passage; use following pages only to clarify continuity and never attribute their claims to the current page. If a page explicitly says text could not be extracted, state that limitation only for that page. Distinguish the author’s words from Mimi’s margin notes and the reader’s notes.\n\nApply every relevant rule above. Current page and selection are the freshest reading evidence. Conversation history helps you adapt, but never assume the reader understood something just because it appears in history.`;
    const historical = options.historyBeforeIndex !== undefined
      ? [...messages.slice(0, options.historyBeforeIndex), nextMessages[nextMessages.length - 1]].slice(-10)
      : nextMessages.slice(-10);
    const historyForModel = historical.slice(0, -1).map((message) => ({
      role: message.role,
      // Keep the actual dialogue, but don't replay each turn's complete PDF window.
      // The current turn below carries the current book/document context once.
      content: message.content,
    }));
    const lastUser = `${userPrompt}\n\n[CURRENT READING CONTEXT]\n${turnContext}`;
    try {
      const requestMessages = fitShelfRuntimeMessages([
        { role: 'system', content: systemPrompt },
        ...historyForModel,
        { role: 'user', content: lastUser },
      ]);
      const result = await runShelfAIThroughRuntime(targetModel, requestMessages, effectiveEffort);
      const responseChoice = result.choices?.[0];
      const responseMessage = responseChoice?.message;
      const providerThinking = [
        getContent(responseMessage?.reasoning_content),
        getContent(responseMessage?.reasoning),
        getContent(responseMessage?.thinking),
        getContent(responseMessage?.analysis),
        getContent(responseChoice?.reasoning_content),
        ...(responseMessage?.reasoning_details ?? []).map((part) => part.text || part.summary || part.content || ''),
        ...(responseChoice?.reasoning_details ?? []).map((part) => part.text || part.summary || part.content || ''),
      ].filter(Boolean).join('\n\n');
      const separatedResponse = separateModelThinking(getContent(responseMessage?.content), providerThinking);
      const rawAnswer = separatedResponse.answer.trim();
      const modelThinking = separatedResponse.thinking;
      if (!rawAnswer && modelThinking) {
        const modelName = result.model?.trim() || targetModel;
        setMessages((current) => [...current, {
          role: 'assistant',
          content: 'This model shared its thinking but did not return a final answer. Try again or switch to another available model.',
          reasoning: modelThinking,
          model: modelName,
          reasoningEffort: effectiveEffort,
          pageNumber: turnPage,
          ...(options.variant ? { variant: options.variant } : {}),
        }]);
        setLastFailedPrompt(userPrompt);
        setError('No final answer was returned. You can retry this prompt or choose another model.');
        return;
      }
      if (!rawAnswer) throw new Error(result.error?.message || 'The selected model returned an empty answer. Try again or choose another available model.');
      const parsedAnswer = parseBookActionResponse(rawAnswer, turnPage);
      let answer = parsedAnswer.visible;
      if (bookMutationRequested) {
        let requestedActions = parsedAnswer.actions;
        // A small repair pass helps weaker models that answered in prose instead of the action schema.
        if (!requestedActions.length) {
          try {
            const repairMessages = fitShelfRuntimeMessages([
                { role: 'system', content: `${coreReliabilitySkill.replace(/^---\s*[\s\S]*?---\s*/, '').trim()}\n\nConvert the reader's requested book edit into a valid action envelope only. Do not answer in prose. Return exactly <idios_book_actions>{"actions":[...]}</idios_book_actions>. Valid actions: highlight {type, page:${turnPage}, quote exact and unique from current page, note, color purple}; note {type,title,content,labels}. This is an action plan for the app to execute, not a claim that it has already been saved. Use the supplied PDF reading context as source. A highlight must quote an exact unique passage from the requested page; if that page is not the current open page, return no highlight action rather than saving it on the wrong page. If no exact anchor can be identified, return {"actions":[]}.` },
                { role: 'user', content: `Reader request: ${userPrompt}\n\nReading context and current page text:\n${turnContext.slice(-12000)}\n\nAuthoritative current PDF page text for highlight anchors (quotes must be copied exactly from here):\n${pageText.slice(0, 16000)}\n\nYour first answer (for intent only, do not copy its HIGHLIGHT:/NOTE: labels):\n${rawAnswer.slice(0, 2500)}` },
              ]);
            const repair = await runShelfAIThroughRuntime(targetModel, repairMessages, effectiveEffort);
            const repairText = getContent(repair.choices?.[0]?.message?.content).trim();
            const repaired = parseBookActionResponse(repairText, turnPage);
            if (repaired.actions.length) requestedActions = repaired.actions;
          } catch { /* The message below will clearly say that no book edit was made. */ }
        }
        const resultLines: string[] = [];
        const promptAskedForHighlight = isExplicitHighlightRequest(userPrompt);
        const promptAskedForNote = isExplicitBookNoteRequest(userPrompt);
        // A reader-selected phrase is authoritative. Always replace any model-generated
        // highlight anchor with the exact selection, even when the model returned an approximate
        // quote. Keep a separately requested note as its own shelf-note action.
        const selectedQuote = selectedText.trim();
        const selectedWordCount = quoteWords(selectedQuote).length;
        if (selectedQuote && selectedWordCount > 0 && selectedWordCount <= 32 && promptAskedForHighlight) {
          const existingHighlight = requestedActions.find((action): action is Extract<RequestedBookAction, { type: 'highlight' }> => action.type === 'highlight');
          let annotationNote = existingHighlight?.note || '';
          if (!annotationNote && promptAskedForNote) {
            const visibleAnswer = parsedAnswer.visible.replace(/\\s+/g, ' ').trim();
            annotationNote = visibleAnswer && !/^(?:done|saved|i(?:'ll| will)\\b)/i.test(visibleAnswer)
              ? visibleAnswer.slice(0, 320)
              : 'A note you wanted to keep with this passage.';
          }
          requestedActions = requestedActions.filter((action) => action.type !== 'highlight');
          requestedActions.push({ type: 'highlight', quote: selectedQuote, note: annotationNote, page: turnPage, color: 'purple' });
        }
        // If a weak model still fails the action schema for a page-wide request, reuse Mimi's
        // already-validated exact page marks rather than returning pretend HIGHLIGHT/NOTE text.
        if (!selectedQuote && !requestedActions.some((action) => action.type === 'highlight')
          && (promptAskedForHighlight || promptAskedForNote) && isBroadPageAnnotationRequest(userPrompt)
          && pageInsight?.page === turnPage && pageText.trim()) {
          const safeMarks = validatedCompanionMarks(pageInsight, pageText);
          const fallbackMarks = safeMarks.length ? safeMarks : [fallbackTopicMark(pageInsight, pageText)].filter((mark): mark is CompanionMark => Boolean(mark));
          fallbackMarks.slice(0, 3).forEach((mark) => requestedActions.push({
            type: 'highlight', page: turnPage, quote: mark.quote,
            note: mark.note || (promptAskedForNote ? pageInsight.companion : ''), color: 'purple',
          }));
        }
        // A model may answer the requested note in natural language but ignore the action
        // envelope. Save that useful answer as the note rather than leaving a note request inert.
        if (promptAskedForNote && !requestedActions.some((action) => action.type === 'note')) {
          const visibleNote = parsedAnswer.visible.replace(/\s+/g, ' ').trim();
          const usefulVisibleNote = visibleNote && !/^(?:done|saved|i(?:'ll| will)\b)/i.test(visibleNote)
            ? visibleNote
            : '';
          const noteContent = usefulVisibleNote || (selectedQuote ? `Passage to remember: “${selectedQuote}”` : '');
          if (noteContent) requestedActions.push({
            type: 'note',
            title: `Reading note · page ${turnPage}`,
            content: selectedQuote && usefulVisibleNote ? `Passage: “${selectedQuote}”\n\n${usefulVisibleNote}` : noteContent,
            labels: ['reading-companion'],
          });
        }
        const highlightActions = requestedActions.filter((action): action is Extract<RequestedBookAction, { type: 'highlight' }> => action.type === 'highlight');
        const noteActions = requestedActions.filter((action): action is Extract<RequestedBookAction, { type: 'note' }> => action.type === 'note');
        if (highlightActions.length) {
          if (onApplyBookHighlights) {
            try {
              const outcome = await onApplyBookHighlights(highlightActions);
              outcome.saved.forEach((summary) => resultLines.push(`Saved in the book: ${summary}`));
              outcome.failed.forEach((item) => resultLines.push(`I couldn’t mark “${item.quote}”: ${item.reason}`));
            } catch (cause) {
              // A highlight persistence failure must not prevent a separately requested
              // Shelf note from being attempted and saved below.
              const reason = cause instanceof Error ? cause.message : 'annotation saving returned an error';
              highlightActions.forEach((item) => resultLines.push(`I couldn’t mark “${item.quote}”: ${reason}`));
            }
          } else {
            highlightActions.forEach((item) => resultLines.push(`I couldn’t save “${item.quote}” because the PDF reader isn’t connected to annotation saving in this view.`));
          }
        }
        if (promptAskedForHighlight && !highlightActions.length) resultLines.push('I could not place a highlight because the model did not provide an exact phrase from the current page. I have not marked any text; try selecting the words first or ask me to highlight a short phrase on this page.');
        for (const action of noteActions) {
          try {
            const saved = await api.createShelfNote(bookID, { title: action.title || `Reading note · page ${turnPage}`, content: `Book: ${bookTitle}\nPage: ${turnPage}\n\n${action.content}`, labels: action.labels?.length ? action.labels : ['reading-companion'] });
            setContext((current) => ({ ...current, notes: [saved, ...current.notes.filter((note) => note.id !== saved.id)] }));
            resultLines.push(`Saved a note to this book: “${saved.title || action.title}”.`);
          } catch (cause) {
            resultLines.push(`I couldn’t save the book note: ${cause instanceof Error ? cause.message : 'the notes API returned an error'}.`);
          }
        }
        answer = resultLines.length ? resultLines.join('\n\n') : 'I didn’t change the book because I couldn’t identify a safe, exact action from the response. Try selecting the phrase first, or ask me to mark a short phrase on this page.';
      } else {
        // Never leak legacy action labels from models into ordinary conversation.
        answer = answer.replace(/^\s*(?:HIGHLIGHT(?:\s+(?:QUOTE|TEXT))?|MARK|QUOTE|NOTE(?:\s+\d+)?)\s*[:=—-].*$/gim, '').replace(/\n{3,}/g, '\n\n').trim();
      }
      setMessages((current) => [...current, {
        role: 'assistant', content: answer, ...(modelThinking ? { reasoning: modelThinking } : {}), model: result.model?.trim() || targetModel,
        reasoningEffort: effectiveEffort, pageNumber: turnPage, ...(options.variant ? { variant: options.variant } : {}),
      }]);
      setLastFailedPrompt('');
      if (autoOverview || options.autoOverview) {
        try { localStorage.setItem(overviewKey(bookID), 'shown'); } catch { /* The overview still appears in this thread. */ }
      }
      setNotice(autoOverview || options.autoOverview ? 'Your book orientation is ready.' : '');
    } catch (cause) {
      const message = userFacingAIError(cause);
      if (await saveSelectedPassageAsNote(false, `Saved directly because AI note generation failed: ${message}`)) {
        setLastFailedPrompt('');
      } else {
        setLastFailedPrompt(userPrompt);
        setError(message);
      }
    } finally { setBusy(false); }
  }, [busy, settings, models, reasoningEffort, context, bookTitle, bookID, pageNumber, pageText, selectedText, hoveredText, messages, pageInsight, onOpenModels, onApplyBookHighlights, onGetDocumentContext, displayMode]);

  function editPromptAt(index: number) {
    const message = messages[index];
    if (!message || message.role !== 'user' || message.hidden) return;
    setEditingMessageIndex(index);
    setQuestion(message.content);
    setError(''); setNotice('');
  }

  function findVisiblePromptBefore(index: number): number {
    for (let cursor = index - 1; cursor >= 0; cursor -= 1) {
      if (messages[cursor].role === 'user' && !messages[cursor].hidden) return cursor;
    }
    return -1;
  }

  function findActualPromptBefore(index: number): number {
    for (let cursor = index - 1; cursor >= 0; cursor -= 1) {
      if (messages[cursor].role === 'user') return cursor;
    }
    return -1;
  }

  function regenerateAnswerAt(index: number, modelOverride?: string, preserveOriginal = false) {
    const promptIndex = findActualPromptBefore(index);
    if (promptIndex < 0) return;
    const promptMessage = messages[promptIndex];
    const targetModel = modelOverride || (availableModels.some((model) => model.id === messages[index]?.model) ? messages[index].model : (availableModels.some((model) => model.id === settings.model) ? settings.model : (availableModels[0]?.id || settings.model)));
    const options: AskOptions = {
      modelOverride: targetModel,
      reasoningEffort: models.find((model) => model.id === targetModel)?.reasoning ? (messages[index]?.reasoningEffort || reasoningEffort) : undefined,
    };
    if (preserveOriginal) {
      // Keep the first answer visible so the reader can compare models. Exclude that
      // answer from the alternate model's input to avoid biasing the comparison.
      void ask(promptMessage.content, false, false, {
        ...options,
        hiddenUser: true,
        variant: 'alternate',
        contextOverride: promptMessage.context,
        pageOverride: promptMessage.pageNumber,
        historyBeforeIndex: promptIndex,
      });
      return;
    }
    void ask(promptMessage.content, false, false, {
      ...options, replaceFromIndex: promptIndex, hiddenUser: Boolean(promptMessage.hidden), variant: messages[index]?.variant,
    });
  }

  function reworkAnswerAt(index: number) {
    const answer = messages[index];
    const promptIndex = findVisiblePromptBefore(index);
    if (!answer || promptIndex < 0) return;
    const originalPrompt = messages[promptIndex].content;
    const reworkPrompt = `Rewrite and improve your previous answer to my question without changing the meaning or adding unsupported claims. Make it warmer, more natural, simpler, and more useful. Do not make it more academic. Keep examples only if they clarify the point.\n\nMy original question: ${originalPrompt}\n\nPrevious answer to improve:\n${answer.content}`;
    const modelOverride = selectedModelAvailable ? settings.model : (availableModels[0]?.id || settings.model);
    void ask(reworkPrompt, false, false, {
      hiddenUser: true, variant: 'reworked', modelOverride,
      contextOverride: messages[promptIndex].context,
      pageOverride: messages[promptIndex].pageNumber,
      historyBeforeIndex: promptIndex,
    });
  }

  function submitQuestion() {
    if (!question.trim()) return;
    if (editingMessageIndex !== null) {
      void ask(question, false, false, { replaceFromIndex: editingMessageIndex });
      return;
    }
    void ask(question);
  }

  const generatePageInsight = useCallback(async (force = false) => {
    const text = pageText.trim();
    const key = `${bookID}:${pageNumber}`;
    if (displayMode !== 'reader') return;
    if (!text || text.length < 40) {
      if (force) setInsightError('There is not enough selectable text on this page to make a useful note.');
      return;
    }
    if (!configured || !selectedModelAvailable) {
      if (force) setInsightError('Reconnect your AI endpoint and select an available model, then try Mimi again.');
      return;
    }
    if (context.loading || busy || insightBusy || insightInFlightRef.current) return;
    const textSignature = `${text.length}:${text.slice(0, 80)}:${text.slice(-80)}`;
    if (!force) {
      const cached = loadPageInsight(bookID, pageNumber);
      if (cached) { setPageInsight(cached); setInsightError(''); return; }
      if (insightLastTriedRef.current === `${key}:${textSignature}`) return;
    }
    insightInFlightRef.current = key;
    insightLastTriedRef.current = `${key}:${textSignature}`;
    setInsightBusy(true);
    setInsightError('');
    try {
      const baseContext = makeStudyContext(context, bookTitle, bookID, pageNumber, pageText, selectedText, hoveredText);
      const recentDiscussion = loadThread(bookID).slice(-8).map((message) => `${message.role === 'assistant' ? 'Tutor' : 'Reader'}: ${message.content.slice(0, 900)}`).join('\n');
      const readingContext = `${baseContext}${recentDiscussion ? `\n\nRecent discussion (use as evidence of what the reader asked or understood, not proof of mastery):\n${recentDiscussion}` : ''}`;
      const systemPrompt = `${readingCompanionSkill}${activeSkillInstructions}

## MUST-FOLLOW ROLE: MIMI, THE TINY MARGIN COMPANION
The reader is in the middle of reading. Do not act like a teacher or write a mini-essay. Sound like a thoughtful friend quietly pointing at one interesting bit. Use everyday language and short sentences. Do not start with “On this page…” or “The author discusses…”. Do not write formal report sections, a second essay, a mandatory critique, or a forced quiz. Never output headings like “Another way to see it” or “A gentle challenge”.

Most important job: put useful, accurate notes ON THE TEXT. Find up to three exact, short places on this page that deserve attention: a core idea, a technical term, or a phrase that may be hard to understand. Prefer 1–2 strong marks. Every quote must be copied word-for-word and contiguously from the current page, ideally 1–6 words (never more than 8) and unique on this page. Write a tiny margin note for each quote in everyday language (5–18 words): explain what the phrase means here or why it matters. Do not quote from memory or paraphrase the anchor. For computing, point to the mechanism; for maths, explain the symbol or step; for English/grammar, give natural usage; for philosophy/history/science, make the idea concrete without sounding like a paper. If a phrase is repeated or cannot safely be pinpointed, choose a shorter unique phrase or write NONE. Never invent highlights just to fill slots.

Write no more than about 100 words. The fields below are app data, not visible headings. Return exactly these labels, one per line, with no prose outside them:
TOPIC: a short, plain label of 2–5 words
COMPANION: one friendly observation in 1–2 short sentences, no more than about 35 words
CURIOUS QUESTION: one natural, interesting question or NONE
LOOK FOR: a small thing to notice next or NONE
MARK 1 KIND: important or unclear
MARK 1 QUOTE: exact contiguous source phrase of 1–8 words or NONE
MARK 1 NOTE: plain, friendly margin note of 5–18 words or NONE
MARK 2 KIND: important or unclear
MARK 2 QUOTE: exact contiguous source phrase of 1–8 words or NONE
MARK 2 NOTE: plain, friendly margin note of 5–18 words or NONE
MARK 3 KIND: important or unclear
MARK 3 QUOTE: exact contiguous source phrase of 1–8 words or NONE
MARK 3 NOTE: plain, friendly margin note of 5–18 words or NONE
Do not output TUTOR ADDS or GENTLE CHALLENGE. Do not reveal private reasoning; only provide the tiny note and marks.`;
      const result = await runShelfAIThroughRuntime(settings.model.trim(), [
          { role: 'system', content: systemPrompt },
          { role: 'user', content: `Leave one small, natural margin thought and precise notes for this page. The current PDF text is the authority for every quote. Use earlier pages and notes only for a genuinely helpful connection. The COMPANION field must sound like something a smart, kind friend would quietly say—not a textbook summary. The question is optional and separate.\n\n[READING CONTEXT]\n${readingContext.slice(0, 14000)}` },
        ], supportsThinking ? reasoningEffort : undefined);
      const pageResponseMessage = result.choices?.[0]?.message;
      const answer = separateModelThinking(
        getContent(pageResponseMessage?.content),
        [getContent(pageResponseMessage?.reasoning_content), getContent(pageResponseMessage?.reasoning), getContent(pageResponseMessage?.thinking), ...(pageResponseMessage?.reasoning_details ?? []).map((part) => part.text || part.summary || part.content || '')].filter(Boolean).join('\n\n'),
      ).answer.trim();
      if (!answer) throw new Error(result.error?.message || 'The page companion returned an empty note.');
      let insight = { ...parsePageInsight(answer, pageNumber), model: result.model?.trim() || settings.model.trim() };
      const firstPassMarks = validatedCompanionMarks(insight, text);
      const responseHasContract = /^\s*TOPIC\s*:/im.test(answer)
        && /^\s*COMPANION\s*:/im.test(answer)
        && /^\s*MARK\s+1\s+QUOTE\s*:/im.test(answer)
        && /^\s*MARK\s+1\s+NOTE\s*:/im.test(answer);
      // Weaker models sometimes answer with an essay or return unusable anchors. One corrective
      // pass asks for the compact natural note and exact quotes instead of silently leaving Mimi
      // without visible marks. If the retry still finds no safe quote, the UI fails closed.
      if (!responseHasContract || !firstPassMarks.length) {
        try {
          const correction = await runShelfAIThroughRuntime(settings.model.trim(), [
              { role: 'system', content: `${coreReliabilitySkill.replace(/^---\s*[\s\S]*?---\s*/, '').trim()}\n\n${readingCompanionSkill}\n\nFollow this exact output format. You are Mimi, a small margin companion, not a teacher. Write only the labeled fields, never an essay. COMPANION must sound like a friend quietly pointing out one useful thing, 1–2 short sentences and at most 35 words. Avoid academic language, “On this page…”, “Another way to see it”, and “A gentle challenge”. Prioritize exact on-page marks. Quotes must be copied word-for-word and contiguously from the supplied current page, ideally 1–6 words, never over 8, and unique. Every note must be a plain, friendly margin comment of 5–18 words. If there is no genuinely useful anchor, write NONE. Do not reveal private reasoning.` },
              { role: 'user', content: `Please produce a fresh, compact Mimi note in the required format. The previous response did not provide a valid margin-note format or its quote could not be pinned precisely. Keep the wording simple and conversational; include 1–3 real marks if the page supports them.\n\n[CURRENT PAGE TEXT — THE ONLY SOURCE FOR QUOTES]\n${text.slice(0, 10000)}\n\n[BOOK AND READING CONTEXT]\n${readingContext.slice(0, 3500)}` },
            ], supportsThinking ? reasoningEffort : undefined);
          const correctionMessage = correction.choices?.[0]?.message;
          const correctionAnswer = separateModelThinking(
            getContent(correctionMessage?.content),
            [getContent(correctionMessage?.reasoning_content), getContent(correctionMessage?.reasoning), getContent(correctionMessage?.thinking), ...(correctionMessage?.reasoning_details ?? []).map((part) => part.text || part.summary || part.content || '')].filter(Boolean).join('\n\n'),
          ).answer.trim();
          if (correctionAnswer) {
            const corrected = parsePageInsight(correctionAnswer, pageNumber);
            const correctedMarks = validatedCompanionMarks(corrected, text);
            const correctionHasCompanion = /^\s*COMPANION\s*:/im.test(correctionAnswer);
            insight = {
              ...insight,
              topic: /^\s*TOPIC\s*:/im.test(correctionAnswer) ? corrected.topic : insight.topic,
              companion: correctionHasCompanion ? corrected.companion : insight.companion,
              curiousQuestion: /^\s*CURIOUS QUESTION\s*:/im.test(correctionAnswer) ? corrected.curiousQuestion : insight.curiousQuestion,
              lookFor: /^\s*LOOK FOR\s*:/im.test(correctionAnswer) ? corrected.lookFor : insight.lookFor,
              marks: correctedMarks.length ? correctedMarks : firstPassMarks,
              model: correction.model?.trim() || insight.model,
              tutorReply: '', challenge: '',
            };
          }
        } catch {
          // A failed formatting repair must not discard the observation already returned.
        }
      } else {
        insight.marks = firstPassMarks;
      }
      if (!(insight.marks?.length)) {
        const fallbackMark = fallbackTopicMark(insight, text);
        if (fallbackMark) insight.marks = [fallbackMark];
      }
      if (activePageKeyRef.current === key) { setPageInsight(insight); setInsightError(''); }
      setInsightErrorsByPage((current) => { if (!current[key]) return current; const next = { ...current }; delete next[key]; return next; });
      try { localStorage.setItem(pageInsightKey(bookID, pageNumber), JSON.stringify(insight)); } catch { /* Keep the live insight even if storage is unavailable. */ }
    } catch (cause) {
      const message = userFacingAIError(cause);
      setInsightErrorsByPage((current) => ({ ...current, [key]: message }));
      if (activePageKeyRef.current === key) setInsightError(message);
    } finally {
      if (insightInFlightRef.current === key) insightInFlightRef.current = '';
      setInsightBusy(false);
    }
  }, [pageText, bookID, pageNumber, configured, selectedModelAvailable, context, busy, insightBusy, messages, bookTitle, selectedText, hoveredText, settings, displayMode, supportsThinking, reasoningEffort, quietMode]);

  useEffect(() => {
    activePageKeyRef.current = `${bookID}:${pageNumber}`;
  }, [bookID, pageNumber]);

  useEffect(() => {
    setPageInsight(loadPageInsight(bookID, pageNumber));
    setInsightError(insightErrorsByPage[`${bookID}:${pageNumber}`] || '');
  }, [bookID, pageNumber, insightErrorsByPage]);

  useEffect(() => {
    if (displayMode !== 'reader' || quietMode || context.loading || !pageText.trim() || pageText.trim().length < 40 || !configured || !selectedModelAvailable || busy || insightBusy) return;
    let cancelled = false;
    const timer = window.setTimeout(() => { if (!cancelled) void generatePageInsight(); }, 850);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [displayMode, bookID, pageNumber, pageText, context.loading, configured, selectedModelAvailable, busy, insightBusy, messages.length, quietMode, generatePageInsight]);

  useEffect(() => {
    if ((displayMode !== 'chat' && displayMode !== 'inline-chat') || initialPrompt.trim() || context.loading || !pageText.trim() || !configured || !selectedModelAvailable || busy || messages.length > 0 || didAutoOverview.current) return;
    let alreadyShown = false;
    try { alreadyShown = localStorage.getItem(overviewKey(bookID)) === 'shown'; } catch { /* Session guard below is enough. */ }
    if (alreadyShown) { didAutoOverview.current = true; return; }
    didAutoOverview.current = true;
    void ask(overviewPrompt, false, true);
  }, [displayMode, initialPrompt, bookID, context.loading, pageText, configured, selectedModelAvailable, busy, messages.length, ask]);

  async function copyText(content: string) {
    try { await navigator.clipboard.writeText(content); setNotice('Copied.'); }
    catch { setNotice('Clipboard access is not available in this browser.'); }
  }

  async function saveAnswerAsNote(answer: string, relatedQuestion: string, notePage: number) {
    if (savingNote) return;
    setSavingNote(true); setError(''); setNotice('');
    try {
      const title = `Reading insight · page ${notePage}`;
      const content = `Book: ${bookTitle}\nPage: ${notePage}\n\nQuestion / reflection:\n${relatedQuestion || 'Insight from the reading discussion'}\n\nReading companion:\n${answer}`;
      const saved = await api.createShelfNote(bookID, { title, content, labels: ['reading-companion'] });
      setContext((current) => ({ ...current, notes: [saved, ...current.notes.filter((note) => note.id !== saved.id)] }));
      setNotice('Saved to this book’s notes.');
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Could not save this insight to the book notes.'); }
    finally { setSavingNote(false); }
  }

  function startNewDiscussion() {
    const archived = archivedThreadSnapshot(messages);
    if (archived) setArchivedThreads((current) => [archived, ...current].slice(0, 15));
    setMessages([]); setError(''); setNotice(''); setLastFailedPrompt(''); setQuestion(''); setEditingMessageIndex(null); setOpenTryModelIndex(null); setShowThreadHistory(false);
  }

  function restoreArchivedThread(threadID: string) {
    const chosen = archivedThreads.find((thread) => thread.id === threadID);
    if (!chosen) return;
    const current = archivedThreadSnapshot(messages);
    setArchivedThreads((previous) => [ ...(current ? [current] : []), ...previous.filter((thread) => thread.id !== threadID) ].slice(0, 15));
    setMessages(chosen.messages);
    setError(''); setNotice(`Restored: ${chosen.title}`); setLastFailedPrompt(''); setQuestion(''); setEditingMessageIndex(null); setOpenTryModelIndex(null); setShowThreadHistory(false);
  }

  function forgetCurrentDiscussion() {
    if (!messages.length) return;
    const approved = window.confirm('Delete and forget this conversation? Its messages will be removed from this book’s active chat and won’t be sent as future chat context. Saved book notes and highlights will stay.');
    if (!approved) return;
    setMessages([]);
    setQuestion(''); setEditingMessageIndex(null); setOpenTryModelIndex(null); setError(''); setLastFailedPrompt('');
    try {
      localStorage.setItem(threadKey(bookID), '[]');
      localStorage.removeItem(`idios.shelf.study-thread.v1.${bookID}`);
    } catch { /* State is still cleared for this session. */ }
    setNotice('This conversation was deleted. Book notes and highlights were kept.');
  }

  function deleteArchivedThread(threadID: string) {
    const target = archivedThreads.find((thread) => thread.id === threadID);
    if (!target) return;
    const approved = window.confirm(`Permanently delete “${target.title}”? This chat will be removed from saved conversation history.`);
    if (!approved) return;
    setArchivedThreads((current) => current.filter((thread) => thread.id !== threadID));
  }

  function forgetArchivedThreads() {
    if (!archivedThreads.length) return;
    const approved = window.confirm('Delete all previous chats for this book? The current conversation will stay open.');
    if (!approved) return;
    setArchivedThreads([]);
    try { localStorage.setItem(archivedThreadsKey(bookID), '[]'); } catch { /* State is still cleared for this session. */ }
    setNotice('Previous chats were deleted. The current conversation is unchanged.');
  }

  const filteredModels = useMemo(() => {
    const needle = modelSearch.trim().toLowerCase();
    const availabilityRank = (availability: ModelAvailability): number => {
      switch (availability) {
        case 'available': return 0;
        case 'checking': return 1;
        case 'unknown': return 2;
        case 'unavailable': return 3;
        case 'blocked': return 4;
      }
    };
    return models
      .filter((model) => !needle || model.id.toLowerCase().includes(needle) || model.name.toLowerCase().includes(needle))
      .slice()
      .sort((left, right) => availabilityRank(left.availability) - availabilityRank(right.availability)
        || left.providerName.localeCompare(right.providerName, undefined, { sensitivity: 'base' })
        || left.name.localeCompare(right.name, undefined, { sensitivity: 'base' })
        || left.id.localeCompare(right.id));
  }, [models, modelSearch]);
  const pageInsightForReader = pageInsight?.page === pageNumber ? pageInsight : null;
  const pageMarksForReader = validatedCompanionMarks(pageInsightForReader, pageText);
  const mascotMessage = pageInsightForReader?.companion || insightError || (insightBusy ? 'Hmm… I am looking for one small idea worth noticing.' : !settings.model.trim() ? 'Connect a provider and choose a model in AI Runtime, then I can read along with you.' : !selectedModelAvailable ? 'I am ready to explore this page when the selected provider is reachable.' : 'I am quietly reading along. I will point out useful ideas, gentle critiques, and curious questions.');

  const handleMascotPointerDown = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (event.button !== 0) return;
    const root = event.currentTarget.closest('.shelf-study-mascot') as HTMLElement | null;
    const rect = root?.getBoundingClientRect();
    if (!rect) return;
    mascotDragRef.current = { pointerID: event.pointerId, startX: event.clientX, startY: event.clientY, originLeft: rect.left, originTop: rect.top, moved: false };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handleMascotPointerMove = (event: ReactPointerEvent<HTMLButtonElement>) => {
    const drag = mascotDragRef.current;
    if (!drag || drag.pointerID !== event.pointerId) return;
    const dx = event.clientX - drag.startX;
    const dy = event.clientY - drag.startY;
    if (Math.abs(dx) + Math.abs(dy) > 5) {
      drag.moved = true;
      suppressMascotClickRef.current = true;
      event.preventDefault();
      // The floating root is now a fixed-size anchor around Mimi itself. The thought
      // bubble is absolutely positioned above it, so dragging always follows the avatar
      // rather than the combined (and variable-height) popup.
      const mascotWidth = Math.min(286, Math.max(1, window.innerWidth - 24));
      const left = Math.max(8, Math.min(Math.max(8, window.innerWidth - mascotWidth - 8), drag.originLeft + dx));
      const top = Math.max(8, Math.min(Math.max(8, window.innerHeight - 54), drag.originTop + dy));
      setMascotPosition({ left, top });
    }
  };

  const handleMascotPointerUp = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (mascotDragRef.current?.pointerID !== event.pointerId) return;
    const moved = mascotDragRef.current.moved;
    mascotDragRef.current = null;
    if (moved) window.setTimeout(() => { suppressMascotClickRef.current = false; }, 80);
    try { event.currentTarget.releasePointerCapture(event.pointerId); } catch { /* Pointer capture may already be released. */ }
  };

  const mascotStyle = mascotPosition ? { left: `${mascotPosition.left}px`, top: `${mascotPosition.top}px`, right: 'auto', bottom: 'auto' } : undefined;

  const renderThinkingEffortControl = (mode: 'main' | 'chooser' = 'main', chooserID = ''): ReactNode => {
    const levels: { value: ReasoningEffort; label: string; title: string; description: string }[] = [
      { value: 'low', label: 'Light', title: 'Light reasoning', description: 'Quicker, lighter responses' },
      { value: 'medium', label: 'Balanced', title: 'Balanced reasoning', description: 'A good default for most questions' },
      { value: 'high', label: 'Deep', title: 'Deep reasoning', description: 'More deliberate reasoning when useful' },
    ];
    const controlKey = mode === 'chooser' ? `chooser-${chooserID || openTryModelIndex || 'model'}` : 'main';
    const isOpen = openThinkingControl === controlKey;
    const currentLevel = levels.find((level) => level.value === reasoningEffort) || levels[1];
    return <div
      ref={(node) => { thinkingControlRefs.current[controlKey] = node; }}
      data-thinking-control={controlKey}
      className={`shelf-ai-thinking-control${mode === 'chooser' ? ' shelf-ai-thinking-control--chooser' : ''}${isOpen ? ' is-open' : ''}`}
    >
      <button
        type="button"
        className="shelf-ai-thinking-control__trigger"
        aria-haspopup="menu"
        aria-expanded={isOpen}
        aria-label={`Thinking level: ${currentLevel.label}`}
        title={`Thinking · ${currentLevel.label}`}
        onClick={() => setOpenThinkingControl((current) => current === controlKey ? null : controlKey)}
      >
        <span className="shelf-ai-thinking-control__icon"><Sparkles size={13} /></span>
        <span className="shelf-ai-thinking-control__trigger-label">Thinking</span>
        <span className={`shelf-ai-thinking-control__state-dot is-${reasoningEffort}`} aria-hidden="true" />
        <ChevronDown size={13} className={`shelf-ai-thinking-control__chevron${isOpen ? ' is-open' : ''}`} />
      </button>
      {isOpen && <div className="shelf-ai-thinking-control__menu" role="menu" aria-label="Choose thinking level">
        {levels.map((level) => <button
          key={level.value}
          type="button"
          role="menuitemradio"
          aria-checked={reasoningEffort === level.value}
          className={`shelf-ai-thinking-control__option${reasoningEffort === level.value ? ' is-active' : ''}`}
          title={level.title}
          onClick={() => {
            setReasoningEffort(level.value);
            setOpenThinkingControl(null);
          }}
        >
          <span className="shelf-ai-thinking-control__option-copy"><strong>{level.label}</strong><small>{level.description}</small></span>
          {reasoningEffort === level.value && <Check size={14} aria-hidden="true" />}
        </button>)}
      </div>}
    </div>;
  };

  const renderModelThinking = (message: ChatMessage, index: number, scope: 'inline' | 'page'): ReactNode => {
    const thinking = message.reasoning?.trim();
    if (message.role !== 'assistant' || !thinking) return null;
    const key = `${scope}:${index}:${message.pageNumber ?? 0}:${message.model ?? ''}:${message.content.slice(0, 28)}`;
    const expanded = collapsedModelThinking[key] !== true;
    const className = scope === 'inline' ? 'shelf-study-ai-inline__model-thinking' : 'shelf-study-ai-page__model-thinking';
    return <section className={`${className}${expanded ? ' is-expanded' : ''}`}>
      <button type="button" className={`${className}-toggle`} aria-expanded={expanded} onClick={() => setCollapsedModelThinking((current) => ({ ...current, [key]: !expanded }))}>
        <Sparkles size={12} aria-hidden="true" />
        <span>Model thinking</span>
        <small>separate from answer · not copied</small>
        <ChevronDown size={12} className={expanded ? 'is-open' : ''} aria-hidden="true" />
      </button>
      {expanded && <div className={`${className}-content`}>{thinking.split(/\n{2,}/).map((paragraph, paragraphIndex) => <p key={paragraphIndex}>{paragraph}</p>)}</div>}
    </section>;
  };

  const renderMessageActions = (message: ChatMessage, index: number, scope: 'inline' | 'page'): ReactNode => {
    if (message.hidden) return null;
    const toolbarClass = scope === 'inline' ? 'shelf-study-ai-inline__message-tools' : 'shelf-study-ai-page__message-tools';
    const metaClass = scope === 'inline' ? 'shelf-study-ai-inline__message-provenance' : 'shelf-study-ai-page__message-provenance';
    if (message.role === 'user') return <div className={toolbarClass}><button type="button" onClick={() => editPromptAt(index)} disabled={busy} title="Edit this prompt"><Pencil size={12} /><span>Edit prompt</span></button></div>;
    const defaultTryModel = availableModels.find((model) => model.id !== message.model)?.id || availableModels[0]?.id || '';
    const selectedTryModel = tryModelSelections[index] ?? defaultTryModel;
    const selectedTryModelName = availableModels.find((model) => model.id === selectedTryModel)?.name || selectedTryModel;
    const responseModelName = message.model ? (models.find((model) => model.id === message.model)?.name || message.model) : 'Model not recorded';
    const previousPromptIndex = findVisiblePromptBefore(index);
    const previousPrompt = previousPromptIndex >= 0 ? messages[previousPromptIndex] : null;
    return <>
      <div className={toolbarClass}>
        <button type="button" onClick={() => void copyText(message.content)} title="Copy answer"><Copy size={12} /><span>Copy</span></button>
        <button type="button" onClick={() => void saveAnswerAsNote(message.content, previousPrompt?.content ?? '', previousPrompt?.pageNumber ?? message.pageNumber ?? pageNumber)} disabled={savingNote} title="Save answer to book notes">{savingNote ? <LoaderCircle size={12} className="spin" /> : <BookMarked size={12} />}<span>Save note</span></button>
        <button type="button" onClick={() => regenerateAnswerAt(index)} disabled={busy || previousPromptIndex < 0 || !availableModels.length} title={`Try again with ${message.model || settings.model}`}><RefreshCw size={12} /><span>Try again</span></button>
        <button type="button" onClick={() => reworkAnswerAt(index)} disabled={busy || !availableModels.length} title="Rewrite more clearly and naturally"><Sparkles size={12} /><span>Rework</span></button>
        <button type="button" onClick={() => setOpenTryModelIndex((current) => current === index ? null : index)} disabled={busy || !availableModels.length} aria-expanded={openTryModelIndex === index} title="Try this prompt with another available model"><Layers3 size={12} /><span>Try with model</span></button>
      </div>
      {openTryModelIndex === index && <div className={`${toolbarClass} ${toolbarClass}--chooser`}>
        <label><span>Available model</span><select value={selectedTryModel} onChange={(event) => setTryModelSelections((current) => ({ ...current, [index]: event.target.value }))}><option value="" disabled>Choose a tested model</option>{availableModels.map((model) => <option key={model.id} value={model.id}>{model.name}{model.id !== model.name ? ` · ${model.id}` : ''}</option>)}</select></label>
        {models.find((model) => model.id === selectedTryModel)?.reasoning === true && renderThinkingEffortControl('chooser', String(index))}
        <button type="button" onClick={() => selectedTryModel && regenerateAnswerAt(index, selectedTryModel, true)} disabled={busy || !selectedTryModel || !availableModels.some((model) => model.id === selectedTryModel)}><Send size={12} /><span>Try with {selectedTryModelName || 'model'}</span></button>
      </div>}
      <div className={metaClass}><span>Written with <strong>{responseModelName}</strong></span>{message.reasoningEffort && <span className="shelf-ai-message-thinking"><Sparkles size={10} /> Reasoning · {message.reasoningEffort === 'low' ? 'Light' : message.reasoningEffort === 'medium' ? 'Balanced' : 'Deep'}</span>}{message.variant === 'reworked' && <span>Reworked version</span>}{message.variant === 'alternate' && <span>Alternative answer</span>}</div>
    </>;
  };
  const thinkingControl = supportsThinking ? renderThinkingEffortControl() : null;

  if (displayMode === 'reader') {
    // During Focus Mode, hide the draggable mascot/bubble and any auto-generated margin insight.
    // The dedicated Ask Mimi control still opens the full in-reader assistant when requested.
    if (quietMode) return null;
    return createPortal(<div className={`shelf-study-mascot ${mascotBubbleOpen ? 'is-bubble-open' : 'is-bubble-closed'}`} style={mascotStyle} aria-label="Mimi, your reading companion">
    {mascotBubbleOpen && <section className="shelf-study-mascot__bubble" aria-label={`Mimi's note for page ${pageNumber}`}>
      <div className="shelf-study-mascot__bubble-heading">
        <span className="shelf-study-mascot__identity"><i className={insightBusy ? 'is-thinking' : pageInsightForReader ? 'is-ready' : ''} /> mimi · PAGE {pageNumber}</span>
        <span className={insightBusy ? 'is-thinking' : ''}>{insightBusy ? 'connecting ideas' : pageInsightForReader ? 'a tiny thought' : 'reading along'}</span>
        <div className="shelf-study-mascot__window-actions">
          <button type="button" onClick={() => setThoughtExpanded((open) => !open)} aria-label={thoughtExpanded ? 'Collapse Mimi thought' : 'Expand Mimi thought'} title={thoughtExpanded ? 'Collapse thought' : 'Expand thought'}>{thoughtExpanded ? <ChevronDown size={13} /> : <ChevronUp size={13} />}</button>
          <button type="button" onClick={() => setMascotBubbleOpen(false)} aria-label="Close Mimi thought bubble" title="Hide thought bubble"><X size={13} /></button>
        </div>
      </div>
      {thoughtExpanded ? <div className="shelf-study-mascot__thought-content">
        {insightBusy ? <div className="shelf-study-mascot__thinking-card" aria-live="polite">
          <span className="shelf-study-mascot__thinking-orbit"><i /><i /><i /><b>✦</b></span>
          <div><strong>Give me a tiny moment…</strong><small>I’m looking for one useful idea, then connecting it to the page.</small><div className="shelf-study-mascot__thinking-track"><i /><i /><i /></div></div>
        </div> : <>
          <strong>{pageInsightForReader?.topic || 'A thought for your reading'}</strong>
          <p>{mascotMessage}</p>
          {pageInsightForReader?.curiousQuestion && <p className="shelf-study-mascot__question"><span>Something to wonder</span> {pageInsightForReader.curiousQuestion}</p>}
        </>}
        {insightError && <p className="shelf-study-mascot__error-note" role="status">Mimi couldn’t refresh this page: {insightError}</p>}
        {pageMarksForReader.length > 0 && !insightBusy && <div className="shelf-study-mascot__marks"><span className="shelf-study-mascot__marks-title">A couple of things I marked</span>{pageMarksForReader.map((mark, markIndex) => <button type="button" key={`${mark.quote}-${markIndex}`} className={`shelf-study-mascot__mark shelf-study-mascot__mark--${mark.kind}`} onClick={() => onOpenChat?.(`Can we talk about “${mark.quote}” on page ${pageNumber}? Mimi's margin note: ${mark.note || 'This phrase seems worth noticing.'} Please explain it in an easy, natural way using the current passage.`)}><strong>{mark.quote}</strong><span>{mark.kind === 'unclear' ? 'A word worth unpacking' : 'Why I marked it'}</span><p>{mark.note || 'This idea may be useful to keep in mind.'}</p></button>)}</div>}
        {pageInsightForReader?.model && !insightBusy && <span className="shelf-study-mascot__model-credit">Mimi’s note · {models.find((model) => model.id === pageInsightForReader.model)?.name || pageInsightForReader.model}</span>}
        {pageInsightForReader && !insightBusy && <span className="shelf-study-mascot__highlight-legend"><i /> Lilac = key idea · amber = possibly tricky</span>}
        {insightBusy && <span className="shelf-study-mascot__quiet-note">I’m keeping it brief: one useful insight, not a long lecture.</span>}
      </div> : <button className="shelf-study-mascot__collapsed-thought" type="button" onClick={() => setThoughtExpanded(true)}><Sparkles size={13} /><span>{pageInsightForReader?.topic || 'Mimi has a thought'} · tap to peek</span><ChevronDown size={13} /></button>}
      <form className="shelf-study-mascot__ask" onSubmit={(event: FormEvent<HTMLFormElement>) => { event.preventDefault(); const prompt = mascotQuestion.trim(); if (!prompt) return; onOpenChat?.(`Mimi, I want to discuss this with you in the context of the current book page.

My question: ${prompt}${pageInsightForReader ? `

Your page note: ${pageInsightForReader.companion}
Mimi's marked phrases: ${(pageInsightForReader.marks ?? []).map((mark) => `“${mark.quote}” — ${mark.note}`).join(' | ') || 'none'}${pageInsightForReader.curiousQuestion ? `
Something Mimi wondered: ${pageInsightForReader.curiousQuestion}` : ''}` : ''}`); setMascotQuestion(''); }}>
        <input value={mascotQuestion} onChange={(event) => setMascotQuestion(event.target.value)} placeholder="Ask Mimi about this page…" aria-label="Ask Mimi about this page" />
        <button type="submit" disabled={!mascotQuestion.trim()} aria-label="Discuss question with Mimi" title="Open a discussion"><Send size={14} /></button>
      </form>
      <div className="shelf-study-mascot__actions">
        {insightError ? configured && selectedModelAvailable ? <button type="button" onClick={() => void generatePageInsight(true)} disabled={insightBusy || busy || context.loading || !pageText.trim()}><RefreshCw size={12} className={insightBusy ? 'spin' : ''} /> {insightBusy ? 'Trying again…' : 'Retry Mimi'}</button> : <button type="button" onClick={() => onOpenChat?.()}><Settings2 size={12} /> Check AI connection</button> : pageInsightForReader && <button type="button" onClick={() => void generatePageInsight(true)} disabled={insightBusy || busy || !selectedModelAvailable}><RefreshCw size={12} /> Surprise me</button>}
        {pageInsightForReader?.curiousQuestion && <button type="button" onClick={() => onOpenChat?.(`Let's explore your question about page ${pageNumber}: ${pageInsightForReader.curiousQuestion}`)}><Lightbulb size={12} /> Let's wonder</button>}
        {pageInsightForReader?.challenge && <button type="button" onClick={() => onOpenChat?.(`Help me examine this passage critically, using the text and my reading context. The companion's gentle challenge was: ${pageInsightForReader.challenge}`)}><MessageCircle size={12} /> Discuss this</button>}
        <button type="button" className="shelf-study-mascot__primary" onClick={() => onOpenChat?.(pageInsightForReader ? `Let's discuss your thought about page ${pageNumber}: ${pageInsightForReader.companion}${pageInsightForReader.curiousQuestion ? `

A question you raised: ${pageInsightForReader.curiousQuestion}` : ''}` : `Let's talk about page ${pageNumber}. Help me understand the main idea in simple terms.`)}><MessageCircle size={12} /> Talk with Mimi <ArrowRight size={12} /></button>
      </div>
      <span className="shelf-study-mascot__drag-note"><span aria-hidden="true">↔</span> drag Mimi anywhere · hide this bubble with ×</span>
    </section>}
    <button type="button" className={`shelf-study-mascot__character ${insightBusy ? 'is-thinking' : ''} ${!mascotBubbleOpen ? 'is-waiting' : ''}`} onPointerDown={handleMascotPointerDown} onPointerMove={handleMascotPointerMove} onPointerUp={handleMascotPointerUp} onPointerCancel={handleMascotPointerUp} onClick={() => { if (suppressMascotClickRef.current) { suppressMascotClickRef.current = false; return; } setMascotBubbleOpen((open) => !open); }} aria-label={mascotBubbleOpen ? 'Hide Mimi thought bubble' : 'Show Mimi thought bubble'} title={mascotBubbleOpen ? 'Drag me to move; tap to tuck my thought away' : 'Drag me to move; tap to see my thought'}>
      <span className="shelf-study-mascot__spark shelf-study-mascot__spark--one">✦</span><span className="shelf-study-mascot__spark shelf-study-mascot__spark--two">✧</span><span className="shelf-study-mascot__ear shelf-study-mascot__ear--left" /><span className="shelf-study-mascot__ear shelf-study-mascot__ear--right" />
      <span className="shelf-study-mascot__tuft" aria-hidden="true" />
      <span className="shelf-study-mascot__face"><i className="shelf-study-mascot__eye shelf-study-mascot__eye--left"><b /></i><i className="shelf-study-mascot__eye shelf-study-mascot__eye--right"><b /></i><span className="shelf-study-mascot__nose" /><em /></span><span className="shelf-study-mascot__cheek shelf-study-mascot__cheek--left" /><span className="shelf-study-mascot__cheek shelf-study-mascot__cheek--right" /><span className="shelf-study-mascot__body-dot" />
      {!mascotBubbleOpen && <span className="shelf-study-mascot__mini-badge">{insightBusy ? '…' : '?'}</span>}
    </button>
  </div>, document.body);
  }

  const settingsPane = <section className="shelf-study-ai__settings-pane shelf-study-ai__settings-pane--page">
    <div className="shelf-study-ai__settings-intro"><strong>Shared AI Runtime</strong><span>One provider registry for Cloud, 9Router-compatible endpoints, and local models</span></div>
    <div className="shelf-study-ai__models-title"><span><strong>Choose a model</strong><small>{loadingModels ? modelProbeProgress.total ? `Verifying preferred model ${modelProbeProgress.done}/${modelProbeProgress.total}…` : 'Checking provider catalogues…' : modelsChecked ? `${availableCount} verified / ${models.length} discovered` : 'Refresh to discover models'}</small></span><button type="button" onClick={() => void loadModels(true)} disabled={loadingModels}>{loadingModels ? <LoaderCircle size={14} className="spin" /> : <RefreshCw size={14} />} Refresh selected provider</button></div>
    <div className="shelf-study-ai__model-search"><Search size={14} /><input value={modelSearch} onChange={(event) => setModelSearch(event.target.value)} placeholder="Search models…" aria-label="Search AI models" /></div>
    {loadingModels && <div className="shelf-study-ai__model-scan"><span className="shelf-study-ai__scan-beam" />{modelProbeProgress.total ? `Verifying one preferred model · ${modelProbeProgress.done}/${modelProbeProgress.total}` : 'Contacting configured provider catalogues…'}{modelProbeProgress.total > 0 && <span className="shelf-study-ai__model-scan-progress" style={{ width: `${Math.round((modelProbeProgress.done / modelProbeProgress.total) * 100)}%` }} />}</div>}
    <div className="shelf-study-ai__model-list" role="listbox" aria-label="Available AI models">{filteredModels.map((model) => {
      const available = model.availability === 'available'; const unknown = model.availability === 'unknown'; const selected = settings.model === model.id;
      const disabled = loadingModels || Boolean(testingModelID) || ['unavailable', 'blocked', 'checking'].includes(model.availability);
      return <button type="button" key={model.id} role="option" aria-selected={selected} disabled={disabled} className={`shelf-study-ai__model-option ${selected ? 'is-selected' : ''} ${available ? 'is-available' : ['unavailable', 'blocked'].includes(model.availability) ? 'is-unavailable' : 'is-unknown'}`} onClick={() => void chooseModel(model)} title={unknown ? `Select to test ${model.id}` : model.error || model.id}>
        <span className="shelf-study-ai__model-status"><i className={available ? 'is-ready' : ''} /></span><span className="shelf-study-ai__model-name"><strong>{model.name}</strong><small>{model.providerName} · {model.modelID} · {model.local ? 'local' : 'cloud'}{model.loaded === true ? ' · loaded' : ''}</small></span>
        <span className="shelf-study-ai__model-result">{available ? <><small>{model.loaded === true ? 'Verified' : 'Available'}</small><Check size={14} /></> : model.availability === 'checking' ? 'Testing…' : model.availability === 'unknown' ? 'Click to test' : model.availability === 'blocked' ? 'Blocked' : 'Offline'}</span>
      </button>;
    })}{!loadingModels && filteredModels.length === 0 && <div className="shelf-study-ai__empty-models">{models.length ? 'No model matches this search.' : 'No models are cached for the current execution mode. Configure and enable a provider under AI Runtime settings, then refresh.'} <a href="/settings/ai">Open AI Runtime settings</a></div>}</div>
    <p className="shelf-study-ai__model-note">The catalogue comes from the shared AI Runtime. Refresh checks each provider once and verifies only the preferred model; other models are tested on selection, one at a time. Offline-only never contacts cloud providers; unavailable providers keep their last-known models visible but disabled.</p>
    {error && <div className="shelf-study-ai__error" role="alert"><CircleHelp size={15} /><span>{error}</span><button type="button" onClick={() => setError('')} aria-label="Dismiss error"><X size={12} /></button></div>}
    {notice && <div className="shelf-study-ai__notice" role="status"><Check size={12} /> {notice}</div>}
  </section>;

  if (displayMode === 'inline-chat') return createPortal(<section className="shelf-study-ai-inline" role="dialog" aria-label={`AI reading companion for ${bookTitle}`}>
    <header className="shelf-study-ai-inline__header">
      <span className="shelf-study-ai-inline__avatar" aria-label="Mimi"><span className="shelf-study-ai-inline__avatar-ear shelf-study-ai-inline__avatar-ear--left" /><span className="shelf-study-ai-inline__avatar-ear shelf-study-ai-inline__avatar-ear--right" /><span className="shelf-study-ai-inline__avatar-eyes"><i /><i /><em /></span></span>
      <div className="shelf-study-ai-inline__heading"><strong>Tutor</strong><small>{bookTitle} · p. {pageNumber}</small></div>
      <span className={`shelf-study-ai-inline__model-dot ${selectedModelAvailable ? 'is-ready' : ''}`} title={selectedModelAvailable ? 'Model is ready' : 'Model is not ready'} />
      <div className="shelf-study-ai-inline__menu-wrap">
        <button type="button" onClick={() => setInlineHeaderMenuOpen((open) => !open)} aria-label="Chat options" title="Chat options" aria-expanded={inlineHeaderMenuOpen}><MoreHorizontal size={17} /></button>
        {inlineHeaderMenuOpen && <div className="shelf-study-ai-inline__menu" role="menu">
          <button type="button" role="menuitem" onClick={() => { setShowThreadHistory((open) => !open); setInlineHeaderMenuOpen(false); }}><History size={14} /><span>{showThreadHistory ? 'Hide previous chats' : 'Previous chats'}</span></button>
          <button type="button" role="menuitem" onClick={() => { startNewDiscussion(); setInlineHeaderMenuOpen(false); }}><RotateCcw size={14} /><span>New chat</span></button>
          {messages.length > 0 && <button type="button" role="menuitem" className="is-danger" onClick={() => { forgetCurrentDiscussion(); setInlineHeaderMenuOpen(false); }}><Trash2 size={14} /><span>Forget this chat</span></button>}
        </div>}
      </div>
      <button type="button" onClick={() => { setInlineSkillsOpen((open) => !open); setInlineSettingsOpen(false); }} aria-label={inlineSkillsOpen ? 'Back to conversation' : 'Choose AI skills'} title={inlineSkillsOpen ? 'Back to conversation' : 'AI skills'} aria-pressed={inlineSkillsOpen}><BookMarked size={15} /></button>
      <button type="button" onClick={() => { setInlineSettingsOpen((open) => !open); setInlineSkillsOpen(false); }} aria-label={inlineSettingsOpen ? 'Back to conversation' : 'Model and connection settings'} title={inlineSettingsOpen ? 'Back to conversation' : 'Model settings'}>{inlineSettingsOpen ? <MessageCircle size={15} /> : <Settings2 size={15} />}</button>
      <button type="button" onClick={onCloseInlineChat} aria-label="Close AI assistant" title="Return to reading"><Minimize2 size={15} /></button>
    </header>
    {showThreadHistory && !inlineSettingsOpen && <div className="shelf-study-ai-inline__thread-history"><div><strong>Previous chats</strong><span className="shelf-study-ai__history-heading-actions">{archivedThreads.length > 0 && <button type="button" onClick={forgetArchivedThreads} title="Delete all previous chats">Forget all</button>}<button type="button" onClick={() => setShowThreadHistory(false)} aria-label="Close previous chats"><X size={12} /></button></span></div>{archivedThreads.length ? archivedThreads.map((thread) => <div className="shelf-study-ai__thread-item" key={thread.id}><button type="button" className="shelf-study-ai__thread-restore" onClick={() => restoreArchivedThread(thread.id)}><span>{thread.title}</span><small>{new Date(thread.updatedAt).toLocaleDateString()}</small></button><button type="button" className="shelf-study-ai__thread-delete" onClick={() => deleteArchivedThread(thread.id)} aria-label={`Delete chat ${thread.title}`} title="Delete this chat"><Trash2 size={12} /></button></div>) : <p>Previous chats will appear here when you start a new chat.</p>}</div>}
    {inlineSettingsOpen ? <div className="shelf-study-ai-inline__settings">{settingsPane}<button type="button" className="shelf-study-ai-inline__done" onClick={() => setInlineSettingsOpen(false)}>Done · back to reading <ArrowRight size={14} /></button></div> : inlineSkillsOpen ? <div className="shelf-study-ai-inline__skills" aria-label="Choose reading skills">
      <div className="shelf-study-ai-inline__skills-intro"><span><BookMarked size={17} /></span><div><strong>Reading skills</strong><p>Choose only skills relevant to this reading task. Core reliability is always on. For smaller or unknown models, start with 1–3 relevant skills. Contextual translation is always used by Translate.</p></div></div>
      <div className="shelf-study-ai-inline__skills-list">{READING_SKILLS.map((skill) => { const active = selectedSkillIDs.includes(skill.id); return <label key={skill.id} className={`shelf-study-ai-inline__skill ${active ? 'is-active' : ''}`}><input type="checkbox" checked={active} disabled={skill.alwaysOn} onChange={(event) => setSelectedSkillIDs((current) => event.target.checked ? [...new Set([...current, skill.id])] : current.filter((id) => id !== skill.id))} /><span className="shelf-study-ai-inline__skill-check">{active && <Check size={12} />}</span><span><strong>{skill.name}</strong><small>{skill.description}</small></span><em>{skill.alwaysOn ? 'ALWAYS ON' : skill.category === 'reading' ? 'READING' : skill.category === 'learning' ? 'LEARNING' : 'SUBJECT'}</em></label>; })}</div>
      <div className="shelf-study-ai-inline__skills-footer"><span>{selectedSkillIDs.length} skills enabled</span><button type="button" onClick={() => setInlineSkillsOpen(false)}>Done <ArrowRight size={13} /></button></div>
    </div> : <>
      <div className="shelf-study-ai-inline__context"><BookOpenText size={14} /><span><b>Reading page {pageNumber}</b><small>{selectedOrHovered ? `${selectedText.trim() ? 'Selected passage' : 'Word in focus'} · ${selectedOrHovered.slice(0, 90)}${selectedOrHovered.length > 90 ? '…' : ''}` : `${pageText.trim() ? pageText.trim().split(/\s+/).length : 0} words extracted · notes and previous pages included automatically`}</small></span></div>
      <div className="shelf-study-ai-inline__messages" aria-live="polite">
        {messages.length === 0 && <div className="shelf-study-ai-inline__welcome"><span className="shelf-study-ai-inline__welcome-icon">✦</span><strong>Let’s keep reading together.</strong><p>Ask about this page, explore a confusing idea, or connect it with something you learned earlier. You stay right here in the book.</p><div>{promptModes.slice(0, 3).map((item) => { const Icon = item.icon; return <button type="button" key={item.label} onClick={() => setQuestion(item.prompt)} disabled={!selectedModelAvailable || busy}><Icon size={14} /><span>{item.label}</span></button>; })}</div></div>}
        {messages.map((message, index) => message.hidden ? null : <article key={`${message.role}-${index}`} className={`shelf-study-ai-inline__message shelf-study-ai-inline__message--${message.role}`}>
          <span>{message.role === 'assistant' ? 'Tutor' : `You · p. ${message.pageNumber ?? pageNumber}`}</span>
          <div>{message.role === 'assistant' ? renderAnswer(message.content) : <p>{message.content}</p>}</div>
          {renderModelThinking(message, index, 'inline')}
          {renderMessageActions(message, index, 'inline')}
        </article>)}
        {busy && <div className="shelf-study-ai-inline__thinking"><Sparkles size={14} /><span>Connecting this idea to the book…</span><i /><i /><i /></div>}
        <div ref={messagesEndRef} />
      </div>
      {error && <div className="shelf-study-ai-inline__error" role="alert"><CircleHelp size={14} /><span>{error}</span>{lastFailedPrompt && <button type="button" onClick={() => void ask(lastFailedPrompt, true)} disabled={busy}>Retry</button>}<button type="button" onClick={() => setError('')} aria-label="Dismiss error"><X size={12} /></button></div>}
      {notice && <div className="shelf-study-ai-inline__notice" role="status"><Check size={12} />{notice}</div>}
      {thinkingControl && <div className="shelf-study-ai-inline__thinking-row">{thinkingControl}</div>}
      {editingMessageIndex !== null && <div className="shelf-ai-editing-prompt"><Pencil size={12} /><span>Editing an earlier prompt · later turns will be replaced</span><button type="button" onClick={() => { setEditingMessageIndex(null); setQuestion(''); }} aria-label="Cancel prompt edit"><X size={12} /></button></div>}
      <div className="shelf-study-ai-inline__prompt-bar"><span>Quick prompts</span>{promptModes.slice(0, 3).map((item) => { const Icon = item.icon; return <button type="button" key={item.label} onClick={() => setQuestion(item.prompt)} disabled={busy || !selectedModelAvailable} title={`Add “${item.label}” to your draft`}><Icon size={12} /><span>{item.label}</span></button>; })}</div>
      <form className="shelf-study-ai-inline__composer" onSubmit={(event: FormEvent<HTMLFormElement>) => { event.preventDefault(); submitQuestion(); }}><textarea value={question} onChange={(event) => setQuestion(event.target.value)} placeholder={editingMessageIndex !== null ? 'Edit the previous prompt…' : configured && selectedModelAvailable ? 'Ask about this page…' : 'Open settings above to connect a model…'} rows={3} disabled={busy || !selectedModelAvailable} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); submitQuestion(); } }} /><div><span>Enter to send · Shift+Enter for a new line</span><button type="submit" disabled={busy || !question.trim() || !selectedModelAvailable} aria-label="Send question">{busy ? <LoaderCircle size={16} className="spin" /> : <Send size={16} />}</button></div></form>
      <footer className="shelf-study-ai-inline__footer"><span><i className={selectedModelAvailable ? 'is-ready' : ''} />{selectedModelAvailable ? settings.model : 'Connect a model in settings'}</span><span>{context.loading ? 'Gathering context…' : `${contextCount} saved items in context`}</span></footer>
    </>}
  </section>, document.fullscreenElement instanceof HTMLElement ? document.fullscreenElement : document.body);

  if (displayMode === 'models') return createPortal(<main className="shelf-study-ai-page" data-theme="reading" aria-label="AI model settings">
    <header className="shelf-study-ai-page__header">
      <button type="button" className="shelf-study-ai-page__back" onClick={onBackToChat}><ArrowLeft size={16} /> <span>Back to AI chat</span></button>
      <div className="shelf-study-ai-page__brand"><span className="shelf-study-ai-page__brand-icon"><Settings2 size={17} /></span><div><small>READING COMPANION</small><strong>Models & connection</strong><span title={bookTitle}>{bookTitle}</span></div></div>
      <span className="shelf-study-ai-page__header-page">Separate settings page</span>
    </header>
    <div className="shelf-study-ai-page__settings-layout"><div className="shelf-study-ai-page__settings-aside"><span className="shelf-study-ai-page__settings-mark"><Sparkles size={22} /></span><h1>Choose the mind behind your reading.</h1><p>Refresh to test which models respond right now, search by name, and switch without losing your book discussion.</p><div><span><Check size={13} /> Live availability probe</span><span><Check size={13} /> Searchable model catalogue</span><span><Check size={13} /> Book discussion is saved separately</span></div></div>{settingsPane}</div>
  </main>, document.body);

  return createPortal(<main className="shelf-study-ai-page" aria-label={`AI book discussion for ${bookTitle}`}>
    <header className="shelf-study-ai-page__header shelf-study-ai-page__header--chat">
      <button type="button" className="shelf-study-ai-page__back" onClick={onBackToReading}><ArrowLeft size={16} /><span>Back to reading</span></button>
      <div className="shelf-study-ai-page__brand"><span className="shelf-study-ai-page__brand-icon"><BookOpenText size={18} /></span><div><small>YOUR READING COMPANION</small><strong>AI book discussion</strong><span title={bookTitle}>{bookTitle} · page {pageNumber}</span></div></div>
      <span className="shelf-study-ai-page__header-page">A dedicated space for your questions</span>
    </header>
    <div className="shelf-study-ai-page__layout">
      <section className="shelf-study-ai-page__conversation">
        <div className="shelf-study-ai-page__conversation-intro"><div className="shelf-study-ai-page__intro-row"><div><span>PAGE {pageNumber} · BOOK DISCUSSION</span><h1>Let’s think this through.</h1><p>Ask naturally. I’ll use the passage, what you’ve read, your notes, and our previous discussion—not just the last message.</p></div><div className="shelf-study-ai-page__thread-actions"><button type="button" className="shelf-study-ai-page__history-button" onClick={() => setShowThreadHistory((open) => !open)} aria-expanded={showThreadHistory}><History size={13} /><span>History</span></button>{messages.length > 0 && <button type="button" className="shelf-study-ai-page__history-button shelf-study-ai-page__forget-button" onClick={forgetCurrentDiscussion} title="Delete and forget this chat"><Trash2 size={13} /><span>Forget chat</span></button>}<button type="button" className="shelf-study-ai-page__new-thread" onClick={startNewDiscussion} title="Start a new chat without losing previous chats"><RotateCcw size={14} /><span>New chat</span></button></div></div></div>
        {showThreadHistory && <div className="shelf-study-ai-page__thread-history"><div><strong>Previous chats</strong><span className="shelf-study-ai__history-heading-actions">{archivedThreads.length > 0 && <button type="button" onClick={forgetArchivedThreads} title="Delete all previous chats">Forget all</button>}<button type="button" onClick={() => setShowThreadHistory(false)} aria-label="Close previous chats"><X size={12} /></button></span></div>{archivedThreads.length ? archivedThreads.map((thread) => <div className="shelf-study-ai__thread-item" key={thread.id}><button type="button" className="shelf-study-ai__thread-restore" onClick={() => restoreArchivedThread(thread.id)}><span>{thread.title}</span><small>{new Date(thread.updatedAt).toLocaleDateString()}</small></button><button type="button" className="shelf-study-ai__thread-delete" onClick={() => deleteArchivedThread(thread.id)} aria-label={`Delete chat ${thread.title}`} title="Delete this chat"><Trash2 size={12} /></button></div>) : <p>Previous chats will appear here when you start a new chat.</p>}</div>}
        {selectedOrHovered && <div className="shelf-study-ai-page__focus"><div><span>{selectedText.trim() ? 'SELECTED PASSAGE' : 'WORD IN FOCUS'}</span><button type="button" onClick={() => void copyText(selectedOrHovered)} title="Copy focus text"><Copy size={13} /></button></div><p>{selectedOrHovered.slice(0, 1800)}{selectedOrHovered.length > 1800 ? '…' : ''}</p></div>}
        <div className="shelf-study-ai-page__messages" aria-live="polite">
          {messages.length === 0 && <div className="shelf-study-ai-page__welcome"><div className="shelf-study-ai-page__welcome-icon"><BookOpenText size={24} /><Sparkles size={12} /></div><h2>One book, an ongoing conversation.</h2><p>I can unpack a difficult paragraph, challenge an assumption, connect a concept to your notes, or help you remember what matters. Start wherever your curiosity is.</p><div className="shelf-study-ai-page__starter-grid"><button type="button" onClick={() => void ask('Give me a clear, plain-language explanation of the current page, with one concrete example and the key idea I should remember.')} disabled={busy || !selectedModelAvailable}><Lightbulb size={15} /><span><strong>Make this click</strong><small>Explain the page simply</small></span></button><button type="button" onClick={() => void ask('Look critically at the current passage. Identify one assumption, limitation, or alternative interpretation that is genuinely supported by the text, and explain it fairly.')} disabled={busy || !selectedModelAvailable}><CircleHelp size={15} /><span><strong>Challenge an idea</strong><small>Find a different angle</small></span></button><button type="button" onClick={() => void ask('Connect the current page to my saved notes and earlier pages. Show one meaningful relationship and why it matters.')} disabled={busy || !selectedModelAvailable}><Link2 size={15} /><span><strong>Connect the dots</strong><small>Use my notes and history</small></span></button></div></div>}
          {messages.map((message, index) => message.hidden ? null : <article key={`${message.role}-${index}`} className={`shelf-study-ai-page__message shelf-study-ai-page__message--${message.role}`}>
            <span>{message.role === 'assistant' ? <><GraduationCap size={13} /> TUTOR</> : <>YOU · PAGE {message.pageNumber ?? pageNumber}</>}</span><div className="shelf-study-ai-page__message-content">{message.role === 'assistant' ? renderAnswer(message.content) : <p>{message.content}</p>}</div>
            {renderModelThinking(message, index, 'page')}
            {renderMessageActions(message, index, 'page')}
            {message.role === 'user' && index === messages.length - 1 && lastFailedPrompt === message.content && <small className="shelf-study-ai-page__failed-tag">The request did not complete. Your question is still here.</small>}
          </article>)}
          {busy && <div className="shelf-study-ai-page__thinking"><span><Sparkles size={15} /></span><div><strong>Working through the idea…</strong><small>Reading the passage alongside your notes and earlier questions</small></div><i /><i /><i /></div>}
          <div ref={messagesEndRef} />
        </div>
        {error && <div className="shelf-study-ai-page__error" role="alert"><CircleHelp size={15} /><span>{error}</span>{lastFailedPrompt && <button type="button" onClick={() => void ask(lastFailedPrompt, true)} disabled={busy}>{busy ? <LoaderCircle size={12} className="spin" /> : 'Retry'}</button>}<button type="button" onClick={() => setError('')} aria-label="Dismiss error"><X size={12} /></button></div>}
        {notice && <div className="shelf-study-ai-page__notice" role="status"><Check size={13} /> {notice}</div>}
        {thinkingControl && <div className="shelf-study-ai-page__thinking-row">{thinkingControl}</div>}
        {editingMessageIndex !== null && <div className="shelf-ai-editing-prompt"><Pencil size={12} /><span>Editing an earlier prompt · later turns will be replaced</span><button type="button" onClick={() => { setEditingMessageIndex(null); setQuestion(''); }} aria-label="Cancel prompt edit"><X size={12} /></button></div>}
        <div className="shelf-study-ai-page__prompt-modes"><span>Prompt starters · click one to edit it</span>{promptModes.map((item) => { const Icon = item.icon; return <button key={item.label} type="button" onClick={() => setQuestion(item.prompt)} disabled={busy || !selectedModelAvailable} title="Add this prompt to the composer so you can edit it"><Icon size={14} /><span>{item.label}</span></button>; })}</div>
        <form className="shelf-study-ai-page__composer" onSubmit={(event: FormEvent<HTMLFormElement>) => { event.preventDefault(); submitQuestion(); }}><textarea value={question} onChange={(event) => setQuestion(event.target.value)} placeholder={editingMessageIndex !== null ? 'Edit the previous prompt…' : configured ? 'Ask in your own words. I automatically add the current page, notes, highlights and earlier reading context…' : 'Open Models & connection to choose a model first…'} rows={3} disabled={busy || !selectedModelAvailable} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); submitQuestion(); } }} /><div><span>Enter to send · Shift+Enter for a new line</span><button type="submit" disabled={busy || !question.trim() || !selectedModelAvailable} aria-label="Send message">{busy ? <LoaderCircle size={17} className="spin" /> : <Send size={17} />}</button></div></form>
        <div className="shelf-study-ai-page__footer"><span><i className={selectedModelAvailable ? 'is-ready' : ''} />{selectedModelAvailable ? settings.model : 'Connect a model to begin'}</span><span>{context.loading ? 'Gathering context…' : `${contextCount} saved learning items in context`}</span></div>
      </section>
      <aside className="shelf-study-ai-page__context-rail"><section className="shelf-study-ai-page__context-card"><span className="shelf-study-ai-page__rail-icon"><BookOpenText size={16} /></span><div><small>READING NOW</small><strong title={bookTitle}>{bookTitle}</strong><p>Page {pageNumber} · {pageText.trim() ? `${pageText.trim().split(/\s+/).length} extracted words` : 'No saved page text yet'}</p></div></section><section className="shelf-study-ai-page__context-card"><span className="shelf-study-ai-page__rail-icon"><Layers3 size={16} /></span><div><small>AVAILABLE CONTEXT</small><strong>{context.loading ? 'Collecting…' : `${context.notes.length + context.annotations.length + context.previousQuestions.length} saved items`}</strong><p>{context.notes.length} notes · {context.annotations.length} highlights · {loadReadHistory(bookID).length} visited pages · {context.previousQuestions.length} saved questions</p></div></section><section className="shelf-study-ai-page__context-card shelf-study-ai-page__context-card--model"><span className="shelf-study-ai-page__rail-icon"><Settings2 size={16} /></span><div><small>MODEL CONNECTION</small><strong title={settings.model || 'No model selected'}>{selectedModelAvailable ? settings.model : 'No model selected'}</strong><p><i className={selectedModelAvailable ? 'is-ready' : ''} />{selectedModelAvailable ? 'Available for this discussion' : 'Choose a tested model to begin'}</p><button type="button" onClick={onOpenModels}>Manage models & connection <ArrowRight size={12} /></button></div></section>{selectedOrHovered && <section className="shelf-study-ai-page__context-card shelf-study-ai-page__context-card--focus"><small>{selectedText.trim() ? 'PASSAGE IN FOCUS' : 'WORD IN FOCUS'}</small><p>{selectedOrHovered.slice(0, 900)}{selectedOrHovered.length > 900 ? '…' : ''}</p></section>}<section className="shelf-study-ai-page__context-note"><Sparkles size={14} /><p>I use your current page as the anchor. Notes and past pages are supporting context, not proof that you already understood them.</p></section></aside>
    </div>
  </main>, document.body);
}
