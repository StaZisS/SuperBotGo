import { useEffect, useState, useRef } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { ArrowLeft, Settings } from 'lucide-react'
import { api, PluginDetail } from '@/api/client'
import JsonSchemaForm, { validateSchema } from '@/components/JsonSchemaForm'
import { toast } from 'sonner'
import { getErrorMessage } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

interface ConfigSchema {
  type?: string
  properties?: Record<string, unknown>
  required?: string[]
  [key: string]: unknown
}

export default function PluginConfig() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [plugin, setPlugin] = useState<PluginDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [config, setConfig] = useState<Record<string, unknown>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState(false)
  const initialConfigRef = useRef<string>('')
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)

  useEffect(() => {
    if (!id) return
    setLoading(true)
    api
      .getPlugin(id)
      .then((p) => {
        setPlugin(p)
        if (p.config && typeof p.config === 'object') {
          const cfg = p.config as Record<string, unknown>
          setConfig(cfg)
          initialConfigRef.current = JSON.stringify(cfg)
        } else {
          initialConfigRef.current = JSON.stringify({})
        }
        setHasUnsavedChanges(false)
      })
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [id])

  const schema = plugin?.meta?.config_schema as ConfigSchema | undefined
  const hasSchema = schema?.properties && Object.keys(schema.properties).length > 0

  const handleConfigChange = (v: Record<string, unknown>) => {
    setConfig(v)
    setHasUnsavedChanges(JSON.stringify(v) !== initialConfigRef.current)
    if (Object.keys(errors).length > 0) setErrors({})
  }

  const handleSave = async () => {
    if (!id) return

    if (hasSchema && schema) {
      const validationErrors = validateSchema(
        schema as Parameters<typeof validateSchema>[0],
        config,
      )
      if (Object.keys(validationErrors).length > 0) {
        setErrors(validationErrors)
        toast.error('Исправьте ошибки валидации')
        return
      }
    }
    setErrors({})

    setSaving(true)
    try {
      await api.updateConfig(id, config)
      toast.success('Конфигурация сохранена')
      navigate(`/admin/plugins/${id}`)
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="space-y-6">
        <div>
          <Skeleton className="h-5 w-36" />
        </div>
        <Card>
          <CardHeader>
            <Skeleton className="h-6 w-48" />
            <Skeleton className="h-4 w-32 mt-1" />
          </CardHeader>
          <CardContent className="space-y-5">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="space-y-2">
                <Skeleton className="h-4 w-28" />
                <Skeleton className="h-10 w-full" />
              </div>
            ))}
          </CardContent>
          <CardFooter className="gap-3">
            <Skeleton className="h-10 w-28" />
            <Skeleton className="h-10 w-24" />
          </CardFooter>
        </Card>
      </div>
    )
  }

  if (!plugin) {
    return <div className="text-muted-foreground py-8 text-center">Плагин не найден</div>
  }

  return (
    <div className="space-y-6">
      <div>
        <Button variant="link" asChild className="px-0 text-muted-foreground hover:text-foreground">
          <Link to={`/admin/plugins/${id}`}>
            <ArrowLeft className="mr-1 h-4 w-4" />
            Назад к плагину
          </Link>
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Настройка: {plugin.name || plugin.id}</CardTitle>
          <CardDescription>{plugin.id}</CardDescription>
        </CardHeader>

        <CardContent>
          {hasSchema ? (
            <JsonSchemaForm
              schema={schema as Parameters<typeof JsonSchemaForm>[0]['schema']}
              value={config}
              onChange={handleConfigChange}
              errors={errors}
            />
          ) : (
            <div className="flex flex-col items-center justify-center py-10 text-center">
              <div className="rounded-full bg-muted p-4 mb-4">
                <Settings className="h-8 w-8 text-muted-foreground" />
              </div>
              <p className="text-sm font-medium text-muted-foreground mb-1">
                Нет настраиваемых параметров
              </p>
              <p className="text-xs text-muted-foreground mb-4">
                У этого плагина нет настраиваемых параметров
              </p>
              <Button
                variant="outline"
                size="sm"
                onClick={() => navigate(`/admin/plugins/${id}`)}
              >
                <ArrowLeft className="mr-1 h-3.5 w-3.5" />
                Вернуться
              </Button>
            </div>
          )}
        </CardContent>

        {hasSchema && (
          <CardFooter className="gap-3">
            <div className="relative inline-flex items-center">
              <Button
                onClick={handleSave}
                disabled={saving}
              >
                {saving ? 'Сохранение...' : 'Сохранить'}
              </Button>
              {hasUnsavedChanges && (
                <span className="absolute -top-1 -right-1 h-3 w-3 rounded-full bg-orange-500 border-2 border-background" />
              )}
            </div>
            <Button
              variant="outline"
              onClick={() => navigate(`/admin/plugins/${id}`)}
            >
              Отмена
            </Button>
          </CardFooter>
        )}
      </Card>
    </div>
  )
}
