import { useEffect, useState, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api, VersionInfo, PluginDetail as PluginDetailType } from '../api/client'
import { toast } from '../components/Toast'

export default function PluginVersions() {
  const { id } = useParams<{ id: string }>()
  const [plugin, setPlugin] = useState<PluginDetailType | null>(null)
  const [versions, setVersions] = useState<VersionInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState<number | null>(null)
  const [confirmRollback, setConfirmRollback] = useState<number | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<number | null>(null)

  const load = useCallback(() => {
    if (!id) return
    setLoading(true)
    Promise.all([api.getPlugin(id), api.listVersions(id)])
      .then(([p, v]) => {
        setPlugin(p)
        setVersions(v)
      })
      .catch((e: Error) => toast(e.message, 'error'))
      .finally(() => setLoading(false))
  }, [id])

  useEffect(() => { load() }, [load])

  const handleRollback = async (versionId: number) => {
    if (!id) return
    setActionLoading(versionId)
    try {
      const res = await api.rollbackVersion(id, versionId)
      toast(`Откат выполнен на версию ${res.version}`)
      setConfirmRollback(null)
      load()
    } catch (e: unknown) {
      toast((e as Error).message, 'error')
    } finally {
      setActionLoading(null)
    }
  }

  const handleDelete = async (versionId: number) => {
    if (!id) return
    setActionLoading(versionId)
    try {
      await api.deleteVersion(id, versionId)
      toast('Версия удалена')
      setConfirmDelete(null)
      load()
    } catch (e: unknown) {
      toast((e as Error).message, 'error')
    } finally {
      setActionLoading(null)
    }
  }

  const isActive = (ver: VersionInfo) =>
    plugin?.wasm_hash === ver.wasm_hash

  if (loading && !versions.length) {
    return <div className="text-gray-400 py-8 text-center">Загрузка...</div>
  }

  return (
    <div className="space-y-6">
      {}
      <div className="min-w-0">
        <div className="flex items-center gap-3 mb-1">
          <Link to={`/admin/plugins/${id}`} className="text-gray-400 hover:text-gray-600 text-sm">
            &larr; {plugin?.name || id}
          </Link>
        </div>
        <h2 className="text-lg font-semibold">История версий</h2>
        <p className="text-sm text-gray-500">
          {versions.length} {versions.length === 1 ? 'версия' : versions.length < 5 ? 'версии' : 'версий'}
        </p>
      </div>

      {versions.length === 0 && (
        <div className="text-gray-400 py-8 text-center">Нет сохранённых версий</div>
      )}

      {}
      <div className="space-y-3">
        {versions.map((ver) => {
          const active = isActive(ver)
          return (
            <div
              key={ver.id}
              className={`bg-white rounded-xl border p-5 space-y-3 ${
                active ? 'border-blue-300 ring-1 ring-blue-100' : 'border-gray-200'
              }`}
            >
              {}
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-3 min-w-0">
                  <span className="font-medium text-sm">
                    {ver.version || 'без версии'}
                  </span>
                  {active && (
                    <span className="px-2 py-0.5 bg-blue-100 text-blue-700 text-xs rounded-full font-medium">
                      текущая
                    </span>
                  )}
                  <span className="text-xs text-gray-400">
                    #{ver.id}
                  </span>
                </div>
                <span className="text-xs text-gray-400 shrink-0">
                  {new Date(ver.created_at).toLocaleString()}
                </span>
              </div>

              {}
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                <div>
                  <span className="text-gray-500 block">Hash</span>
                  <span className="font-mono truncate block" title={ver.wasm_hash}>
                    {ver.wasm_hash.slice(0, 16)}...
                  </span>
                </div>
                {ver.permissions && ver.permissions.length > 0 && (
                  <div>
                    <span className="text-gray-500 block">Разрешения</span>
                    <span>{ver.permissions.length} шт.</span>
                  </div>
                )}
                {ver.changelog && (
                  <div className="md:col-span-2">
                    <span className="text-gray-500 block">Заметка</span>
                    <span>{ver.changelog}</span>
                  </div>
                )}
              </div>

              {}
              {!active && (
                <div className="flex gap-2 pt-1">
                  {confirmRollback === ver.id ? (
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-amber-700">Откатить на эту версию?</span>
                      <button
                        onClick={() => handleRollback(ver.id)}
                        disabled={actionLoading === ver.id}
                        className="px-3 py-1 bg-amber-500 text-white rounded text-xs hover:bg-amber-600 disabled:opacity-50 transition-colors"
                      >
                        {actionLoading === ver.id ? 'Откат...' : 'Подтвердить'}
                      </button>
                      <button
                        onClick={() => setConfirmRollback(null)}
                        disabled={actionLoading === ver.id}
                        className="px-3 py-1 border border-gray-300 rounded text-xs hover:bg-gray-50 transition-colors"
                      >
                        Отмена
                      </button>
                    </div>
                  ) : confirmDelete === ver.id ? (
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-red-700">Удалить эту версию?</span>
                      <button
                        onClick={() => handleDelete(ver.id)}
                        disabled={actionLoading === ver.id}
                        className="px-3 py-1 bg-red-500 text-white rounded text-xs hover:bg-red-600 disabled:opacity-50 transition-colors"
                      >
                        {actionLoading === ver.id ? 'Удаление...' : 'Удалить'}
                      </button>
                      <button
                        onClick={() => setConfirmDelete(null)}
                        disabled={actionLoading === ver.id}
                        className="px-3 py-1 border border-gray-300 rounded text-xs hover:bg-gray-50 transition-colors"
                      >
                        Отмена
                      </button>
                    </div>
                  ) : (
                    <>
                      <button
                        onClick={() => setConfirmRollback(ver.id)}
                        className="px-3 py-1.5 border border-amber-300 text-amber-700 rounded-lg text-xs hover:bg-amber-50 transition-colors"
                      >
                        Откат
                      </button>
                      <button
                        onClick={() => setConfirmDelete(ver.id)}
                        className="px-3 py-1.5 border border-red-300 text-red-600 rounded-lg text-xs hover:bg-red-50 transition-colors"
                      >
                        Удалить
                      </button>
                    </>
                  )}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
