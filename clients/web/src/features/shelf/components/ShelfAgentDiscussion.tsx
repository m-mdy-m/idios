import { Fragment, useCallback, useEffect, useMemo, useState } from 'react';
import { Check, ChevronDown, ChevronRight, CircleHelp, EyeOff, LoaderCircle, Maximize2, MessagesSquare, Minimize2, Search, Send, SlidersHorizontal, Sparkles, UsersRound, X } from 'lucide-react';
import { api, type AIAgent, type AIAgentContribution, type AIModel, type AIProvider, type AIRuntimeConfig, type AITask } from '@/lib/api';
import type { ShelfAnnotation } from '@/types/idios';
import './ShelfAgentDiscussion.css';

type Props = {
  open: boolean;
  bookID: string;
  bookTitle: string;
  pageNumber: number;
  pageText: string;
  selectedText?: string;
  annotations: ShelfAnnotation[];
  onClose: () => void;
  onOpenWebSearch?: (query: string) => void;
};
type ThreadTurn = AIAgentContribution & { threadKey: string; runID: string };
type ModelRoute = { label: string; local: boolean; usable: boolean; supportsThinking: boolean; providerID?: string; providerLabel?: string; speed?: number; reason?: string };
type AgentCategory = { id: string; label: string; description: string };
const agentCategories: AgentCategory[] = [
  { id: 'reading-research', label: 'Reading & research', description: 'Close reading, source grounding, context and concepts' },
  { id: 'learning-tutoring', label: 'Learning & tutoring', description: 'Explanations, questions and examples' },
  { id: 'translation-language', label: 'Translation & language', description: 'Translation, terminology and language learning' },
  { id: 'notes-writing', label: 'Notes & writing', description: 'Study notes, summaries and annotation drafts' },
  { id: 'review-reasoning', label: 'Review & critical thinking', description: 'Verification, criticism and counterexamples' },
  { id: 'discussion-facilitation', label: 'Discussion & facilitation', description: 'Keep debate coherent and productive' },
  { id: 'subject-specialists', label: 'Subject specialists', description: 'Programming, mathematics, history, philosophy and sciences' },
  { id: 'other', label: 'Other agents', description: 'Custom or uncategorized roles' },
];
function categoryForAgent(agent: AIAgent): string {
  const identity = `${agent.id} ${agent.name}`.toLowerCase();
  const role = agent.role.toLowerCase();
  const secondary = `${agent.description ?? ''} ${(agent.tasks ?? []).join(' ')} ${(agent.skill_ids ?? []).join(' ')}`.toLowerCase();
  const classify = (text: string): string | undefined => {
    if (/translat|language|glossary|locali[sz]/.test(text)) return 'translation-language';
    if (/note.writer|note-writer|writing|summari[sz]|annotation|highlight|synthesis|draft/.test(text)) return 'notes-writing';
    if (/tutor|teach|learn|coach|socratic|questioner|example.builder|example-builder|learning.questions/.test(text)) return 'learning-tutoring';
    if (/review|critic|counterexample|counter-example|verification|verify|logic|reasoning|audit/.test(text)) return 'review-reasoning';
    if (/facilitat|discussion.continuity|context.keeper|context-keeper|moderator|debate.host/.test(text)) return 'discussion-facilitation';
    if (/programming|computer.science|computer-architecture|mathemat|history|philosophy|science|statistics|subject.specialist|code.cartographer|signal.scout|assumption.scout|coding|debugging/.test(text)) return 'subject-specialists';
    if (/reader|reading|research|source.quality|concept.connector|conceptual.bridges|active.reading|book.analysis|close.reading/.test(text)) return 'reading-research';
    return undefined;
  };
  // Identity wins over broad roles/tasks: a Reader with a highlight permission
  // stays in Reading & research instead of being mislabeled as Notes & writing.
  return classify(identity) ?? classify(role) ?? classify(secondary) ?? 'other';
}
function prettySkill(skillID: string): string { return skillID.replace(/[-_]+/g, ' ').replace(/\b\w/g, (letter) => letter.toUpperCase()); }
const terminalStatuses = new Set(['completed', 'partially_completed', 'failed', 'cancelled']);
const defaultQuestion = '';

