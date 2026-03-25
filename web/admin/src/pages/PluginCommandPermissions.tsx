import { useEffect, useState, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, CommandSetting, PluginDetail } from '@/api/client'
import { toast } from 'sonner'
import RuleBuilder from '@/components/RuleBuilder'
import { Card, CardContent } from '@/components/ui/card'
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from '@/components/ui/collapsible'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog'
import { ChevronRight, ArrowLeft, Shield } from 'lucide-react'
import { cn } from '@/lib/utils'

interface CommandRow {
  name: string
  description: string
  enabled: boolean
  policyExpression: string
  hasSetting: boolean
}

export default function PluginCommandPermissions() {
  const { id } = useParams<{ id: string }>()
  const [plugin, setPlugin] = useState<PluginDetail | null>(null)
  const [rows, setRows] = useState<CommandRow[]>([])
  const [loading, setLoading] = useState(true)

  const loadData = useCallback(async () => {
    if (!id) return
    try {
      const p = await api.getPlugin(id)
      setPlugin(p)

      let settings: CommandSetting[] = []
      try {
        settings = await api.listCommandSettings(id)
      } catch {}

      const settingMap = new Map(settings.map((s) => [s.command_name, s]))
      const commands = p.commands ?? p.meta?.triggers?.filter((t) => t.type === 'messenger') ?? []

      setRows(
        commands.map((cmd) => {
          const setting = settingMap.get(cmd.name)
          return {
            name: cmd.name,
            description: cmd.description ?? '',
            enabled: setting?.enabled ?? true,
            policyExpression: setting?.policy_expression ?? '',
            hasSetting: !!setting,
          }
        }),
      )
    } catch {
      toast.error('Не удалось загрузить плагин')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { loadData() }, [loadData])

  const enabledCount = rows.filter((r) => r.enabled).length

  if (loading) {
    return (
      <div>
        <div className="mb-6">
          <Skeleton className="h-4 w-32 mb-2" />
          <Skeleton className="h-8 w-64 mb-1" />
          <Skeleton className="h-4 w-80" />
        </div>
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <Card key={i}>
              <div className="flex items-center justify-between p-5">
                <div className="flex items-center gap-3">
                  <Skeleton className="h-4 w-4" />
                  <Skeleton className="h-4 w-28" />
                  <Skeleton className="h-4 w-48" />
                </div>
                <Skeleton className="h-5 w-10 rounded-full" />
              </div>
            </Card>
          ))}
        </div>
      </div>
    )
  }

  if (!plugin) return <div className="text-destructive text-sm">Плагин не найден</div>

  return (
    <div>
      <div className="mb-6">
        <Button variant="link" asChild className="p-0 h-auto text-sm">
          <Link to={`/admin/plugins/${id}`}>
            <ArrowLeft className="mr-1 h-3.5 w-3.5" />
            Назад к {plugin.name || id}
          </Link>
        </Button>
        <div className="flex items-center gap-3 mt-2">
          <h1 className="text-2xl font-semibold">Права доступа к командам</h1>
          {rows.length > 0 && (
            <Badge variant="secondary" className="font-normal">
              {enabledCount} из {rows.length} включены
            </Badge>
          )}
        </div>
        <p className="text-sm text-muted-foreground mt-1">
          Управление доступом к командам <strong>{plugin.name || id}</strong> через политики доступа.
        </p>
      </div>

      {rows.length === 0 ? (
        <Card className="p-10">
          <div className="flex flex-col items-center justify-center text-center">
            <div className="rounded-full bg-muted p-4 mb-4">
              <Shield className="h-8 w-8 text-muted-foreground" />
            </div>
            <p className="text-sm font-medium text-muted-foreground mb-1">
              Нет команд
            </p>
            <p className="text-xs text-muted-foreground">
              У этого плагина нет команд
            </p>
          </div>
        </Card>
      ) : (
        <div className="space-y-4">
          {rows.map((row) => (
            <CommandCard key={row.name} pluginId={id!} row={row} onUpdate={loadData} />
          ))}
        </div>
      )}
    </div>
  )
}

