import { useEffect, useState, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { api, PluginInfo } from '../api/client'
import PluginStatusBadge from '../components/PluginStatusBadge'
import { toast } from '../components/Toast'

export default function PluginList() {
  const [plugins, setPlugins] = useState<PluginInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('all')

  useEffect(() => {
    setLoading(true)
    api
      .listPlugins()
      .then(setPlugins)
      .catch((e: Error) => toast(e.message, 'error'))
      .finally(() => setLoading(false))
  }, [])

  const filtered = useMemo(() => {
    const q = search.toLowerCase()
    return plugins.filter((p) => {
      if (typeFilter !== 'all' && p.type !== typeFilter) return false
      if (statusFilter !== 'all' && p.status !== statusFilter) return false
      if (q && !p.name.toLowerCase().includes(q) && !p.id.toLowerCase().includes(q)) return false
      return true
    })
  }, [plugins, typeFilter, statusFilter, search])

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-lg font-semibold">Плагины</h2>
        <Link
          to="/admin/plugins/upload"
          className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition-colors"
        >
          Загрузить плагин
        </Link>
      </div>

      {}
      <div className="flex flex-wrap gap-3 mb-4">
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Поиск по названию или ID..."
          className="px-3 py-1.5 border border-gray-300 rounded-lg text-sm w-full sm:w-64 focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        <select
          value={typeFilter}
          onChange={(e) => setTypeFilter(e.target.value)}
          className="px-3 py-1.5 border border-gray-300 rounded-lg text-sm"
        >
          <option value="all">Все типы</option>
          <option value="go">Go</option>
          <option value="wasm">Wasm</option>
        </select>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="px-3 py-1.5 border border-gray-300 rounded-lg text-sm"
        >
          <option value="all">Все статусы</option>
          <option value="active">Активные</option>
          <option value="disabled">Отключённые</option>
          <option value="error">С ошибкой</option>
        </select>
      </div>

      {}
      <div className="bg-white rounded-xl border border-gray-200 overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
            <tr>
              <th className="px-4 py-3 text-left">Название</th>
              <th className="px-4 py-3 text-left hidden sm:table-cell">Версия</th>
              <th className="px-4 py-3 text-left">Тип</th>
              <th className="px-4 py-3 text-left">Статус</th>
              <th className="px-4 py-3 text-right hidden sm:table-cell">Команды</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {loading && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-gray-400">
                  Загрузка...
                </td>
              </tr>
            )}
            {!loading && filtered.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-gray-400">
                  {plugins.length === 0 ? 'Плагины не установлены' : 'Нет плагинов, подходящих под фильтры'}
                </td>
              </tr>
            )}
            {!loading &&
              filtered.map((p) => (
                <tr key={p.id} className="hover:bg-gray-50 transition-colors">
                  <td className="px-4 py-3">
                    <Link to={`/admin/plugins/${p.id}`} className="text-blue-600 hover:underline font-medium">
                      {p.name || p.id}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-gray-500 hidden sm:table-cell">{p.version || '-'}</td>
                  <td className="px-4 py-3">
                    <span
                      className={`px-2 py-0.5 rounded text-xs font-medium ${
                        p.type === 'wasm' ? 'bg-purple-100 text-purple-700' : 'bg-blue-100 text-blue-700'
                      }`}
                    >
                      {p.type}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <PluginStatusBadge status={p.status} />
                  </td>
                  <td className="px-4 py-3 text-right text-gray-500 hidden sm:table-cell">{p.commands}</td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
