import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, PluginMeta } from '../api/client'
import WasmUploader from '../components/WasmUploader'
import PermissionsPanel from '../components/PermissionsPanel'
import { toast } from '../components/Toast'

export default function PluginUpload() {
  const navigate = useNavigate()
  const [uploading, setUploading] = useState(false)
  const [installing, setInstalling] = useState(false)
  const [meta, setMeta] = useState<PluginMeta | null>(null)
  const [selectedPerms, setSelectedPerms] = useState<string[]>([])

  const handleFile = async (file: File) => {
    setUploading(true)
    try {
      const result = await api.uploadPlugin(file)
      result.commands = result.commands ?? []
      result.permissions = result.permissions ?? []
      // Pre-select required permissions
      const required = result.permissions.filter((p) => p.required).map((p) => p.key)
      setSelectedPerms(required)
      setMeta(result)
      toast('File uploaded, review metadata below')
    } catch (e: unknown) {
      toast((e as Error).message, 'error')
    } finally {
      setUploading(false)
    }
  }

  const handleInstall = async () => {
    if (!meta) return
    setInstalling(true)
    try {
      await api.installPlugin(meta.id, {
        wasm_key: meta.wasm_key,
        config: {},
        permissions: selectedPerms,
      })
      toast('Plugin installed successfully')
      navigate(`/admin/plugins/${meta.id}/config`)
    } catch (e: unknown) {
      toast((e as Error).message, 'error')
    } finally {
      setInstalling(false)
    }
  }

  const handleReset = () => {
    setMeta(null)
    setSelectedPerms([])
  }

  return (
    <div>
      <h2 className="text-lg font-semibold mb-6">Upload Plugin</h2>

      {!meta && <WasmUploader onFile={handleFile} loading={uploading} />}

      {meta && (
        <div className="bg-white rounded-xl border border-gray-200 p-6 space-y-6">
          {/* Plugin info */}
          <div>
            <h3 className="font-semibold text-lg">{meta.name}</h3>
            <p className="text-sm text-gray-500">
              {meta.id} &middot; v{meta.version}
            </p>
            {meta.wasm_hash && (
              <p className="text-xs text-gray-400 font-mono mt-1 truncate">
                SHA: {meta.wasm_hash}
              </p>
            )}
          </div>

          {/* Commands */}
          {meta.commands.length > 0 && (
            <div>
              <h4 className="text-sm font-medium text-gray-700 mb-2">
                Commands ({meta.commands.length})
              </h4>
              <div className="space-y-1">
                {meta.commands.map((cmd) => (
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
          {meta.permissions.length > 0 && (
            <PermissionsPanel
              permissions={meta.permissions}
              selected={selectedPerms}
              onChange={setSelectedPerms}
            />
          )}

          {/* Config schema hint */}
          {meta.config_schema && Object.keys(meta.config_schema).length > 0 && (
            <p className="text-sm text-gray-500 bg-blue-50 border border-blue-100 rounded-lg p-3">
              This plugin has configuration options. You can set them after installation.
            </p>
          )}

          {/* Actions */}
          <div className="flex gap-3 pt-2">
            <button
              onClick={handleInstall}
              disabled={installing}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors"
            >
              {installing ? 'Installing...' : 'Install'}
            </button>
            <button
              onClick={handleReset}
              disabled={installing}
              className="px-4 py-2 border border-gray-300 rounded-lg text-sm hover:bg-gray-50 disabled:opacity-50"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
