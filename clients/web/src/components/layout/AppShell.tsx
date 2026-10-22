import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router';
import type { LucideIcon } from 'lucide-react';
import {
  Activity, Bell, Blocks, BookOpen, CalendarClock, CalendarCheck, CheckSquare, ChevronRight, CircleUserRound,
  Command, FolderKanban, GitBranch, GitPullRequest, Home, Menu, Moon, PanelLeftClose, PanelLeftOpen,
  Plus, Quote, Search, Settings2, Sparkles, Sun, Tags, X, BrainCircuit, ScrollText, Trash2, CircleHelp, UserRound, ChevronDown,
} from 'lucide-react';
import { useWorkspaceStore } from '@/store/workspace-store';
import { api } from '@/lib/api';
import type { AppConfig, DashboardMetrics, FeatureDefinition, ProtocolCommand } from '@/types/idios';
import { applyWebTheme } from '@/lib/web-theme';
import { CommandRunner } from '@/components/commands/CommandRunner';
import { activateNotificationAudio, notificationSoundEnabled, playNotificationChime, playPlanReminderSequence, stopPlanReminderSequence } from '@/lib/notification-sound';

type NavigationItem = {
  to: string
  label: string
  icon: LucideIcon
  count?: keyof DashboardMetrics
}
type NavigationGroup = { label: string; items: NavigationItem[] }

