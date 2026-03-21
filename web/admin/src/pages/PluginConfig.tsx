import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api, PluginDetail } from '../api/client'
import JsonSchemaForm, { validateSchema } from '../components/JsonSchemaForm'
import { toast } from '../components/Toast'

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

  useEffect(() => {
    if (!id) return
    setLoading(true)
    api
      .getPlugin(id)
      .then((p) => {
        setPlugin(p)
        if (p.config && typeof p.config === 'object') {
          setConfig(p.config as Record<string, unknown>)
        }
      })
      .catch((e: Error) => toast(e.message, 'error'))
      .finally(() => setLoading(false))
  }, [id])

  const schema = plugin?.meta?.config_schema as ConfigSchema | undefined
  const hasSchema = schema?.properties && Object.keys(schema.properties).length > 0

  const handleSave = async () => {
    if (!id) return

    if (hasSchema && schema) {
      const validationErrors = validateSchema(
        schema as Parameters<typeof validateSchema>[0],
        config,
      )
      if (Object.keys(validationErrors).length > 0) {
        setErrors(validationErrors)
        toast('Исправьте ошибки валидации', 'error')
        return
      }
    }
    setErrors({})

    setSaving(true)
    try {
      await api.updateConfig(id, config)
      toast('Конфигурация сохранена')
      navigate(`/admin/plugins/${id}`)
    } catch (e: unknown) {
      toast((e as Error).message, 'error')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <div className="text-gray-400 py-8 text-center">Загрузка...</div>
  }

  if (!plugin) {
    return <div className="text-gray-400 py-8 text-center">Плагин не найден</div>
  }

  return (
    <div className="space-y-6">
      <div>
        <Link to={`/admin/plugins/${id}`} className="text-gray-400 hover:text-gray-600 text-sm">
          &larr; Назад к плагину
        </Link>
        <h2 className="text-lg font-semibold mt-1">Настройка: {plugin.name || plugin.id}</h2>
        <p className="text-sm text-gray-500">{plugin.id}</p>
      </div>

      <div className="bg-white rounded-xl border border-gray-200 p-6">
        {hasSchema ? (
          <JsonSchemaForm
            schema={schema as Parameters<typeof JsonSchemaForm>[0]['schema']}
            value={config}
            onChange={(v) => {
              setConfig(v)
              if (Object.keys(errors).length > 0) setErrors({})
            }}
            errors={errors}
          />
        ) : (
          <p className="text-gray-400 text-sm">У этого плагина нет настраиваемых параметров.</p>
        )}
      </div>

      <div className="flex gap-3">
        <button
          onClick={handleSave}
          disabled={saving || (!hasSchema)}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors"
        >
          {saving ? 'Сохранение...' : 'Сохранить'}
        </button>
        <button
          onClick={() => navigate(`/admin/plugins/${id}`)}
          className="px-4 py-2 border border-gray-300 rounded-lg text-sm hover:bg-gray-50"
        >
          Отмена
        </button>
      </div>
    </div>
  )
}
