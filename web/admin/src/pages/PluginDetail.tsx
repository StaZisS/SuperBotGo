import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api, PluginDetail as PluginDetailType, PluginMeta } from '@/api/client'
import {
  ArrowLeft,
  Settings,
  Shield,
  History,
  Upload,
  Trash2,
  Power,
  Lock,
  Copy,
  Package,
  TriangleAlert,
} from 'lucide-react'
import { toast } from 'sonner'
import PluginStatusBadge from '@/components/PluginStatusBadge'
import WasmUploader from '@/components/WasmUploader'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogAction,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog'
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'

function compareVersions(a: string, b: string): number {
  const pa = a.split('.').map(Number)
  const pb = b.split('.').map(Number)
  const len = Math.max(pa.length, pb.length)
  for (let i = 0; i < len; i++) {
    const va = pa[i] || 0
    const vb = pb[i] || 0
    if (va < vb) return -1
    if (va > vb) return 1
  }
  return 0
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('ru-RU', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  })
}

function LoadingSkeleton() {
  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0 space-y-2">
          <Skeleton className="h-8 w-20" />
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-4 w-32" />
        </div>
        <Skeleton className="h-6 w-24 rounded-full" />
      </div>

      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-4 w-48" />
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="space-y-1.5">
                <Skeleton className="h-3 w-16" />
                <Skeleton className="h-5 w-24" />
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-24" />
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-3">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-8 w-28 rounded-md" />
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

export default function PluginDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [plugin, setPlugin] = useState<PluginDetailType | null>(null)
  const [loading, setLoading] = useState(true)
  const [showUpdate, setShowUpdate] = useState(false)
  const [actionLoading, setActionLoading] = useState(false)
  const [updateMeta, setUpdateMeta] = useState<PluginMeta | null>(null)
  const [updateFile, setUpdateFile] = useState<File | null>(null)
  const [showVersionConfirm, setShowVersionConfirm] = useState(false)

  const load = useCallback(() => {
    if (!id) return
    setLoading(true)
    api
      .getPlugin(id)
      .then(setPlugin)
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  const handleToggle = async () => {
    if (!id || !plugin) return
    const wasActive = plugin.status === 'active'
    setPlugin((prev) =>
      prev ? { ...prev, status: wasActive ? 'disabled' : 'active' } : prev,
    )
    try {
      if (wasActive) {
        await api.disablePlugin(id)
        toast.success('Плагин отключён')
      } else {
        await api.enablePlugin(id)
        toast.success('Плагин включён')
      }
      load()
    } catch (e: unknown) {
      setPlugin((prev) =>
        prev ? { ...prev, status: wasActive ? 'active' : 'disabled' } : prev,
      )
      toast.error((e as Error).message)
    }
  }

  const handleDelete = async () => {
    if (!id) return
    setActionLoading(true)
    try {
      await api.deletePlugin(id)
      toast.success('Плагин удалён')
      navigate('/admin/plugins')
    } catch (e: unknown) {
      toast.error((e as Error).message)
    } finally {
      setActionLoading(false)
    }
  }

  const handleUpdateFile = async (file: File) => {
    if (!id) return
    setActionLoading(true)
    try {
      const meta = await api.uploadPlugin(file)
      setUpdateFile(file)
      setUpdateMeta(meta)

      const currentVersion = plugin?.version
      if (currentVersion) {
        const cmp = compareVersions(meta.version, currentVersion)
        if (cmp <= 0) {
          setShowVersionConfirm(true)
          return
        }
      }

      await doUpdate(file)
    } catch (e: unknown) {
      toast.error((e as Error).message)
    } finally {
      setActionLoading(false)
    }
  }

  const doUpdate = async (file: File) => {
    if (!id) return
    setActionLoading(true)
    try {
      await api.updatePlugin(id, file)
      toast.success('Плагин обновлён')
      setShowUpdate(false)
      setUpdateMeta(null)
      setUpdateFile(null)
      load()
    } catch (e: unknown) {
      toast.error((e as Error).message)
    } finally {
      setActionLoading(false)
    }
  }

  const handleCopyHash = () => {
    if (!plugin?.wasm_hash) return
    navigator.clipboard.writeText(plugin.wasm_hash).then(
      () => toast.success('Hash скопирован'),
      () => toast.error('Не удалось скопировать'),
    )
  }

  if (loading && !plugin) {
    return <LoadingSkeleton />
  }

  if (!plugin) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <Package className="h-12 w-12 text-muted-foreground/50 mb-4" />
        <h3 className="text-lg font-semibold mb-1">Плагин не найден</h3>
        <p className="text-sm text-muted-foreground mb-4">
          Плагин не существует или был удалён
        </p>
        <Button variant="outline" asChild>
          <Link to="/admin/plugins">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            Вернуться к списку
          </Link>
        </Button>
      </div>
    )
  }

  const statusBorderColor =
    plugin.status === 'active'
      ? 'border-l-green-500'
      : plugin.status === 'error'
        ? 'border-l-red-500'
        : 'border-l-muted-foreground/40'

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <Button variant="ghost" size="sm" asChild className="mb-2 -ml-2">
            <Link to="/admin/plugins">
              <ArrowLeft className="mr-1 h-4 w-4" />
              Назад
            </Link>
          </Button>
          <h2 className="text-lg font-semibold truncate">
            {plugin.name || plugin.id}
          </h2>
          <p className="text-sm text-muted-foreground">
            {plugin.id}
            {plugin.version && <span> &middot; v{plugin.version}</span>}
          </p>
        </div>
        <PluginStatusBadge status={plugin.status || 'disabled'} />
      </div>

      {/* Plugin info card */}
      <Card className={`border-l-4 ${statusBorderColor}`}>
        <CardHeader>
          <CardTitle className="text-base">Информация</CardTitle>
          <CardDescription>Основные параметры плагина</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                Тип
              </span>
              <div className="font-medium mt-0.5">{plugin.type || 'wasm'}</div>
            </div>
            <div>
              <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                Версия
              </span>
              <div className="font-medium mt-0.5">
                {plugin.version || '-'}
              </div>
            </div>
            {plugin.wasm_hash && (
              <div className="col-span-2 md:col-span-1">
                <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                  Hash
                </span>
                <div className="flex items-center gap-1.5 mt-0.5">
                  <span
                    className="font-mono text-xs truncate"
                    title={plugin.wasm_hash}
                  >
                    {plugin.wasm_hash}
                  </span>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-5 w-5 shrink-0"
                    onClick={handleCopyHash}
                  >
                    <Copy className="h-3 w-3" />
                  </Button>
                </div>
              </div>
            )}
            {plugin.installed_at && (
              <div>
                <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                  Установлен
                </span>
                <div className="font-medium mt-0.5">
                  {formatDate(plugin.installed_at)}
                </div>
              </div>
            )}
            {plugin.updated_at && (
              <div>
                <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                  Обновлён
                </span>
                <div className="font-medium mt-0.5">
                  {formatDate(plugin.updated_at)}
                </div>
              </div>
            )}
          </div>

          {/* Commands */}
          {plugin.commands && plugin.commands.length > 0 && (
            <>
              <Separator />
              <div>
                <h4 className="text-sm font-medium mb-2">
                  Команды ({plugin.commands.length})
                </h4>
                <div className="space-y-1">
                  {plugin.commands.map((cmd) => (
                    <div
                      key={cmd.name}
                      className="flex items-center gap-3 text-sm p-2 bg-muted/50 rounded-md"
                    >
                      <span className="font-mono text-primary shrink-0">
                        /{cmd.name}
                      </span>
                      <span className="text-muted-foreground min-w-0 truncate">
                        {cmd.description}
                      </span>
                      {cmd.min_role && (
                        <Badge variant="outline" className="ml-auto shrink-0">
                          {cmd.min_role}
                        </Badge>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}

          {/* Permissions */}
          {plugin.permissions && plugin.permissions.length > 0 && (
            <>
              <Separator />
              <div>
                <h4 className="text-sm font-medium mb-2">Разрешения</h4>
                <div className="flex flex-wrap gap-2">
                  {plugin.permissions.map((p) => (
                    <Badge key={p} variant="secondary" className="font-mono">
                      {p}
                    </Badge>
                  ))}
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* Actions */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Действия</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Navigation group */}
          <div>
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              Навигация
            </span>
            <div className="flex flex-wrap gap-3 mt-2">
              <Button variant="outline" size="sm" asChild>
                <Link to={`/admin/plugins/${id}/config`}>
                  <Settings className="mr-1.5 h-4 w-4" />
                  Настроить
                </Link>
              </Button>

              <Button variant="outline" size="sm" asChild>
                <Link to={`/admin/plugins/${id}/permissions`}>
                  <Shield className="mr-1.5 h-4 w-4" />
                  Права команд
                </Link>
              </Button>

              <Button variant="outline" size="sm" asChild>
                <Link to={`/admin/plugins/${id}/plugin-permissions`}>
                  <Lock className="mr-1.5 h-4 w-4" />
                  Права плагина
                </Link>
              </Button>

              <Button variant="outline" size="sm" asChild>
                <Link to={`/admin/plugins/${id}/versions`}>
                  <History className="mr-1.5 h-4 w-4" />
                  Версии
                </Link>
              </Button>
            </div>
          </div>

          <Separator />

          {/* Management group */}
          <div>
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              Управление
            </span>
            <div className="flex flex-wrap gap-3 mt-2">
              <Button
                variant="outline"
                size="sm"
                onClick={handleToggle}
                disabled={actionLoading}
              >
                <Power className="mr-1.5 h-4 w-4" />
                {plugin.status === 'active' ? 'Отключить' : 'Включить'}
              </Button>

              {/* Update .wasm dialog */}
              <Dialog open={showUpdate} onOpenChange={(open) => {
                setShowUpdate(open)
                if (!open) {
                  setUpdateMeta(null)
                  setUpdateFile(null)
                }
              }}>
                <DialogTrigger asChild>
                  <Button variant="outline" size="sm">
                    <Upload className="mr-1.5 h-4 w-4" />
                    Обновить .wasm
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Загрузить новый .wasm</DialogTitle>
                    <DialogDescription>
                      Выберите файл .wasm для обновления плагина{' '}
                      <strong>{plugin.name || plugin.id}</strong>.
                    </DialogDescription>
                  </DialogHeader>
                  <WasmUploader onFile={handleUpdateFile} loading={actionLoading} />
                </DialogContent>
              </Dialog>

              {/* Version conflict confirmation */}
              <AlertDialog open={showVersionConfirm} onOpenChange={setShowVersionConfirm}>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle className="flex items-center gap-2">
                      <TriangleAlert className="h-5 w-5 text-amber-500" />
                      {updateMeta && plugin.version && compareVersions(updateMeta.version, plugin.version) === 0
                        ? 'Такая версия уже установлена'
                        : 'Откат на старую версию'}
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                      {updateMeta && plugin.version && compareVersions(updateMeta.version, plugin.version) === 0 ? (
                        <>
                          Плагин <strong>{plugin.name || plugin.id}</strong> версии{' '}
                          <strong>v{plugin.version}</strong> уже установлен.
                          Вы уверены, что хотите переустановить ту же версию?
                        </>
                      ) : (
                        <>
                          Сейчас установлена версия <strong>v{plugin.version}</strong>,
                          а вы загружаете более старую — <strong>v{updateMeta?.version}</strong>.
                          Продолжить?
                        </>
                      )}
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>Отмена</AlertDialogCancel>
                    <AlertDialogAction
                      onClick={() => {
                        setShowVersionConfirm(false)
                        if (updateFile) doUpdate(updateFile)
                      }}
                    >
                      Всё равно обновить
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>

              {/* Delete alert dialog */}
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive" size="sm">
                    <Trash2 className="mr-1.5 h-4 w-4" />
                    Удалить
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>Удалить плагин</AlertDialogTitle>
                    <AlertDialogDescription>
                      Вы уверены, что хотите удалить{' '}
                      <strong>{plugin.name || plugin.id}</strong>? Это действие
                      нельзя отменить.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={actionLoading}>
                      Отмена
                    </AlertDialogCancel>
                    <AlertDialogAction
                      onClick={handleDelete}
                      disabled={actionLoading}
                      className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                    >
                      {actionLoading ? 'Удаление...' : 'Подтвердить удаление'}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
