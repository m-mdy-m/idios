const PREFERENCE_PREFIX = ['idios', 'notifications'].join('.');
const SOUND_KEY = `${PREFERENCE_PREFIX}.sound.v1`;
const VOLUME_KEY = `${PREFERENCE_PREFIX}.sound-volume.v2`;
let audioContext: AudioContext | null = null;
let cachedNoise: AudioBuffer | null = null;
let sequenceVersion = 0;
let sequenceTimers: number[] = [];
let speechTimer: number | null = null;
const activeAlarmBuses = new Set<GainNode>();

export function notificationSoundEnabled(): boolean {
  try { return localStorage.getItem(SOUND_KEY) !== 'false'; }
  catch { return true; }
}

export function setNotificationSoundEnabled(enabled: boolean): void {
  try { localStorage.setItem(SOUND_KEY, String(enabled)); } catch { /* Preference still applies for this session. */ }
  if (!enabled) stopPlanReminderSequence();
  window.dispatchEvent(new CustomEvent('idios:notification-sound-updated', { detail: enabled }));
}

export function notificationSoundVolume(): number {
  try {
    const raw = localStorage.getItem(VOLUME_KEY);
    if (raw === null || raw.trim() === '') return 1;
    const stored = Number(raw);
    return Number.isFinite(stored) ? Math.min(1, Math.max(0, stored)) : 1;
  } catch { return 1; }
}

export function setNotificationSoundVolume(volume: number): void {
  const normalized = Number.isFinite(volume) ? Math.min(1, Math.max(0, volume)) : 1;
  try { localStorage.setItem(VOLUME_KEY, String(normalized)); } catch { /* Keep the current session preference. */ }
  window.dispatchEvent(new CustomEvent('idios:notification-volume-updated', { detail: normalized }));
}

export async function activateNotificationAudio(): Promise<boolean> {
  if (typeof window === 'undefined') return false;
  const AudioContextCtor = window.AudioContext;
  if (!AudioContextCtor) return false;
  try {
    audioContext ??= new AudioContextCtor();
    if (audioContext.state === 'suspended') await audioContext.resume();
    return audioContext.state === 'running';
  } catch { return false; }
}

function noiseBuffer(context: AudioContext): AudioBuffer {
  if (cachedNoise && cachedNoise.sampleRate === context.sampleRate) return cachedNoise;
  const length = Math.ceil(context.sampleRate * 3.2);
  const buffer = context.createBuffer(1, length, context.sampleRate);
  const channel = buffer.getChannelData(0);
  let brown = 0;
  for (let i = 0; i < length; i += 1) {
    const white = Math.random() * 2 - 1;
    brown = (brown + 0.035 * white) / 1.035;
    channel[i] = Math.max(-1, Math.min(1, brown * 7));
  }
  cachedNoise = buffer;
  return buffer;
}

function createAlarmBus(context: AudioContext, volume: number): GainNode {
  const master = context.createGain();
  const shaper = context.createWaveShaper();
  const compressor = context.createDynamicsCompressor();
  const curve = new Float32Array(1024);
  for (let i = 0; i < curve.length; i += 1) {
    const x = (i * 2) / (curve.length - 1) - 1;
    curve[i] = Math.tanh(x * 3.2) / Math.tanh(3.2);
  }
  shaper.curve = curve;
  shaper.oversample = '4x';
  compressor.threshold.setValueAtTime(-20, context.currentTime);
  compressor.knee.setValueAtTime(5, context.currentTime);
  compressor.ratio.setValueAtTime(12, context.currentTime);
  compressor.attack.setValueAtTime(0.002, context.currentTime);
  compressor.release.setValueAtTime(0.15, context.currentTime);
  // Keep peaks controlled even at full preference volume; the deliberate grit comes from distortion.
  master.gain.setValueAtTime(0.78 * volume, context.currentTime);
  master.connect(shaper);
  shaper.connect(compressor);
  compressor.connect(context.destination);
  return master;
}

