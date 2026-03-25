import { useEffect, useState, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, History } from 'lucide-react'
import { api, VersionInfo, PluginDetail as PluginDetailType } from '@/api/client'
import { toast } from 'sonner'
import { Card, CardHeader, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogFooter,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogAction,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog'
import { cn, formatRelativeDate } from '@/lib/utils'

function LoadingSkeleton() {
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Skeleton className="h-8 w-32" />
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-4 w-24" />
      </div>

      <div className="relative pl-6 border-l-2 border-muted space-y-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <Card key={i}>
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Skeleton className="h-5 w-20" />
                  <Skeleton className="h-5 w-16 rounded-full" />
                </div>
                <Skeleton className="h-4 w-28" />
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                <div className="space-y-1.5">
                  <Skeleton className="h-3 w-10" />
                  <Skeleton className="h-4 w-32" />
                </div>
                <div className="space-y-1.5">
                  <Skeleton className="h-3 w-16" />
                  <Skeleton className="h-4 w-12" />
                </div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}

export default function PluginVersions() {
  const { id } = useParams<{ id: string }>()
  const [plugin, setPlugin] = useState<PluginDetailType | null>(null)
  const [versions, setVersions] = useState<VersionInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState<number | null>(null)

  const load = useCallback(() => {
    if (!id) return
    setLoading(true)
    Promise.all([api.getPlugin(id), api.listVersions(id)])
      .then(([p, v]) => {
        setPlugin(p)
        setVersions(v)
      })
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [id])

  useEffect(() => { load() }, [load])

  const handleRollback = async (versionId: number) => {
    if (!id) return
    setActionLoading(versionId)
    try {
      const res = await api.rollbackVersion(id, versionId)
      toast.success(`Откат выполнен на версию ${res.version}`)
      load()
    } catch (e: unknown) {
      toast.error((e as Error).message)
    } finally {
      setActionLoading(null)
    }
  }

  const handleDelete = async (versionId: number) => {
    if (!id) return
    setActionLoading(versionId)
    try {
      await api.deleteVersion(id, versionId)
      toast.success('Версия удалена')
      load()
    } catch (e: unknown) {
      toast.error((e as Error).message)
    } finally {
      setActionLoading(null)
    }
  }

  const isActive = (ver: VersionInfo) => plugin?.wasm_hash === ver.wasm_hash

  if (loading && !versions.length) {
    return <LoadingSkeleton />
  }

  return (
    <div className="space-y-6">
      <div className="min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <Button variant="ghost" size="sm" asChild>
            <Link to={`/admin/plugins/${id}`}>
              <ArrowLeft className="h-4 w-4 mr-1" />
              {plugin?.name || id}
            </Link>
          </Button>
        </div>
        <h2 className="text-lg font-semibold">История версий</h2>
        <p className="text-sm text-muted-foreground">
          {versions.length} {versions.length === 1 ? 'версия' : versions.length < 5 ? 'версии' : 'версий'}
        </p>
      </div>

      {versions.length === 0 && (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <History className="h-12 w-12 text-muted-foreground/50 mb-4" />
          <h3 className="text-lg font-semibold mb-1">Нет сохранённых версий</h3>
          <p className="text-sm text-muted-foreground">
            История версий появится после обновления плагина
          </p>
        </div>
      )}

      {versions.length > 0 && (
        <div className="relative pl-6 border-l-2 border-muted">
          <div className="space-y-3">
            {versions.map((ver) => {
              const active = isActive(ver)
              return (
                <div key={ver.id} className="relative">
                  {/* Timeline dot */}
                  <div
                    className={cn(
                      'absolute -left-[calc(1.5rem+5px)] top-5 h-2.5 w-2.5 rounded-full border-2 border-background',
                      active ? 'bg-blue-500' : 'bg-muted-foreground/40',
                    )}
                  />
                  <Card
                    className={cn(active && 'border-blue-300 ring-1 ring-blue-100')}
                  >
                    <CardHeader className="pb-3">
                      <div className="flex items-center justify-between gap-3">
                        <div className="flex items-center gap-3 min-w-0">
                          <span className="font-medium text-sm">
                            {ver.version || 'без версии'}
                          </span>
                          {active && <Badge variant="default">текущая</Badge>}
                          {!active && (
                            <span className="text-xs text-muted-foreground italic">
                              отличается от текущей
                            </span>
                          )}
                          <span className="text-xs text-muted-foreground">#{ver.id}</span>
                        </div>
                        <span className="text-xs text-muted-foreground shrink-0">
                          {formatRelativeDate(ver.created_at)}
                        </span>
                      </div>
                    </CardHeader>

                    <CardContent className="space-y-3">
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                        <div>
                          <span className="text-muted-foreground block">Hash</span>
                          <span className="font-mono truncate block" title={ver.wasm_hash}>
                            {ver.wasm_hash.slice(0, 16)}...
                          </span>
                        </div>
                        {ver.permissions && ver.permissions.length > 0 && (
                          <div>
                            <span className="text-muted-foreground block">Разрешения</span>
                            <span>{ver.permissions.length} шт.</span>
                          </div>
                        )}
                        {ver.changelog && (
                          <div className="md:col-span-2">
                            <span className="text-muted-foreground block">Заметка</span>
                            <span>{ver.changelog}</span>
                          </div>
                        )}
                      </div>

                      {!active && (
                        <>
                          <Separator />
                          <div className="flex gap-2">
                            <AlertDialog>
                              <AlertDialogTrigger asChild>
                                <Button variant="outline" size="sm" className="text-amber-700 border-amber-300 hover:bg-amber-50">
                                  Откат
                                </Button>
                              </AlertDialogTrigger>
                              <AlertDialogContent>
                                <AlertDialogHeader>
                                  <AlertDialogTitle>Откат версии</AlertDialogTitle>
                                  <AlertDialogDescription>
                                    Откатить плагин на версию {ver.version || ver.id}? Текущая версия будет заменена.
                                  </AlertDialogDescription>
                                </AlertDialogHeader>
                                <AlertDialogFooter>
                                  <AlertDialogCancel>Отмена</AlertDialogCancel>
                                  <AlertDialogAction
                                    disabled={actionLoading === ver.id}
                                    onClick={() => handleRollback(ver.id)}
                                  >
                                    {actionLoading === ver.id ? 'Откат...' : 'Подтвердить'}
                                  </AlertDialogAction>
                                </AlertDialogFooter>
                              </AlertDialogContent>
                            </AlertDialog>

                            <AlertDialog>
                              <AlertDialogTrigger asChild>
                                <Button variant="destructive" size="sm">
                                  Удалить
                                </Button>
                              </AlertDialogTrigger>
                              <AlertDialogContent>
                                <AlertDialogHeader>
                                  <AlertDialogTitle>Удаление версии</AlertDialogTitle>
                                  <AlertDialogDescription>
                                    Вы уверены, что хотите удалить версию {ver.version || ver.id}? Это действие необратимо.
                                  </AlertDialogDescription>
                                </AlertDialogHeader>
                                <AlertDialogFooter>
                                  <AlertDialogCancel>Отмена</AlertDialogCancel>
                                  <AlertDialogAction
                                    disabled={actionLoading === ver.id}
                                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                                    onClick={() => handleDelete(ver.id)}
                                  >
                                    {actionLoading === ver.id ? 'Удаление...' : 'Удалить'}
                                  </AlertDialogAction>
                                </AlertDialogFooter>
                              </AlertDialogContent>
                            </AlertDialog>
                          </div>
                        </>
                      )}
                    </CardContent>
                  </Card>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
