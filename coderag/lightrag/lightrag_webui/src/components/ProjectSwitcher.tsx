import { useState, useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/Popover'
import Button from '@/components/ui/Button'
import { cn } from '@/lib/utils'
import { useProjectsStore } from '@/stores/projects'
import { ChevronDownIcon, FolderIcon, LoaderIcon } from 'lucide-react'
import { toast } from 'sonner'

const stateBadgeClass: Record<string, string> = {
  idle: 'bg-zinc-500 text-white',
  scanning: 'bg-emerald-500 text-white',
  error: 'bg-red-500 text-white',
}

function formatTime(ts: number): string {
  if (!ts) return '—'
  const diff = Date.now() / 1000 - ts
  if (diff < 60) return `${Math.floor(diff)}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

function totalsFor(p: ReturnType<typeof useProjectsStore.use.projects>[number]) {
  const t = p.total_counts || {}
  return `${(t.entities_created ?? 0) + (t.entities_updated ?? 0)} entities / ${
    (t.relations_created ?? 0) + (t.relations_updated ?? 0)
  } relations`
}

export default function ProjectSwitcher() {
  const [open, setOpen] = useState(false)
  const { t } = useTranslation()
  const projects = useProjectsStore.use.projects()
  const activeId = useProjectsStore.use.activeId()
  const readonly = useProjectsStore.use.readonly()
  const focus = useProjectsStore.use.focus()
  const refresh = useProjectsStore.use.refresh()

  const active = useMemo(
    () => projects.find((p) => p.id === activeId) ?? null,
    [projects, activeId]
  )

  const handleFocus = useCallback(
    async (id: string) => {
      try {
        await focus(id)
        setOpen(false)
        toast.success(t('projectSwitcher.focused', 'Focused project'))
      } catch (err) {
        toast.error(
          err instanceof Error ? err.message : 'Failed to change focus',
        )
      }
    },
    [focus, t]
  )

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 gap-1.5 px-2 text-sm font-medium"
        >
          <FolderIcon className="size-4 shrink-0 text-emerald-500" />
          <span className="max-w-[180px] truncate">
            {active ? active.name : t('projectSwitcher.none', 'No project')}
          </span>
          {active && (
            <span
              className={cn(
                'ml-1 inline-flex h-4 items-center rounded px-1.5 text-[10px] font-semibold uppercase tracking-wide',
                stateBadgeClass[active.last_status] ?? 'bg-zinc-500 text-white'
              )}
            >
              {active.last_status || 'idle'}
            </span>
          )}
          <ChevronDownIcon className="size-3 opacity-60" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        side="bottom"
        align="start"
        className="w-[420px] max-h-[70vh] overflow-auto p-2"
      >
        <div className="mb-2 flex items-center justify-between px-1.5">
          <h3 className="text-xs font-semibold uppercase text-muted-foreground">
            {t('projectSwitcher.title', 'Projects')}
          </h3>
          <button
            type="button"
            onClick={() => refresh()}
            className="text-xs text-muted-foreground hover:text-foreground"
          >
            {t('projectSwitcher.refresh', 'Refresh')}
          </button>
        </div>

        {readonly && (
          <div className="rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300">
            {t(
              'projectSwitcher.readonly',
              'Project switcher not available on this server. Update coderag UI build.'
            )}
          </div>
        )}

        {!readonly && projects.length === 0 && (
          <div className="px-2 py-6 text-center text-sm text-muted-foreground">
            <p>{t('projectSwitcher.empty', 'No projects yet.')}</p>
            <p className="mt-2 text-xs text-muted-foreground/80">
              {t(
                'projectSwitcher.hint',
                "Run `coderag scan <dir>` on the host to add one."
              )}
            </p>
          </div>
        )}

        {!readonly && projects.length > 0 && (
          <ul className="space-y-1">
            {projects.map((p) => (
              <li key={p.id}>
                <button
                  type="button"
                  onClick={() => handleFocus(p.id)}
                  className={cn(
                    'group w-full rounded-md px-2 py-2 text-left text-sm transition-colors',
                    'hover:bg-accent/40',
                    p.id === activeId && 'bg-emerald-500/10'
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex min-w-0 items-center gap-2">
                      <FolderIcon className="size-4 shrink-0 text-emerald-500" />
                      <span className="truncate font-medium">{p.name}</span>
                      <span className="shrink-0 rounded bg-secondary px-1.5 py-0.5 text-[10px] text-secondary-foreground">
                        {p.kind || 'mixed'}
                      </span>
                      {p.id === activeId && (
                        <span className="shrink-0 text-[10px] font-semibold uppercase text-emerald-500">
                          {t('projectSwitcher.active', 'active')}
                        </span>
                      )}
                    </div>
                    <span
                      className={cn(
                        'shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide',
                        stateBadgeClass[p.last_status] ?? 'bg-zinc-500 text-white'
                      )}
                    >
                      {p.last_status || 'idle'}
                    </span>
                  </div>
                  <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                    {p.path}
                  </div>
                  <div className="mt-1 flex justify-between text-[11px] text-muted-foreground">
                    <span>
                      scan #{p.scan_count} · {formatTime(p.last_scan_at)}
                    </span>
                    <span>{totalsFor(p)}</span>
                  </div>
                </button>
              </li>
            ))}
          </ul>
        )}

        <div className="mt-2 border-t pt-2 px-1.5 text-[11px] text-muted-foreground">
          {t(
            'projectSwitcher.terminalHint',
            `Add a project: \`coderag scan <dir>\` on the host.  Manage: \`coderag projects list|focus|forget\`.`
          )}
        </div>
        {useProjectsStore.getState().loading && (
          <div className="mt-1 flex items-center gap-1 px-1.5 text-[10px] text-muted-foreground">
            <LoaderIcon className="size-3 animate-spin" />
            {t('projectSwitcher.refreshing', 'Refreshing…')}
          </div>
        )}
      </PopoverContent>
    </Popover>
  )
}