function scheduleTone(
  context: AudioContext,
  output: AudioNode,
  start: number,
  duration: number,
  frequency: number,
  peak: number,
  type: OscillatorType,
  endFrequency?: number,
): void {
  const oscillator = context.createOscillator();
  const envelope = context.createGain();
  oscillator.type = type;
  oscillator.frequency.setValueAtTime(Math.max(1, frequency), start);
  if (endFrequency && endFrequency > 0) oscillator.frequency.exponentialRampToValueAtTime(endFrequency, start + duration);
  envelope.gain.setValueAtTime(0.0001, start);
  envelope.gain.linearRampToValueAtTime(peak, start + Math.min(0.012, duration * 0.2));
  envelope.gain.setValueAtTime(peak, Math.max(start + 0.014, start + duration * 0.36));
  envelope.gain.exponentialRampToValueAtTime(0.0001, start + duration);
  oscillator.connect(envelope);
  envelope.connect(output);
  oscillator.start(start);
  oscillator.stop(start + duration + 0.015);
}

function scheduleNoise(
  context: AudioContext,
  output: AudioNode,
  start: number,
  duration: number,
  peak: number,
  lowpassHz: number,
  highpassHz = 0,
): void {
  const source = context.createBufferSource();
  source.buffer = noiseBuffer(context);
  const lowpass = context.createBiquadFilter();
  lowpass.type = 'lowpass';
  lowpass.frequency.setValueAtTime(Math.max(100, lowpassHz), start);
  lowpass.Q.setValueAtTime(0.7, start);
  let lastNode: AudioNode = source;
  if (highpassHz > 0) {
    const highpass = context.createBiquadFilter();
    highpass.type = 'highpass';
    highpass.frequency.setValueAtTime(highpassHz, start);
    highpass.Q.setValueAtTime(0.65, start);
    lastNode.connect(highpass);
    lastNode = highpass;
  }
  lastNode.connect(lowpass);
  const envelope = context.createGain();
  envelope.gain.setValueAtTime(0.0001, start);
  envelope.gain.linearRampToValueAtTime(peak, start + Math.min(0.006, duration * 0.15));
  envelope.gain.setValueAtTime(peak * 0.62, start + duration * 0.18);
  envelope.gain.exponentialRampToValueAtTime(0.0001, start + duration);
  lowpass.connect(envelope);
  envelope.connect(output);
  source.start(start);
  source.stop(start + duration + 0.02);
}

function scheduleImpact(context: AudioContext, output: AudioNode, start: number, intensity: number): void {
  // Original synthesized action-movie impact: low body hit + short gritty air burst.
  scheduleTone(context, output, start, 0.34, 118, 0.48 * intensity, 'sine', 34);
  scheduleTone(context, output, start + 0.004, 0.17, 56, 0.22 * intensity, 'square', 29);
  scheduleNoise(context, output, start, 0.28, 0.43 * intensity, 620, 35);
}

function scheduleCrack(context: AudioContext, output: AudioNode, start: number, intensity: number): void {
  // A stylized, synthetic action crack; this is generated sound, not a firearm recording.
  scheduleNoise(context, output, start, 0.085, 0.52 * intensity, 7600, 1050);
  scheduleTone(context, output, start + 0.002, 0.11, 230, 0.18 * intensity, 'triangle', 76);
  scheduleTone(context, output, start + 0.035, 0.2, 2200, 0.12 * intensity, 'sawtooth', 640);
}

function scheduleRicochet(context: AudioContext, output: AudioNode, start: number, intensity: number): void {
  scheduleTone(context, output, start, 0.31, 1900, 0.13 * intensity, 'sawtooth', 420);
  scheduleTone(context, output, start + 0.025, 0.24, 1180, 0.1 * intensity, 'triangle', 2750);
  scheduleNoise(context, output, start + 0.03, 0.09, 0.1 * intensity, 6200, 2500);
}

