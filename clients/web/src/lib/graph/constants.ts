export const GRAPH_COLORS: Record<string, string> = {
  issue: 'var(--graph-issue)',
  repository: 'var(--graph-repo)',
  branch: 'var(--graph-branch)',
  project: 'var(--graph-project)',
  concept: 'var(--graph-concept)',
  note: 'var(--graph-note)',
  pull_request: 'var(--graph-pr)',
  label: 'var(--graph-label)',
  milestone: 'var(--graph-milestone)',
  completion: 'var(--graph-completion)',
};

export function graphTypeLabel(type: string): string {
  return type.replaceAll('_', ' ').replace(/\b\w/g, (character) => character.toUpperCase());
}
