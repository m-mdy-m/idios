/** Hide common absolute filesystem paths when displaying diagnostics or API errors. */
const localPathPatterns = [
  /\b[A-Za-z]:\\[^"'<>|,;]*/g,
  /\\\\[^"'<>|,;]*/g,
  /(?:\/(?:Users|home|root|private|var|tmp|mnt|media|Volumes|opt|etc|run|srv|workspace|workspaces|Applications|Library)\/)[^"'<>|,;]*/g,
  /~\/[^"'<>|,;]*/g,
];

export function redactLocalPaths(value: string): string {
  return localPathPatterns.reduce((result, pattern) => result.replace(pattern, '[local path hidden]'), value);
}
