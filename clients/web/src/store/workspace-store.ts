import { create } from 'zustand';
import { api } from '@/lib/api';
import type { DashboardData, HealthStatus } from '@/types/idios';

type LoadState = 'idle' | 'loading' | 'ready' | 'error';
interface WorkspaceState {
  dashboard: DashboardData | null;
  health: HealthStatus | null;
  state: LoadState;
  error: string | null;
  lastUpdated: string | null;
  load: (quiet?: boolean) => Promise<void>;
  refreshHealth: () => Promise<void>;
}

export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  dashboard: null,
  health: null,
  state: 'idle',
  error: null,
  lastUpdated: null,
  load: async (quiet = false) => {
    if (!quiet) set({ state: 'loading', error: null });
    try {
      const [dashboard, health] = await Promise.all([api.dashboard(), api.health()]);
      set({ dashboard, health, state: 'ready', error: null, lastUpdated: new Date().toISOString() });
    } catch (error) {
      set((state) => ({
        state: state.dashboard ? 'ready' : 'error',
        error: error instanceof Error ? error.message : 'Unable to connect to the IDIOS server.',
      }));
    }
  },
  refreshHealth: async () => {
    try {
      const health = await api.health();
      set({ health });
    } catch {
      set({ health: null });
    }
  },
}));
