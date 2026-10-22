import { Navigate, Route, Routes } from 'react-router';
import { IntroPage } from '@/pages/IntroPage';
import { AppShell } from '@/components/layout/AppShell';
import { ActivityPage } from '@/pages/ActivityPage';
import { CommandsPage } from '@/pages/CommandsPage';
import { ConceptsPage } from '@/pages/ConceptsPage';
import { DashboardPage } from '@/pages/DashboardPage';
import { GraphPage } from '@/pages/GraphPage';
import { IssuesPage } from '@/pages/IssuesPage';
import { NotesPage } from '@/pages/NotesPage';
import { JobsPage } from '@/pages/JobsPage';
import { PlanningPage } from '@/pages/PlanningPage';
import { PlansPage } from '@/pages/PlansPage';
import { PullRequestsPage } from '@/pages/PullRequestsPage';
import { QuotesPage } from '@/pages/QuotesPage';
import { NotificationsPage } from '@/pages/NotificationsPage';
import { ProjectsPage } from '@/pages/ProjectsPage';
import { RepositoriesPage } from '@/pages/RepositoriesPage';
import { SettingsPage } from '@/pages/SettingsPage';
import { WorkspacePage } from '@/pages/WorkspacePage';
import { RemovePage } from '@/pages/RemovePage';
import { LogsPage } from '@/pages/LogsPage';
import { HelpPage } from '@/pages/HelpPage';

export default function App() {
  return <Routes><Route element={<AppShell />}><Route index element={<DashboardPage />} /><Route path="intro" element={<IntroPage />} /><Route path="issues" element={<IssuesPage />} /><Route path="pull-requests" element={<PullRequestsPage />} /><Route path="repositories" element={<RepositoriesPage />} /><Route path="repositories/:slug" element={<RepositoriesPage />} /><Route path="projects" element={<ProjectsPage />} /><Route path="graph" element={<GraphPage />} /><Route path="concepts" element={<ConceptsPage />} /><Route path="notes" element={<NotesPage />} /><Route path="quotes" element={<QuotesPage />} /><Route path="planning" element={<PlanningPage />} /><Route path="plans" element={<PlansPage />} /><Route path="jobs" element={<JobsPage />} /><Route path="commands" element={<CommandsPage />} /><Route path="profile" element={<WorkspacePage />} /><Route path="workspace" element={<Navigate to="/profile" replace />} /><Route path="remove" element={<RemovePage />} /><Route path="logs" element={<LogsPage />} /><Route path="help" element={<HelpPage />} /><Route path="activity" element={<ActivityPage />} /><Route path="notifications" element={<NotificationsPage />} /><Route path="settings" element={<SettingsPage />} /><Route path="settings/notifications" element={<SettingsPage focusSection="notifications" />} /><Route path="settings/updates" element={<SettingsPage focusSection="updates" />} /><Route path="settings/keyboard" element={<SettingsPage focusSection="keyboard" />} /><Route path="*" element={<Navigate to="/" replace />} /></Route></Routes>;
}
