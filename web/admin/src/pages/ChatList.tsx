import { useEffect, useState } from 'react'
import { api, ChatReference, BroadcastResult } from '../api/client'

const CHANNEL_TYPES = ['', 'TELEGRAM', 'DISCORD'] as const
const CHAT_KINDS = ['', 'GROUP', 'PRIVATE', 'CHANNEL'] as const

const kindLabel: Record<string, string> = {
  GROUP: 'Группа',
  PRIVATE: 'Личный',
  CHANNEL: 'Канал',
}

const channelLabel: Record<string, string> = {
  TELEGRAM: 'Telegram',
  DISCORD: 'Discord',
}

export default function ChatList() {
  const [chats, setChats] = useState<ChatReference[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [filterChannel, setFilterChannel] = useState('')
  const [filterKind, setFilterKind] = useState('')

  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [broadcastText, setBroadcastText] = useState('')
  const [sending, setSending] = useState(false)
  const [results, setResults] = useState<BroadcastResult[] | null>(null)

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const params: { channel_type?: string; chat_kind?: string } = {}
      if (filterChannel) params.channel_type = filterChannel
      if (filterKind) params.chat_kind = filterKind
      const data = await api.listChats(params)
      setChats(data)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load chats')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [filterChannel, filterKind])

  const toggleSelect = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleAll = () => {
    if (selected.size === chats.length) {
      setSelected(new Set())
    } else {
      setSelected(new Set(chats.map((c) => c.id)))
    }
  }

  const handleBroadcast = async () => {
    if (!broadcastText.trim() || selected.size === 0) return
    setSending(true)
    setResults(null)
    try {
      const res = await api.broadcast(Array.from(selected), broadcastText.trim())
      setResults(res)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Broadcast failed')
    } finally {
      setSending(false)
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-gray-900">Чаты</h1>

      {/* Filters */}
      <div className="flex gap-4 items-end">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Мессенджер</label>
          <select
            className="border border-gray-300 rounded-md px-3 py-2 text-sm bg-white"
            value={filterChannel}
            onChange={(e) => setFilterChannel(e.target.value)}
          >
            <option value="">Все</option>
            {CHANNEL_TYPES.filter(Boolean).map((t) => (
              <option key={t} value={t}>
                {channelLabel[t] ?? t}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Тип чата</label>
          <select
            className="border border-gray-300 rounded-md px-3 py-2 text-sm bg-white"
            value={filterKind}
            onChange={(e) => setFilterKind(e.target.value)}
          >
            <option value="">Все</option>
            {CHAT_KINDS.filter(Boolean).map((k) => (
              <option key={k} value={k}>
                {kindLabel[k] ?? k}
              </option>
            ))}
          </select>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-md text-sm">
          {error}
        </div>
      )}

      {/* Chat table */}
      <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left">
                <input
                  type="checkbox"
                  checked={chats.length > 0 && selected.size === chats.length}
                  onChange={toggleAll}
                  className="rounded border-gray-300"
                />
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">ID</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Название</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Мессенджер</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Тип</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Platform ID</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {loading ? (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-500">
                  Загрузка...
                </td>
              </tr>
            ) : chats.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-500">
                  Чаты не найдены
                </td>
              </tr>
            ) : (
              chats.map((chat) => (
                <tr
                  key={chat.id}
                  className={`hover:bg-gray-50 cursor-pointer ${selected.has(chat.id) ? 'bg-blue-50' : ''}`}
                  onClick={() => toggleSelect(chat.id)}
                >
                  <td className="px-4 py-3">
                    <input
                      type="checkbox"
                      checked={selected.has(chat.id)}
                      onChange={() => toggleSelect(chat.id)}
                      onClick={(e) => e.stopPropagation()}
                      className="rounded border-gray-300"
                    />
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-900">{chat.id}</td>
                  <td className="px-4 py-3 text-sm text-gray-900 font-medium">
                    {chat.title || <span className="text-gray-400 italic">Без названия</span>}
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                        chat.channel_type === 'TELEGRAM'
                          ? 'bg-blue-100 text-blue-800'
                          : 'bg-indigo-100 text-indigo-800'
                      }`}
                    >
                      {channelLabel[chat.channel_type] ?? chat.channel_type}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800">
                      {kindLabel[chat.chat_kind] ?? chat.chat_kind}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-500 font-mono">{chat.platform_chat_id}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Broadcast panel */}
      {selected.size > 0 && (
        <div className="bg-white border border-gray-200 rounded-lg p-6 space-y-4">
          <h2 className="text-lg font-medium text-gray-900">
            Рассылка ({selected.size} {selected.size === 1 ? 'чат' : selected.size < 5 ? 'чата' : 'чатов'})
          </h2>
          <textarea
            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm min-h-[120px] focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            placeholder="Введите текст сообщения..."
            value={broadcastText}
            onChange={(e) => setBroadcastText(e.target.value)}
          />
          <div className="flex items-center gap-4">
            <button
              className="bg-blue-600 text-white px-4 py-2 rounded-md text-sm font-medium hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              disabled={!broadcastText.trim() || sending}
              onClick={handleBroadcast}
            >
              {sending ? 'Отправка...' : 'Отправить'}
            </button>
            <button
              className="text-gray-600 hover:text-gray-900 text-sm"
              onClick={() => {
                setSelected(new Set())
                setResults(null)
              }}
            >
              Отменить выбор
            </button>
          </div>

          {results && (
            <div className="space-y-2">
              <h3 className="text-sm font-medium text-gray-700">Результат рассылки:</h3>
              <div className="space-y-1">
                {results.map((r, i) => (
                  <div
                    key={i}
                    className={`text-sm px-3 py-2 rounded ${
                      r.status === 'sent' ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'
                    }`}
                  >
                    Chat #{r.chat_id} ({r.channel_type}): {r.status === 'sent' ? 'Отправлено' : `Ошибка: ${r.error}`}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
