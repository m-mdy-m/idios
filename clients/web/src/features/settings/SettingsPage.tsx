import { useEffect, useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router';
import {
  BellRing, Check, ExternalLink, Palette, RefreshCw, Save,
  Server, Settings2, ShieldCheck, Sparkles, Download, ArrowUpRight,
  CalendarClock, Volume2, VolumeX, Play,
} from 'lucide-react';
import { api } from '@/lib/api';
import { Modal } from '@/design-system';
import type { AppConfig, ConfigPayload, HealthStatus } from '@/types/idios';
import { Badge, Button, Card, Field, LoadingState, PageHeading } from '@/design-system';
import { activateNotificationAudio, notificationSoundEnabled, notificationSoundVolume, playPlanReminderDemo, setNotificationSoundEnabled, setNotificationSoundVolume } from '@/lib/notification-sound';
import { PrivatePath } from '@/components/privacy/PrivatePath';

type SettingsSection = 'appearance' | 'workspace' | 'notifications' | 'updates';

const BUILTIN_WEB_THEMES = [
  { id: 'dark', name: 'Classic', description: 'Graphite, muted terminal green, no glow.' },
  { id: 'midnight', name: 'Midnight', description: 'Cool blue-black surfaces.' },
  { id: 'oled', name: 'OLED', description: 'Near-black, high contrast.' },
  { id: 'forest', name: 'Forest', description: 'Quiet green tones.' },
  { id: 'violet', name: 'Violet', description: 'Low-light violet accents.' },
  { id: 'rose', name: 'Rose', description: 'Muted rose on charcoal.' },
  { id: 'light', name: 'Light', description: 'A restrained light workspace.' },
  { id: 'system', name: 'System', description: 'Follow the operating-system preference.' },
] as const;

type QuoteReminderSettings = {
  enabled?: boolean;
  desktop_enabled?: boolean;
  min_interval_minutes?: number;
  max_interval_minutes?: number;
  next_at?: string;
  last_quote_id?: string;
  [key: string]: unknown;
};

export function SettingsPage({ focusSection }: { focusSection?: 'notifications' | 'updates' } = {}) {
  const navigate = useNavigate();
  const initialSection: SettingsSection = focusSection === 'notifications' ? 'notifications' : focusSection === 'updates' ? 'updates' : 'appearance';
  const [activeSection, setActiveSection] = useState<SettingsSection>(initialSection);
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [meta, setMeta] = useState<ConfigPayload | null>(null);
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testingReminder, setTestingReminder] = useState(false);
  const [soundEnabled, setSoundEnabled] = useState(() => notificationSoundEnabled());
  const [soundVolume, setSoundVolume] = useState(() => notificationSoundVolume());
  const [testingSound, setTestingSound] = useState(false);
  const [browserPermission, setBrowserPermission] = useState<'default' | 'granted' | 'denied' | 'unsupported'>(() => typeof Notification === 'undefined' ? 'unsupported' : Notification.permission);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [restartRequired, setRestartRequired] = useState(false);
  const [updateCheck, setUpdateCheck] = useState<Awaited<ReturnType<typeof api.checkUpdate>> | null>(null);
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [confirmUpdate, setConfirmUpdate] = useState(false);
  const [installingUpdate, setInstallingUpdate] = useState(false);
  const [updateInProgress, setUpdateInProgress] = useState(false);

  async function load() {
    setLoading(true);
    setError('');
    try {
      const [payload, nextHealth] = await Promise.all([api.config(), api.health()]);
      setMeta(payload);
      setConfig(payload.config);
      setHealth(nextHealth);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to load settings.');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void load(); }, []);
  useEffect(() => {
    if (!focusSection) return;
    setActiveSection(focusSection);
  }, [focusSection]);
  useEffect(() => {
    if (!updateInProgress) return;
    let cancelled = false;
    let timer: number | undefined;
    let attempts = 0;
    const poll = async () => {
      attempts += 1;
      try {
        const [status, currentHealth] = await Promise.all([api.updateStatus(), api.health()]);
        if (cancelled) return;
        if (updateCheck?.release?.version && currentHealth.version === updateCheck.release.version) {
          setHealth(currentHealth);
          setNotice(`IDIOS ${currentHealth.version} is installed. The local server has restarted successfully.`);
          setError('');
          setUpdateInProgress(false);
          setUpdateCheck(null);
          return;
        }
        if (status.status === 'failed') {
          setError(status.error || 'The update could not be prepared. Your existing installation was kept.');
          setNotice('');
          setUpdateInProgress(false);
          return;
        }
        if (status.status === 'scheduled') {
          setNotice(`IDIOS ${status.version || updateCheck?.release?.version || ''} is verified. Waiting for the local service to restart…`);
        }
      } catch {
        // Expected for a short time while the verified updater replaces and restarts the binary.
        setNotice('The local server is restarting. Waiting for IDIOS to come back online…');
      }
      if (attempts >= 180) {
        setError('The local server did not reconnect after the update. Check the IDIOS process, then run `idios serve` if needed.');
        setNotice('');
        setUpdateInProgress(false);
        return;
      }
      if (!cancelled) timer = window.setTimeout(() => void poll(), 1500);
    };
    void poll();
    return () => { cancelled = true; if (timer !== undefined) window.clearTimeout(timer); };
  }, [updateInProgress, updateCheck?.release?.version]);


  function patchConfig(patch: Partial<AppConfig>) {
    const next = config ? { ...config, ...patch } : null;
    setConfig(next);
    if (next && patch.web) window.dispatchEvent(new CustomEvent('idios:config-updated', { detail: next }));
    setSaved(false);
    setNotice('');
  }

  function patchQuoteReminders(patch: Partial<QuoteReminderSettings>, reschedule = false) {
    if (!config) return;
    const current = config.notifications.quote_reminders as QuoteReminderSettings;
    const next = { ...current, ...patch };
    if (reschedule) next.next_at = '';
    patchConfig({ notifications: { ...config.notifications, quote_reminders: next } });
  }

  function patchReminderRange(kind: 'min_interval_minutes' | 'max_interval_minutes', value: number) {
    if (!config) return;
    const current = config.notifications.quote_reminders as QuoteReminderSettings;
    const min = kind === 'min_interval_minutes' ? value : Number(current.min_interval_minutes ?? 60);
    const max = kind === 'max_interval_minutes' ? value : Number(current.max_interval_minutes ?? 240);
    patchQuoteReminders({
      min_interval_minutes: Math.min(min, max),
      max_interval_minutes: Math.max(min, max),
    }, true);
  }

  function patchPlanReminders(patch: Partial<AppConfig['notifications']['plan_reminders']>) {
    if (!config) return;
    patchConfig({ notifications: { ...config.notifications, plan_reminders: { ...config.notifications.plan_reminders, ...patch } } });
  }

  async function testNotificationSound() {
    setTestingSound(true);
    try {
      await activateNotificationAudio();
      await playPlanReminderDemo();
      if (!soundEnabled) setNotice('Notification sound is muted. Turn it on to preview the chime.');
      else setNotice('Played the gentle IDIOS notification chime.');
    } finally { setTestingSound(false); }
  }

  function toggleNotificationSound(enabled: boolean) {
    setSoundEnabled(enabled);
    setNotificationSoundEnabled(enabled);
    if (enabled) void activateNotificationAudio();
  }

  async function requestBrowserNotifications() {
    if (typeof Notification === 'undefined') { setBrowserPermission('unsupported'); setNotice('This browser does not support desktop notifications.'); return; }
    try { const permission = await Notification.requestPermission(); setBrowserPermission(permission); setNotice(permission === 'granted' ? 'Browser notifications are enabled.' : permission === 'denied' ? 'Browser notifications were blocked. Allow them from this site’s browser settings.' : 'Browser notification permission was not changed.'); }
    catch { setNotice('Could not request browser notification permission.'); }
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!config) return;
    setSaving(true);
    setSaved(false);
    setError('');
    try {
      const nextConfig = config;
      const result = await api.updateConfig(nextConfig);
      setConfig(nextConfig);
      setSaved(true);
      setRestartRequired(Boolean(result.restart_required));
      window.dispatchEvent(new CustomEvent('idios:config-updated', { detail: nextConfig }));
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to save settings.');
    } finally {
      setSaving(false);
    }
  }

  async function checkUpdates() {
    setCheckingUpdate(true);
    setError('');
    setNotice('');
    try {
      const result = await api.checkUpdate();
      setUpdateCheck(result);
      setNotice(result.update_available ? `IDIOS ${result.release?.version ?? ''} is available.` : 'IDIOS is up to date.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to check for IDIOS updates. Check your internet connection and try again.');
    } finally {
      setCheckingUpdate(false);
    }
  }

  async function installUpdate() {
    setInstallingUpdate(true);
    setError('');
    setNotice('Preparing a verified update. IDIOS will briefly restart the local service; this browser tab should reconnect automatically.');
    try {
      await api.installUpdate();
      setConfirmUpdate(false);
      setUpdateInProgress(true);
      setNotice('Update started. IDIOS will verify the release bundle and restart the local service.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to prepare the update. Your current installation has been kept.');
      setNotice('');
    } finally {
      setInstallingUpdate(false);
    }
  }

  async function sendTestQuoteReminder() {
    setTestingReminder(true);
    setError('');
    setNotice('');
    try {
      const response = await api.executeCommand('quote.reminder.test');
      const result = response.result as { quote?: { text?: string; author?: string }; notification?: { title?: string; body?: string } };
      const title = result.notification?.title || 'IDIOS · Quote reminder test';
      const body = result.notification?.body || result.quote?.text || 'Test reminder saved to your IDIOS inbox.';
      if ('Notification' in window) {
        let permission = Notification.permission;
        if (permission === 'default') permission = await Notification.requestPermission();
        if (permission === 'granted') new Notification(title, { body });
        else setNotice('The test was saved to the IDIOS inbox. Browser notifications are not permitted.');
      } else {
        setNotice('The test was saved to the IDIOS inbox. This browser does not support desktop notifications.');
      }
      setNotice((current) => current || 'Test reminder created. Check Notifications to confirm it arrived.');
      window.dispatchEvent(new CustomEvent('idios:notifications-updated'));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to create a test quote reminder.');
    } finally {
      setTestingReminder(false);
    }
  }

  if (loading && !config) return <LoadingState />;
  if (!config) return <div className="page-stack"><div className="notice notice--error">{error || 'Settings are unavailable.'}</div><Button onClick={() => void load()}>Try again</Button></div>;

  const reminder = config.notifications.quote_reminders as QuoteReminderSettings;
  const sectionInfo: Record<SettingsSection, { title: string; description: string }> = {
    appearance: { title: 'Theme & appearance', description: 'Choose a quiet palette and tune the density of your workspace.' },
    workspace: { title: 'Workspace & server', description: 'Control the local service and how often the workspace refreshes.' },
    notifications: { title: 'Notifications', description: 'Choose which reminders IDIOS creates and when they are delivered.' },
    updates: { title: 'Application updates', description: 'Check for a release and install a verified update.' },
  };
  return <div className="page-stack settings-page">
    <PageHeading eyebrow="PREFERENCES" title="Settings" description="Configure the local Web workspace and Go service." actions={<Button variant="secondary" onClick={() => void load()}><RefreshCw size={15} /> Reload</Button>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    {notice && <div className="notice notice--success" role="status"><Check size={16} /> {notice}</div>}
    {saved && <div className="notice notice--success" role="status"><Check size={16} /> Settings saved to your local config file.{restartRequired ? ' Restart the server to apply the new listener address.' : ''}</div>}

    <form onSubmit={save} className="settings-layout">
      <nav className="settings-nav" aria-label="Settings categories">
        <p className="settings-nav__label">Preferences</p>
        <button type="button" className="settings-nav__item" onClick={() => navigate('/settings/ai')}><Sparkles size={16} /><span>AI Runtime</span><ArrowUpRight size={13} /></button>
        <button type="button" className={activeSection === 'appearance' ? 'settings-nav__item settings-nav__item--active' : 'settings-nav__item'} onClick={() => setActiveSection('appearance')} aria-current={activeSection === 'appearance' ? 'page' : undefined}><Palette size={16} /><span>Theme</span></button>
        <button type="button" className={activeSection === 'workspace' ? 'settings-nav__item settings-nav__item--active' : 'settings-nav__item'} onClick={() => setActiveSection('workspace')} aria-current={activeSection === 'workspace' ? 'page' : undefined}><Server size={16} /><span>Workspace</span></button>
        <button type="button" className={activeSection === 'notifications' ? 'settings-nav__item settings-nav__item--active' : 'settings-nav__item'} onClick={() => setActiveSection('notifications')} aria-current={activeSection === 'notifications' ? 'page' : undefined}><BellRing size={16} /><span>Notifications</span></button>
        <button type="button" className={activeSection === 'updates' ? 'settings-nav__item settings-nav__item--active' : 'settings-nav__item'} onClick={() => setActiveSection('updates')} aria-current={activeSection === 'updates' ? 'page' : undefined}><Download size={16} /><span>Updates</span></button>
        <div className="settings-nav__divider" />
        <div className="settings-nav__hint"><ShieldCheck size={15} /><span>Settings are saved to your local IDIOS config. Theme, layout, and browser preferences are stored locally in your IDIOS config.</span></div>
      </nav>
      <div className="settings-main">
        <div className="settings-active-heading"><h2>{sectionInfo[activeSection].title}</h2><p>{sectionInfo[activeSection].description}</p></div>
        {activeSection === 'appearance' && <>
        <Card className="settings-card">
          <div className="settings-card__heading"><span className="settings-heading-icon"><Palette size={17} /></span><div><h2>Color palette</h2><p>An old-school workstation aesthetic: dark surfaces, restrained contrast and no bright decorative colors.</p></div></div>
          <div className="settings-section">
            <div className="theme-options theme-options--extended" role="group" aria-label="Color themes">
              {BUILTIN_WEB_THEMES.map((item) => {
                const active = (config.web.theme || 'dark') === item.id;
                return <button
                  type="button"
                  key={item.id}
                  className={active ? 'theme-option theme-option--selected' : 'theme-option'}
                  aria-pressed={active}
                  onClick={() => patchConfig({ web: { ...config.web, theme: item.id } })}
                >
                  <span className={`theme-preview theme-preview--${item.id}`} aria-hidden="true"><span className="theme-preview__accent" /><span className="theme-preview__surface" /></span>
                  <strong>{item.name}</strong>
                  <small>{item.description}</small>
                </button>;
              })}
              {Object.entries(config.web.custom_themes ?? {}).map(([id, item]) => {
                const active = config.web.theme === id;
                return <button
                  type="button"
                  key={id}
                  className={active ? 'theme-option theme-option--selected' : 'theme-option'}
                  aria-pressed={active}
                  onClick={() => patchConfig({ web: { ...config.web, theme: id } })}
                >
                  <span className="theme-preview" style={{ background: item.background, color: item.accent }} aria-hidden="true"><span className="theme-preview__accent" /><span className="theme-preview__surface" /></span>
                  <strong>{item.name}</strong>
                  <small>Custom palette</small>
                </button>;
              })}
            </div>
            <p className="settings-muted-copy">The selected palette applies immediately. Save changes to keep it in your IDIOS configuration.</p>

            <ToggleSetting title="Compact density" description="Fit more rows and activity into the same viewport." checked={config.web.compact} onChange={(value) => patchConfig({ web: { ...config.web, compact: value } })} />
            <ToggleSetting title="Collapse sidebar by default" description="Keep more space for content on smaller screens." checked={config.web.sidebar_collapsed} onChange={(value) => patchConfig({ web: { ...config.web, sidebar_collapsed: value } })} />
          </div>
        </Card>

        </>}


        {activeSection === 'workspace' && <Card className="settings-card">
          <div className="settings-card__heading"><span className="settings-heading-icon settings-heading-icon--violet"><Server size={17} /></span><div><h2>Local server</h2><p>The web interface and API run inside the Go process.</p></div><Badge tone="green">Loopback only</Badge></div>
          <div className="settings-section">
            <div className="settings-field-grid">
              <Field label="Host" hint="Must remain a loopback address."><select value={config.server.host} onChange={(event) => patchConfig({ server: { ...config.server, host: event.target.value } })}><option value="127.0.0.1">127.0.0.1</option><option value="localhost">localhost</option><option value="::1">::1</option></select></Field>
              <Field label="Port" hint="Restart the service after changing it."><input type="number" min={1024} max={65535} value={config.server.port} onChange={(event) => patchConfig({ server: { ...config.server, port: Number(event.target.value) } })} required /></Field>
            </div>
            <ToggleSetting title="Open browser on startup" description="Open the local workspace when the server starts interactively." checked={config.server.open_browser} onChange={(value) => patchConfig({ server: { ...config.server, open_browser: value } })} />
            <Field label="Workspace refresh interval" hint="How often the web dashboard refreshes from the local Engine."><select value={config.server.refresh_interval_seconds} onChange={(event) => patchConfig({ server: { ...config.server, refresh_interval_seconds: Number(event.target.value) } })}><option value={10}>10 seconds</option><option value={15}>15 seconds</option><option value={30}>30 seconds</option><option value={60}>1 minute</option><option value={300}>5 minutes</option></select></Field>
            <ToggleSetting title="Debug logging" description="Enable extra diagnostic events for troubleshooting." checked={config.debug} onChange={(value) => patchConfig({ debug: value })} />
          </div>
        </Card>}

        {activeSection === 'updates' && <Card id="settings-updates" className="settings-card">
          <div className="settings-card__heading"><span className="settings-heading-icon settings-heading-icon--green"><Download size={17} /></span><div><h2>Application updates</h2><p>Download and install a verified release of the CLI and embedded Web workspace.</p></div><Badge tone={updateCheck?.update_available ? 'blue' : 'neutral'}>{updateCheck?.update_available ? 'Update available' : 'Release channel'}</Badge></div>
          <div className="settings-section">
            <div className="update-current-version"><span>Installed version</span><strong>{health?.version || '—'}</strong></div>
            {updateCheck?.update_available && updateCheck.release ? <div className="update-release-panel">
              <div className="update-release-panel__heading"><div><p className="eyebrow">NEW RELEASE</p><h3>IDIOS {updateCheck.release.version}</h3></div><Badge tone="green">Ready</Badge></div>
              <details><summary>Release notes</summary><pre>{updateCheck.release.notes || 'No release notes were published.'}</pre></details>
              <div className="update-release-actions"><a href={updateCheck.release.url} target="_blank" rel="noreferrer">Release page <ArrowUpRight size={14} /></a><Button type="button" disabled={updateInProgress} onClick={() => setConfirmUpdate(true)}><Download size={15} /> Install and restart</Button></div>
            </div> : <p className="settings-muted-copy">{updateCheck ? 'No newer stable release was found.' : 'Check GitHub Releases for a newer stable version. The current installation stays in place until the downloaded bundle passes checksum verification.'}</p>}
            <div className="settings-reminder-actions"><Button type="button" variant="secondary" onClick={() => void checkUpdates()} loading={checkingUpdate}><RefreshCw size={15} /> Check for updates</Button></div>
          </div>
        </Card>}

        {activeSection === 'notifications' && <Card id="settings-notifications" className="settings-card">
          <div className="settings-card__heading"><span className="settings-heading-icon settings-heading-icon--green"><ShieldCheck size={17} /></span><div><h2>Notifications</h2><p>The Go service owns scheduled reminders, including when the browser is closed.</p></div></div>
          <div className="settings-section">
            <ToggleSetting title="Enable workflow notifications" description="Let the Engine create notifications for scheduled actions." checked={config.notifications.enabled} onChange={(value) => patchConfig({ notifications: { ...config.notifications, enabled: value } })} />
            <Field label="Scheduled-job check interval" hint="How frequently the local service checks for due jobs."><select value={config.notifications.poll_interval_minutes} onChange={(event) => patchConfig({ notifications: { ...config.notifications, poll_interval_minutes: Number(event.target.value) } })}><option value={1}>Every minute</option><option value={5}>Every 5 minutes</option><option value={15}>Every 15 minutes</option><option value={30}>Every 30 minutes</option><option value={60}>Every hour</option><option value={240}>Every 4 hours</option><option value={1440}>Every 24 hours</option></select></Field>

            <div className="settings-subsection-heading"><span className="settings-heading-icon"><CalendarClock size={16} /></span><div><h3>Daily plan check-ins</h3><p>Three gentle reminders for plans scheduled for the current local date. IDIOS only creates a reminder when that day has at least one plan; keep workflow notifications enabled above for these check-ins to run.</p></div></div>
            <ToggleSetting title="Enable daily plan reminders" description="Create morning, midday and evening reminders in your IDIOS inbox." checked={Boolean(config.notifications.plan_reminders?.enabled)} onChange={(value) => patchPlanReminders({ enabled: value })} />
            <ToggleSetting title="Send native desktop reminders" description="Ask the local IDIOS service to show an operating-system notification as well." checked={Boolean(config.notifications.plan_reminders?.desktop_enabled)} onChange={(value) => patchPlanReminders({ desktop_enabled: value })} />
            <div className="settings-field-grid settings-plan-reminder-times">
              <Field label="Morning" hint="Start the day"><input type="time" required value={config.notifications.plan_reminders?.morning_time || '08:30'} onChange={(event) => patchPlanReminders({ morning_time: event.target.value })} /></Field>
              <Field label="Midday" hint="A kind check-in"><input type="time" required value={config.notifications.plan_reminders?.midday_time || '13:00'} onChange={(event) => patchPlanReminders({ midday_time: event.target.value })} /></Field>
              <Field label="Evening" hint="Close the day"><input type="time" required value={config.notifications.plan_reminders?.evening_time || '20:30'} onChange={(event) => patchPlanReminders({ evening_time: event.target.value })} /></Field>
            </div>
            <div className="settings-subsection-heading"><span className="settings-heading-icon"><Volume2 size={16} /></span><div><h3>Notification sound</h3><p>New inbox updates play a short, warm three-note chime. Plan reminders use a slightly longer variation of the same sound. Audio is generated locally in your browser, stays gentle at every volume setting, and requires an open IDIOS tab after a user interaction.</p></div></div>
            <ToggleSetting title="Play a sound for new notifications" description="A soft, short chime for new inbox updates and plan reminders. This preference is saved in this browser." checked={soundEnabled} onChange={toggleNotificationSound} />
            <div className="settings-sound-volume">
              <label htmlFor="idios-notification-volume"><span>Alert volume</span><strong>{Math.round(soundVolume * 100)}%</strong></label>
              <input id="idios-notification-volume" type="range" min="0" max="1" step="0.05" value={soundVolume} onChange={(event) => { const next = Number(event.target.value); setSoundVolume(next); setNotificationSoundVolume(next); }} aria-label="Notification alert volume" />
              <small>A few soft bell-like notes, with no speech, alarm bursts, or sudden loud peaks.</small>
            </div>
            <div className="settings-reminder-actions settings-notification-actions"><Button type="button" variant="secondary" onClick={() => void testNotificationSound()} loading={testingSound}>{soundEnabled ? <Volume2 size={15} /> : <VolumeX size={15} />} <Play size={13}/> Preview chime</Button><Button type="button" variant="secondary" onClick={() => void requestBrowserNotifications()} disabled={browserPermission === 'granted' || browserPermission === 'unsupported'}><BellRing size={14}/>{browserPermission === 'granted' ? 'Browser alerts enabled' : browserPermission === 'denied' ? 'Browser alerts blocked' : browserPermission === 'unsupported' ? 'Browser alerts unavailable' : 'Allow browser alerts'}</Button><span>{soundEnabled ? 'Sound enabled in this browser' : 'Sound muted'}</span></div>

            <div className="settings-subsection-heading"><span className="settings-heading-icon"><Sparkles size={16} /></span><div><h3>Quote of the moment</h3><p>Random quote reminders are scheduled by the Go background service.</p></div></div>
            <ToggleSetting title="Enable random quote reminders" description="Save a quote to your IDIOS inbox at a random interval." checked={Boolean(reminder.enabled)} onChange={(value) => patchQuoteReminders({ enabled: value }, true)} />
            <ToggleSetting title="Send desktop notifications" description="Also send a native desktop notification when supported by the operating system." checked={Boolean(reminder.desktop_enabled)} onChange={(value) => patchQuoteReminders({ desktop_enabled: value })} />
            <div className="settings-field-grid">
              <Field label="Minimum interval (minutes)" hint="At least 1 minute."><input type="number" min={1} max={525600} value={Number(reminder.min_interval_minutes ?? 60)} onChange={(event) => patchReminderRange('min_interval_minutes', Math.max(1, Number(event.target.value) || 1))} required /></Field>
              <Field label="Maximum interval (minutes)" hint="Must be at least the minimum."><input type="number" min={1} max={525600} value={Number(reminder.max_interval_minutes ?? 240)} onChange={(event) => patchReminderRange('max_interval_minutes', Math.max(1, Number(event.target.value) || 1))} required /></Field>
            </div>
            <div className="settings-reminder-actions"><Button type="button" variant="secondary" onClick={() => void sendTestQuoteReminder()} loading={testingReminder}><BellRing size={15} /> Send test reminder</Button>{reminder.next_at && <span>Next reminder {new Date(String(reminder.next_at)).toLocaleString()}</span>}</div>
          </div>
        </Card>}
      </div>

      <aside className="settings-aside">
        <Card className="settings-status-card">
          <div className="settings-card__heading"><span className="settings-heading-icon"><Settings2 size={17} /></span><div><h2>Runtime status</h2><p>Current local process details.</p></div></div>
          <div className="runtime-status"><span className={`status-light ${health ? 'status-light--online' : ''}`} /><strong>{health ? 'Engine healthy' : 'Not connected'}</strong><Badge tone={health ? 'green' : 'red'}>{health?.status || 'offline'}</Badge></div>
          <div className="runtime-details"><div><span>Server URL</span><a href={health?.address || '#'} target="_blank" rel="noreferrer">{health?.address || 'Unavailable'} <ExternalLink size={12} /></a></div><div><span>Version</span><strong>{health?.version || '—'}</strong></div><div><span>Database</span><strong>{health?.database || '—'}</strong></div><div><span>Config file</span><PrivatePath value={meta?.config_path} /></div><div><span>Config directory</span><PrivatePath value={meta?.config_dir} /></div><div><span>Database file</span><PrivatePath value={meta?.db_path} /></div></div>
          <div className="settings-security-note"><ShieldCheck size={15} /><span>Bound to loopback only. IDIOS does not expose this API to your LAN by default.</span></div>
        </Card>
        <div className="settings-save-sticky"><Button type="submit" loading={saving} className="save-settings-button"><Save size={15} /> Save changes</Button><p>Changes are written to your shared IDIOS config.</p></div>
      </aside>
    </form>
    <Modal open={confirmUpdate} title={`Install IDIOS ${updateCheck?.release?.version ?? 'update'}?`} description="This replaces the IDIOS executable with the verified release. The local server will restart; your database, config and open browser tab are kept." onClose={() => { if (!installingUpdate) setConfirmUpdate(false); }} onSubmit={installUpdate} submitLabel={installingUpdate ? 'Preparing update…' : 'Verify, install & restart'} busy={installingUpdate} wide>
      <div className="update-confirm-panel"><ShieldCheck size={20} /><div><strong>Safe staged update</strong><p>The release archive is checked against the published SHA-256 checksum before the installed executable is replaced. If staging or replacement fails, the existing executable is restored where possible.</p><p>Keep this page open. It should reconnect after the local Go server restarts.</p></div></div>
    </Modal>
  </div>;
}


function ToggleSetting({ title, description, checked, onChange }: { title: string; description: string; checked: boolean; onChange: (value: boolean) => void }) {
  return <div className="toggle-setting"><div><strong>{title}</strong><p>{description}</p></div><button type="button" role="switch" aria-checked={checked} className={`toggle-switch ${checked ? 'toggle-switch--on' : ''}`} onClick={() => onChange(!checked)}><span /></button></div>;
}
