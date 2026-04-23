import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, CreatedServiceKey, PluginDetail, PluginInfo, ServiceKeyInfo, ServiceKeyScope } from '@/api/client'
import { toast } from 'sonner'
import { getErrorMessage } from '@/lib/utils'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { Copy, KeyRound, Loader2, Plus, ShieldCheck, Trash2 } from 'lucide-react'

type HTTPTriggerOption = {
  pluginId: string
  pluginName: string
  triggerName: string
  description: string
  path: string
  methods: string[]
}

function scopeKey(scope: Pick<ServiceKeyScope, 'plugin_id' | 'trigger_name'>): string {
  return `${scope.plugin_id}:${scope.trigger_name}`
}

function optionKey(option: Pick<HTTPTriggerOption, 'pluginId' | 'triggerName'>): string {
  return `${option.pluginId}:${option.triggerName}`
}

function formatDateTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString('ru-RU', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatMethods(methods: string[]): string {
  return methods.length > 0 ? methods.join(', ') : '—'
}

function extractHTTPTriggerOptions(plugin: PluginInfo, detail: PluginDetail): HTTPTriggerOption[] {
  const triggers = detail.meta?.triggers ?? []
  return triggers
    .filter((trigger) => trigger.type === 'http')
    .map((trigger) => ({
      pluginId: plugin.id,
      pluginName: detail.name || plugin.name || plugin.id,
      triggerName: trigger.name,
      description: trigger.description ?? '',
      path: trigger.path ?? '',
      methods: trigger.methods ?? [],
    }))
}

export default function HTTPServiceKeysPage() {
  const [keys, setKeys] = useState<ServiceKeyInfo[]>([])
  const [triggerOptions, setTriggerOptions] = useState<HTTPTriggerOption[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [selectedScopes, setSelectedScopes] = useState<string[]>([])
  const [createdKey, setCreatedKey] = useState<CreatedServiceKey | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)

  const loadKeys = useCallback(async () => {
    const data = await api.listServiceKeys()
    setKeys(data)
  }, [])

  const loadTriggerOptions = useCallback(async () => {
    const plugins = await api.listPlugins()
    const details = await Promise.allSettled(plugins.map((plugin) => api.getPlugin(plugin.id)))
    const options = details.flatMap((result, index) => {
      if (result.status !== 'fulfilled') {
        return []
      }
      return extractHTTPTriggerOptions(plugins[index], result.value)
    })
    options.sort((a, b) => {
      const byPlugin = a.pluginName.localeCompare(b.pluginName, 'ru')
      if (byPlugin !== 0) return byPlugin
      return a.triggerName.localeCompare(b.triggerName, 'ru')
    })
    setTriggerOptions(options)
  }, [])

  useEffect(() => {
    setLoading(true)
    Promise.all([loadKeys(), loadTriggerOptions()])
      .catch((e: unknown) => toast.error(getErrorMessage(e)))
      .finally(() => setLoading(false))
  }, [loadKeys, loadTriggerOptions])

  const groupedOptions = useMemo(() => {
    const groups = new Map<string, { pluginId: string; pluginName: string; items: HTTPTriggerOption[] }>()
    for (const option of triggerOptions) {
      const current = groups.get(option.pluginId)
      if (current) {
        current.items.push(option)
        continue
      }
      groups.set(option.pluginId, {
        pluginId: option.pluginId,
        pluginName: option.pluginName,
        items: [option],
      })
    }
    return Array.from(groups.values())
  }, [triggerOptions])

  const optionMap = useMemo(
    () => new Map(triggerOptions.map((option) => [optionKey(option), option])),
    [triggerOptions],
  )

  const selectedScopeCount = selectedScopes.length
  const canCreate = name.trim() !== '' && selectedScopeCount > 0 && !creating

  const resetCreateForm = () => {
    setName('')
    setExpiresAt('')
    setSelectedScopes([])
  }

  const toggleScope = (value: string, checked: boolean) => {
    setSelectedScopes((prev) => {
      if (checked) {
        return prev.includes(value) ? prev : [...prev, value]
      }
      return prev.filter((item) => item !== value)
    })
  }

  const handleCreate = async () => {
    if (!canCreate) return

    let expiresAtRFC3339: string | undefined
    if (expiresAt.trim() !== '') {
      const parsed = new Date(expiresAt)
      if (Number.isNaN(parsed.getTime())) {
        toast.error('Некорректная дата истечения')
        return
      }
      expiresAtRFC3339 = parsed.toISOString()
    }

    const scopes = selectedScopes.map((value) => {
      const [pluginId, triggerName] = value.split(':', 2)
      return { plugin_id: pluginId, trigger_name: triggerName }
    })

    setCreating(true)
    try {
      const created = await api.createServiceKey({
        name: name.trim(),
        scopes,
        expires_at: expiresAtRFC3339,
      })
      setCreatedKey(created)
      setCreateOpen(false)
      resetCreateForm()
      await loadKeys()
      toast.success('Service key создан')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setCreating(false)
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await api.deleteServiceKey(id)
      setKeys((prev) => prev.filter((key) => key.id !== id))
      toast.success('Service key удалён')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setDeletingId(null)
    }
  }

  const handleCopyToken = async () => {
    if (!createdKey?.token) return
    try {
      await navigator.clipboard.writeText(createdKey.token)
      toast.success('Токен скопирован')
    } catch {
      toast.error('Не удалось скопировать токен')
    }
  }

  return (
    <div>
      <div className="flex items-start justify-between gap-4 mb-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">HTTP Service Keys</h1>
          <p className="text-muted-foreground mt-1">
            Ключи для server-to-server доступа к HTTP trigger по bearer token. Scope задаётся на конкретный trigger.
          </p>
        </div>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="h-4 w-4 mr-2" />
              Создать ключ
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-3xl">
            <DialogHeader>
              <DialogTitle>Новый service key</DialogTitle>
              <DialogDescription>
                Выбери HTTP trigger, к которым ключ получит доступ. Сам ключ сработает только если у trigger включён
                `Service key` на странице прав.
              </DialogDescription>
            </DialogHeader>

            <div className="grid gap-4">
              <div className="grid gap-2">
                <Label htmlFor="service-key-name">Название</Label>
                <Input
                  id="service-key-name"
                  placeholder="Например: CRM integration"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="service-key-expiry">Истекает</Label>
                <Input
                  id="service-key-expiry"
                  type="datetime-local"
                  value={expiresAt}
                  onChange={(e) => setExpiresAt(e.target.value)}
                />
                <p className="text-xs text-muted-foreground">Необязательно. Если пусто, ключ будет бессрочным.</p>
              </div>

              <div className="grid gap-2">
                <div className="flex items-center justify-between">
                  <Label>Scope на HTTP trigger</Label>
                  <span className="text-xs text-muted-foreground">Выбрано: {selectedScopeCount}</span>
                </div>

                {triggerOptions.length === 0 ? (
                  <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
                    Нет доступных HTTP trigger. Сначала добавь их в плагины.
                  </div>
                ) : (
                  <div className="max-h-[360px] overflow-y-auto space-y-4 rounded-lg border p-4">
                    {groupedOptions.map((group) => (
                      <div key={group.pluginId} className="space-y-2">
                        <div className="flex items-center justify-between">
                          <div className="font-medium">{group.pluginName}</div>
                          <Button variant="link" size="sm" asChild className="h-auto p-0">
                            <Link to={`/admin/plugins/${group.pluginId}/permissions`} onClick={() => setCreateOpen(false)}>
                              Права trigger
                            </Link>
                          </Button>
                        </div>
                        <div className="space-y-2">
                          {group.items.map((option) => {
                            const key = optionKey(option)
                            const checked = selectedScopes.includes(key)
                            return (
                              <label
                                key={key}
                                className="flex items-start gap-3 rounded-lg border p-3 hover:bg-muted/40 cursor-pointer"
                              >
                                <Checkbox
                                  checked={checked}
                                  onCheckedChange={(value) => toggleScope(key, value === true)}
                                  className="mt-0.5"
                                />
                                <div className="min-w-0">
                                  <div className="flex flex-wrap items-center gap-2">
                                    <span className="font-mono text-sm font-medium">{option.triggerName}</span>
                                    <Badge variant="secondary">{formatMethods(option.methods)}</Badge>
                                  </div>
                                  <div className="text-xs text-muted-foreground mt-1">
                                    {option.path || 'path не указан'}
                                  </div>
                                  {option.description && (
                                    <div className="text-xs text-muted-foreground mt-1">{option.description}</div>
                                  )}
                                </div>
                              </label>
                            )
                          })}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => setCreateOpen(false)} disabled={creating}>
                Отмена
              </Button>
              <Button onClick={handleCreate} disabled={!canCreate}>
                {creating && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Создать ключ
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {createdKey && (
        <Card className="mb-6 border-emerald-200 bg-emerald-50/60">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-emerald-900">
              <ShieldCheck className="h-5 w-5" />
              Token показан только один раз
            </CardTitle>
            <CardDescription className="text-emerald-900/80">
              Сохрани bearer token сейчас. Позже в списке будет доступен только `public_id`, но не секретная часть.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Textarea readOnly value={createdKey.token} className="min-h-[88px] bg-white" />
            <div className="flex gap-2">
              <Button onClick={handleCopyToken}>
                <Copy className="h-4 w-4 mr-2" />
                Скопировать token
              </Button>
              <Button variant="outline" onClick={() => setCreatedKey(null)}>
                Скрыть
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      <Card className="mb-6">
        <CardContent className="pt-6">
          <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
            <div className="flex items-center gap-2">
              <KeyRound className="h-4 w-4" />
              Bearer token format: <span className="font-mono text-foreground">sbsk_&lt;public&gt;.&lt;secret&gt;</span>
            </div>
            <div>Для работы ключа нужны оба условия: scope на этом trigger и включённый `Service key` в правах trigger.</div>
          </div>
        </CardContent>
      </Card>

      {loading ? (
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, index) => (
            <Card key={index}>
              <CardHeader>
                <Skeleton className="h-5 w-40" />
                <Skeleton className="h-4 w-72" />
              </CardHeader>
              <CardContent className="space-y-3">
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-2/3" />
                <Skeleton className="h-10 w-full" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : keys.length === 0 ? (
        <Card>
          <CardContent className="py-16 text-center">
            <div className="rounded-full bg-muted w-16 h-16 mx-auto mb-4 flex items-center justify-center">
              <KeyRound className="h-8 w-8 text-muted-foreground" />
            </div>
            <h3 className="text-lg font-semibold mb-1">Service keys ещё не созданы</h3>
            <p className="text-sm text-muted-foreground max-w-md mx-auto">
              Создай ключ для внешней системы и назначь ему scope на нужные HTTP trigger.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {keys.map((key) => (
            <ServiceKeyCard
              key={key.id}
              serviceKey={key}
              optionMap={optionMap}
              deleting={deletingId === key.id}
              onDelete={() => {
                setDeletingId(key.id)
                void handleDelete(key.id)
              }}
              onDeleteOpenChange={(open) => {
                if (!open && deletingId === key.id) {
                  setDeletingId(null)
                }
              }}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function ServiceKeyCard({
  serviceKey,
  optionMap,
  deleting,
  onDelete,
  onDeleteOpenChange,
}: {
  serviceKey: ServiceKeyInfo
  optionMap: Map<string, HTTPTriggerOption>
  deleting: boolean
  onDelete: () => void
  onDeleteOpenChange: (open: boolean) => void
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
        <div className="min-w-0">
          <CardTitle className="flex items-center gap-2">
            <span>{serviceKey.name}</span>
            <Badge variant={serviceKey.active ? 'secondary' : 'outline'}>
              {serviceKey.active ? 'active' : 'inactive'}
            </Badge>
          </CardTitle>
          <CardDescription className="mt-1">
            public_id: <span className="font-mono text-foreground">{serviceKey.public_id}</span>
          </CardDescription>
        </div>

        <AlertDialog onOpenChange={onDeleteOpenChange}>
          <AlertDialogTrigger asChild>
            <Button variant="outline" size="sm">
              <Trash2 className="h-4 w-4 mr-2" />
              Удалить
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Удалить service key?</AlertDialogTitle>
              <AlertDialogDescription>
                Ключ `{serviceKey.name}` перестанет работать сразу. Восстановить его будет нельзя.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Отмена</AlertDialogCancel>
              <AlertDialogAction onClick={onDelete} disabled={deleting}>
                {deleting && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Удалить
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </CardHeader>

      <CardContent className="space-y-4">
        <div className="grid gap-3 md:grid-cols-3 text-sm">
          <div>
            <div className="text-muted-foreground">Создан</div>
            <div className="font-medium">{formatDateTime(serviceKey.created_at)}</div>
          </div>
          <div>
            <div className="text-muted-foreground">Истекает</div>
            <div className="font-medium">{formatDateTime(serviceKey.expires_at)}</div>
          </div>
          <div>
            <div className="text-muted-foreground">Последнее использование</div>
            <div className="font-medium">{formatDateTime(serviceKey.last_used_at)}</div>
          </div>
        </div>

        <div>
          <div className="text-sm font-medium mb-2">Scopes</div>
          <div className="space-y-2">
            {serviceKey.scopes.map((scope) => {
              const option = optionMap.get(scopeKey(scope))
              return (
                <div key={scopeKey(scope)} className="rounded-lg border p-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant="secondary">{option?.pluginName || scope.plugin_id}</Badge>
                    <span className="font-mono text-sm">{scope.trigger_name}</span>
                    {option && <Badge variant="outline">{formatMethods(option.methods)}</Badge>}
                  </div>
                  {option?.path && (
                    <div className="text-xs text-muted-foreground mt-1">{option.path}</div>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