function array<T>(value: T[] | undefined | null): T[] { return Array.isArray(value) ? value : []; }
const roleModelCandidates: Record<string, string[]> = {
  reader: ['qwen2.5:3b', 'qwen2.5:7b'],
  tutor: ['qwen3:1.7b', 'qwen2.5:3b', 'deepseek-r1:1.5b', 'qwen2.5:7b'],
  translator: ['translategemma:4b', 'qwen2.5:3b', 'qwen2.5:7b'],
  'note-writer': ['qwen2.5:3b', 'qwen2.5:7b'],
  reviewer: ['qwen2.5:7b', 'qwen2.5:3b'],
  'concept-connector': ['qwen2.5:3b', 'qwen2.5:7b'],
  'socratic-questioner': ['deepseek-r1:1.5b', 'qwen2.5:3b'],
  'counterexample-tester': ['qwen2.5:7b', 'qwen2.5:3b', 'deepseek-r1:1.5b'],
  'context-keeper': ['qwen2.5:3b', 'qwen3:0.6b', 'qwen2.5:7b'],
  'example-builder': ['qwen2.5:3b', 'qwen2.5:7b'],
  'discussion-chair': ['qwen2.5:7b', 'qwen2.5:3b'],
  'evidence-auditor': ['qwen2.5:7b', 'qwen2.5:3b'],
  'ambiguity-scout': ['translategemma:4b', 'qwen2.5:3b', 'qwen2.5:7b'],
  steelman: ['qwen2.5:3b', 'qwen2.5:7b', 'deepseek-r1:1.5b'],
  'scope-keeper': ['qwen3:0.6b', 'qwen2.5:3b', 'qwen2.5:7b'],
  'code-cartographer': ['qwen2.5-coder:latest', 'qwen2.5:7b'],
  'signal-scout': ['qwen3:0.6b', 'qwen2.5:3b'],
};
function rolePreferredModel(agentID: string, providers: AIProvider[], models: AIModel[]): string | undefined {
  for (const candidate of roleModelCandidates[agentID] ?? []) {
    const model = models.find((item) => item.id.toLocaleLowerCase() === candidate.toLocaleLowerCase() && item.availability === 'available' && providers.some((provider) => provider.id === item.provider_id && provider.enabled && (provider.local || provider.type === 'ollama')));
    if (model) return `${model.provider_id}/${model.id}`;
  }
  return undefined;
}
function modelRoute(agent: AIAgent, config: AIRuntimeConfig, providers: AIProvider[], models: AIModel[]): ModelRoute {
  const ref = agent.preferred_model || rolePreferredModel(agent.id, providers, models) || config.task_models?.book_discussion || config.default_model || '';
  const slash = ref.indexOf('/');
  if (slash < 1) return { label: ref || 'No model assigned', local: false, usable: false, supportsThinking: false, reason: 'Choose a model under Settings → AI Runtime → Agents.' };
  const providerID = ref.slice(0, slash);
  const modelID = ref.slice(slash + 1);
  const provider = providers.find((item) => item.id === providerID);
  const model = models.find((item) => item.provider_id === providerID && item.id === modelID);
  const local = Boolean(provider && (provider.local || provider.type === 'ollama'));
  const state = model?.availability ?? 'unknown';
  const usable = Boolean(provider?.enabled && local && state === 'available');
  const speed = model?.performance?.tokens_per_second;
  return {
    label: `${provider?.name ?? providerID} · ${model?.name || model?.id || modelID}`,
    providerID,
    providerLabel: provider?.name ?? providerID,
    local,
    usable,
    supportsThinking: Boolean(model?.reasoning === true || model?.capabilities?.thinking === true || model?.capabilities?.reasoning === true || model?.supported_parameters?.some((parameter) => ['think', 'reasoning', 'reasoning_effort'].includes(parameter.toLowerCase())) || (provider?.type === 'ollama' && /^(deepseek-r1|qwen3:|qwen3\.5:)/i.test(modelID))),
    speed: typeof speed === 'number' && speed > 0 ? speed : undefined,
    reason: !provider?.enabled ? 'Provider is disabled.' : !local ? 'For this offline discussion, assign an Ollama Local model.' : state !== 'available' ? 'Run Refresh & test in AI Runtime before choosing this model.' : undefined,
  };
}
function questionFrom(content: string): string {
  const match = content.match(/(?:^|\n)\s*(?:[-*]\s*)?(?:\*\*)?Question for you(?:\*\*)?\s*:\s*(.+?)(?=\n|$)/i);
  return (match?.[1] ?? '').trim();
}
function visibleTurnText(content: string): string {
  return content.replace(/(?:^|\n)\s*(?:\*\*)?Question for you(?:\*\*)?\s*:\s*.+(?=\n|$)/i, '').replace(/\*\*(.*?)\*\*/g, '$1').replace(/`([^`]+)`/g, '$1').trim();
}
function loadSavedAgentIDs(bookID: string): string[] {
  try {
    const value = localStorage.getItem(`idios.shelf.agent-discussion.${bookID}`);
    const parsed: unknown = value ? JSON.parse(value) : [];
    return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : [];
  } catch { return []; }
}

export function ShelfAgentDiscussion({ open, bookID, bookTitle, pageNumber, pageText, selectedText = '', annotations, onClose, onOpenWebSearch }: Props) {
  const [agents, setAgents] = useState<AIAgent[]>([]);
  const [providers, setProviders] = useState<AIProvider[]>([]);
  const [models, setModels] = useState<AIModel[]>([]);
  const [config, setConfig] = useState<AIRuntimeConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [agentIDs, setAgentIDs] = useState<string[]>([]);
  const [agentSearch, setAgentSearch] = useState('');
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [categoryFilter, setCategoryFilter] = useState('all');
  const [availabilityFilter, setAvailabilityFilter] = useState('all');
  const [providerFilter, setProviderFilter] = useState('all');
  const [skillFilter, setSkillFilter] = useState('all');
  const [selectedOnly, setSelectedOnly] = useState(false);
  const [participantsVisible, setParticipantsVisible] = useState(() => {
    try { return localStorage.getItem('idios.shelf.agent-discussion.participants-visible') !== 'false'; } catch { return true; }
  });
  const [expandedCategories, setExpandedCategories] = useState<string[]>([]);
  const [expandedInitializedBookID, setExpandedInitializedBookID] = useState('');
  const [selectionReadyBookID, setSelectionReadyBookID] = useState('');
  const [topic, setTopic] = useState(defaultQuestion);
  const [readerReply, setReaderReply] = useState('');
  const [expanded, setExpanded] = useState(false);
  const [requestedRounds, setRequestedRounds] = useState(3);
  const [thinkingEffort, setThinkingEffort] = useState<'off' | 'low' | 'medium' | 'high'>('off');
  const [starting, setStarting] = useState(false);
  const [activeRun, setActiveRun] = useState<AITask | null>(null);
  const [turns, setTurns] = useState<ThreadTurn[]>([]);
  const [answeredQuestionKeys, setAnsweredQuestionKeys] = useState<string[]>([]);
  const [sourcePages, setSourcePages] = useState<Record<number, string>>({});
  const busy = Boolean(activeRun && !terminalStatuses.has(activeRun.status));
  const requestClose = useCallback(() => {
    if (starting) return;
    if (activeRun && !terminalStatuses.has(activeRun.status)) {
      void api.cancelAIAgentRun(activeRun.id).catch(() => undefined);
      setActiveRun(null);
    }
    onClose();
  }, [activeRun, onClose, starting]);

  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => { if (event.key !== 'Escape') return; if (expanded) { setExpanded(false); return; } requestClose(); };
    window.addEventListener('keydown', onKeyDown);
    return () => { window.removeEventListener('keydown', onKeyDown); };
  }, [open, requestClose, expanded]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true); setError('');
    Promise.all([api.aiAgents(), api.aiProviders(), api.aiModels(), api.aiConfig()]).then(([nextAgents, nextProviders, nextModels, nextConfig]) => {
      if (cancelled) return;
      setAgents(array(nextAgents));
      setProviders(array(nextProviders));
      setModels(array(nextModels?.models));
      setConfig(nextConfig);
    }).catch((cause: unknown) => {
      if (!cancelled) setError(cause instanceof Error ? cause.message : 'Could not load configured agents.');
    }).finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [bookID]);

  useEffect(() => {
    if (!pageText.trim()) return;
    setSourcePages((current) => {
      const next = { ...current, [pageNumber]: pageText.trim().slice(0, 12000) };
      const keys = Object.keys(next).map(Number).sort((a, b) => a - b);
      for (const key of keys.slice(0, Math.max(0, keys.length - 4))) delete next[key];
      return next;
    });
  }, [pageNumber, pageText]);

  const eligibleAgents = useMemo(() => {
    if (!config) return [];
    const rank: Record<string, number> = { reader: 0, tutor: 10, steelman: 20, 'concept-connector': 30, 'context-keeper': 31, 'example-builder': 32, 'note-writer': 33, 'socratic-questioner': 40, 'scope-keeper': 50, 'signal-scout': 51, translator: 60, 'ambiguity-scout': 61, reviewer: 70, 'evidence-auditor': 71, 'counterexample-tester': 72, 'code-cartographer': 80, 'discussion-chair': 1000 };
    return agents.filter((agent) => agent.enabled && array(agent.tasks).includes('book_discussion'))
      .map((agent) => ({ agent, route: modelRoute(agent, config, providers, models) }))
      .sort((a, b) => (rank[a.agent.id] ?? 10) - (rank[b.agent.id] ?? 10) || a.agent.name.localeCompare(b.agent.name));
  }, [agents, config, providers, models]);
  const usableIDs = useMemo(() => eligibleAgents.filter((item) => item.route.usable).map((item) => item.agent.id), [eligibleAgents]);
  const skillOptions = useMemo(() => [...new Set(eligibleAgents.flatMap(({ agent }) => array(agent.skill_ids)))].sort((a, b) => a.localeCompare(b)), [eligibleAgents]);
  const providerOptions = useMemo(() => {
    const entries = new Map<string, string>();
    for (const { route } of eligibleAgents) if (route.providerID) entries.set(route.providerID, route.providerLabel || route.providerID);
    return [...entries.entries()].sort((a, b) => a[1].localeCompare(b[1]));
  }, [eligibleAgents]);
  const categoryOptions = useMemo(() => agentCategories.filter((category) => eligibleAgents.some(({ agent }) => categoryForAgent(agent) === category.id)), [eligibleAgents]);
  const filteredAgents = useMemo(() => {
    const query = agentSearch.trim().toLocaleLowerCase();
    return eligibleAgents.filter(({ agent, route }) => {
      const category = categoryForAgent(agent);
      const haystack = `${agent.id} ${agent.name} ${agent.description ?? ''} ${agent.role} ${route.label} ${(agent.tasks ?? []).join(' ')} ${(agent.skill_ids ?? []).join(' ')}`.toLocaleLowerCase();
      if (query && !haystack.includes(query)) return false;
      if (categoryFilter !== 'all' && category !== categoryFilter) return false;
      if (availabilityFilter === 'ready' && !route.usable) return false;
      if (availabilityFilter === 'attention' && route.usable) return false;
      if (providerFilter !== 'all' && route.providerID !== providerFilter) return false;
      if (skillFilter !== 'all' && !array(agent.skill_ids).includes(skillFilter)) return false;
      if (selectedOnly && !agentIDs.includes(agent.id)) return false;
      return true;
    });
  }, [eligibleAgents, agentSearch, categoryFilter, availabilityFilter, providerFilter, skillFilter, selectedOnly, agentIDs]);
  const groupedAgents = useMemo(() => agentCategories.map((category) => ({
    ...category,
    agents: filteredAgents.filter(({ agent }) => categoryForAgent(agent) === category.id),
    selectedCount: filteredAgents.filter(({ agent }) => categoryForAgent(agent) === category.id && agentIDs.includes(agent.id)).length,
  })).filter((category) => category.agents.length > 0), [filteredAgents, eligibleAgents, agentIDs]);
  const hasActiveFilters = categoryFilter !== 'all' || availabilityFilter !== 'all' || providerFilter !== 'all' || skillFilter !== 'all' || selectedOnly;
  const allCategoriesExpanded = categoryOptions.length > 0 && categoryOptions.every((item) => expandedCategories.includes(item.id));
  const forceExpandCategories = Boolean(agentSearch.trim()) || categoryFilter !== 'all';
  const clearFilters = () => { setCategoryFilter('all'); setAvailabilityFilter('all'); setProviderFilter('all'); setSkillFilter('all'); setSelectedOnly(false); setAgentSearch(''); };
  const maxAgents = Math.max(1, Math.min(8, config?.max_agents || 5));
  const maxDiscussionRounds = Math.max(1, Math.min(5, config?.max_discussion_rounds || 5));
  const discussionRounds = Math.max(1, Math.min(requestedRounds, maxDiscussionRounds));
  const selectedThinkingRoutes = eligibleAgents.filter(({ agent, route }) => agentIDs.includes(agent.id) && route.usable && route.supportsThinking);
  const hasThinkingSupport = selectedThinkingRoutes.length > 0;
  const hasOllamaThinking = selectedThinkingRoutes.some(({ route }) => providers.some((provider) => provider.id === route.providerID && provider.type === 'ollama'));
  const hasEffortThinking = selectedThinkingRoutes.some(({ route }) => providers.some((provider) => provider.id === route.providerID && provider.type !== 'ollama'));
  const nativeThinkingOnly = hasOllamaThinking && !hasEffortThinking;
  const effectiveThinking = hasThinkingSupport ? thinkingEffort : 'off';

  useEffect(() => {
    if (loading || !eligibleAgents.length || !usableIDs.length) return;
    const saved = loadSavedAgentIDs(bookID).filter((id) => usableIDs.includes(id));
    const chairID = 'discussion-chair';
    const core = ['reader', 'evidence-auditor', 'tutor', 'steelman'].filter((id) => usableIDs.includes(id));
    const savedParticipants = saved.filter((id) => id !== chairID);
    const fallback = [...core, ...usableIDs.filter((id) => !core.includes(id) && id !== chairID)];
    // Keep a user's previously saved specialist choices ahead of the default roster,
    // while filling any remaining slots with the core evidence-led council.
    const preferred = savedParticipants.length
      ? [...savedParticipants, ...core.filter((id) => !savedParticipants.includes(id)), ...usableIDs.filter((id) => !savedParticipants.includes(id) && !core.includes(id) && id !== chairID)]
      : fallback;
    const unique = [...new Set(preferred)];
    const chairAvailable = usableIDs.includes(chairID);
    const participants = unique.slice(0, Math.max(0, maxAgents - (chairAvailable ? 1 : 0)));
    setAgentIDs(chairAvailable ? [...participants.filter((id) => id !== chairID), chairID] : participants);
    setSelectionReadyBookID(bookID);
  }, [bookID, loading, eligibleAgents.length, usableIDs, maxAgents]);

  useEffect(() => {
    if (selectionReadyBookID !== bookID) return;
    try { localStorage.setItem(`idios.shelf.agent-discussion.${bookID}`, JSON.stringify(agentIDs)); } catch { /* Remembering selection is optional. */ }
  }, [bookID, agentIDs, selectionReadyBookID]);

  useEffect(() => {
    try { localStorage.setItem('idios.shelf.agent-discussion.participants-visible', String(participantsVisible)); } catch { /* Remembering panel visibility is optional. */ }
  }, [participantsVisible]);

  useEffect(() => {
    if (selectionReadyBookID !== bookID || !eligibleAgents.length || expandedInitializedBookID === bookID) return;
    const selectedCategories = [...new Set(eligibleAgents.filter(({ agent }) => agentIDs.includes(agent.id)).map(({ agent }) => categoryForAgent(agent)))];
    setExpandedCategories(selectedCategories.length ? selectedCategories : ['reading-research', 'learning-tutoring']);
    setExpandedInitializedBookID(bookID);
  }, [bookID, selectionReadyBookID, eligibleAgents, agentIDs, expandedInitializedBookID]);

  useEffect(() => {
    if (!activeRun || terminalStatuses.has(activeRun.status)) return;
    let cancelled = false;
    const timer = window.setTimeout(async () => {
      try {
        const next = await api.aiAgentRun(activeRun.id);
        if (!cancelled) setActiveRun(next);
      } catch (cause) {
        if (!cancelled) setError(cause instanceof Error ? cause.message : 'Could not update the agent discussion.');
      }
    }, 1100);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [activeRun]);

  useEffect(() => {
    const contributionList = array(activeRun?.workflow?.contributions);
    if (!activeRun || contributionList.length === 0) return;
    setTurns((current) => {
      const next = [...current];
      const previousRoundOffset = current.reduce((max, item) => item.runID === activeRun.id ? max : Math.max(max, item.round ?? 0), 0);
      for (const contribution of contributionList) {
        const displayRound = (contribution.round ?? 1) + previousRoundOffset;
        const threadKey = `${activeRun.id}:${contribution.round ?? 1}:${contribution.agent_id}`;
        const index = next.findIndex((item) => item.threadKey === threadKey);
        const turn = { ...contribution, round: displayRound, threadKey, runID: activeRun.id };
        if (index >= 0) next[index] = turn;
        else next.push(turn);
      }
      return next;
    });
  }, [activeRun]);

  const startTurn = useCallback(async (readerMessage = '', firstTurn = false) => {
    if (!config || agentIDs.length === 0) return;
    const waitingTurns = activeRun?.status === 'completed'
      ? turns.filter((turn) => turn.runID === activeRun.id && turn.status === 'waiting' && agentIDs.includes(turn.agent_id))
      : [];
    const waitingIDs = [...new Set(waitingTurns.map((turn) => turn.agent_id))];
    const questionerIDs = readerMessage.trim() && activeRun?.status === 'completed'
      ? [...new Set(turns.filter((turn) => turn.runID === activeRun.id && turn.status === 'completed' && questionFrom(turn.content ?? '') && agentIDs.includes(turn.agent_id)).map((turn) => turn.agent_id))]
      : [];
    // Let the agent who asked the reader a question acknowledge the answer,
    // then pass the thread to the agents that were waiting for the reader.
    const continuationIDs = readerMessage.trim()
      ? [...new Set([...questionerIDs, ...agentIDs.filter((id) => !questionerIDs.includes(id))])]
      : [...new Set([...questionerIDs, ...waitingIDs])];
    const turnOrder = continuationIDs.length ? continuationIDs : agentIDs;
    const localAgents = turnOrder.filter((id) => usableIDs.includes(id));
    if (!localAgents.length) { setError('Choose at least one agent assigned to an available Ollama Local model.'); return; }
    const originalTopic = topic.trim();
    const base = firstTurn ? originalTopic : readerMessage.trim()
      ? `Original discussion topic: ${originalTopic}\nReader's reply: ${readerMessage.trim()}\n\nContinue from the waiting agents. Respond to the reader first, then add only one useful point.`
      : `Original discussion topic: ${originalTopic}\nContinue from the waiting agents or respond to one unresolved point from earlier turns. Add only one new useful observation.`;
    if (readerMessage.trim()) {
      const answered = turns.filter((turn) => questionFrom(turn.content ?? '')).map((turn) => turn.threadKey);
      if (answered.length) setAnsweredQuestionKeys((current) => [...new Set([...current, ...answered])]);
    }
    if (!base) { setError('Enter a topic or a reply for the agents.'); return; }
    const pages = { ...sourcePages };
    if (pageText.trim()) pages[pageNumber] = pageText.trim().slice(0, 12000);
    const sourceEntries = Object.entries(pages).map(([page, text]) => ({ page: Number(page), text })).sort((a, b) => a.page - b.page).slice(-4);
    const context = sourceEntries.map((item) => ({ id: `page-${item.page}`, book_id: bookID, page: item.page, text: item.text }));
    if (!context.length && selectedText.trim()) context.push({ id: `selection-page-${pageNumber}`, book_id: bookID, page: pageNumber, text: selectedText.trim().slice(0, 5000) });
    if (selectedText.trim()) context.push({ id: `selection-page-${pageNumber}`, book_id: bookID, page: pageNumber, text: `Reader-selected passage (also part of this page):\n${selectedText.trim().slice(0, 3000)}` });
    const selectedAnnotations = [...annotations.filter((item) => item.page === pageNumber), ...annotations.slice(-8)]
      .filter((item, index, all) => all.findIndex((candidate) => candidate.id === item.id) === index).slice(-12);
    for (const annotation of selectedAnnotations) {
      const text = [`Reader's saved highlight on page ${annotation.page}: “${annotation.quote.slice(0, 700)}”`, annotation.note?.trim() ? `Reader's own note: ${annotation.note.trim().slice(0, 700)}` : ''].filter(Boolean).join('\n');
      context.push({ id: `annotation-${annotation.id}`, book_id: bookID, page: annotation.page, text });
    }
    // Avoid duplicate source IDs if the selected text is the only context.
    const uniqueContext = context.filter((item, index) => context.findIndex((candidate) => candidate.id === item.id) === index);
    const prior = turns.filter((item) => item.status === 'completed' && item.content?.trim()).slice(-8).map(({ threadKey: _key, runID: _runID, ...item }) => {
      void _key;
      void _runID;
      return { ...item, content: item.content!.slice(0, 1200) };
    });
    const payload = {
      book_id: bookID,
      book_title: bookTitle,
      page_start: Math.min(pageNumber, ...sourceEntries.map((item) => item.page)),
      page_end: pageNumber,
      question: `${base}${selectedText.trim() && firstTurn ? `\n\nFocus phrase selected on page ${pageNumber}: “${selectedText.trim().slice(0, 1200)}”` : ''}\n\nRound-table contract: conduct ${discussionRounds} sequential debate rounds, never parallel. Every agent after the first must respond to one specific earlier claim by the speaker's displayed name, assess it as supported/partly supported/unclear/unsupported, give a textual reason, and add a correction or refinement. The final agent is Mira · Chair: compare actual positions, name remaining disagreement, and give a refined conclusion. Use only supplied page/highlight/note evidence; never invent citations or make claims about unseen chapters. Separate text from inference. Do not repeat, offer generic summaries, or ask questions during peer debate. Only the Chair at the final turn may ask exactly one \"Question for you: ...\" line, and only if the answer could materially change the interpretation. Explain public reasons, not hidden chain-of-thought.`,
      context: uniqueContext,
      agent_ids: localAgents.slice(0, maxAgents),
      parallel: false,
      rounds: discussionRounds,
      ...(effectiveThinking !== 'off' ? { reasoning_effort: effectiveThinking } : {}),
      skip_synthesis: true,
      previous_contributions: firstTurn ? [] : prior,
    };
    setStarting(true); setError('');
    try {
      const task = await api.startAIAgentRun(payload);
      if (readerMessage.trim()) {
        const priorRound = turns.reduce((max, turn) => Math.max(max, turn.round ?? 0), 0);
        setTurns((current) => [...current, {
          round: priorRound + 1, agent_id: 'reader-input', agent_name: 'You · Reply', local: false,
          status: 'completed', content: readerMessage.trim(), threadKey: `reader:${task.id}`, runID: task.id,
        }]);
      }
      if (waitingTurns.length) {
        const waitingKeys = new Set(waitingTurns.map((turn) => turn.threadKey));
        setTurns((current) => current.map((turn) => waitingKeys.has(turn.threadKey)
          ? { ...turn, status: 'resumed', error: { code: 'reader_wait_resumed', message: 'This waiting turn was resumed in the discussion below.' } }
          : turn));
      }
      setActiveRun(task);
      setReaderReply('');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not start the agent discussion.');
    } finally { setStarting(false); }
  }, [config, agentIDs, usableIDs, topic, sourcePages, pageNumber, pageText, selectedText, annotations, turns, activeRun, bookID, bookTitle, maxAgents, discussionRounds, effectiveThinking]);

  const activeSpeaker = [...(activeRun?.workflow?.contributions ?? [])].reverse().find((item) => item.status === 'running');
  const visibleTurns = turns.slice(-24);

  if (!open) return null;
  const promptCanStart = Boolean(topic.trim() && agentIDs.length > 0 && usableIDs.some((id) => agentIDs.includes(id)) && pageText.trim());
  const dialog = <Fragment>
    {expanded && <button type="button" className="shelf-agent-room__expanded-backdrop" aria-label="Collapse expanded discussion" onClick={() => setExpanded(false)} />}
    <aside className={`shelf-agent-room ${expanded ? 'shelf-agent-room--expanded' : ''}`} role="dialog" aria-modal={expanded} aria-label={`Agent discussion for ${bookTitle}`}>
    <header className="shelf-agent-room__topbar">
      <div className="shelf-agent-room__brand">
        <span className="shelf-agent-room__brand-mark"><MessagesSquare size={15} /></span>
        <span className="shelf-agent-room__brand-copy"><strong>Agent discussion</strong><small>Page {pageNumber}</small></span>
      </div>
      <div className="shelf-agent-room__top-actions">
        {onOpenWebSearch && <button type="button" className="shelf-agent-room__icon-button" onClick={() => onOpenWebSearch(`${bookTitle} page ${pageNumber}${topic.trim() ? ` ${topic.trim()}` : selectedText.trim() ? ` ${selectedText.trim().slice(0, 180)}` : ''}`.trim())} aria-label="Search web about this discussion" title="Open a web search for this book and discussion topic"><Search size={14} /></button>}
        <button type="button" className={`shelf-agent-room__icon-button ${expanded ? 'is-active' : ''}`} onClick={() => setExpanded((value) => !value)} aria-label={expanded ? 'Collapse discussion' : 'Expand discussion'} title={expanded ? 'Collapse discussion' : 'Expand discussion'}>{expanded ? <Minimize2 size={14} /> : <Maximize2 size={14} />}</button>
        <button type="button" className={`shelf-agent-room__icon-button ${!participantsVisible ? 'is-active' : ''}`} onClick={() => setParticipantsVisible((visible) => !visible)} aria-label={participantsVisible ? 'Hide agent selector' : 'Show agent selector'} aria-expanded={participantsVisible} aria-controls="shelf-agent-room-agent-selector" title={participantsVisible ? 'Hide agent list' : `Show agents (${agentIDs.length} selected)`}>{participantsVisible ? <EyeOff size={14} /> : <UsersRound size={14} />}</button>
        {turns.length > 0 && <button type="button" className="shelf-agent-room__icon-button" disabled={busy || starting} onClick={() => { setTurns([]); setAnsweredQuestionKeys([]); setActiveRun(null); setReaderReply(''); setTopic(''); setError(''); }} aria-label="New discussion" title="New discussion"><Sparkles size={14} /></button>}
        <button type="button" className="shelf-agent-room__icon-button" onClick={requestClose} disabled={starting} aria-label="Close discussion" title="Close"><X size={15} /></button>
      </div>
    </header>

    {participantsVisible && <section id="shelf-agent-room-agent-selector" className="shelf-agent-room__participants" aria-label="Choose discussion agents">
      <div className="shelf-agent-room__participant-toolbar">
        <span className="shelf-agent-room__participant-count" title={`${eligibleAgents.length} eligible agents`}>{agentIDs.length}/{maxAgents} selected <span>· {eligibleAgents.length} agents</span></span>
        <label className="shelf-agent-room__search">
          <Search size={12} aria-hidden="true" />
          <input aria-label="Search agents" value={agentSearch} onChange={(event) => setAgentSearch(event.target.value)} placeholder="Search agents, skills…" disabled={loading || busy || starting} />
          {agentSearch && <button type="button" onClick={() => setAgentSearch('')} aria-label="Clear agent search"><X size={11} /></button>}
        </label>
      </div>
      <div className="shelf-agent-room__filter-toolbar">
        <button type="button" className={`shelf-agent-room__filter-toggle ${filtersOpen || hasActiveFilters ? 'is-active' : ''}`} onClick={() => setFiltersOpen((value) => !value)} aria-expanded={filtersOpen}><SlidersHorizontal size={11}/><span>Filters</span>{hasActiveFilters && <span className="shelf-agent-room__filter-dot"/>}</button>
        <button type="button" className={`shelf-agent-room__filter-toggle ${selectedOnly ? 'is-active' : ''}`} onClick={() => setSelectedOnly((value) => !value)} aria-pressed={selectedOnly}><Check size={11}/><span>Selected only</span></button>
        <span className="shelf-agent-room__filter-spacer"/>
        {!forceExpandCategories && <button type="button" className="shelf-agent-room__text-action" onClick={() => setExpandedCategories((current) => categoryOptions.every((item) => current.includes(item.id)) ? [] : categoryOptions.map((item) => item.id))} disabled={!categoryOptions.length}>{allCategoriesExpanded ? 'Collapse all' : 'Expand all'}</button>}
      </div>
      {filtersOpen && <div className="shelf-agent-room__filters">
        <label><span>Category</span><select value={categoryFilter} onChange={(event) => setCategoryFilter(event.target.value)}><option value="all">All categories</option>{categoryOptions.map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}</select></label>
        <label><span>Availability</span><select value={availabilityFilter} onChange={(event) => setAvailabilityFilter(event.target.value)}><option value="all">All states</option><option value="ready">Ready to run</option><option value="attention">Needs attention</option></select></label>
        <label><span>Provider</span><select value={providerFilter} onChange={(event) => setProviderFilter(event.target.value)}><option value="all">All providers</option>{providerOptions.map(([id, name]) => <option key={id} value={id}>{name}</option>)}</select></label>
        <label><span>Skill</span><select value={skillFilter} onChange={(event) => setSkillFilter(event.target.value)}><option value="all">Any skill</option>{skillOptions.map((id) => <option key={id} value={id}>{prettySkill(id)}</option>)}</select></label>
        {hasActiveFilters && <button type="button" className="shelf-agent-room__text-action shelf-agent-room__clear-filters" onClick={clearFilters}>Clear filters</button>}
      </div>}
      <div className="shelf-agent-room__category-list" role="group" aria-label="Agents grouped by category">
        {loading ? <span className="shelf-agent-room__loading"><LoaderCircle size={13} className="shelf-agent-room__spin" /> Loading agents…</span> : groupedAgents.map((category) => {
          const expanded = expandedCategories.includes(category.id) || forceExpandCategories;
          return <section className="shelf-agent-room__category" key={category.id}>
            <button type="button" className="shelf-agent-room__category-heading" title={category.description} aria-expanded={expanded} onClick={() => setExpandedCategories((current) => current.includes(category.id) ? current.filter((id) => id !== category.id) : [...current, category.id])}>
              {expanded ? <ChevronDown size={12}/> : <ChevronRight size={12}/>}<span>{category.label}</span><small>{category.agents.length}{category.selectedCount ? ` · ${category.selectedCount} selected` : ''}</small>
            </button>
            {expanded && <div className="shelf-agent-room__category-agents">{category.agents.map(({ agent, route }) => {
              const selected = agentIDs.includes(agent.id);
              const initials = agent.name.trim().split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase();
              const isRequiredChair = agent.id === 'discussion-chair';
              return <button key={agent.id} type="button" className={`shelf-agent-room__profile-chip profile-${agent.id.replace(/[^a-z0-9-]/gi, '-')} ${selected ? 'is-selected' : ''} ${!route.usable ? 'is-unavailable' : ''}`} title={isRequiredChair ? `${agent.name} · Required final synthesis role · ${route.label}` : route.usable ? `${agent.name} · ${route.label}${route.speed ? ` · ${route.speed.toFixed(1)} tok/s` : ''}` : `${agent.name}: ${route.reason || 'Model unavailable'}`} aria-pressed={selected} disabled={!route.usable || (isRequiredChair && selected) || (!selected && agentIDs.length >= maxAgents) || busy || starting} onClick={() => setAgentIDs((current) => selected ? (isRequiredChair ? current : current.filter((id) => id !== agent.id)) : [...current, agent.id].slice(0, maxAgents))}>
                <span className="shelf-agent-room__avatar">{initials}</span><span>{agent.name}</span>{selected && <Check size={12} />}
              </button>;
            })}</div>}
          </section>;
        })}
        {!loading && !eligibleAgents.length && <span className="shelf-agent-room__empty-small">No discussion agents enabled.</span>}
        {!loading && eligibleAgents.length > 0 && filteredAgents.length === 0 && <span className="shelf-agent-room__empty-small">No agents match the current search or filters.</span>}
      </div>
    </section>}

    <main className={`shelf-agent-room__main ${!participantsVisible ? 'has-hidden-agent-selector' : ''}`}>
      {error && <div className="shelf-agent-room__error" role="alert">{error}</div>}
      <div className="shelf-agent-room__timeline" aria-live="polite">
        {!turns.length && !busy && <div className="shelf-agent-room__empty-state">Start with your thought, a question, or a highlighted line.</div>}
        {turns.length > 0 && <article className="shelf-agent-room__reader-opening"><span className="shelf-agent-room__reader-avatar">Y</span><div><div className="shelf-agent-room__message-head"><strong>You</strong></div><p>{topic}</p></div></article>}
        {visibleTurns.map((turn, turnIndex) => {
          const q = questionFrom(turn.content ?? '');
          const profile = agents.find((agent) => agent.id === turn.agent_id);
          const previousRound = visibleTurns[turnIndex - 1]?.round;
          const status = turn.status === 'running' ? 'thinking' : turn.status === 'waiting' ? 'waiting for you' : turn.status === 'resumed' ? 'continued' : turn.status === 'failed' ? 'failed' : turn.status === 'not_run' ? 'queued' : '';
          return <Fragment key={turn.threadKey}>
            {turn.round !== previousRound && <div className="shelf-agent-room__round-heading"><span /> Round {turn.round ?? 1} <span /></div>}
            <article className={`shelf-agent-room__message profile-${turn.agent_id.replace(/[^a-z0-9-]/gi, '-')} ${turn.agent_id === 'reader-input' ? 'is-reader-reply' : ''} ${turn.status === 'running' ? 'is-running' : ''} ${turn.status === 'failed' ? 'is-failed' : ''}`}>
            <div className="shelf-agent-room__message-avatar">{(turn.agent_name || 'AI').trim().split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase()}</div>
            <div className="shelf-agent-room__message-body">
              <div className="shelf-agent-room__message-head"><strong>{turn.agent_name}</strong>{profile?.role && <small className="shelf-agent-room__role">{profile.role}</small>}{status && <small>{status}</small>}</div>
              <p>{turn.status === 'running' ? 'Reading the page…' : turn.content ? visibleTurnText(turn.content) || turn.content : turn.error?.message || 'No response returned.'}</p>
              {turn.warnings?.map((warning) => <div className="shelf-agent-room__turn-warning" key={warning} role="note">{warning}</div>)}
              {q && <div className="shelf-agent-room__question"><CircleHelp size={13} /><span>{q}{answeredQuestionKeys.includes(turn.threadKey) ? ' · answered' : ''}</span></div>}
              {turn.status === 'completed' && turn.agent_id !== 'reader-input' && <details className="shelf-agent-room__turn-details"><summary>Model details</summary><span>{turn.model || 'Configured model'}{turn.provider_id ? ` · ${turn.provider_id}` : ''}{turn.local ? ' · local' : ''}</span></details>}
            </div>
            </article>
          </Fragment>;
        })}
        {busy && <div className="shelf-agent-room__speaking"><LoaderCircle size={14} className="shelf-agent-room__spin" /><span>{activeSpeaker?.agent_name || 'An agent'} is thinking. Other models are waiting.</span></div>}
        {activeRun && terminalStatuses.has(activeRun.status) && activeRun.workflow?.warnings?.length ? <div className="shelf-agent-room__warning">{activeRun.workflow.warnings.join(' ')}</div> : null}
      </div>
      <form className="shelf-agent-room__composer" onSubmit={(event) => { event.preventDefault(); if (!turns.length) void startTurn('', true); else void startTurn(readerReply, false); }}>
        {!turns.length ? <textarea aria-label="Your opening thought" rows={2} value={topic} onChange={(event) => setTopic(event.target.value)} placeholder="Start the discussion…" disabled={busy || starting} /> : <textarea aria-label="Your reply" rows={2} value={readerReply} onChange={(event) => setReaderReply(event.target.value)} placeholder="Reply to an agent…" disabled={busy || starting} />}
        <div className="shelf-agent-room__composer-options" aria-label="Discussion settings">
          <label title="More rounds permit deeper critique but make the discussion slower."><span>Depth</span><select aria-label="Discussion depth" value={discussionRounds} onChange={(event) => setRequestedRounds(Number(event.target.value))} disabled={busy || starting}>{Array.from({ length: maxDiscussionRounds }, (_, index) => index + 1).filter((value) => value >= 2 || maxDiscussionRounds === 1).map((value) => <option key={value} value={value}>{value} {value === 1 ? 'round' : 'rounds'}</option>)}</select></label>
          <label title={hasThinkingSupport ? (nativeThinkingOnly ? 'Enables native thinking for supported Ollama models; unsupported selected models ignore it.' : 'Reasoning effort is sent only to selected models that support it.') : 'The selected models do not advertise native thinking support.'}><span>Thinking</span><select aria-label={nativeThinkingOnly ? 'Native thinking' : 'Thinking effort'} value={effectiveThinking} onChange={(event) => setThinkingEffort(event.target.value as 'off' | 'low' | 'medium' | 'high')} disabled={!hasThinkingSupport || busy || starting}><option value="off">Off</option>{nativeThinkingOnly ? <option value="high">On</option> : <><option value="low">Low</option><option value="medium">Medium</option><option value="high">High</option></>}</select></label>
          <span className="shelf-agent-room__source-hint" title="Only the current page and saved notes are evidence; web results are not automatically injected into the discussion.">Page {pageNumber} · {annotations.length} notes</span>
        </div>
        <div className="shelf-agent-room__composer-bottom">
          <button type="submit" className="shelf-agent-room__send" aria-label={turns.length ? (readerReply.trim() ? 'Reply and continue' : 'Continue discussion') : 'Start discussion'} title={turns.length ? (readerReply.trim() ? 'Reply and continue' : 'Continue discussion') : 'Start discussion'} disabled={starting || busy || (!turns.length ? !promptCanStart : !agentIDs.length || !usableIDs.some((id) => agentIDs.includes(id)))}>{starting || busy ? <LoaderCircle size={15} className="shelf-agent-room__spin" /> : <Send size={14} />}</button>
        </div>
        {selectedText.trim() && !turns.length && !topic.trim() && <button type="button" className="shelf-agent-room__use-selection" onClick={() => setTopic(`Let's discuss this passage: “${selectedText.trim().slice(0, 500)}”`)}>Use selected passage</button>}
        {busy && <button type="button" className="shelf-agent-room__stop" onClick={() => activeRun && void api.cancelAIAgentRun(activeRun.id).catch((cause: unknown) => setError(cause instanceof Error ? cause.message : 'Could not stop the discussion.'))}>Stop</button>}
      </form>
    </main>
    </aside>
  </Fragment>;
  return dialog;
}
