const PREFERENCE_PREFIX = ['idios', 'notifications'].join('.');
const SOUND_KEY = `${PREFERENCE_PREFIX}.sound.v1`;
const VOLUME_KEY = `${PREFERENCE_PREFIX}.sound-volume.v2`;

let audioContext: AudioContext | null = null;
let sequenceVersion = 0;
const activeBuses = new Set<GainNode>();
const busCleanupTimers = new Map<GainNode, number>();

export function notificationSoundEnabled(): boolean {
  try { return localStorage.getItem(SOUND_KEY) !== 'false'; }
  catch { return true; }
}

export function setNotificationSoundEnabled(enabled: boolean): void {
  try { localStorage.setItem(SOUND_KEY, String(enabled)); } catch { /* Keep the in-session preference. */ }
  if (!enabled) stopPlanReminderSequence();
  window.dispatchEvent(new CustomEvent('idios:notification-sound-updated', { detail: enabled }));
}

export function notificationSoundVolume(): number {
  try {
    const raw = localStorage.getItem(VOLUME_KEY);
    if (raw === null || raw.trim() === '') return 0.55;
    const stored = Number(raw);
    return Number.isFinite(stored) ? Math.min(1, Math.max(0, stored)) : 0.55;
  } catch { return 0.55; }
}

export function setNotificationSoundVolume(volume: number): void {
  const normalized = Number.isFinite(volume) ? Math.min(1, Math.max(0, volume)) : 0.55;
  try { localStorage.setItem(VOLUME_KEY, String(normalized)); } catch { /* Keep the in-session preference. */ }
  window.dispatchEvent(new CustomEvent('idios:notification-volume-updated', { detail: normalized }));
}

export async function activateNotificationAudio(): Promise<boolean> {
  if (typeof window === 'undefined' || !window.AudioContext) return false;
  try {
    audioContext ??= new window.AudioContext();
    if (audioContext.state === 'suspended') await audioContext.resume();
    return audioContext.state === 'running';
  } catch { return false; }
}

type ChimeNote = { frequency: number; delay: number; duration: number; strength: number };

const inboxNotes: ChimeNote[] = [
  { frequency: 587.33, delay: 0, duration: 0.62, strength: 0.27 },       // D5
  { frequency: 739.99, delay: 0.12, duration: 0.66, strength: 0.24 },    // F#5
  { frequency: 880, delay: 0.25, duration: 0.78, strength: 0.22 },      // A5
];

const reminderNotes: ChimeNote[] = [
  { frequency: 523.25, delay: 0, duration: 0.72, strength: 0.25 },       // C5
  { frequency: 659.25, delay: 0.14, duration: 0.76, strength: 0.23 },    // E5
  { frequency: 783.99, delay: 0.29, duration: 0.86, strength: 0.21 },    // G5
  { frequency: 1046.5, delay: 0.44, duration: 0.92, strength: 0.18 },    // C6
];

function scheduleBellPartial(
  context: AudioContext,
  output: AudioNode,
  start: number,
  duration: number,
  frequency: number,
  peak: number,
  type: OscillatorType,
): void {
  const oscillator = context.createOscillator();
  const envelope = context.createGain();
  oscillator.type = type;
  oscillator.frequency.setValueAtTime(frequency, start);
  envelope.gain.setValueAtTime(0.0001, start);
  envelope.gain.exponentialRampToValueAtTime(Math.max(0.0002, peak), start + 0.014);
  envelope.gain.exponentialRampToValueAtTime(0.0001, start + duration);
  oscillator.connect(envelope);
  envelope.connect(output);
  oscillator.start(start);
  oscillator.stop(start + duration + 0.025);
}

function scheduleBell(context: AudioContext, output: AudioNode, start: number, note: ChimeNote): void {
  const frequency = Math.max(80, note.frequency);
  const peak = note.strength;
  scheduleBellPartial(context, output, start, note.duration, frequency, peak, 'sine');
  // Quiet upper partials give the sine tone a soft, glass-like bell character without a sharp attack.
  scheduleBellPartial(context, output, start + 0.002, note.duration * 0.72, frequency * 2.01, peak * 0.12, 'sine');
  scheduleBellPartial(context, output, start + 0.004, note.duration * 0.48, frequency * 3.01, peak * 0.035, 'sine');
}

function createChimeBus(context: AudioContext, volume: number): GainNode {
  const bus = context.createGain();
  // Limit the master gain so old saved 100% preferences cannot turn the new sound into an alarm.
  bus.gain.setValueAtTime(0.2 * volume, context.currentTime);
  bus.connect(context.destination);
  activeBuses.add(bus);
  return bus;
}

function releaseBus(bus: GainNode, token: number, durationMS: number): void {
  const timer = window.setTimeout(() => {
    busCleanupTimers.delete(bus);
    if (token !== sequenceVersion) return;
    try { bus.disconnect(); } catch { /* The browser may already have closed the context. */ }
    activeBuses.delete(bus);
  }, durationMS);
  busCleanupTimers.set(bus, timer);
}

async function playGentleChime(notes: ChimeNote[]): Promise<void> {
  if (!notificationSoundEnabled() || notificationSoundVolume() <= 0) return;
  const token = sequenceVersion;
  const ready = await activateNotificationAudio();
  if (!ready || !audioContext || token !== sequenceVersion || !notificationSoundEnabled()) return;

  const context = audioContext;
  const bus = createChimeBus(context, notificationSoundVolume());
  const firstNoteAt = context.currentTime + 0.025;
  for (const note of notes) {
    scheduleBell(context, bus, firstNoteAt + note.delay, note);
  }
  const tail = Math.max(...notes.map((note) => note.delay + note.duration));
  releaseBus(bus, token, Math.ceil((tail + 0.2) * 1000));
}

/** A short, warm three-note chime for newly received inbox notifications. */
export function playNotificationChime(): Promise<void> {
  return playGentleChime(inboxNotes);
}

/** A slightly longer four-note variation for plan reminders; no repeats or spoken alarm. */
export function playPlanReminderSequence(): void {
  void playGentleChime(reminderNotes);
}

/** Preview the same gentle reminder sound used by the background notification flow. */
export function playPlanReminderDemo(): Promise<void> {
  stopPlanReminderSequence();
  return playGentleChime(reminderNotes);
}

/** Stop any in-progress reminder chime when the reader opens its related plan or sound is muted. */
export function stopPlanReminderSequence(): void {
  sequenceVersion += 1;
  if (typeof window === 'undefined') return;
  for (const timer of busCleanupTimers.values()) window.clearTimeout(timer);
  busCleanupTimers.clear();
  if (audioContext?.state === 'running') {
    for (const bus of activeBuses) {
      try {
        bus.gain.cancelScheduledValues(audioContext.currentTime);
        bus.gain.setTargetAtTime(0.0001, audioContext.currentTime, 0.035);
      } catch { /* The audio context may be shutting down. */ }
      try { bus.disconnect(); } catch { /* Safe when a bus was already released. */ }
    }
  }
  activeBuses.clear();
}
