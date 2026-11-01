import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router';
import type { LucideIcon } from 'lucide-react';
import {
  Activity, ArrowRight, Bell, Blocks, BookOpen, LibraryBig, CalendarClock, CalendarCheck, CheckSquare, ChevronRight, CircleUserRound,
  Command, FolderKanban, GitBranch, GitPullRequest, Home, Menu, PanelLeftClose, PanelLeftOpen,
  Plus, Quote, Search, Settings2, Sparkles, Tags, X, BrainCircuit, ScrollText, Trash2, CircleHelp, UserRound, ChevronDown,
} from 'lucide-react';
import { useWorkspaceStore } from '@/store/workspace-store';
import { api } from '@/lib/api';
import type { AppConfig, DashboardMetrics, FeatureDefinition, ProtocolCommand } from '@/types/idios';
import { CommandRunner } from '@/features/commands';
import { activateNotificationAudio, notificationSoundEnabled, playNotificationChime, playPlanReminderSequence, stopPlanReminderSequence } from '@/lib/notification-sound';

type NavigationItem = {
  to: string
  label: string
  icon: LucideIcon
  count?: keyof DashboardMetrics
}
type NavigationGroup = { label: string; items: NavigationItem[] }

const hiddenModulePaths = new Set(['/issues', '/projects', '/pull-requests']);
const hiddenUtilityPaths = new Set(['/settings', '/notifications', '/help']);

const iconByName: Record<string, LucideIcon> = {
  activity: Activity, bell: Bell, blocks: Blocks, 'book-open': BookOpen, 'library-big': LibraryBig, 'calendar-check': CalendarCheck,
  'calendar-clock': CalendarClock, 'check-square': CheckSquare, 'circle-user-round': CircleUserRound,
  command: Command, 'folder-kanban': FolderKanban, 'git-branch': GitBranch,
  'git-pull-request': GitPullRequest, home: Home, quote: Quote, 'settings-2': Settings2,
  sparkles: Sparkles, tags: Tags, 'brain-circuit': BrainCircuit, 'scroll-text': ScrollText,
  'trash-2': Trash2, 'circle-help': CircleHelp,
};

function buildNavigation(features: FeatureDefinition[]): { main: NavigationGroup[]; bottom: NavigationItem[] } {
  const groups = new Map<string, NavigationItem[]>();
  const bottom: NavigationItem[] = [];
  const ordered = [...features].sort((a, b) => (a.web_group_order ?? 99) - (b.web_group_order ?? 99) || (a.web_order ?? 99) - (b.web_order ?? 99));
  for (const feature of ordered) {
    if (!feature.web_navigation || !feature.web_path) continue;
    const item: NavigationItem = {
      to: feature.web_path,
      label: feature.label,
      icon: iconByName[feature.icon] ?? CircleHelp,
      ...(['repositories', 'open_issues', 'projects', 'notes'].includes(feature.metric ?? '') ? { count: feature.metric as keyof DashboardMetrics } : {}),
    };
    if (feature.web_bottom_navigation || !feature.group) bottom.push(item);
    else {
      const group = groups.get(feature.group) ?? [];
      group.push(item);
      groups.set(feature.group, group);
    }
  }
  return { main: [...groups.entries()].map(([label, items]) => ({ label, items })), bottom };
}