const iconByName: Record<string, LucideIcon> = {
  activity: Activity, bell: Bell, blocks: Blocks, 'book-open': BookOpen, 'calendar-check': CalendarCheck,
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
  const applyConfig = useCallback((config: AppConfig) => {
    appConfigRef.current = config;
    setAppConfig(config);
    setCollapsed(config.web.sidebar_collapsed);
    setTheme(applyWebTheme(config));
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

  return <div className={`app-shell ${collapsed ? 'app-shell--collapsed' : ''}`}>
    {sidebarOpen && <button className="sidebar-scrim" aria-label="Close navigation" onClick={() => setSidebarOpen(false)} />}
    <aside className={`sidebar ${sidebarOpen ? 'sidebar--open' : ''}`}>
      <div className="brand-row">
        <NavLink to="/" className="brand" aria-label="IDIOS home"><span className="brand-mark"><Blocks size={19} strokeWidth={2.2} /></span><span className="brand-name">idios<span className="brand-dot">.</span></span></NavLink>
        <button className="icon-button sidebar-collapse" onClick={() => { const next = !collapsed; setCollapsed(next); persistWeb({ sidebar_collapsed: next }); }} aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'} title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}>{collapsed ? <PanelLeftOpen size={17} /> : <PanelLeftClose size={17} />}</button>
        <button className="icon-button mobile-sidebar-close" onClick={() => setSidebarOpen(false)} aria-label="Close navigation"><X size={18} /></button>
      </div>
      <button className="workspace-switcher" onClick={() => navigate('/profile')} aria-label="Open your profile"><span className="workspace-avatar">{(dashboard?.profile.full_name || 'I').slice(0, 1).toUpperCase()}</span><span className="workspace-switcher__text"><strong>{dashboard?.profile.full_name || 'Personal workspace'}</strong><small>Personal workspace · View profile</small></span><ChevronRight className="workspace-switcher__chevron" size={15} /></button>
      <nav className="nav-groups" aria-label="Main navigation">
        {nav.main.map((group) => {
          const groupCollapsed = collapsed ? false : (collapsedGroups[group.label] ?? false);
          return <div className={`nav-group ${groupCollapsed ? 'nav-group--collapsed' : ''}`} key={group.label}>
            <button type="button" className="nav-group__toggle" onClick={() => setCollapsedGroups((current) => ({ ...current, [group.label]: !(current[group.label] ?? false) }))} aria-expanded={!groupCollapsed} title={collapsed ? group.label : undefined}>
              <span className="nav-group__label">{group.label}</span><ChevronDown size={13} className="nav-group__chevron" />
            </button>
            {!groupCollapsed && <div className="nav-group__items">{group.items.map((item) => <NavLink key={item.to} to={item.to} end={item.to === '/'} className={({ isActive }) => `nav-link ${isActive ? 'nav-link--active' : ''}`} title={collapsed ? item.label : undefined}><item.icon size={17} strokeWidth={1.8} /><span className="nav-link__label">{item.label}</span>{item.count && navCount(item.count) !== undefined && <span className="nav-count">{navCount(item.count)}</span>}</NavLink>)}</div>}
          </div>;
        })}
      </nav>
      <div className="sidebar-bottom">
        {nav.bottom.map((item) => {
          const Icon = item.icon;
          return <NavLink key={item.to} to={item.to} end={item.to === '/'} className={({ isActive }) => `nav-link ${isActive ? 'nav-link--active' : ''}`} title={collapsed ? item.label : undefined}><Icon size={17} /><span className="nav-link__label">{item.label}</span>{item.to === '/notifications' && (dashboard?.metrics.unread_notifications ?? 0) > 0 && <span className="notification-dot" />}</NavLink>;
        })}
        <div className="sidebar-status"><span className={`status-light ${health ? 'status-light--online' : ''}`} /><span className="nav-link__label">{health ? 'Engine running' : 'Connecting to Engine'}</span></div>
        <div className="sidebar-footer"><span>LOCAL WORKSPACE</span><span className="version-mini">{health?.version ? `v${health.version}` : 'IDIOS'}</span></div>
      </div>
    </aside>

    <div className="main-column">
      <header className="topbar">
        <div className="topbar__left"><button className="icon-button mobile-menu" onClick={() => setSidebarOpen(true)} aria-label="Open navigation"><Menu size={19} /></button><span className="breadcrumb-root">Workspace</span><ChevronRight size={14} className="muted-icon" /><span className="breadcrumb-current">{pageTitle(location.pathname)}</span></div>
        <div className="topbar__actions"><button className="quick-create-trigger" onClick={() => { setQuickCreateOpen((value) => !value); setProfileMenuOpen(false); setSearchOpen(false); }} aria-expanded={quickCreateOpen} aria-haspopup="dialog"><Plus size={15} /><span>New</span><ChevronDown size={13} /></button><button className="search-trigger" onClick={() => { setSearchOpen(true); setQuickCreateOpen(false); setProfileMenuOpen(false); }} aria-label="Search IDIOS"><Search size={16} /><span>Search anything...</span><kbd>⌘ K</kbd></button><span className="topbar-divider" /><button className="icon-button" title={`Switch to ${theme === 'light' ? 'dark' : 'light'} appearance`} aria-label="Toggle theme appearance" onClick={() => { const next = theme === 'light' ? 'dark' : 'light'; const selected = appConfig?.web.theme ?? theme; const custom = appConfig?.web.custom_themes?.[selected]; if (custom) persistWeb({ theme: selected, custom_themes: { ...(appConfig?.web.custom_themes ?? {}), [selected]: { ...custom, base: next } } }); else persistWeb({ theme: next }); }}>{theme === 'light' ? <Moon size={17} /> : <Sun size={17} />}</button><NavLink to="/notifications" className="icon-button notification-button" aria-label="Notifications"><Bell size={17} />{(dashboard?.metrics.unread_notifications ?? 0) > 0 && <span />}</NavLink><div className="profile-menu-anchor"><button type="button" className={`profile-avatar ${profileMenuOpen ? 'profile-avatar--active' : ''}`} onClick={() => { setProfileMenuOpen((value) => !value); setQuickCreateOpen(false); setSearchOpen(false); }} aria-label="Open account menu" aria-expanded={profileMenuOpen} title="Account menu">{(dashboard?.profile.full_name || 'I').slice(0, 1).toUpperCase()}</button></div></div>
      </header>
      <main className="page-container"><Outlet /></main>
      <footer className="app-footer"><span><span className={`status-light ${health ? 'status-light--online' : ''}`} />{health ? 'IDIOS is running locally' : 'Waiting for IDIOS server'}</span><span>{lastUpdated ? `Synced ${new Date(lastUpdated).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}` : 'No sync yet'} · Your data stays on this device</span></footer>
    </div>

    {quickCreateOpen && <div className="quick-create-overlay" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setQuickCreateOpen(false); }}><section className="quick-create-menu" role="dialog" aria-modal="false" aria-label="Create in IDIOS"><div className="quick-create-menu__header"><span className="quick-create-menu__mark"><Plus size={16} /></span><div><strong>Create new</strong><small>Choose what you want to add</small></div><button className="icon-button" onClick={() => setQuickCreateOpen(false)} aria-label="Close create menu"><X size={15} /></button></div><div className="quick-create-menu__grid">{quickCreateItems.map((item) => <button type="button" className="quick-create-item" key={item.id} onClick={() => runQuickCreate(item.id)} disabled={!protocolCommands.some((command) => command.id === item.id)}><span className="quick-create-item__icon"><Plus size={14} /></span><span><strong>{item.label}</strong><small>{item.detail}</small></span><ChevronRight size={14} /></button>)}{!quickCreateItems.length && <p className="quick-create-empty">Loading available actions…</p>}</div><div className="quick-create-menu__footer"><button onClick={() => { setQuickCreateOpen(false); navigate('/commands'); }}>Browse all actions <Command size={13} /></button><kbd>ESC</kbd></div></section></div>}
    {searchOpen && <div className="command-overlay" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setSearchOpen(false); }}><section className="command-dialog" role="dialog" aria-modal="true" aria-label="Search IDIOS"><div className="command-input-row"><Search size={18} /><input autoFocus placeholder="Search issues, repositories, projects, notes…" value={searchQuery} onChange={(event) => setSearchQuery(event.target.value)} /><kbd>ESC</kbd><button className="icon-button" onClick={() => setSearchOpen(false)} aria-label="Close search"><X size={16} /></button></div><p className="command-label">{searchQuery ? 'SEARCH RESULTS' : 'QUICK ACCESS'}</p><div className="command-results">{searchable.length ? searchable.map((item, index) => <NavLink key={`${item.to}-${item.label}-${index}`} to={item.to} onClick={() => { setSearchOpen(false); setSearchQuery(''); }} className="command-result"><span className="command-result__icon"><Command size={15} /></span><span><strong>{item.label}</strong><small>{item.detail}</small></span><ChevronRight size={15} /></NavLink>) : <p className="command-empty">No matches. Try a different search.</p>}</div><div className="command-hint"><span><kbd>↵</kbd> Open result</span><span><kbd>ESC</kbd> Close</span></div></section></div>}
    {profileMenuOpen && <><button className="profile-menu-scrim" aria-label="Close account menu" onClick={() => setProfileMenuOpen(false)} /><section className="profile-menu" role="menu" aria-label="Account menu"><div className="profile-menu__identity"><span className="profile-menu__avatar">{(dashboard?.profile.full_name || 'I').slice(0, 1).toUpperCase()}</span><div><strong>{dashboard?.profile.full_name || 'IDIOS user'}</strong><small>{dashboard?.profile.username ? `@${dashboard.profile.username}` : 'Personal workspace'}</small></div></div><div className="profile-menu__divider" /><NavLink to="/profile" role="menuitem" onClick={() => setProfileMenuOpen(false)}><UserRound size={16} /><span><strong>Profile & pins</strong><small>Edit your name, bio, and workspace README</small></span></NavLink><NavLink to="/repositories" role="menuitem" onClick={() => setProfileMenuOpen(false)}><Blocks size={16} /><span><strong>Repositories</strong><small>Browse connected workspaces</small></span></NavLink><div className="profile-menu__divider" /><NavLink to="/settings" role="menuitem" onClick={() => setProfileMenuOpen(false)}><Settings2 size={16} /><span><strong>Settings</strong><small>Appearance, notifications, and shortcuts</small></span></NavLink><NavLink to="/notifications" role="menuitem" onClick={() => setProfileMenuOpen(false)}><Bell size={16} /><span><strong>Notifications</strong><small>{(dashboard?.metrics.unread_notifications ?? 0)} unread</small></span></NavLink><NavLink to="/help" role="menuitem" onClick={() => setProfileMenuOpen(false)}><CircleHelp size={16} /><span><strong>Help & shortcuts</strong><small>Learn the main workflows</small></span></NavLink><div className="profile-menu__divider" /><NavLink to="/logs" role="menuitem" onClick={() => setProfileMenuOpen(false)}><ScrollText size={16} /><span><strong>Logs & diagnostics</strong><small>Inspect the local Engine</small></span></NavLink><div className="profile-menu__footer"><span className={`status-light ${health ? 'status-light--online' : ''}`} />{health ? `IDIOS ${health.version || ''} · Running locally` : 'Waiting for the local Engine'}</div></section></>}
    <CommandRunner command={activeCommand} open={Boolean(activeCommand)} initialValues={activeCommandValues} onClose={() => setActiveCommand(null)} onSuccess={afterCommand} />
  </div>;
}

function pageTitle(pathname: string) {
  if (pathname === '/') return 'Overview';
  const known: Record<string, string> = { '/issues': 'Issues', '/pull-requests': 'Pull requests', '/repositories': 'Repositories', '/projects': 'Projects', '/graph': 'Knowledge graph', '/concepts': 'Concepts', '/notes': 'Notes', '/quotes': 'Quote library', '/activity': 'Activity', '/plans': 'Daily plans', '/planning': 'Labels & milestones', '/jobs': 'Scheduled jobs', '/commands': 'Action center', '/workspace': 'Profile & pins', '/profile': 'Profile & pins', '/intro': 'Introduction', '/settings/notifications': 'Notification settings', '/settings/keyboard': 'Keyboard shortcuts', '/settings/updates': 'Application updates', '/remove': 'Remove items', '/logs': 'Logs & diagnostics', '/help': 'Help & shortcuts', '/notifications': 'Notifications', '/settings': 'Settings' };
  return known[pathname] || 'Workspace';
}
