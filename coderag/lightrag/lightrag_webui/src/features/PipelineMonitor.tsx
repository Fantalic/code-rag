import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Card } from '@/components/ui/Card'
import Button from '@/components/ui/Button'
import Badge from '@/components/ui/Badge'
import {
  ActivityIcon,
  RefreshCwIcon,
  FolderIcon,
  ListChecksIcon,
  LayersIcon,
} from 'lucide-react'
import { getScanStatus, type ScanStatusSnapshot } from '@/api/lightrag'
import { useProjectsStore } from '@/stores/projects'
import ProjectSwitcher from '@/components/ProjectSwitcher'
import HostStatusCard from '@/components/HostStatusCard'
import { cn } from '@/lib/utils'

const POLL_INTERVAL_MS = 2000

// Pipeline stage ordering used to render the indicator pills.
const STAGE_ORDER = ['watch', 'debounce', 'analyse', 'filter', 'diff', 'crud', 'llm', 'scan']

const COUNTER_KEYS: Array<{ key: string; labelKey: string }> = [
  { key: 'files', labelKey: 'scan.counters.files' },
  { key: 'entities_created', labelKey: 'scan.counters.entitiesCreated' },
  { key: 'entities_updated', labelKey: 'scan.counters.entitiesUpdated' },
  { key: 'entities_deleted', labelKey: 'scan.counters.entitiesDeleted' },
  { key: 'relations_created', labelKey: 'scan.counters.relationsCreated' },
  { key: 'relations_updated', labelKey: 'scan.counters.relationsUpdated' },
  { key: 'relations_deleted', labelKey: 'scan.counters.relationsDeleted' },
]

const stateBadgeClass: Record<string, string> = {
  idle: 'bg-zinc-500 text-white',
  scanning: 'bg-emerald-500 text-white',
  error: 'bg-red-500 text-white',
}

const levelDotClass: Record<string, string> = {
  info: 'bg-emerald-400',
  warn: 'bg-amber-400',
  error: 'bg-red-400',
}

function formatTime(ts: number): string {
  if (!ts) return '—'
  return new Date(ts * 1000).toLocaleTimeString([], { hour12: false })
}

