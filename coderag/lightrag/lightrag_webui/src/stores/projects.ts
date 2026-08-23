import { create } from 'zustand'
import { createSelectors } from '@/lib/utils'
import {
  clearFocusedProject,
  focusProject,
  getProjects,
  ProjectInfo,
} from '@/api/lightrag'

const POLL_INTERVAL_MS = 3000

interface ProjectsState {
  projects: ProjectInfo[]
  activeId: string | null
  lastFetchAt: number
  loading: boolean
  error: string | null
  readonly: boolean
  /** Tracks whether the CLI pushed an event in the last ~60 s. */
  cliConnected: boolean

  // Actions
  refresh: () => Promise<void>
  startPolling: () => void
  stopPolling: () => void
  focus: (id: string) => Promise<void>
  clearFocused: (id: string) => Promise<void>
}

let pollHandle: ReturnType<typeof setInterval> | null = null

const useProjectsStoreBase = create<ProjectsState>((set, get) => ({
  projects: [],
  activeId: null,
  lastFetchAt: 0,
  loading: false,
  error: null,
  readonly: false,
  cliConnected: false,

  refresh: async () => {
    set({ loading: true })
    try {
      const data = await getProjects()
      set({
        projects: data.projects ?? [],
        activeId: data.active_id ?? null,
        lastFetchAt: Date.now(),
        loading: false,
        error: null,
      })
    } catch (err: any) {
      // 404 = endpoint missing (older server); degrade silently.
      if (err?.response?.status === 404) {
        set({
          projects: [],
          activeId: null,
          loading: false,
          error: null,
          readonly: true,
        })
        return
      }
      set({
        loading: false,
        error: err instanceof Error ? err.message : String(err),
      })
    }
  },

  startPolling: () => {
    if (pollHandle) return
    // Fire one immediately, then poll.
    void get().refresh()
    pollHandle = setInterval(() => {
      void get().refresh()
    }, POLL_INTERVAL_MS)
  },

  stopPolling: () => {
    if (pollHandle) {
      clearInterval(pollHandle)
      pollHandle = null
    }
  },

  focus: async (id: string) => {
    await focusProject(id)
    await get().refresh()
  },

  clearFocused: async (id: string) => {
    await clearFocusedProject(id)
    await get().refresh()
  },
}))

const useProjectsStore = createSelectors(useProjectsStoreBase)

export { useProjectsStore }
