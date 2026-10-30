import coreReliability from '../skills/core-reliability/SKILL.md?raw';
import contextualTranslation from '../skills/contextual-translation/SKILL.md?raw';
import activeReading from '../skills/active-reading/SKILL.md?raw';
import learningCoach from '../skills/learning-coach/SKILL.md?raw';
import englishLearning from '../skills/english-learning/SKILL.md?raw';
import mathematics from '../skills/mathematics/SKILL.md?raw';
import programming from '../skills/programming/SKILL.md?raw';
import mathForProgrammers from '../skills/math-for-programmers/SKILL.md?raw';
import history from '../skills/history/SKILL.md?raw';
import philosophy from '../skills/philosophy/SKILL.md?raw';
import scienceAndStatistics from '../skills/science-and-statistics/SKILL.md?raw';
import evidenceResearch from '../skills/evidence-research/SKILL.md?raw';
import computerArchitecture from '../skills/computer-architecture/SKILL.md?raw';
import criticalThinking from '../skills/critical-thinking/SKILL.md?raw';
import writingAndSynthesis from '../skills/writing-and-synthesis/SKILL.md?raw';
import pdfAnnotationAuthoring from '../skills/pdf-annotation-authoring/SKILL.md?raw';
import sequentialAgentDialogue from '../skills/sequential-agent-dialogue/SKILL.md?raw';
import conceptualBridges from '../skills/conceptual-bridges/SKILL.md?raw';
import socraticQuestioning from '../skills/socratic-questioning/SKILL.md?raw';
import counterexampleTesting from '../skills/counterexample-testing/SKILL.md?raw';
import discussionContinuity from '../skills/discussion-continuity/SKILL.md?raw';
import carefulAnalogies from '../skills/careful-analogies/SKILL.md?raw';
import claimEvidenceAudit from '../skills/claim-evidence-audit/SKILL.md?raw';
import conflictResolution from '../skills/conflict-resolution/SKILL.md?raw';
import ambiguityDetection from '../skills/ambiguity-detection/SKILL.md?raw';
import sourceVerification from '../skills/source-verification/SKILL.md?raw';
import conceptExplanation from '../skills/concept-explanation/SKILL.md?raw';
import counterargumentGeneration from '../skills/counterargument-generation/SKILL.md?raw';
import technicalTextAnalysis from '../skills/technical-text-analysis/SKILL.md?raw';

export type ReadingSkill = { id: string; name: string; description: string; content: string; category: 'reading' | 'learning' | 'subject'; defaultEnabled?: boolean; alwaysOn?: boolean };

