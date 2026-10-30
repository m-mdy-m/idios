export function formatRelative(raw: string) {
  if (!raw) return '—';
  const value = new Date(raw).getTime();
  if (!Number.isFinite(value)) return '—';
  const minutes = Math.round((value - Date.now()) / 60_000);
  const abs = Math.abs(minutes);
  if (abs < 1) return 'just now';
  if (abs < 60) return `${abs}m ${minutes < 0 ? 'ago' : 'from now'}`;
  const hours = Math.round(abs / 60);
  if (hours < 24) return `${hours}h ${minutes < 0 ? 'ago' : 'from now'}`;
  const days = Math.round(hours / 24);
  if (days < 7) return `${days}d ${minutes < 0 ? 'ago' : 'from now'}`;
  return new Date(raw).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}
