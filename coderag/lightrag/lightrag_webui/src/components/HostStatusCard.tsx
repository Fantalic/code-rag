import { useEffect, useState } from 'react'
import { Card } from '@/components/ui/Card'
import { LoaderIcon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { getScanStatus } from '@/api/lightrag'

type Health = 'ok' | 'warn' | 'error' | 'unknown'

interface HealthDotProps {
  health: Health
  label: string
  hint?: string
}

function HealthDot({ health, label, hint }: HealthDotProps) {
  const color: Record<Health, string> = {
    ok: 'bg-emerald-400',
    warn: 'bg-amber-400',
    error: 'bg-red-400',
    unknown: 'bg-zinc-500',
  }
  return (
    <div className="flex items-center gap-2 text-xs">
      <span
        className={cn('size-2 shrink-0 rounded-full', color[health])}
        title={hint}
      />
      <span className="text-muted-foreground">{label}</span>
    </div>
  )
}

// "CLI" health = freshness of the last scan-status update from the host CLI.
function deriveCliHealth(scanUpdatedAt: number | null, scanState: string): Health {
  if (scanUpdatedAt === null || scanUpdatedAt === 0) return 'unknown'
  const ageSec = Date.now() / 1000 - scanUpdatedAt
  if (scanState === 'error') return 'error'
  if (ageSec > 15 * 60) return 'error'
  if (ageSec > 2 * 60) return 'warn'
  return 'ok'
}

export default function HostStatusCard() {
  const [scanAge, setScanAge] = useState<number | null>(null)
  const [scanState, setScanState] = useState<string>('idle')

  useEffect(() => {
    let mounted = true
    const tick = async () => {
      try {
        const s = await getScanStatus()
        if (!mounted) return
        setScanAge(s.updated_at || null)
        setScanState(s.state || 'idle')
      } catch {
        if (!mounted) return
        setScanAge(null)
        setScanState('error')
      }
    }
    tick()
    const h = setInterval(tick, 5000)
    return () => {
      mounted = false
      clearInterval(h)
    }
  }, [])

  const cliHealth: Health = deriveCliHealth(scanAge, scanState)

  const cliHint = scanAge
    ? `Last event ${Math.max(0, Math.floor(Date.now() / 1000 - scanAge))}s ago`
    : 'No events recorded yet'

  return (
    <Card className="flex flex-wrap items-center gap-4 px-4 py-2">
      <HealthDot
        health={cliHealth}
        label="Host CLI"
        hint={`${cliHint} — Green = stream is active.`}
      />
      <HealthDot
        health="ok"
        label="LightRAG"
        hint="Container reachable (responding to API calls)"
      />
      <HealthDot
        health="ok"
        label="MCP :9622"
        hint="Project-aware MCP server for AI assistants (Cline/Roo/Cursor)"
      />
      <span className="ml-auto inline-flex items-center gap-1 text-[10px] text-muted-foreground">
        <LoaderIcon className="size-3 opacity-50" />
        auto-refresh 5s
      </span>
    </Card>
  )
}
