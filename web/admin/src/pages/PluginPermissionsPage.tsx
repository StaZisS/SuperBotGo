import { useEffect, useState, useCallback, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, PluginDetail as PluginDetailType, PluginPermissionsDetail, HostPermissionInfo, DeclaredPermission } from '../api/client'
import { toast } from '../components/Toast'

const CATEGORY_LABELS: Record<string, string> = {
  database: 'База данных',
  network: 'Сеть',
  plugins: 'Плагины',
}

const CATEGORY_ORDER = ['database', 'network', 'plugins']

export default function PluginPermissionsPage() {
  const { id } = useParams<{ id: string }>()
  const [plugin, setPlugin] = useState<PluginDetailType | null>(null)
  const [permDetail, setPermDetail] = useState<PluginPermissionsDetail | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)

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
      .catch((e: Error) => toast(e.message, 'error'))
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
      toast('Права сохранены')
      setDirty(false)
    } catch (e: unknown) {
      toast((e as Error).message, 'error')
    } finally {
      setSaving(false)
    }
  }

  if (loading && !permDetail) {
    return <div className="text-gray-400 py-8 text-center">Загрузка...</div>
  }

  if (!permDetail) {
    return <div className="text-gray-400 py-8 text-center">Плагин не найден</div>
  }

  const byCategory = useMemo(() => {
    const map = new Map<string, HostPermissionInfo[]>()
    for (const p of permDetail.all_available) {
      const list = map.get(p.category) || []
      list.push(p)
      map.set(p.category, list)
    }
    return map
  }, [permDetail])

  const callTargets = useMemo(() => {
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
  }, [permDetail, declaredMap])

  const isDisabled = plugin?.status === 'disabled'

  return (
    <div className="space-y-6">
      {}
      <div>
        <div className="flex items-center gap-3 mb-1">
          <Link to={`/admin/plugins/${id}`} className="text-gray-400 hover:text-gray-600 text-sm">
            &larr; Назад
          </Link>
        </div>
        <h2 className="text-lg font-semibold">Права плагина</h2>
        <p className="text-sm text-gray-500">{plugin?.name || id}</p>
      </div>

      {isDisabled && permDetail.declared.length === 0 && (
        <div className="bg-yellow-50 border border-yellow-200 rounded-xl p-4 text-sm text-yellow-800">
          Плагин отключён. Объявленные разрешения недоступны.
        </div>
      )}

      {}
      <div className="space-y-6">
        {CATEGORY_ORDER.map((cat) => {
          const perms = byCategory.get(cat)
          if (!perms || perms.length === 0) return null
          return (
            <div key={cat} className="bg-white rounded-xl border border-gray-200 p-6">
              <h3 className="text-sm font-semibold text-gray-700 mb-3">
                {CATEGORY_LABELS[cat] || cat}
              </h3>
              <div className="space-y-2">
                {perms.map((p) => {
                  const decl = declaredMap.get(p.key)
                  const required = decl?.required === true
                  const checked = required || selected.has(p.key)
                  return (
                    <label
                      key={p.key}
                      className={`flex items-start gap-3 p-2 rounded ${
                        required ? 'opacity-75' : 'hover:bg-gray-50 cursor-pointer'
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={checked}
                        disabled={required}
                        onChange={() => toggle(p.key)}
                        className="mt-0.5 h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                      />
                      <div className="min-w-0">
                        <div className="text-sm font-mono break-all">
                          {p.key}
                          {required && (
                            <span className="ml-2 text-xs text-red-600 font-medium font-sans">
                              Обязательно
                            </span>
                          )}
                          {decl && !required && (
                            <span className="ml-2 text-xs text-gray-400 font-sans">
                              Опционально
                            </span>
                          )}
                        </div>
                        <div className="text-xs text-gray-500 mt-0.5">
                          {decl?.description || p.description}
                        </div>
                      </div>
                    </label>
                  )
                })}
              </div>
            </div>
          )
        })}

        {}
        {callTargets.size > 0 && (
          <div className="bg-white rounded-xl border border-gray-200 p-6">
            <h3 className="text-sm font-semibold text-gray-700 mb-3">
              Вызов других плагинов
            </h3>
            <div className="space-y-2">
              {Array.from(callTargets.entries()).map(([permKey, target]) => {
                const required = target.declared?.required === true
                const checked = required || selected.has(permKey)
                return (
                  <label
                    key={permKey}
                    className={`flex items-start gap-3 p-2 rounded ${
                      required ? 'opacity-75' : 'hover:bg-gray-50 cursor-pointer'
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      disabled={required}
                      onChange={() => toggle(permKey)}
                      className="mt-0.5 h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                    />
                    <div className="min-w-0">
                      <div className="text-sm font-mono break-all">
                        {permKey}
                        {required && (
                          <span className="ml-2 text-xs text-red-600 font-medium font-sans">
                            Обязательно
                          </span>
                        )}
                        {target.declared && !required && (
                          <span className="ml-2 text-xs text-gray-400 font-sans">
                            Опционально
                          </span>
                        )}
                      </div>
                      <div className="text-xs text-gray-500 mt-0.5">
                        {target.declared?.description || `Вызов плагина ${target.name}`}
                      </div>
                    </div>
                  </label>
                )
              })}
            </div>
          </div>
        )}
      </div>

      {}
      <div className="flex gap-3">
        <button
          onClick={handleSave}
          disabled={saving || !dirty}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors"
        >
          {saving ? 'Сохранение...' : 'Сохранить'}
        </button>
        {dirty && (
          <button
            onClick={load}
            disabled={saving}
            className="px-4 py-2 border border-gray-300 rounded-lg text-sm hover:bg-gray-50 transition-colors"
          >
            Отменить изменения
          </button>
        )}
      </div>
    </div>
  )
}