function CommandCard({
  pluginId,
  row,
  onUpdate,
}: {
  pluginId: string
  row: CommandRow
  onUpdate: () => void
}) {
  const [expanded, setExpanded] = useState(false)
  const [toggling, setToggling] = useState(false)
  const [policyExpr, setPolicyExpr] = useState(row.policyExpression)
  const [savingPolicy, setSavingPolicy] = useState(false)
  const [builderKey, setBuilderKey] = useState(0)
  const [clearOpen, setClearOpen] = useState(false)

  useEffect(() => { setPolicyExpr(row.policyExpression) }, [row.policyExpression])

  const handleToggle = async () => {
    setToggling(true)
    try {
      await api.setCommandEnabled(pluginId, row.name, !row.enabled)
      onUpdate()
    } catch {
      toast.error('Не удалось переключить команду')
    } finally {
      setToggling(false)
    }
  }

  const handleSavePolicy = async () => {
    setSavingPolicy(true)
    try {
      await api.setCommandPolicy(pluginId, row.name, policyExpr)
      onUpdate()
      toast.success('Политика сохранена')
    } catch {
      toast.error('Не удалось сохранить политику')
    } finally {
      setSavingPolicy(false)
    }
  }

  const policyChanged = policyExpr !== row.policyExpression

  return (
    <Card>
      <Collapsible open={expanded} onOpenChange={setExpanded}>
        <div className="flex items-center justify-between p-5">
          <CollapsibleTrigger asChild>
            <button className="flex items-center gap-3 cursor-pointer hover:opacity-80 transition-opacity">
              <ChevronRight
                className={cn(
                  'h-4 w-4 text-muted-foreground transition-transform',
                  expanded && 'rotate-90',
                )}
              />
              <span
                className={cn(
                  'inline-block h-2 w-2 rounded-full shrink-0',
                  row.enabled ? 'bg-green-500' : 'bg-red-500',
                )}
              />
              <span className="font-mono text-sm font-medium">/{row.name}</span>
              {row.description && (
                <span className="text-sm text-muted-foreground">{row.description}</span>
              )}
            </button>
          </CollapsibleTrigger>
          <div className="flex items-center gap-3">
            {row.policyExpression && (
              <Badge variant="secondary">policy</Badge>
            )}
            <Switch
              checked={row.enabled}
              onCheckedChange={handleToggle}
              disabled={toggling}
            />
          </div>
        </div>

        <CollapsibleContent>
          <Separator />
          <CardContent
            className={cn(
              'p-5',
              !row.enabled && 'opacity-50 pointer-events-none',
            )}
          >
            <div className="flex items-center justify-between mb-3">
              <h4 className="text-sm font-medium">
                Политика доступа
                {row.policyExpression
                  ? <span className="ml-2 text-xs text-primary font-normal">(активна)</span>
                  : <span className="ml-2 text-xs text-muted-foreground font-normal">(пусто = доступно всем)</span>
                }
              </h4>
            </div>

            <RuleBuilder key={builderKey} expression={policyExpr} onChange={setPolicyExpr} />

            <Separator className="my-3" />
            <div className="flex justify-end gap-2">
              {row.policyExpression && (
                <AlertDialog open={clearOpen} onOpenChange={setClearOpen}>
                  <AlertDialogTrigger asChild>
                    <Button variant="outline" size="sm">
                      Очистить
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Очистить политику?</AlertDialogTitle>
                      <AlertDialogDescription>
                        Политика доступа будет удалена. Команда станет доступна всем пользователям.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>Отмена</AlertDialogCancel>
                      <Button
                        size="sm"
                        onClick={() => {
                          setPolicyExpr('')
                          setBuilderKey((k) => k + 1)
                          setClearOpen(false)
                        }}
                      >
                        Очистить
                      </Button>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              )}
              <Button
                size="sm"
                onClick={handleSavePolicy}
                disabled={savingPolicy || !policyChanged}
              >
                {savingPolicy ? 'Сохранение...' : 'Сохранить'}
              </Button>
            </div>
          </CardContent>
        </CollapsibleContent>
      </Collapsible>
    </Card>
  )
}