function formatRelative(ts: number): string {
  if (!ts) return '—'
  const diff = Math.max(0, Date.now() / 1000 - ts)
  if (diff < 60) return `${Math.floor(diff)}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return new Date(ts * 1000).toLocaleDateString()
}

export default function PipelineMonitor() {
  const { t } = useTranslation()
  const [status, setStatus] = useState<ScanStatusSnapshot | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const mountedRef = useRef(true)

  const projects = useProjectsStore.use.projects()
  const activeId = useProjectsStore.use.activeId()

  const activeProject = useMemo(
    () => projects.find((p) => p.id === activeId) ?? null,
    [projects, activeId]
  )

  const fetchStatus = useCallback(async () => {
    try {
      const snapshot = await getScanStatus()
      if (!mountedRef.current) return
      setStatus(snapshot)
      setError(null)
    } catch (err) {
      if (!mountedRef.current) return
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  useEffect(() => {
    mountedRef.current = true
    const tick = async () => {
      if (!document.hidden) await fetchStatus()
      if (mountedRef.current) {
        pollTimerRef.current = setTimeout(tick, POLL_INTERVAL_MS)
      }
    }
    tick()
    return () => {
      mountedRef.current = false
      if (pollTimerRef.current) clearTimeout(pollTimerRef.current)
    }
  }, [fetchStatus])

  const handleManualRefresh = useCallback(async () => {
    setRefreshing(true)
    await fetchStatus()
    setRefreshing(false)
  }, [fetchStatus])

  const state = status?.state ?? 'idle'
  const currentStage = status?.stage ?? ''
  const showPipeline =
    status?.events && status.events.length > 0 ? true : false

  return (
    <div className="mx-auto w-full max-w-5xl space-y-4 p-4">
      {/* ── Intro / explainer ───────────────────────── */}
      <Card className="border-emerald-500/30 bg-emerald-500/5 p-4">
        <div className="flex items-start gap-3">
          <ActivityIcon className="mt-0.5 size-5 shrink-0 text-emerald-500" />
          <div className="min-w-0 flex-1">
            <h2 className="text-base font-semibold">{t('scan.title')}</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              {t(
                'scan.intro',
                'This panel reflects pipeline events pushed by the host-side CodeRAG CLI. Run `coderag scan <dir>` for an end-to-end scan, or `coderag watch <dir>` for a live file watcher. Events are batched and posted every ~0.4 s. The page polls every 2 s and pauses when the tab is hidden.'
              )}
            </p>
            <p className="mt-1 text-xs text-muted-foreground/80">
              {t('scan.terminalHint', 'Add projects from the host:')}
              <code className="ml-1 rounded bg-zinc-800 px-1.5 py-0.5 text-emerald-300">
                coderag scan &lt;dir&gt;
              </code>
              <code className="ml-1 rounded bg-zinc-800 px-1.5 py-0.5 text-emerald-300">
                coderag projects list
              </code>
            </p>
          </div>
        </div>
      </Card>

      {/* ── Active project banner ──────────────────── */}
      <Card className="flex items-center gap-3 p-3">
        <ProjectSwitcher />
        {activeProject && (
          <div className="ml-auto flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <span>
              <FolderIcon className="mr-1 inline-block size-3 align-middle" />
              <span className="font-mono">{activeProject.path}</span>
            </span>
            <span>
              <ListChecksIcon className="mr-1 inline-block size-3 align-middle" />
              scan #{activeProject.scan_count} · {formatRelative(activeProject.last_scan_at)}
            </span>
            <span>
              <LayersIcon className="mr-1 inline-block size-3 align-middle" />
              {(activeProject.total_counts?.entities_created ?? 0) +
                (activeProject.total_counts?.entities_updated ?? 0)}{' '}
              entities /{' '}
              {(activeProject.total_counts?.relations_created ?? 0) +
                (activeProject.total_counts?.relations_updated ?? 0)}{' '}
              relations
            </span>
          </div>
        )}
      </Card>

      {/* ── Host health card ───────────────────────── */}
      <HostStatusCard />

      {/* ── Current run header ─────────────────────── */}
      <Card className="p-4">
        <div className="mb-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <h3 className="text-sm font-semibold">{t('scan.currentRun')}</h3>
            <Badge className={cn('border-transparent text-[10px]', stateBadgeClass[state] ?? 'bg-zinc-500 text-white')}>
              {t(`scan.state.${state}`, state)}
            </Badge>
            {currentStage && (
              <Badge variant="outline" className="text-[10px]">
                {currentStage}
              </Badge>
            )}
          </div>
          <Button
            variant="ghost"
            size="icon"
            onClick={handleManualRefresh}
            disabled={refreshing}
            tooltip={t('scan.refreshTooltip', 'Refresh now')}
          >
            <RefreshCwIcon className={cn('size-4', refreshing && 'animate-spin')} />
          </Button>
        </div>

        {error && (
          <div className="mb-2 rounded-md border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs text-red-500">
            {error}
          </div>
        )}

        <div className="grid grid-cols-1 gap-y-1 text-sm sm:grid-cols-2">
          <div className="sm:flex sm:items-center">
            <span className="text-muted-foreground w-32 shrink-0">
              {t('scan.message')}
            </span>
            <span className="font-medium">{status?.message || t('scan.noActivity')}</span>
          </div>
          <div className="sm:flex sm:items-center">
            <span className="text-muted-foreground w-32 shrink-0">
              {t('scan.lastUpdate')}
            </span>
            <span>{status ? formatTime(status.updated_at) : '—'}</span>
          </div>
          {status?.detail && (
            <div className="sm:col-span-2">
              <span className="text-muted-foreground text-xs">
                {t('scan.detail')}:{' '}
              </span>
              <span className="font-mono text-xs">{status.detail}</span>
            </div>
          )}
        </div>

        {/* Stage strip — show which pipeline stages have run */}
        {showPipeline && (
          <div className="mt-3 flex flex-wrap gap-1.5">
            {STAGE_ORDER.map((s) => {
              const passed = (status?.events ?? []).some((e) => e.stage === s)
              return (
                <span
                  key={s}
                  className={cn(
                    'rounded-full border px-2 py-0.5 text-[10px] uppercase tracking-wide',
                    passed
                      ? s === currentStage
                        ? 'border-emerald-500 bg-emerald-500/10 text-emerald-600 dark:text-emerald-300'
                        : 'border-zinc-500 bg-zinc-500/10 text-zinc-600 dark:text-zinc-300'
                      : 'border-zinc-700 text-zinc-500 opacity-50'
                  )}
                >
                  {s}
                </span>
              )
            })}
          </div>
        )}

        {/* Counters */}
        {status?.counts && Object.keys(status.counts).length > 0 && (
          <div className="mt-4 grid grid-cols-2 gap-2 border-t pt-3 sm:grid-cols-4 lg:grid-cols-7">
            {COUNTER_KEYS.map(({ key, labelKey }) =>
              status.counts[key] !== undefined ? (
                <div key={key} className="rounded-md border p-2 text-center">
                  <div className="text-lg font-bold tabular-nums">
                    {status.counts[key]}
                  </div>
                  <div className="text-muted-foreground text-[10px] leading-tight">
                    {t(labelKey)}
                  </div>
                </div>
              ) : null
            )}
          </div>
        )}
      </Card>

      {/* ── Recent events ─────────────────────────── */}
      <Card className="p-4">
        <h3 className="mb-2 text-sm font-semibold">{t('scan.eventsTitle')}</h3>
        {!status || status.events.length === 0 ? (
          <div className="px-2 py-8 text-center text-sm text-muted-foreground">
            <p>{t('scan.noEvents')}</p>
            <p className="mt-1 text-xs text-muted-foreground/80">
              {t(
                'scan.noEventsHint',
                'Waiting for the host CLI to push progress. Start a scan with `coderag scan <dir>` on the host.'
              )}
            </p>
          </div>
        ) : (
          <ul className="max-h-[28rem] space-y-1.5 overflow-auto pr-1 font-mono text-xs">
            {status.events.map((event, idx) => (
              <li
                key={`${event.ts}-${idx}`}
                className={cn(
                  'flex items-start gap-2 rounded-md border px-2 py-1.5',
                  event.level === 'error' && 'border-red-500/40 bg-red-500/10',
                  event.level === 'warn' && 'border-amber-500/40 bg-amber-500/10'
                )}
              >
                <span
                  className={cn(
                    'mt-1.5 size-2 shrink-0 rounded-full',
                    levelDotClass[event.level] ?? ''
                  )}
                />
                <span className="text-muted-foreground shrink-0 tabular-nums">
                  {formatTime(event.ts)}
                </span>
                <Badge variant="outline" className="shrink-0 px-1.5 py-0 text-[10px]">
                  {event.stage}
                </Badge>
                <span className="min-w-0 break-words">
                  {event.message}
                  {event.detail && (
                    <span className="text-muted-foreground">
                      {' '}
                      — {event.detail}
                    </span>
                  )}
                </span>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  )
}