function scheduleKlaxon(context: AudioContext, output: AudioNode, start: number, duration: number, low: number, high: number, intensity: number): void {
  const oscillator = context.createOscillator();
  const envelope = context.createGain();
  oscillator.type = 'sawtooth';
  oscillator.frequency.setValueAtTime(low, start);
  oscillator.frequency.linearRampToValueAtTime(high, start + duration * 0.4);
  oscillator.frequency.linearRampToValueAtTime(low * 0.82, start + duration);
  envelope.gain.setValueAtTime(0.0001, start);
  envelope.gain.linearRampToValueAtTime(0.24 * intensity, start + 0.018);
  envelope.gain.setValueAtTime(0.2 * intensity, start + duration * 0.6);
  envelope.gain.exponentialRampToValueAtTime(0.0001, start + duration);
  oscillator.connect(envelope);
  envelope.connect(output);
  oscillator.start(start);
  oscillator.stop(start + duration + 0.015);
  scheduleTone(context, output, start, duration, low * 0.5, 0.11 * intensity, 'square', high * 0.68);
}

function speakAlarmLine(stage: 2 | 3, volume: number, token: number): void {
  if (typeof window === 'undefined' || !('speechSynthesis' in window)) return;
  const speech = window.speechSynthesis;
  const voices = speech.getVoices();
  const persianVoice = voices.find((voice) => /^fa(?:-|$)/i.test(voice.lang));
  const utterance = new SpeechSynthesisUtterance();
  if (persianVoice) {
    utterance.text = stage === 2
      ? 'آی‌دیوس! پلان هنوز انجام نشده! همین حالا شروع کن!'
      : 'آی‌دیوس! هشدار نهایی! وقت تلف نکن؛ همین الان پلان را انجام بده!';
    utterance.lang = persianVoice.lang;
    utterance.voice = persianVoice;
  } else {
    utterance.text = stage === 2
      ? 'IDIOS! Plan check! Your task is still waiting. Get moving!'
      : 'IDIOS! Final plan warning! Stop stalling and start the task NOW!';
    const englishVoices = voices.filter((voice) => /^en(?:-|$)/i.test(voice.lang));
    const lowerVoice = englishVoices.find((voice) => /david|mark|guy|daniel|alex|james|aaron|male/i.test(voice.name));
    const englishVoice = lowerVoice || englishVoices.find((voice) => /^en-US/i.test(voice.lang)) || englishVoices[0];
    if (englishVoice) { utterance.voice = englishVoice; utterance.lang = englishVoice.lang; }
    else utterance.lang = 'en-US';
  }
  // Low pitch + brisk delivery is an original urgent announcer style, not a copied film character.
  utterance.rate = stage === 3 ? 1.22 : 1.14;
  utterance.pitch = stage === 3 ? 0.38 : 0.48;
  utterance.volume = Math.min(1, Math.max(0, volume));
  speech.cancel();
  if (token !== sequenceVersion) return;
  speech.speak(utterance);
}

