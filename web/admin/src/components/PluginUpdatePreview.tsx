import {
  AlertCircle,
  ArrowRight,
  CheckCircle2,
  Minus,
  Plus,
  RefreshCw,
  TriangleAlert,
} from 'lucide-react'
import type { PluginUpdatePreviewResponse } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

interface Props {
  open: boolean
  loading: boolean
  preview: PluginUpdatePreviewResponse
  changelog: string
  onChangelogChange: (value: string) => void
  onCancel: () => void
  onConfirm: () => void
}

function ChangeBadge({ change }: { change: 'added' | 'removed' | 'changed' }) {
  if (change === 'added') {
    return <Badge className="bg-emerald-600 hover:bg-emerald-600">Добавлено</Badge>
  }
  if (change === 'removed') {
    return <Badge variant="destructive">Удалено</Badge>
  }
  return <Badge variant="secondary">Изменено</Badge>
}

function ChangeIcon({ change }: { change: 'added' | 'removed' | 'changed' }) {
  if (change === 'added') return <Plus className="h-3.5 w-3.5 text-emerald-600" />
  if (change === 'removed') return <Minus className="h-3.5 w-3.5 text-destructive" />
  return <RefreshCw className="h-3.5 w-3.5 text-amber-600" />
}

function WarningCard({
  level,
  title,
  message,
}: {
  level: 'info' | 'warn' | 'error'
  title: string
  message: string
}) {
  const meta = {
    info: {
      icon: <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-sky-600" />,
      className: 'border-sky-200 bg-sky-50/70 text-sky-950',
    },
    warn: {
      icon: <TriangleAlert className="mt-0.5 h-5 w-5 shrink-0 text-amber-600" />,
      className: 'border-amber-300 bg-amber-50/70 text-amber-950',
    },
    error: {
      icon: <TriangleAlert className="mt-0.5 h-5 w-5 shrink-0 text-destructive" />,
      className: 'border-destructive/30 bg-destructive/5 text-foreground',
    },
  }[level]

  return (
    <Card className={meta.className}>
      <CardContent className="flex items-start gap-3 p-4 text-sm">
        {meta.icon}
        <div className="space-y-1">
          <div className="font-medium">{title}</div>
          <div className="leading-relaxed">{message}</div>
        </div>
      </CardContent>
    </Card>
  )
}

function SummaryCard({
  title,
  current,
  next,
  changed,
}: {
  title: string
  current: string
  next: string
  changed: boolean
}) {
  return (
    <div className="rounded-xl border bg-muted/20 p-3">
      <div className="text-[11px] uppercase tracking-[0.12em] text-muted-foreground">{title}</div>
      <div className="mt-2 flex items-center gap-2 text-sm font-medium">
        <span className="min-w-0 truncate">{current || '-'}</span>
        <ArrowRight className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <span className="min-w-0 truncate">{next || '-'}</span>
      </div>
      <div className="mt-2">
        <Badge variant={changed ? 'secondary' : 'outline'}>
          {changed ? 'Изменено' : 'Без изменений'}
        </Badge>
      </div>
    </div>
  )
}

function DeltaChip({
  tone,
  value,
  label,
}: {
  tone: 'neutral' | 'added' | 'removed' | 'changed'
  value: number
  label: string
}) {
  const className = {
    neutral: 'border-border bg-background text-foreground',
    added: 'border-emerald-200 bg-emerald-50 text-emerald-900',
    removed: 'border-red-200 bg-red-50 text-red-900',
    changed: 'border-amber-200 bg-amber-50 text-amber-900',
  }[tone]

  return (
    <div className={cn('rounded-full border px-3 py-1.5 text-xs font-medium', className)}>
      {value} {label}
    </div>
  )
}

function CompareTile({
  label,
  name,
  id,
  version,
  emphasis,
}: {
  label: string
  name: string
  id: string
  version?: string
  emphasis?: boolean
}) {
  return (
    <div
      className={cn(
        'rounded-2xl border px-4 py-4',
        emphasis ? 'border-primary/25 bg-primary/5' : 'bg-background/80',
      )}
    >
      <div className="text-[11px] uppercase tracking-[0.12em] text-muted-foreground">{label}</div>
      <div className="mt-2 text-base font-semibold">{name}</div>
      <div className="mt-1 text-sm text-muted-foreground">{id}</div>
      <div className="mt-3 inline-flex rounded-full border bg-background px-2.5 py-1 text-xs font-medium">
        v{version || '-'}
      </div>
    </div>
  )
}

