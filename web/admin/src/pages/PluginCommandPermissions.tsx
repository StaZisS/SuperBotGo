import { useEffect, useState, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, CommandSetting, PluginDetail } from '../api/client'
import { toast } from '../components/Toast'
import RuleBuilder from '../components/RuleBuilder'

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
      const commands = p.commands ?? p.meta?.commands ?? []

      setRows(
        commands.map((cmd) => {
          const setting = settingMap.get(cmd.name)
          return {
            name: cmd.name,
            description: cmd.description,
            enabled: setting?.enabled ?? true,
            policyExpression: setting?.policy_expression ?? '',
            hasSetting: !!setting,
          }
        }),
      )
    } catch {
      toast('Не удалось загрузить плагин', 'error')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { loadData() }, [loadData])

  if (loading) return <div className="text-gray-500 text-sm">Загрузка...</div>
  if (!plugin) return <div className="text-red-600 text-sm">Плагин не найден</div>

  return (
    <div>
      <div className="mb-6">
        <Link to={`/admin/plugins/${id}`} className="text-sm text-blue-600 hover:underline">
          &larr; Назад к {plugin.name || id}
        </Link>
        <h1 className="text-2xl font-semibold text-gray-900 mt-2">Права доступа к командам</h1>
        <p className="text-sm text-gray-500 mt-1">
          Управление доступом к командам <strong>{plugin.name || id}</strong> через политики доступа.
        </p>
      </div>

      {rows.length === 0 ? (
        <div className="bg-white rounded-xl border border-gray-200 p-6 text-center text-gray-500">
          У этого плагина нет команд.
        </div>
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

  useEffect(() => { setPolicyExpr(row.policyExpression) }, [row.policyExpression])

  const handleToggle = async () => {
    setToggling(true)
    try {
      await api.setCommandEnabled(pluginId, row.name, !row.enabled)
      onUpdate()
    } catch {
      toast('Не удалось переключить команду', 'error')
    } finally {
      setToggling(false)
    }
  }

  const handleSavePolicy = async () => {
    setSavingPolicy(true)
    try {
      await api.setCommandPolicy(pluginId, row.name, policyExpr)
      onUpdate()
      toast('Политика сохранена', 'success')
    } catch {
      toast('Не удалось сохранить политику', 'error')
    } finally {
      setSavingPolicy(false)
    }
  }

  const policyChanged = policyExpr !== row.policyExpression

  return (
    <div className="bg-white rounded-xl border border-gray-200">
      {}
      <div
        className="flex items-center justify-between p-5 cursor-pointer hover:bg-gray-50 rounded-t-xl"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center gap-3">
          <span className={`text-xs transform transition-transform ${expanded ? 'rotate-90' : ''}`}>&#9654;</span>
          <span className="font-mono text-sm font-medium text-gray-900">/{row.name}</span>
          {row.description && <span className="text-sm text-gray-500">{row.description}</span>}
        </div>
        <div className="flex items-center gap-3" onClick={(e) => e.stopPropagation()}>
          {row.policyExpression && (
            <span className="text-xs bg-purple-100 text-purple-700 px-2 py-0.5 rounded">policy</span>
          )}
          <button
            onClick={handleToggle}
            disabled={toggling}
            className={`relative w-9 h-5 rounded-full transition-colors ${
              row.enabled ? 'bg-blue-600' : 'bg-gray-300'
            } ${toggling ? 'opacity-50' : ''}`}
          >
            <span
              className={`absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform ${
                row.enabled ? 'translate-x-4' : ''
              }`}
            />
          </button>
        </div>
      </div>

      {}
      {expanded && (
        <div className={`border-t border-gray-100 p-5 ${!row.enabled ? 'opacity-50 pointer-events-none' : ''}`}>
          <div className="flex items-center justify-between mb-3">
            <h4 className="text-sm font-medium text-gray-700">
              Политика доступа
              {row.policyExpression
                ? <span className="ml-2 text-xs text-purple-600 font-normal">(активна)</span>
                : <span className="ml-2 text-xs text-gray-400 font-normal">(пусто = доступно всем)</span>
              }
            </h4>
          </div>

          <RuleBuilder expression={policyExpr} onChange={setPolicyExpr} />

          <div className="flex justify-end gap-2 mt-3 pt-3 border-t border-gray-100">
            {row.policyExpression && (
              <button
                onClick={() => setPolicyExpr('')}
                className="px-3 py-1.5 text-xs border border-gray-300 text-gray-600 rounded-lg hover:bg-gray-50"
              >
                Очистить
              </button>
            )}
            <button
              onClick={handleSavePolicy}
              disabled={savingPolicy || !policyChanged}
              className="px-4 py-1.5 text-xs bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50"
            >
              {savingPolicy ? 'Сохранение...' : 'Сохранить'}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
