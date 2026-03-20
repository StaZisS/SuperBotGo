import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api, PluginDetail as PluginDetailType } from '../api/client'
import PluginStatusBadge from '../components/PluginStatusBadge'
import WasmUploader from '../components/WasmUploader'
import { toast } from '../components/Toast'

export default function PluginDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [plugin, setPlugin] = useState<PluginDetailType | null>(null)
  const [loading, setLoading] = useState(true)
  const [showDelete, setShowDelete] = useState(false)
  const [showUpdate, setShowUpdate] = useState(false)
  const [actionLoading, setActionLoading] = useState(false)

  const load = useCallback(() => {
    if (!id) return
    setLoading(true)
    api
      .getPlugin(id)
      .then(setPlugin)
      .catch((e: Error) => toast(e.message, 'error'))
      .finally(() => setLoading(false))
  }, [id])

  useEffect(() => { load() }, [load])

  const handleToggle = async () => {
    if (!id || !plugin) return
    const wasActive = plugin.status === 'active'
    // Optimistic update
    setPlugin((prev) => prev ? { ...prev, status: wasActive ? 'disabled' : 'active' } : prev)
    try {
      if (wasActive) {
        await api.disablePlugin(id)
        toast('Plugin disabled')
      } else {
        await api.enablePlugin(id)
        toast('Plugin enabled')
      }
      load()
    } catch (e: unknown) {
      // Revert optimistic update
      setPlugin((prev) => prev ? { ...prev, status: wasActive ? 'active' : 'disabled' } : prev)
      toast((e as Error).message, 'error')
    }
  }

  const handleDelete = async () => {
    if (!id) return
    setActionLoading(true)
    try {
      await api.deletePlugin(id)
      toast('Plugin deleted')
      navigate('/admin/plugins')
    } catch (e: unknown) {
      toast((e as Error).message, 'error')
    } finally {
      setActionLoading(false)
      setShowDelete(false)
    }
  }

  const handleUpdate = async (file: File) => {
    if (!id) return
    setActionLoading(true)
    try {
      await api.updatePlugin(id, file)
      toast('Plugin updated')
      setShowUpdate(false)
      load()
    } catch (e: unknown) {
      toast((e as Error).message, 'error')
    } finally {
      setActionLoading(false)
    }
  }

  if (loading && !plugin) {
    return <div className="text-gray-400 py-8 text-center">Loading...</div>
  }

  if (!plugin) {
    return <div className="text-gray-400 py-8 text-center">Plugin not found</div>
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-3 mb-1">
            <Link to="/admin/plugins" className="text-gray-400 hover:text-gray-600 text-sm">
              &larr; Back
            </Link>
          </div>
          <h2 className="text-lg font-semibold truncate">{plugin.name || plugin.id}</h2>
          <p className="text-sm text-gray-500">
            {plugin.id} {plugin.version && <span>&middot; v{plugin.version}</span>}
          </p>
        </div>
        <PluginStatusBadge status={plugin.status || 'disabled'} />
      </div>

      {/* Info card */}
      <div className="bg-white rounded-xl border border-gray-200 p-6 space-y-4">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
          <div>
            <span className="text-gray-500 block text-xs uppercase tracking-wide">Type</span>
            <div className="font-medium mt-0.5">{plugin.type || 'wasm'}</div>
          </div>
          <div>
            <span className="text-gray-500 block text-xs uppercase tracking-wide">Version</span>
            <div className="font-medium mt-0.5">{plugin.version || '-'}</div>
          </div>
          {plugin.wasm_hash && (
            <div className="col-span-2 md:col-span-1">
              <span className="text-gray-500 block text-xs uppercase tracking-wide">Hash</span>
              <div className="font-mono text-xs mt-0.5 truncate" title={plugin.wasm_hash}>
                {plugin.wasm_hash}
              </div>
            </div>
          )}
          {plugin.installed_at && (
            <div>
              <span className="text-gray-500 block text-xs uppercase tracking-wide">Installed</span>
              <div className="font-medium mt-0.5">{new Date(plugin.installed_at).toLocaleDateString()}</div>
            </div>
          )}
          {plugin.updated_at && (
            <div>
              <span className="text-gray-500 block text-xs uppercase tracking-wide">Updated</span>
              <div className="font-medium mt-0.5">{new Date(plugin.updated_at).toLocaleDateString()}</div>
            </div>
          )}
        </div>

        {/* Commands */}
        {plugin.commands && plugin.commands.length > 0 && (
          <div>
            <h4 className="text-sm font-medium text-gray-700 mb-2">Commands ({plugin.commands.length})</h4>
            <div className="space-y-1">
              {plugin.commands.map((cmd) => (
                <div key={cmd.name} className="flex items-center gap-3 text-sm p-2 bg-gray-50 rounded">
                  <span className="font-mono text-blue-600 shrink-0">/{cmd.name}</span>
                  <span className="text-gray-500 min-w-0 truncate">{cmd.description}</span>
                  {cmd.min_role && (
                    <span className="ml-auto text-xs text-gray-400 shrink-0">{cmd.min_role}</span>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Permissions */}
        {plugin.permissions && plugin.permissions.length > 0 && (
          <div>
            <h4 className="text-sm font-medium text-gray-700 mb-2">Permissions</h4>
            <div className="flex flex-wrap gap-2">
              {plugin.permissions.map((p) => (
                <span key={p} className="px-2 py-1 bg-gray-100 rounded text-xs font-mono">
                  {p}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Actions */}
      <div className="flex flex-wrap gap-3">
        <button
          onClick={handleToggle}
          disabled={actionLoading}
          className="px-4 py-2 border border-gray-300 rounded-lg text-sm hover:bg-gray-50 disabled:opacity-50 transition-colors"
        >
          {plugin.status === 'active' ? 'Disable' : 'Enable'}
        </button>
        <Link
          to={`/admin/plugins/${id}/config`}
          className="px-4 py-2 border border-gray-300 rounded-lg text-sm hover:bg-gray-50 transition-colors"
        >
          Configure
        </Link>
        <button
          onClick={() => setShowUpdate((v) => !v)}
          className="px-4 py-2 border border-gray-300 rounded-lg text-sm hover:bg-gray-50 transition-colors"
        >
          Update .wasm
        </button>
        <button
          onClick={() => setShowDelete(true)}
          className="px-4 py-2 border border-red-300 text-red-600 rounded-lg text-sm hover:bg-red-50 transition-colors"
        >
          Delete
        </button>
      </div>

      {/* Update panel */}
      {showUpdate && (
        <div className="bg-white rounded-xl border border-gray-200 p-6">
          <h3 className="font-medium mb-4">Upload new .wasm</h3>
          <WasmUploader onFile={handleUpdate} loading={actionLoading} />
          <button
            onClick={() => setShowUpdate(false)}
            className="mt-3 text-sm text-gray-500 hover:text-gray-700"
          >
            Cancel
          </button>
        </div>
      )}

      {/* Delete confirmation */}
      {showDelete && (
        <div className="bg-red-50 border border-red-200 rounded-xl p-6">
          <p className="text-sm text-red-800 mb-4">
            Are you sure you want to delete <strong>{plugin.name || plugin.id}</strong>? This action cannot be undone.
          </p>
          <div className="flex gap-3">
            <button
              onClick={handleDelete}
              disabled={actionLoading}
              className="px-4 py-2 bg-red-600 text-white rounded-lg text-sm hover:bg-red-700 disabled:opacity-50 transition-colors"
            >
              {actionLoading ? 'Deleting...' : 'Confirm Delete'}
            </button>
            <button
              onClick={() => setShowDelete(false)}
              disabled={actionLoading}
              className="px-4 py-2 border border-gray-300 rounded-lg text-sm hover:bg-white transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
