import { useEffect, useState, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, PluginDetail as PluginDetailType, PluginRequirementsDetail } from '@/api/client'
import { toast } from 'sonner'
import { ArrowLeft, Database, Globe, HardDrive, Bell, Radio, Puzzle, Archive } from 'lucide-react'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'

const TYPE_META: Record<string, { label: string; icon: React.ReactNode }> = {
  database: { label: 'База данных (SQL)', icon: <Database className="h-4 w-4" /> },
  db: { label: 'Legacy БД', icon: <Archive className="h-4 w-4" /> },
  http: { label: 'HTTP-запросы', icon: <Globe className="h-4 w-4" /> },
  kv: { label: 'Key-Value хранилище', icon: <HardDrive className="h-4 w-4" /> },
  notify: { label: 'Уведомления', icon: <Bell className="h-4 w-4" /> },
  events: { label: 'Публикация событий', icon: <Radio className="h-4 w-4" /> },
  plugin: { label: 'Вызов плагина', icon: <Puzzle className="h-4 w-4" /> },
}

export default function PluginPermissionsPage() {
  const { id } = useParams<{ id: string }>()
  const [plugin, setPlugin] = useState<PluginDetailType | null>(null)
  const [reqDetail, setReqDetail] = useState<PluginRequirementsDetail | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(() => {
    if (!id) return
    setLoading(true)
    Promise.all([api.getPlugin(id), api.getPluginRequirements(id)])
      .then(([p, reqs]) => {
        setPlugin(p)
        setReqDetail(reqs)
      })
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [id])

  useEffect(() => { load() }, [load])

  if (loading && !reqDetail) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-6 w-40 mb-1" />
        <Card>
          <CardContent className="space-y-3 pt-6">
            {[1, 2, 3].map((i) => (
              <div key={i} className="flex items-start gap-3 p-2">
                <Skeleton className="h-4 w-4 mt-0.5" />
                <div className="space-y-1.5 flex-1">
                  <Skeleton className="h-4 w-48" />
                  <Skeleton className="h-3 w-64" />
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    )
  }

  if (!reqDetail) {
    return <div className="text-muted-foreground py-8 text-center">Плагин не найден</div>
  }

  const requirements = reqDetail.requirements ?? []

  return (
    <div className="space-y-6">
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
        <h2 className="text-lg font-semibold">Требования плагина</h2>
        <p className="text-sm text-muted-foreground">{plugin?.name || id}</p>
      </div>

      {requirements.length === 0 ? (
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">
            Плагин не требует дополнительных ресурсов.
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-semibold text-muted-foreground">
              Ресурсы ({requirements.length})
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {requirements.map((req, i) => {
              const meta = TYPE_META[req.type] || { label: req.type, icon: null }
              return (
                <div
                  key={`${req.type}-${req.target || ''}-${i}`}
                  className="flex items-start gap-3 p-3 rounded-lg border"
                >
                  <span className="mt-0.5 text-muted-foreground shrink-0">{meta.icon}</span>
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-medium flex items-center gap-2 flex-wrap">
                      {meta.label}
                      {req.target && (
                        <Badge variant="outline" className="font-mono text-xs">
                          {req.target}
                        </Badge>
                      )}
                      {req.required ? (
                        <Badge variant="destructive" className="text-xs">
                          Обязательно
                        </Badge>
                      ) : (
                        <Badge variant="secondary" className="text-xs">
                          Опционально
                        </Badge>
                      )}
                    </div>
                    {req.description && (
                      <div className="text-xs text-muted-foreground mt-1">{req.description}</div>
                    )}
                  </div>
                </div>
              )
            })}
          </CardContent>
        </Card>
      )}

      <p className="text-xs text-muted-foreground">
        Доступ к ресурсам выдаётся автоматически на основе требований плагина.
        Безопасность контролируется на уровне самих ресурсов (например, права пользователя БД).
      </p>
    </div>
  )
}
