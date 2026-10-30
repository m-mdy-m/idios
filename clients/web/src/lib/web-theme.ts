import type { AppConfig, CustomWebTheme } from '@/types/idios';

const CUSTOM_PROPERTIES = [
  '--bg', '--bg-elevated', '--surface', '--surface-raised', '--surface-hover', '--surface-subtle',
  '--text', '--text-soft', '--muted', '--muted-2', '--border', '--border-strong',
  '--accent', '--accent-strong', '--accent-soft', '--green', '--amber', '--red', '--violet',
] as const;

function applyCustomTheme(theme?: CustomWebTheme) {
  const root = document.documentElement;
  for (const property of CUSTOM_PROPERTIES) root.style.removeProperty(property);
  if (!theme) return;

  const set = (property: typeof CUSTOM_PROPERTIES[number], value: string) => root.style.setProperty(property, value);
  set('--bg', theme.background);
  set('--bg-elevated', theme.background);
  set('--surface', theme.surface);
  set('--surface-raised', `color-mix(in srgb, ${theme.surface} 84%, ${theme.text})`);
  set('--surface-hover', `color-mix(in srgb, ${theme.surface} 70%, ${theme.text})`);
  set('--surface-subtle', theme.background);
  set('--text', theme.text);
  set('--text-soft', `color-mix(in srgb, ${theme.text} 82%, ${theme.muted})`);
  set('--muted', theme.muted);
  set('--muted-2', `color-mix(in srgb, ${theme.muted} 70%, ${theme.background})`);
  set('--border', theme.border);
  set('--border-strong', `color-mix(in srgb, ${theme.border} 68%, ${theme.text})`);
  set('--accent', theme.accent);
  set('--accent-strong', `color-mix(in srgb, ${theme.accent} 80%, ${theme.text})`);
  set('--accent-soft', `color-mix(in srgb, ${theme.accent} 16%, transparent)`);
  set('--green', theme.success);
  set('--amber', theme.warning);
  set('--red', theme.danger);
  set('--violet', theme.accent);
}

/** Apply user-defined CSS tokens and return the base mode for the root data-theme attribute. */
export function applyWebTheme(config: AppConfig): string {
  const selected = config.web.theme || 'dark';
  const customTheme = config.web.custom_themes?.[selected];
  applyCustomTheme(customTheme);
  if (customTheme) return customTheme.base;
  if (selected === 'system') {
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  }
  return selected;
}