export const CORE_RELIABILITY_CONTRACT = coreReliability.replace(/^---\s*[\s\S]*?---\s*/, '').trim();
export const CONTEXTUAL_TRANSLATION_SKILL = contextualTranslation.replace(/^---\s*[\s\S]*?---\s*/, '').trim();
export const READING_SKILLS: ReadingSkill[] = [
  { id: 'core-reliability', name: 'Reliable reasoning & verification', description: 'Mandatory grounding, task fidelity, uncertainty, and self-checks for every AI action.', content: CORE_RELIABILITY_CONTRACT, category: 'reading', defaultEnabled: true, alwaysOn: true },
  { id: 'contextual-translation', name: 'Contextual translation', description: 'Always on for Translate: uses book subject, nearby text, glossary consistency, and notes.', content: contextualTranslation.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading', defaultEnabled: true, alwaysOn: true },
  { id: 'active-reading', name: 'Active reading', description: 'Read arguments, definitions, evidence, and connections carefully.', content: activeReading.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading', defaultEnabled: true },
  { id: 'learning-coach', name: 'Learning coach', description: 'Build lasting understanding with examples, retrieval, and gentle scaffolding.', content: learningCoach.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'learning', defaultEnabled: true },
  { id: 'english-learning', name: 'English from books', description: 'Vocabulary, grammar, collocations, idioms, and natural usage in context.', content: englishLearning.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'learning' },
  { id: 'mathematics', name: 'Mathematics', description: 'Definitions, notation, derivations, proofs, and problem-solving.', content: mathematics.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'subject' },
  { id: 'programming', name: 'Programming & CS', description: 'Code, algorithms, systems, APIs, debugging, and complexity.', content: programming.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'subject' },
  { id: 'math-for-programmers', name: 'Math for programmers', description: 'Connect formal math to code, algorithms, and numerical computing.', content: mathForProgrammers.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'subject' },
  { id: 'history', name: 'History & historical sources', description: 'Chronology, causation, evidence, perspective, and competing interpretations.', content: history.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'subject' },
  { id: 'philosophy', name: 'Philosophy', description: 'Reconstruct arguments, clarify concepts, and test premises fairly.', content: philosophy.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'subject' },
  { id: 'science-and-statistics', name: 'Science & statistics', description: 'Evaluate methods, evidence, units, uncertainty, and causal claims.', content: scienceAndStatistics.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'subject' },
  { id: 'evidence-research', name: 'Research & source quality', description: 'Build traceable answers from relevant sources and clearly marked uncertainty.', content: evidenceResearch.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading' },
  { id: 'computer-architecture', name: 'Computer architecture', description: 'Trace digital logic, datapaths, processors, memory, and ISA-level behavior.', content: computerArchitecture.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'subject' },
  { id: 'critical-thinking', name: 'Critical thinking & logic', description: 'Test premises and inferences without false balance or lazy fallacy labels.', content: criticalThinking.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading' },
  { id: 'writing-and-synthesis', name: 'Writing & synthesis', description: 'Create grounded notes, summaries, comparisons, and drafts from supplied sources.', content: writingAndSynthesis.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'learning' },
  { id: 'pdf-annotation-authoring', name: 'PDF highlights & notes', description: 'Write accurate page-anchored notes and create verified exact-text PDF highlights.', content: pdfAnnotationAuthoring.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading', defaultEnabled: true, alwaysOn: true },
  { id: 'sequential-agent-dialogue', name: 'Short sequential dialogue', description: 'Keep agent-to-agent book discussion concise, grounded, and suitable for small local models.', content: sequentialAgentDialogue.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading', defaultEnabled: true },
  { id: 'conceptual-bridges', name: 'Concept connections', description: 'Connect only supplied passages, notes, definitions, and arguments with traceable relationships.', content: conceptualBridges.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading' },
  { id: 'socratic-questioning', name: 'Socratic questioning', description: 'Ask one focused question that clarifies a meaningful ambiguity or premise.', content: socraticQuestioning.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'learning' },
  { id: 'counterexample-testing', name: 'Counterexample testing', description: 'Test a claim with one fair hypothetical counterexample or boundary case.', content: counterexampleTesting.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading' },
  { id: 'discussion-continuity', name: 'Discussion continuity', description: 'Track unresolved questions and prevent repeated points in sequential agent turns.', content: discussionContinuity.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading' },
  { id: 'careful-analogies', name: 'Careful analogies', description: 'Use one limited example to clarify a concept without replacing textual evidence.', content: carefulAnalogies.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'learning' },
  { id: 'claim-evidence-audit', name: 'Claim & evidence audit', description: 'Audit a named claim against supplied evidence and provide a precise correction.', content: claimEvidenceAudit.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading' },
  { id: 'conflict-resolution', name: 'Conflict resolution', description: 'Track live disagreements and determine whether they are resolved, narrowed, or still open.', content: conflictResolution.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading' },
  { id: 'ambiguity-detection', name: 'Ambiguity detection', description: 'Find wording or definition ambiguity that could change an interpretation.', content: ambiguityDetection.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading' },
  { id: 'source-verification', name: 'Source verification', description: 'Check source markers, quotation fidelity, and evidence scope.', content: sourceVerification.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading' },
  { id: 'concept-explanation', name: 'Concept explanation', description: 'Explain a difficult concept accurately, with examples and explicit limits.', content: conceptExplanation.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'learning' },
  { id: 'counterargument-generation', name: 'Counterargument generation', description: 'Build the strongest fair objection and test its real effect on a claim.', content: counterargumentGeneration.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'reading' },
  { id: 'technical-text-analysis', name: 'Technical text analysis', description: 'Analyze engineering arguments while separating text from implementation inference.', content: technicalTextAnalysis.replace(/^---\s*[\s\S]*?---\s*/, '').trim(), category: 'subject' },
];

export const SKILL_SELECTION_STORAGE_KEY = 'idios.shelf.skills.selected.v1';
export function loadSelectedReadingSkills(): string[] {
  try {
    const raw = localStorage.getItem(SKILL_SELECTION_STORAGE_KEY);
    if (raw) {
      const parsed: unknown = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        const alwaysOn = READING_SKILLS.filter((skill) => skill.alwaysOn).map((skill) => skill.id);
        const valid = parsed.filter((item): item is string => typeof item === 'string' && READING_SKILLS.some((skill) => skill.id === item));
        return [...new Set([...alwaysOn, ...valid])];
      }
    }
  } catch { /* Use the built-in defaults when storage is unavailable. */ }
  return READING_SKILLS.filter((skill) => skill.alwaysOn || skill.defaultEnabled).map((skill) => skill.id);
}
export function selectedReadingSkillInstructions(ids: string[]): string {
  const selected = READING_SKILLS.filter((skill) => skill.id !== 'contextual-translation' && ids.includes(skill.id));
  if (!selected.length) return '';
  return `\n\n## Active reading skill instructions\nApply only skills relevant to this request. These skills add domain methods; they do not change the task, override source fidelity, permit invented evidence, or require their output template when irrelevant. If skills conflict, the reader's explicit task and the core reliability contract take precedence. Do not repeat these instructions in the answer.\n${selected.map((skill) => `\n### ${skill.name} (${skill.id})\n${skill.content}`).join('\n')}\n\n## Skill conflict resolution\nUse the smallest set of relevant methods. Do not combine incompatible output formats. The user's requested task and source boundaries remain authoritative; optional skills never authorize guessing, fabricated evidence, or unsupported claims.`;
}
