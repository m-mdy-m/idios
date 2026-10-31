import type { LucideIcon } from 'lucide-react';
import type { Note, ShelfAnnotation, ShelfAsk, ShelfBook, ShelfBookRelation } from '@/types/idios';

export type ChatMessage = { role: 'user' | 'assistant'; content: string; reasoning?: string; context?: string; pageNumber?: number; model?: string; reasoningEffort?: 'low' | 'medium' | 'high'; hidden?: boolean; variant?: 'reworked' | 'alternate' };
export type ArchivedThread = { id: string; title: string; messages: ChatMessage[]; updatedAt: string };
export type AISettings = { model: string };
export type ModelAvailability = 'available' | 'unavailable' | 'blocked' | 'checking' | 'unknown';
export type ModelOption = { id: string; providerID: string; modelID: string; providerName: string; local: boolean; name: string; availability: ModelAvailability; error?: string; reasoning?: boolean; loaded?: boolean; contextSize?: number };
export type LegacyAISettings = { baseURL?: string; apiKey?: string; model?: string };
export type ShelfRuntimeMessage = { role: 'system' | 'user' | 'assistant'; content: string };
export type ShelfRuntimeResponse = {
  model?: string;
  error?: { message?: string };
  choices?: Array<{
    reasoning_content?: string;
    reasoning_details?: Array<{ type?: string; text?: string; summary?: string; content?: string }>;
    message?: {
      content?: string | Array<{ type?: string; text?: string }>;
      reasoning_content?: string;
      reasoning?: string;
      thinking?: string;
      analysis?: string;
      reasoning_details?: Array<{ type?: string; text?: string; summary?: string; content?: string }>;
    };
  }>;
};
export type ReadPage = { page: number; text: string; visitedAt: string };
export type CompanionMark = { quote: string; note: string; kind: 'important' | 'unclear' };
export type PageInsight = { page: number; topic: string; companion: string; tutorReply: string; lookFor: string; challenge?: string; curiousQuestion?: string; highlightQuote?: string; marks?: CompanionMark[]; model?: string; createdAt: string };
export type ReasoningEffort = 'low' | 'medium' | 'high';
export type AskOptions = { modelOverride?: string; replaceFromIndex?: number; hiddenUser?: boolean; variant?: 'reworked' | 'alternate'; reasoningEffort?: ReasoningEffort; autoOverview?: boolean; contextOverride?: string; pageOverride?: number; historyBeforeIndex?: number };
export type StudyContext = {
  book: ShelfBook | null;
  notes: Note[];
  annotations: ShelfAnnotation[];
  previousQuestions: ShelfAsk[];
  relations: ShelfBookRelation[];
  relatedBooks: ShelfBook[];
  loading: boolean;
};
export type DisplayMode = 'reader' | 'chat' | 'models' | 'inline-chat';
export type MascotPosition = { left: number; top: number };
export type MascotDrag = { pointerID: number; startX: number; startY: number; originLeft: number; originTop: number; moved: boolean };
export type RequestedBookHighlight = { quote: string; note: string; page?: number; color?: 'yellow' | 'green' | 'blue' | 'pink' | 'purple' };
export type RequestedBookAction =
  | ({ type: 'highlight' } & RequestedBookHighlight)
  | { type: 'note'; title: string; content: string; labels?: string[] }
  | { type: 'quote'; text: string; author?: string; source?: string; source_url?: string; category?: string };
export type ApplyBookHighlightsResult = { saved: string[]; failed: Array<{ quote: string; reason: string }> };
export type Props = { bookID: string; bookTitle: string; pageNumber: number; pageText: string; selectedText: string; hoveredText: string; displayMode: DisplayMode; onOpenChat?: (prompt?: string) => void; onBackToReading?: () => void; onOpenModels?: () => void; onBackToChat?: () => void; onCloseInlineChat?: () => void; onApplyBookHighlights?: (marks: RequestedBookHighlight[]) => Promise<ApplyBookHighlightsResult>; onGetDocumentContext?: (pageNumber: number) => Promise<string>; initialPrompt?: string; initialSettingsOpen?: boolean; quietMode?: boolean };
export type PromptMode = { label: string; icon: LucideIcon; prompt: string };

