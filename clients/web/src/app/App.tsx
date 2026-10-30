import { Navigate, Route, Routes, useLocation } from 'react-router';
import { useEffect } from 'react';
import { api } from '@/lib/api';
import { IntroPage } from '@/features/onboarding';
import { AppShell } from '@/app/layout/AppShell';
import { ActivityPage } from '@/features/activity';
import { CommandsPage } from '@/features/commands';
import { ConceptsPage } from '@/features/concepts';
import { DashboardPage } from '@/features/dashboard';
import { GraphPage } from '@/features/graph';
import { IssuesPage } from '@/features/issues';
import { NotesPage } from '@/features/notes';
import { JobsPage } from '@/features/jobs';
import { PlanningPage } from '@/features/planning';
import { PlansPage } from '@/features/plans';
import { PullRequestsPage } from '@/features/pull-requests';
import { QuotesPage } from '@/features/quotes';
import { NotificationsPage } from '@/features/notifications';
import { ProjectsPage } from '@/features/projects';
import { RepositoriesPage } from '@/features/repositories';
import { SettingsPage } from '@/features/settings';
import { AISettingsPage } from '@/features/ai-settings';
import { WorkspacePage } from '@/features/workspace';
import { RemovePage } from '@/features/remove';
import { LogsPage } from '@/features/logs';
import { HelpPage } from '@/features/help';
import { ShelfAIChatPage, ShelfAsksPage, ShelfGraphPage, ShelfPage, ShelfPDFReaderPage } from '@/features/shelf';

export default function App() {
  const location = useLocation();
  useEffect(() => {
    const now = new Date();
    const localDate = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
    // Presence is recorded server-side once per local date; this effect is deliberately safe
    // under React StrictMode and does not treat navigation as multiple activity days.
    void api.recordActivityPresence(localDate).catch(() => undefined);
  }, [location.pathname]);
  return <Routes><Route path="shelf/:slug/read" element={<ShelfPDFReaderPage />} /><Route path="shelf/:slug/ai/models" element={<ShelfAIChatPage modelsOnly />} /><Route path="shelf/:slug/ai" element={<ShelfAIChatPage />} /><Route element={<AppShell />}><Route index element={<DashboardPage />} /><Route path="intro" element={<IntroPage />} /><Route path="issues" element={<IssuesPage />} /><Route path="pull-requests" element={<PullRequestsPage />} /><Route path="repositories" element={<RepositoriesPage />} /><Route path="repositories/:slug" element={<RepositoriesPage />} /><Route path="projects" element={<ProjectsPage />} /><Route path="graph" element={<GraphPage />} /><Route path="concepts" element={<ConceptsPage />} /><Route path="notes" element={<NotesPage />} /><Route path="shelf" element={<ShelfPage />} /><Route path="shelf/asks" element={<ShelfAsksPage />} /><Route path="shelf/graph" element={<ShelfGraphPage />} /><Route path="shelf/:slug" element={<ShelfPage />} /><Route path="quotes" element={<QuotesPage />} /><Route path="planning" element={<PlanningPage />} /><Route path="plans" element={<PlansPage />} /><Route path="jobs" element={<JobsPage />} /><Route path="commands" element={<CommandsPage />} /><Route path="profile" element={<WorkspacePage />} /><Route path="workspace" element={<Navigate to="/profile" replace />} /><Route path="remove" element={<RemovePage />} /><Route path="logs" element={<LogsPage />} /><Route path="help" element={<HelpPage />} /><Route path="activity" element={<ActivityPage />} /><Route path="notifications" element={<NotificationsPage />} /><Route path="settings" element={<SettingsPage />} /><Route path="settings/ai" element={<AISettingsPage />} /><Route path="settings/notifications" element={<SettingsPage focusSection="notifications" />} /><Route path="settings/updates" element={<SettingsPage focusSection="updates" />} /><Route path="*" element={<Navigate to="/" replace />} /></Route></Routes>;
}