function SectionCard({
  section,
}: {
  section: NonNullable<PluginUpdatePreviewResponse['sections']>[number]
}) {
  const items = section.items ?? []
  const hasChanges = section.added > 0 || section.changed > 0 || section.removed > 0

  return (
    <Card className={cn(!hasChanges && 'border-dashed')}>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center justify-between gap-3 text-base">
          <span>{section.title}</span>
          <div className="flex flex-wrap items-center justify-end gap-2">
            {section.added > 0 && <Badge className="bg-emerald-600 hover:bg-emerald-600">+{section.added}</Badge>}
            {section.changed > 0 && <Badge variant="secondary">~{section.changed}</Badge>}
            {section.removed > 0 && <Badge variant="destructive">-{section.removed}</Badge>}
            {!hasChanges && <Badge variant="outline">Без изменений</Badge>}
          </div>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {items.length === 0 ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <CheckCircle2 className="h-4 w-4 text-emerald-600" />
            {section.empty_message || 'Изменений нет.'}
          </div>
        ) : (
          items.map((item) => (
            <div
              key={item.key}
              className={cn(
                'rounded-xl border p-3',
                item.change === 'added' && 'border-emerald-200 bg-emerald-50/50',
                item.change === 'removed' && 'border-red-200 bg-red-50/40',
                item.change === 'changed' && 'border-amber-200 bg-amber-50/40',
              )}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <ChangeIcon change={item.change} />
                    <span className="truncate">{item.title}</span>
                  </div>

                  {item.change === 'changed' ? (
                    <div className="mt-3 grid gap-2 md:grid-cols-2">
                      <div className="rounded-lg border bg-background/80 p-2">
                        <div className="mb-1 text-[11px] uppercase tracking-[0.12em] text-muted-foreground">
                          Было
                        </div>
                        <div className="text-xs text-muted-foreground">
                          {item.before || 'без деталей'}
                        </div>
                      </div>
                      <div className="rounded-lg border bg-background/80 p-2">
                        <div className="mb-1 text-[11px] uppercase tracking-[0.12em] text-muted-foreground">
                          Стало
                        </div>
                        <div className="text-xs text-muted-foreground">
                          {item.after || 'без деталей'}
                        </div>
                      </div>
                    </div>
                  ) : (
                    item.detail && (
                      <div className="mt-2 text-xs leading-relaxed text-muted-foreground">
                        {item.detail}
                      </div>
                    )
                  )}
                </div>
                <ChangeBadge change={item.change} />
              </div>
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}

export default function PluginUpdatePreview({
  open,
  loading,
  preview,
  changelog,
  onChangelogChange,
  onCancel,
  onConfirm,
}: Props) {
  const warnings = preview.warnings ?? []
  const summary = preview.summary ?? []
  const sections = preview.sections ?? []

  const changedSections = sections.filter(
    (section) => section.added > 0 || section.changed > 0 || section.removed > 0,
  )
  const unchangedSections = sections.filter(
    (section) => section.added === 0 && section.changed === 0 && section.removed === 0,
  )

  const totalAdded = sections.reduce((sum, section) => sum + section.added, 0)
  const totalRemoved = sections.reduce((sum, section) => sum + section.removed, 0)
  const totalChanged = sections.reduce((sum, section) => sum + section.changed, 0)

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen && !loading) onCancel()
      }}
    >
      <DialogContent className="max-h-[88vh] grid-rows-[auto_minmax(0,1fr)_auto] gap-0 overflow-hidden p-0 sm:max-w-5xl">
        <div className="border-b bg-gradient-to-r from-muted/60 via-background to-primary/5 px-6 py-5">
          <DialogHeader className="space-y-4 text-left">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <DialogTitle className="text-xl">Обновление плагина</DialogTitle>
                <DialogDescription className="sr-only">
                  Сравнение текущей и новой версии плагина перед обновлением.
                </DialogDescription>
              </div>
              <Badge variant={preview.can_update ? 'secondary' : 'destructive'}>
                {preview.can_update ? 'Готово к обновлению' : 'Обновление заблокировано'}
              </Badge>
            </div>

            <div className="grid items-center gap-3 md:grid-cols-[1fr_auto_1fr]">
              <CompareTile
                label="Сейчас"
                name={preview.current.name}
                id={preview.current.id}
                version={preview.current.version}
              />

              <div className="flex items-center justify-center">
                <div className="flex h-10 w-10 items-center justify-center rounded-full border bg-background shadow-sm">
                  <ArrowRight className="h-4 w-4 text-muted-foreground" />
                </div>
              </div>

              <CompareTile
                label="После обновления"
                name={preview.next.name}
                id={preview.next.id}
                version={preview.next.version}
                emphasis
              />
            </div>

            <div className="flex flex-wrap gap-2">
              <DeltaChip tone="neutral" value={changedSections.length} label="разделов изменится" />
              {warnings.length > 0 && (
                <DeltaChip tone="changed" value={warnings.length} label="предупреждений" />
              )}
              {totalAdded > 0 && <DeltaChip tone="added" value={totalAdded} label="добавлено" />}
              {totalChanged > 0 && <DeltaChip tone="changed" value={totalChanged} label="изменено" />}
              {totalRemoved > 0 && <DeltaChip tone="removed" value={totalRemoved} label="удалено" />}
            </div>
          </DialogHeader>
        </div>

        <div className="min-h-0 overflow-y-auto px-6 py-5">
          <div className="space-y-5">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">Заметка к обновлению</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <Label htmlFor="plugin-update-changelog">Комментарий</Label>
                <Textarea
                  id="plugin-update-changelog"
                  value={changelog}
                  onChange={(e) => onChangelogChange(e.target.value)}
                  placeholder="Например: обновили RPC-метод, добавили новую точку запуска, исправили авторизацию"
                  rows={3}
                />
              </CardContent>
            </Card>

            {warnings.length > 0 && (
              <div className="space-y-3">
                {warnings.map((warning) => (
                  <WarningCard
                    key={warning.code}
                    level={warning.level}
                    title={warning.title}
                    message={warning.message}
                  />
                ))}
              </div>
            )}

            {!preview.has_changes && (
              <Card>
                <CardContent className="flex items-center gap-2 p-4 text-sm text-muted-foreground">
                  <CheckCircle2 className="h-4 w-4 text-emerald-600" />
                  По публичным метаданным различий нет. Обновится только wasm-бандл.
                </CardContent>
              </Card>
            )}

            <Tabs defaultValue="overview" className="space-y-4">
              <TabsList className="h-auto flex-wrap justify-start gap-1 bg-muted/70">
                <TabsTrigger value="overview">Обзор</TabsTrigger>
                <TabsTrigger value="changes">
                  Изменения
                  {changedSections.length > 0 && (
                    <span className="ml-1 text-xs text-muted-foreground">
                      ({changedSections.length})
                    </span>
                  )}
                </TabsTrigger>
              </TabsList>

              <TabsContent value="overview" className="space-y-4">
                <Card>
                  <CardHeader className="pb-3">
                    <CardTitle className="text-base">Ключевые изменения</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                      {summary.map((item) => (
                        <SummaryCard
                          key={item.key}
                          title={item.title}
                          current={item.current}
                          next={item.next}
                          changed={item.changed}
                        />
                      ))}
                    </div>

                    {changedSections.length > 0 && (
                      <div className="rounded-xl border bg-muted/20 p-4">
                        <div className="mb-3 text-sm font-medium">
                          Затронутые разделы
                        </div>
                        <div className="flex flex-wrap gap-2">
                          {changedSections.map((section) => (
                            <Badge key={section.key} variant="secondary" className="rounded-full">
                              {section.title}
                            </Badge>
                          ))}
                        </div>
                      </div>
                    )}

                    {unchangedSections.length > 0 && (
                      <div className="rounded-xl border border-dashed bg-background/70 p-4">
                        <div className="mb-3 text-sm font-medium text-muted-foreground">
                          Без изменений
                        </div>
                        <div className="flex flex-wrap gap-2">
                          {unchangedSections.map((section) => (
                            <Badge key={section.key} variant="outline" className="rounded-full">
                              {section.title}
                            </Badge>
                          ))}
                        </div>
                      </div>
                    )}
                  </CardContent>
                </Card>
              </TabsContent>

              <TabsContent value="changes" className="space-y-4">
                {changedSections.length === 0 ? (
                  <Card>
                    <CardContent className="flex items-center gap-2 p-4 text-sm text-muted-foreground">
                      <CheckCircle2 className="h-4 w-4 text-emerald-600" />
                      Существенных изменений в публичном контракте не найдено.
                    </CardContent>
                  </Card>
                ) : (
                  changedSections.map((section) => (
                    <SectionCard key={section.key} section={section} />
                  ))
                )}

                {unchangedSections.length > 0 && (
                  <Card className="border-dashed">
                    <CardHeader className="pb-3">
                      <CardTitle className="text-base">Без изменений</CardTitle>
                    </CardHeader>
                    <CardContent>
                      <div className="flex flex-wrap gap-2">
                        {unchangedSections.map((section) => (
                          <Badge key={section.key} variant="outline" className="rounded-full">
                            {section.title}
                          </Badge>
                        ))}
                      </div>
                    </CardContent>
                  </Card>
                )}
              </TabsContent>
            </Tabs>
          </div>
        </div>

        <DialogFooter className="border-t bg-background px-6 py-4">
          <Button variant="outline" onClick={onCancel} disabled={loading}>
            Отмена
          </Button>
          <Button onClick={onConfirm} disabled={loading || !preview.can_update}>
            {loading ? 'Обновление...' : 'Обновить плагин'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