async function playAlarmStage(stage: 1 | 2 | 3, token: number, includeSpeech: boolean): Promise<void> {
  if (!notificationSoundEnabled()) return;
  const ready = await activateNotificationAudio();
  if (!ready || !audioContext || token !== sequenceVersion) return;
  const context = audioContext;
  const volume = notificationSoundVolume();
  if (volume <= 0) return;
  const bus = createAlarmBus(context, volume);
  activeAlarmBuses.add(bus);
  const start = context.currentTime + 0.025;

  if (stage === 1) {
    // First alarm: a recognisable industrial klaxon followed by a short impact.
    scheduleKlaxon(context, bus, start, 0.26, 240, 670, 0.85);
    scheduleKlaxon(context, bus, start + 0.34, 0.27, 260, 720, 0.9);
    scheduleKlaxon(context, bus, start + 0.72, 0.42, 205, 810, 1);
    scheduleNoise(context, bus, start + 1.0, 0.22, 0.23, 4200, 700);
    scheduleImpact(context, bus, start + 1.02, 0.62);
  } else if (stage === 2) {
    // Second alarm: more urgency, layered claxon and three sharp action cracks.
    scheduleKlaxon(context, bus, start, 0.38, 190, 790, 1.08);
    scheduleKlaxon(context, bus, start + 0.46, 0.36, 235, 870, 1.05);
    scheduleCrack(context, bus, start + 0.16, 0.82);
    scheduleCrack(context, bus, start + 0.59, 0.9);
    scheduleImpact(context, bus, start + 0.9, 0.72);
    scheduleCrack(context, bus, start + 1.12, 0.94);
    scheduleRicochet(context, bus, start + 1.34, 0.8);
  } else {
    // Final alarm: deliberately busy, stylised action-sequence sound design.
    scheduleKlaxon(context, bus, start, 0.55, 170, 920, 1.12);
    scheduleImpact(context, bus, start + 0.02, 0.9);
    [0.12, 0.34, 0.68, 0.81, 1.04, 1.29, 1.42, 1.77, 2.03].forEach((offset, index) => {
      scheduleCrack(context, bus, start + offset, index % 3 === 0 ? 1.05 : 0.82);
    });
    scheduleImpact(context, bus, start + 0.61, 0.95);
    scheduleImpact(context, bus, start + 1.52, 1.08);
    scheduleRicochet(context, bus, start + 0.9, 1);
    scheduleRicochet(context, bus, start + 1.86, 0.88);
    scheduleKlaxon(context, bus, start + 1.5, 0.62, 220, 1080, 1.05);
    scheduleNoise(context, bus, start + 2.18, 0.44, 0.33, 720, 25);
    scheduleTone(context, bus, start + 2.18, 0.5, 94, 0.22, 'sine', 28);
  }

  if (includeSpeech && (stage === 2 || stage === 3)) {
    if (speechTimer !== null) window.clearTimeout(speechTimer);
    speechTimer = window.setTimeout(() => {
      speechTimer = null;
      if (token === sequenceVersion && notificationSoundEnabled()) speakAlarmLine(stage, volume, token);
    }, stage === 3 ? 2350 : 1650);
  }
  const duration = stage === 1 ? 1.65 : stage === 2 ? 1.9 : 2.9;
  window.setTimeout(() => { try { bus.disconnect(); } catch { /* Browser may already have closed the audio context. */ } activeAlarmBuses.delete(bus); }, duration * 1000 + 500);
}

/** General inbox alerts use the signature IDIOS klaxon. */
export async function playNotificationChime(): Promise<void> {
  if (!notificationSoundEnabled()) return;
  const token = sequenceVersion;
  await playAlarmStage(1, token, false);
}

/**
 * Plays a plan reminder when it arrives, repeats at +5 seconds, then gives the
 * final noisy/action-style warning 15 seconds after repeat two (t=+20 seconds).
 * The browser must have been unlocked by a user gesture; see activateNotificationAudio().
 */
export function playPlanReminderSequence(): void {
  if (!notificationSoundEnabled()) return;
  stopPlanReminderSequence();
  const token = sequenceVersion;
  void playAlarmStage(1, token, false);
  sequenceTimers.push(window.setTimeout(() => {
    if (token !== sequenceVersion) return;
    void playAlarmStage(2, token, true);
    sequenceTimers.push(window.setTimeout(() => {
      if (token !== sequenceVersion) return;
      void playAlarmStage(3, token, true);
    }, 15000));
  }, 5000));
}

/** Preview the loudest alert immediately from Settings. */
export function playPlanReminderDemo(): Promise<void> {
  if (!notificationSoundEnabled()) return Promise.resolve();
  stopPlanReminderSequence();
  const token = sequenceVersion;
  return playAlarmStage(3, token, true);
}

/** Stop pending repeats when the user opens the plan reminder or mutes notifications. */
export function stopPlanReminderSequence(): void {
  sequenceVersion += 1;
  if (audioContext?.state === 'running') {
    for (const bus of activeAlarmBuses) {
      try {
        bus.gain.cancelScheduledValues(audioContext.currentTime);
        bus.gain.setTargetAtTime(0.0001, audioContext.currentTime, 0.025);
      } catch { /* The context may be shutting down. */ }
    }
  }
  for (const timer of sequenceTimers) window.clearTimeout(timer);
  sequenceTimers = [];
  if (speechTimer !== null) { window.clearTimeout(speechTimer); speechTimer = null; }
  if (typeof window !== 'undefined' && 'speechSynthesis' in window) window.speechSynthesis.cancel();
}