export function AppShell() {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [allToolsOpen, setAllToolsOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({ Organize: true, Knowledge: true });
  const [appConfig, setAppConfig] = useState<AppConfig | null>(null);
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [quickCreateOpen, setQuickCreateOpen] = useState(false);
  const [profileMenuOpen, setProfileMenuOpen] = useState(false);
  const [protocolCommands, setProtocolCommands] = useState<ProtocolCommand[]>([]);
  const [featureCatalog, setFeatureCatalog] = useState<FeatureDefinition[]>([]);
  const [activeCommand, setActiveCommand] = useState<ProtocolCommand | null>(null);
  const [activeCommandValues, setActiveCommandValues] = useState<Record<string, unknown>>({});
  const [theme, setTheme] = useState<string>('dark');
  const seenNotificationIDs = useRef<Set<string> | null>(null);
  const appConfigRef = useRef<AppConfig | null>(null);
  const location = useLocation();
  const navigate = useNavigate();
  const { dashboard, health, load, lastUpdated } = useWorkspaceStore();

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    let active = true;
    void Promise.all([api.protocol(), api.features()]).then(([commands, catalog]) => {
      if (!active) return;
      setProtocolCommands(commands.commands);
      setFeatureCatalog(catalog.features);
    }).catch(() => undefined);
    return () => { active = false; };
  }, []);
  const nav = useMemo(() => buildNavigation(featureCatalog), [featureCatalog]);
  const primaryItems = useMemo<NavigationItem[]>(() => {
    const fromCatalog = new Map([...nav.main.flatMap((group) => group.items), ...nav.bottom].map((item) => [item.to, item]));
    const defaults: NavigationItem[] = [
      { to: '/', label: 'Overview', icon: Home },
      { to: '/shelf', label: 'Shelf', icon: LibraryBig },
      { to: '/repositories', label: 'Repos', icon: Blocks },

      { to: '/notes', label: 'Notes', icon: BookOpen },
      { to: '/activity', label: 'Activity', icon: Activity },
    ];
    return defaults.map((item) => ({ ...item, ...(fromCatalog.get(item.to) ?? {}), to: item.to, label: item.label, icon: item.icon, count: item.count }));
  }, [nav]);
  const primaryPaths = useMemo(() => new Set(primaryItems.map((item) => item.to)), [primaryItems]);
  const toolsGroups = useMemo(() => {
    const groups = nav.main
      .map((group) => ({ ...group, items: group.items.filter((item) => !primaryPaths.has(item.to) && !hiddenModulePaths.has(item.to)) }))
      .filter((group) => group.items.length > 0);
    const bottomExtras = nav.bottom.filter((item) => !primaryPaths.has(item.to) && !hiddenModulePaths.has(item.to) && !hiddenUtilityPaths.has(item.to));
    return bottomExtras.length ? [...groups, { label: 'Other', items: bottomExtras }] : groups;
  }, [nav, primaryPaths]);
  const applyConfig = useCallback((config: AppConfig) => {
    appConfigRef.current = config;
    setAppConfig(config);
    setCollapsed(config.web.sidebar_collapsed);
    setTheme('dark');
  }, []);
  useEffect(() => {
    let active = true;
    void api.config().then((payload) => { if (active) applyConfig(payload.config); }).catch(() => undefined);
    const onConfigUpdated = (event: Event) => {
      const detail = (event as CustomEvent<AppConfig>).detail;
      if (detail) applyConfig(detail);
    };
    window.addEventListener('idios:config-updated', onConfigUpdated);
    return () => { active = false; window.removeEventListener('idios:config-updated', onConfigUpdated); };
  }, [applyConfig]);
  const persistWeb = useCallback((patch: Partial<AppConfig['web']>) => {
    if (!appConfig) return;
    const next = { ...appConfig, web: { ...appConfig.web, ...patch } };
    window.dispatchEvent(new CustomEvent('idios:config-updated', { detail: next }));
    void api.updateConfig(next).catch(() => undefined);
  }, [appConfig]);
  const refreshInterval = Math.max(10, health?.refresh_interval_seconds ?? 30) * 1000;
  useEffect(() => {
    const timer = window.setInterval(() => void load(true), refreshInterval);
    return () => window.clearInterval(timer);
  }, [load, refreshInterval]);
  useEffect(() => { document.documentElement.dataset.theme = theme; }, [theme]);
  useEffect(() => {
    let active = true;
    const checkNotifications = async () => {
      try {
        const notifications = await api.notifications(50);
        if (!active) return;
        const currentIDs = new Set(notifications.map((item) => item.id));
        const previous = seenNotificationIDs.current;
        seenNotificationIDs.current = currentIDs;
        // Seed the known inbox on first load so opening IDIOS does not chime old notices.
        if (!previous) return;
        const fresh = notifications.filter((item) => !previous.has(item.id));
        if (!fresh.length) return;
        const hasPlanReminder = fresh.some((item) => item.related_entity_type === 'plan-date');
        if (notificationSoundEnabled()) {
          if (hasPlanReminder) playPlanReminderSequence();
          else void playNotificationChime();
        }
        // If native plan reminders are enabled, the Engine handles those. If a user has
        // disabled native delivery, permit the browser banner as the background fallback.
        const nativePlanReminders = Boolean(appConfigRef.current?.notifications.plan_reminders?.desktop_enabled);
        if (document.hidden && 'Notification' in window && Notification.permission === 'granted') {
          fresh.filter((item) => item.related_entity_type !== 'plan-date' || !nativePlanReminders).slice(0, 3).forEach((item) => {
            const banner = new Notification(item.title, { body: item.body, tag: item.id });
            banner.onclick = () => {
              if (item.related_entity_type === 'plan-date') stopPlanReminderSequence();
              window.focus();
              window.location.assign(item.related_entity_type === 'plan-date' && item.related_entity_id
                ? `/plans?date=${encodeURIComponent(item.related_entity_id)}` : '/notifications');
              banner.close();
            };
          });
        }
      } catch { /* The local server can restart; the next poll will recover. */ }
    };
    const activateAudio = () => { if (notificationSoundEnabled()) void activateNotificationAudio(); };
    const onNotificationsUpdated = () => { void checkNotifications(); };
    void checkNotifications();
    const timer = window.setInterval(() => void checkNotifications(), 15000);
    window.addEventListener('pointerdown', activateAudio, { passive: true });
    window.addEventListener('idios:notifications-updated', onNotificationsUpdated);
    return () => {
      active = false;
      window.clearInterval(timer);
      window.removeEventListener('pointerdown', activateAudio);
      window.removeEventListener('idios:notifications-updated', onNotificationsUpdated);
    };
  }, []);
  useEffect(() => {
    setSidebarOpen(false); setQuickCreateOpen(false); setProfileMenuOpen(false);
    if (location.pathname === '/plans' || location.pathname === '/notifications') stopPlanReminderSequence();
  }, [location.pathname]);
  useEffect(() => {
    const activeGroup = nav.main.find((group) => group.items.some((item) => item.to === location.pathname));
    if (activeGroup) setCollapsedGroups((current) => current[activeGroup.label] ? { ...current, [activeGroup.label]: false } : current);
    // Expand the new route's group on navigation, but do not undo a user's manual collapse.
  }, [location.pathname, nav.main]);
  useEffect(() => {
    const keydown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); setSearchOpen((v) => !v); setQuickCreateOpen(false); setProfileMenuOpen(false); }
      if ((event.metaKey || event.ctrlKey) && event.shiftKey && event.key.toLowerCase() === 'p') { event.preventDefault(); setSearchOpen(false); setQuickCreateOpen(false); setProfileMenuOpen(false); navigate('/commands'); }
      if (event.key === 'Escape') { setSearchOpen(false); setQuickCreateOpen(false); setProfileMenuOpen(false); setSidebarOpen(false); }
    };
    window.addEventListener('keydown', keydown);
    return () => window.removeEventListener('keydown', keydown);
  }, [navigate]);

  const searchable = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    const items = [
      ...((dashboard?.issues ?? []).map((item) => ({ label: item.title, detail: `${item.repository_name || 'Issue'} · #${item.number}`, to: '/issues' }))),
      ...((dashboard?.repositories ?? []).map((item) => ({ label: item.name, detail: 'Repository', to: '/repositories' }))),
      ...((dashboard?.projects ?? []).map((item) => ({ label: item.name, detail: 'Project', to: '/projects' }))),
      ...((dashboard?.notes ?? []).map((item) => ({ label: item.title, detail: 'Note', to: '/notes' }))),
    ];
    return query ? items.filter((item) => `${item.label} ${item.detail}`.toLowerCase().includes(query)).slice(0, 8) : items.slice(0, 6);
  }, [dashboard, searchQuery]);

  const quickCreateItems = useMemo(() => protocolCommands
    .filter((command) => command.id.endsWith('.create') || command.id.endsWith('.bulk_create'))
    .map((command) => ({ id: command.id, label: command.title, detail: command.description })), [protocolCommands]);
  function runQuickCreate(id: string) {
    const command = protocolCommands.find((item) => item.id === id);
    if (!command) return;
    setActiveCommandValues({});
    setActiveCommand(command);
    setQuickCreateOpen(false);
  }
  async function afterCommand() { await load(true); }

  function navCount(key?: keyof DashboardMetrics) {
    if (!key || !dashboard) return undefined;
    return dashboard.metrics[key] ?? 0;
  }

  const corePaths = new Set(['/', '/repositories', '/shelf', '/notes', '/activity']);
  const coreItems = primaryItems.filter((item) => corePaths.has(item.to));

  return <div className={`unix-workspace ${collapsed ? 'unix-workspace--collapsed' : ''}`}>
    {sidebarOpen && <button className="unix-backdrop" aria-label="Close navigation" onClick={() => setSidebarOpen(false)} />}
    <aside className={`unix-sidebar ${sidebarOpen ? 'unix-sidebar--open' : ''}`} aria-label="Workspace navigation">
      <div className="unix-brand-row">
        <NavLink to="/" className="unix-brand" aria-label="IDIOS overview" onClick={() => setSidebarOpen(false)}>
          <img
            className={`unix-brand-image${collapsed ? ' unix-brand-image--mark' : ''}`}
            src={collapsed ? '/brand/idios-mark.svg' : '/brand/idios-wordmark.svg'}
            alt=""
            draggable={false}
          />
        </NavLink>
        <button type="button" className="unix-icon-button unix-collapse" onClick={() => { const next = !collapsed; setCollapsed(next); persistWeb({ sidebar_collapsed: next }); }} aria-label={collapsed ? 'Expand navigation' : 'Collapse navigation'} title={collapsed ? 'Expand navigation' : 'Collapse navigation'}>
          {collapsed ? <PanelLeftOpen size={15} /> : <PanelLeftClose size={15} />}
        </button>
        <button type="button" className="unix-icon-button unix-mobile-close" onClick={() => setSidebarOpen(false)} aria-label="Close navigation"><X size={16} /></button>
      </div>

      {!collapsed && <div className="unix-environment"><span className={`unix-status ${health ? 'is-online' : ''}`} /><span>LOCAL WORKSPACE</span><code>{health?.version ? `v${health.version}` : 'DEV'}</code></div>}
      <button type="button" className="unix-search-trigger" onClick={() => { setSearchOpen(true); setQuickCreateOpen(false); setProfileMenuOpen(false); }} aria-label="Search IDIOS" title={collapsed ? "Search IDIOS" : "Find anything"}>
        <Search size={15} /><span>{collapsed ? 'Search' : 'Find anything'}</span>{!collapsed && <kbd>⌘ K</kbd>}
      </button>

      <nav className="unix-navigation" aria-label="Main navigation">
        {!collapsed && <p className="unix-section-label">Workspace</p>}
        {coreItems.map((item) => <NavLink key={item.to} to={item.to} end={item.to === '/'} onClick={() => setSidebarOpen(false)} title={item.label} className={({ isActive }) => `unix-nav-link ${isActive ? 'is-active' : ''}`}>
          <item.icon size={16} strokeWidth={1.75} /><span className="unix-nav-label">{item.to === '/' ? 'Overview' : item.label}</span>{!collapsed && item.count && navCount(item.count) !== undefined && <span className="unix-count">{navCount(item.count)}</span>}
        </NavLink>)}
        <div className="unix-nav-divider" />
        <button type="button" className={`unix-nav-link unix-module-trigger ${allToolsOpen ? 'is-active' : ''}`} onClick={() => setAllToolsOpen((value) => !value)} aria-expanded={allToolsOpen} title="All modules">
          <Blocks size={16} strokeWidth={1.75} /><span className="unix-nav-label">All modules</span>{!collapsed && <><span className="unix-count unix-count--muted">{toolsGroups.reduce((sum, group) => sum + group.items.length, 0)}</span><ChevronDown size={13} className={`unix-chevron ${allToolsOpen ? 'is-open' : ''}`} /></>}
        </button>
        {allToolsOpen && <div className="unix-module-tree">{toolsGroups.map((group) => <section className="unix-module-group" key={group.label}>
          {!collapsed && <button type="button" className="unix-group-toggle" aria-expanded={!collapsedGroups[group.label]} onClick={() => setCollapsedGroups((current) => ({ ...current, [group.label]: !current[group.label] }))}><span>{group.label}</span><ChevronDown size={12} className={collapsedGroups[group.label] ? '' : 'is-open'} /></button>}
          {(!collapsed && !collapsedGroups[group.label] || collapsed) && group.items.map((item) => <NavLink key={item.to} to={item.to} end={item.to === '/'} onClick={() => setSidebarOpen(false)} title={item.label} className={({ isActive }) => `unix-nav-link unix-nav-link--sub ${isActive ? 'is-active' : ''}`}>
            <item.icon size={15} strokeWidth={1.7} /><span className="unix-nav-label">{item.label}</span>
          </NavLink>)}
        </section>)}</div>}
      </nav>

      <div className="unix-sidebar-bottom">
        <NavLink to="/notifications" onClick={() => setSidebarOpen(false)} title="Notifications" className={({ isActive }) => `unix-nav-link ${isActive ? 'is-active' : ''}`}>
          <Bell size={16} /><span className="unix-nav-label">Notifications</span>{!collapsed && (dashboard?.metrics.unread_notifications ?? 0) > 0 && <span className="unix-count">{dashboard?.metrics.unread_notifications}</span>}
        </NavLink>
        <NavLink to="/settings" onClick={() => setSidebarOpen(false)} title="Settings" className={({ isActive }) => `unix-nav-link ${isActive ? 'is-active' : ''}`}><Settings2 size={16} /><span className="unix-nav-label">Settings</span></NavLink>
        <NavLink to="/help" onClick={() => setSidebarOpen(false)} title="Help" className={({ isActive }) => `unix-nav-link ${isActive ? 'is-active' : ''}`}><CircleHelp size={16} /><span className="unix-nav-label">Help</span></NavLink>
        {!collapsed && <div className="unix-engine-tile"><span className={`unix-status ${health ? 'is-online' : ''}`} /><span className="unix-engine-copy"><strong>{health ? 'Engine ready' : 'Engine offline'}</strong><small>{health ? 'Local services connected' : 'Run idios web in terminal'}</small></span><span className="unix-engine-symbol">{health ? '●' : '○'}</span></div>}
      </div>
    </aside>

    <div className="unix-main-column">
      <header className="unix-topbar">
        <div className="unix-topbar-title"><button type="button" className="unix-icon-button unix-mobile-menu" onClick={() => setSidebarOpen(true)} aria-label="Open navigation"><Menu size={17} /></button><span className="unix-topbar-root">IDIOS</span><span className="unix-path-separator">/</span><span className="unix-current-path">{pageTitle(location.pathname)}</span></div>
        <div className="unix-topbar-actions"><span className="unix-local-first"><span className="unix-status is-online" />LOCAL-FIRST</span><button type="button" className="unix-new-button" onClick={() => { setQuickCreateOpen((value) => !value); setProfileMenuOpen(false); setSearchOpen(false); }} aria-expanded={quickCreateOpen} aria-haspopup="dialog"><Plus size={14} /><span>New</span></button><NavLink to="/notifications" className="unix-icon-button unix-notification-link" aria-label="Notifications"><Bell size={16} />{(dashboard?.metrics.unread_notifications ?? 0) > 0 && <span className="unix-notification-dot" />}</NavLink><div className="unix-profile-anchor"><button type="button" className={`unix-profile-button ${profileMenuOpen ? 'is-active' : ''}`} onClick={() => { setProfileMenuOpen((value) => !value); setQuickCreateOpen(false); setSearchOpen(false); }} aria-label="Open profile menu" aria-expanded={profileMenuOpen}>{(dashboard?.profile.full_name || 'I').slice(0, 1).toUpperCase()}</button></div></div>
      </header>
      <main className="unix-page-content"><Outlet /></main>
      <footer className="unix-footer"><span><span className={`unix-status ${health ? 'is-online' : ''}`} />{health ? 'Local engine running' : 'Waiting for local engine'}</span><span><code>CLI</code><span className="unix-footer-slash">/</span><code>WEB</code><span className="unix-footer-slash">·</span>{lastUpdated ? `Synced ${new Date(lastUpdated).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}` : 'No sync yet'}</span></footer>
    </div>

    {quickCreateOpen && <div className="unix-overlay" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setQuickCreateOpen(false); }}><section className="unix-dialog unix-create-dialog" role="dialog" aria-modal="false" aria-label="Create new"><div className="unix-dialog-heading"><div><span className="unix-dialog-kicker">QUICK ACTION</span><h2>Create something</h2><p>Choose a local workspace operation.</p></div><button type="button" className="unix-icon-button" onClick={() => setQuickCreateOpen(false)} aria-label="Close create dialog"><X size={15} /></button></div><div className="unix-create-list">{quickCreateItems.map((item) => <button type="button" className="unix-create-item" key={item.id} onClick={() => runQuickCreate(item.id)}><span className="unix-create-item-icon"><Plus size={14} /></span><span><strong>{item.label}</strong><small>{item.detail}</small></span><ChevronRight size={14} /></button>)}{!quickCreateItems.length && <p className="unix-empty-copy">Loading actions…</p>}</div><div className="unix-dialog-foot"><button type="button" onClick={() => { setQuickCreateOpen(false); navigate('/commands'); }}>Open action center <ArrowRight size={13} /></button><kbd>Esc</kbd></div></section></div>}
    {searchOpen && <div className="unix-overlay" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setSearchOpen(false); }}><section className="unix-dialog unix-search-dialog" role="dialog" aria-modal="true" aria-label="Search workspace"><div className="unix-search-input"><Search size={17} /><input autoFocus placeholder="Search issues, repositories, projects, notes…" value={searchQuery} onChange={(event) => setSearchQuery(event.target.value)} /><kbd>Esc</kbd><button type="button" className="unix-icon-button" onClick={() => setSearchOpen(false)} aria-label="Close search"><X size={15} /></button></div><p className="unix-dialog-kicker">{searchQuery ? 'SEARCH RESULTS' : 'QUICK ACCESS'}</p><div className="unix-search-results">{searchable.length ? searchable.map((item, index) => <NavLink key={`${item.to}-${item.label}-${index}`} to={item.to} onClick={() => { setSearchOpen(false); setSearchQuery(''); }} className="unix-search-result"><span className="unix-search-result-icon"><Search size={13} /></span><span><strong>{item.label}</strong><small>{item.detail}</small></span><ChevronRight size={14} /></NavLink>) : <p className="unix-empty-copy">No matches found.</p>}</div><div className="unix-dialog-foot"><span><kbd>↵</kbd> Open</span><span><kbd>Esc</kbd> Close</span></div></section></div>}
    {profileMenuOpen && <><button type="button" className="unix-profile-scrim" aria-label="Close profile menu" onClick={() => setProfileMenuOpen(false)} /><section className="unix-profile-menu" role="menu" aria-label="Profile menu"><div className="unix-profile-identity"><span className="unix-profile-large">{(dashboard?.profile.full_name || 'I').slice(0, 1).toUpperCase()}</span><div><strong>{dashboard?.profile.full_name || 'IDIOS user'}</strong><small>{dashboard?.profile.username ? `@${dashboard.profile.username}` : 'Personal workspace'}</small></div></div><NavLink to="/profile" role="menuitem" onClick={() => setProfileMenuOpen(false)}><UserRound size={15} /><span><strong>Profile and pins</strong><small>Workspace identity</small></span></NavLink><NavLink to="/settings" role="menuitem" onClick={() => setProfileMenuOpen(false)}><Settings2 size={15} /><span><strong>Settings</strong><small>Theme and runtime</small></span></NavLink><NavLink to="/logs" role="menuitem" onClick={() => setProfileMenuOpen(false)}><ScrollText size={15} /><span><strong>Logs and diagnostics</strong><small>Local engine activity</small></span></NavLink><div className="unix-profile-foot"><span className={`unix-status ${health ? 'is-online' : ''}`} />{health ? `IDIOS ${health.version || ''} · Local` : 'Engine disconnected'}</div></section></>}
    <CommandRunner command={activeCommand} open={Boolean(activeCommand)} initialValues={activeCommandValues} onClose={() => setActiveCommand(null)} onSuccess={afterCommand} />
  </div>;
}

function pageTitle(pathname: string) {
  if (pathname === '/') return 'Overview';
  const known: Record<string, string> = { '/issues': 'Issues', '/pull-requests': 'Pull requests', '/repositories': 'Repositories', '/projects': 'Projects', '/graph': 'Knowledge graph', '/concepts': 'Concepts', '/notes': 'Notes', '/quotes': 'Quote library', '/activity': 'Activity', '/plans': 'Daily plans', '/planning': 'Labels & milestones', '/jobs': 'Scheduled jobs', '/commands': 'Action center', '/workspace': 'Profile & pins', '/profile': 'Profile & pins', '/intro': 'Introduction', '/settings/ai': 'AI Runtime', '/settings/notifications': 'Notification settings', '/settings/updates': 'Application updates', '/remove': 'Remove items', '/logs': 'Logs & diagnostics', '/help': 'Help & shortcuts', '/notifications': 'Notifications', '/settings': 'Settings' };
  return known[pathname] || 'Workspace';
}
