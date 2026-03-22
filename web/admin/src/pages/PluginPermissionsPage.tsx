import { useEffect, useState, useCallback, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, PluginDetail as PluginDetailType, PluginPermissionsDetail, HostPermissionInfo, DeclaredPermission } from '@/api/client'
import { toast } from 'sonner'
import { ArrowLeft, Database, Globe, Puzzle, Search, Timer } from 'lucide-react'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

const CATEGORY_LABELS: Record<string, string> = {
  database: 'База данных',
  network: 'Сеть',
  plugins: 'Плагины',
  triggers: 'Триггеры',
}

const CATEGORY_ORDER = ['database', 'network', 'plugins', 'triggers']

const CATEGORY_ICONS: Record<string, React.ReactNode> = {
  database: <Database className="h-4 w-4" />,
  network: <Globe className="h-4 w-4" />,
  plugins: <Puzzle className="h-4 w-4" />,
  triggers: <Timer className="h-4 w-4" />,
}

export default function PluginPermissionsPage() {
  const { id } = useParams<{ id: string }>()
  const [plugin, setPlugin] = useState<PluginDetailType | null>(null)
  const [permDetail, setPermDetail] = useState<PluginPermissionsDetail | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')

  const load = useCallback(() => {
    if (!id) return
    setLoading(true)
    Promise.all([api.getPlugin(id), api.getPluginPermissions(id)])
      .then(([p, perms]) => {
        setPlugin(p)
        setPermDetail(perms)
        setSelected(new Set(perms.granted))
        setDirty(false)
      })
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [id])

  useEffect(() => { load() }, [load])

  const declaredMap = useMemo(() => {
    const map = new Map<string, DeclaredPermission>()
    if (permDetail) {
      for (const d of permDetail.declared) {
        map.set(d.key, d)
      }
    }
    return map
  }, [permDetail])

  const isRequired = useCallback(
    (key: string) => declaredMap.get(key)?.required === true,
    [declaredMap],
  )

  const toggle = (key: string) => {
    if (isRequired(key)) return
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(key)) {
        next.delete(key)
      } else {
        next.add(key)
      }
      return next
    })
    setDirty(true)
  }

  const handleSave = async () => {
    if (!id) return
    setSaving(true)
    try {
      await api.updatePluginPermissions(id, Array.from(selected))
      toast.success('Права сохранены')
      setDirty(false)
    } catch (e: unknown) {
      toast.error((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  const totalPermissions = useMemo(() => {
    if (!permDetail) return 0
    return permDetail.all_available.length + (permDetail.callable_plugins?.length ?? 0)
  }, [permDetail])

  const grantedCount = selected.size

  const matchesSearch = useCallback(
    (key: string, description?: string) => {
      if (!searchQuery) return true
      const q = searchQuery.toLowerCase()
      return (
        key.toLowerCase().includes(q) ||
        (description?.toLowerCase().includes(q) ?? false)
      )
    },
    [searchQuery],
  )

  if (loading && !permDetail) {
    return (
      <div className="space-y-6">
        <div>
          <Skeleton className="h-4 w-20 mb-3" />
          <Skeleton className="h-6 w-40 mb-1" />
          <Skeleton className="h-4 w-32" />
        </div>
        <div className="space-y-6">
          {[1, 2, 3].map((i) => (
            <Card key={i}>
              <CardHeader>
                <Skeleton className="h-4 w-28" />
              </CardHeader>
              <CardContent className="space-y-3">
                {[1, 2].map((j) => (
                  <div key={j} className="flex items-start gap-3 p-2">
                    <Skeleton className="h-4 w-4 mt-0.5 shrink-0" />
                    <div className="space-y-1.5 flex-1">
                      <Skeleton className="h-4 w-48" />
                      <Skeleton className="h-3 w-64" />
                    </div>
                  </div>
                ))}
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    )
  }

  if (!permDetail) {
    return <div className="text-muted-foreground py-8 text-center">Плагин не найден</div>
  }

  const byCategory = (() => {
    const map = new Map<string, HostPermissionInfo[]>()
    for (const p of permDetail.all_available) {
      const list = map.get(p.category) || []
      list.push(p)
      map.set(p.category, list)
    }
    return map
  })()

  const callTargets = (() => {
    const map = new Map<string, { id: string; name: string; declared?: DeclaredPermission }>()
    for (const cp of permDetail.callable_plugins) {
      const key = `plugins:call:${cp.id}`
      map.set(key, { id: cp.id, name: cp.name, declared: declaredMap.get(key) })
    }
    for (const d of permDetail.declared) {
      if (d.key.startsWith('plugins:call:') && !map.has(d.key)) {
        const targetId = d.key.slice('plugins:call:'.length)
        map.set(d.key, { id: targetId, name: targetId, declared: d })
      }
    }
    for (const g of permDetail.granted) {
      if (g.startsWith('plugins:call:') && !map.has(g)) {
        const targetId = g.slice('plugins:call:'.length)
        map.set(g, { id: targetId, name: targetId, declared: declaredMap.get(g) })
      }
    }
    return map
  })()

  const isDisabled = plugin?.status === 'disabled'
  const showSearch = totalPermissions >= 6

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="flex items-center gap-3 mb-1">
          <Link
            to={`/admin/plugins/${id}`}
            className="inline-flex items-center gap-1 text-muted-foreground hover:text-foreground text-sm transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            Назад
          </Link>
        </div>
        <div className="flex items-center gap-3">
          <h2 className="text-lg font-semibold">Права плагина</h2>
          <Badge variant="secondary" className="font-normal">
            Выдано {grantedCount} из {totalPermissions} разрешений
          </Badge>
        </div>
        <p className="text-sm text-muted-foreground">{plugin?.name || id}</p>
      </div>

      {/* Search */}
      {showSearch && (
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Поиск разрешений..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9"
          />
        </div>
      )}

      {/* Disabled plugin warning */}
      {isDisabled && permDetail.declared.length === 0 && (
        <div className="rounded-lg border border-yellow-200 bg-yellow-50 p-4 text-sm text-yellow-800">
          Плагин отключён. Объявленные разрешения недоступны.
        </div>
      )}

      {/* Permission categories */}
      <div className="space-y-6">
        {CATEGORY_ORDER.map((cat) => {
          const perms = byCategory.get(cat)
          if (!perms || perms.length === 0) return null

          const filteredPerms = perms.filter((p) => {
            const decl = declaredMap.get(p.key)
            return matchesSearch(p.key, decl?.description || p.description)
          })

          if (filteredPerms.length === 0) return null

          return (
            <Card key={cat}>
              <CardHeader>
                <CardTitle className="text-sm font-semibold text-muted-foreground flex items-center gap-2">
                  {CATEGORY_ICONS[cat]}
                  {CATEGORY_LABELS[cat] || cat}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {filteredPerms.map((p) => {
                    const decl = declaredMap.get(p.key)
                    const required = decl?.required === true
                    const checked = required || selected.has(p.key)
                    return (
                      <Label
                        key={p.key}
                        htmlFor={`perm-page-${p.key}`}
                        className={cn(
                          'flex items-start gap-3 p-2 rounded font-normal',
                          required ? 'opacity-75' : 'hover:bg-accent cursor-pointer',
                        )}
                      >
                        <Checkbox
                          id={`perm-page-${p.key}`}
                          checked={checked}
                          disabled={required}
                          onCheckedChange={() => toggle(p.key)}
                          className="mt-0.5"
                        />
                        <div className="min-w-0">
                          <div className="text-sm font-mono break-all">
                            {p.key}
                            {required && (
                              <Badge variant="destructive" className="ml-2 font-sans text-xs">
                                Обязательно
                              </Badge>
                            )}
                            {decl && !required && (
                              <Badge variant="secondary" className="ml-2 font-sans text-xs">
                                Опционально
                              </Badge>
                            )}
                          </div>
                          <div className="text-xs text-muted-foreground mt-0.5">
                            {decl?.description || p.description}
                          </div>
                        </div>
                      </Label>
                    )
                  })}
                </div>
              </CardContent>
            </Card>
          )
        })}

        {/* Call targets */}
        {callTargets.size > 0 && (() => {
          const filteredTargets = Array.from(callTargets.entries()).filter(
            ([permKey, target]) =>
              matchesSearch(permKey, target.declared?.description || `Вызов плагина ${target.name}`),
          )
          if (filteredTargets.length === 0) return null
          return (
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-semibold text-muted-foreground flex items-center gap-2">
                  <Puzzle className="h-4 w-4" />
                  Вызов других плагинов
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {filteredTargets.map(([permKey, target]) => {
                    const required = target.declared?.required === true
                    const checked = required || selected.has(permKey)
                    return (
                      <Label
                        key={permKey}
                        htmlFor={`perm-page-${permKey}`}
                        className={cn(
                          'flex items-start gap-3 p-2 rounded font-normal',
                          required ? 'opacity-75' : 'hover:bg-accent cursor-pointer',
                        )}
                      >
                        <Checkbox
                          id={`perm-page-${permKey}`}
                          checked={checked}
                          disabled={required}
                          onCheckedChange={() => toggle(permKey)}
                          className="mt-0.5"
                        />
                        <div className="min-w-0">
                          <div className="text-sm font-mono break-all">
                            {permKey}
                            {required && (
                              <Badge variant="destructive" className="ml-2 font-sans text-xs">
                                Обязательно
                              </Badge>
                            )}
                            {target.declared && !required && (
                              <Badge variant="secondary" className="ml-2 font-sans text-xs">
                                Опционально
                              </Badge>
                            )}
                          </div>
                          <div className="text-xs text-muted-foreground mt-0.5">
                            {target.declared?.description || `Вызов плагина ${target.name}`}
                          </div>
                        </div>
                      </Label>
                    )
                  })}
                </div>
              </CardContent>
            </Card>
          )
        })()}
      </div>

      {/* Actions */}
      <div className="flex gap-3">
        <Button onClick={handleSave} disabled={saving || !dirty}>
          {saving ? 'Сохранение...' : 'Сохранить'}
        </Button>
        {dirty && (
          <Button variant="outline" onClick={load} disabled={saving}>
            Отменить изменения
          </Button>
        )}
      </div>
    </div>
  )
}
