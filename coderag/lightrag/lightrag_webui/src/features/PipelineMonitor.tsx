import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import Badge from '@/components/ui/Badge'
import Button from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { RefreshCwIcon } from 'lucide-react'
import { getScanStatus, type ScanStatusSnapshot } from '@/api/lightrag'
import { cn } from '@/lib/utils'

const POLL_INTERVAL_MS = 2000

// Counter keys pushed by the coderag CLI (crud + analyse stages)
const COUNTER_KEYS: Array<{ key: string; labelKey: string }> = [
  { key: 'files', labelKey: 'scan.counters.files' },
  { key: 'entities_created', labelKey: 'scan.counters.entitiesCreated' },
  { key: 'entities_updated', labelKey: 'scan.counters.entitiesUpdated' },
  { key: 'entities_deleted', labelKey: 'scan.counters.entitiesDeleted' },
  { key: 'relations_created', labelKey: 'scan.counters.relationsCreated' },
  { key: 'relations_updated', labelKey: 'scan.counters.relationsUpdated' },
  { key: 'relations_deleted', labelKey: 'scan.counters.relationsDeleted' }
]

function formatTime(ts: number): string {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleTimeString()
}

// Visual state for the run-state badge
const stateBadgeClass: Record<string, string> = {
  idle: 'bg-zinc-500 text-white',
  scanning: 'bg-emerald-500 text-white',
  error: 'bg-red-500 text-white'
}

// Level color for event rows
const levelDotClass: Record<string, string> = {
  info: 'bg-emerald-400',
  warn: 'bg-amber-400',
  error: 'bg-red-400'
}

export default function PipelineMonitor() {
  const { t } = useTranslation()
  const [status, setStatus] = useState<ScanStatusSnapshot | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const mountedRef = useRef(true)

  const fetchStatus = useCallback(async () => {
    try {
      const snapshot = await getScanStatus()
      if (!mountedRef.current) return
      setStatus(snapshot)
      setError(null)
    } catch (err) {
      if (!mountedRef.current) return
      // Keep last known status visible, just surface the fetch error
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  // Poll loop: runs only while this tab is mounted; pauses when the browser tab is hidden
  useEffect(() => {
    mountedRef.current = true

    const tick = async () => {
      if (!document.hidden) {
        await fetchStatus()
      }
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

  return (
    <div className="mx-auto w-full max-w-4xl space-y-4 p-4">
      {/* Current run header */}
      <Card className="p-4">
        <div className="mb-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold">{t('scan.title')}</h2>
            <Badge className={cn('border-transparent', stateBadgeClass[state] ?? '')}>
              {t(`scan.state.${state}`)}
            </Badge>
          </div>
          <Button variant="ghost" size="icon" onClick={handleManualRefresh} disabled={refreshing}>
            <RefreshCwIcon className={cn('size-4', refreshing && 'animate-spin')} />
          </Button>
        </div>

        {error && (
          <div className="mb-2 rounded-md border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs text-red-500">
            {error}
          </div>
        )}

        <div className="grid grid-cols-1 gap-x-6 gap-y-1 text-sm sm:grid-cols-2">
          <div className="flex justify-between gap-2 sm:block">
            <span className="text-muted-foreground">{t('scan.stage')}</span>
            <span className="font-mono sm:ml-2">{status?.stage || '-'}</span>
          </div>
          <div className="flex justify-between gap-2 sm:block">
            <span className="text-muted-foreground">{t('scan.lastUpdate')}</span>
            <span className="sm:ml-2">{status ? formatTime(status.updated_at) : '-'}</span>
          </div>
          <div className="col-span-1 sm:col-span-2 sm:mt-1">
            <span className="text-muted-foreground">{t('scan.currentMessage')}</span>
            <p className="mt-0.5 break-words font-medium">{status?.message || t('scan.noActivity')}</p>
            {status?.detail && (
              <p className="text-muted-foreground mt-0.5 break-words font-mono text-xs">{status.detail}</p>
            )}
          </div>
        </div>

        {/* Run counters */}
        {status?.counts && Object.keys(status.counts).length > 0 && (
          <div className="mt-4 grid grid-cols-2 gap-2 border-t pt-3 sm:grid-cols-4 lg:grid-cols-7">
            {COUNTER_KEYS.map(({ key, labelKey }) =>
              status.counts[key] !== undefined ? (
                <div key={key} className="rounded-md border p-2 text-center">
                  <div className="text-lg font-bold tabular-nums">{status.counts[key]}</div>
                  <div className="text-muted-foreground text-[10px] leading-tight">{t(labelKey)}</div>
                </div>
              ) : null
            )}
          </div>
        )}
      </Card>

      {/* Recent pipeline events */}
      <Card className="p-4">
        <h3 className="mb-2 text-sm font-semibold">{t('scan.eventsTitle')}</h3>
        {!status || status.events.length === 0 ? (
          <p className="text-muted-foreground text-sm">{t('scan.noEvents')}</p>
        ) : (
          <ul className="max-h-96 space-y-1.5 overflow-auto pr-1 font-mono text-xs">
            {status.events.map((event, idx) => (
              <li
                key={`${event.ts}-${idx}`}
                className={cn(
                  'flex items-start gap-2 rounded-md border px-2 py-1.5',
                  event.level === 'error' && 'border-red-500/40 bg-red-500/10',
                  event.level === 'warn' && 'border-amber-500/40 bg-amber-500/10'
                )}
              >
                <span className={cn('mt-1.5 size-2 shrink-0 rounded-full', levelDotClass[event.level] ?? '')} />
                <span className="text-muted-foreground shrink-0 tabular-nums">{formatTime(event.ts)}</span>
                <Badge variant="outline" className="shrink-0 px-1.5 py-0 text-[10px]">
                  {event.stage}
                </Badge>
                <span className="min-w-0 break-words">
                  {event.message}
                  {event.detail && <span className="text-muted-foreground"> — {event.detail}</span>}
                </span>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  )
}
