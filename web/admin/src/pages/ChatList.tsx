import { useEffect, useState } from 'react'
import { MessageSquare, Send, X, CheckCircle2, XCircle } from 'lucide-react'
import { api, ChatReference, BroadcastResult } from '@/api/client'
import { toast } from 'sonner'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '@/components/ui/card'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Separator } from '@/components/ui/separator'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

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

const channelVariant: Record<string, 'default' | 'secondary' | 'outline'> = {
  TELEGRAM: 'default',
  DISCORD: 'secondary',
}

const MAX_BROADCAST_LENGTH = 4096

export default function ChatList() {
  const [chats, setChats] = useState<ChatReference[]>([])
  const [loading, setLoading] = useState(true)
  const [filterChannel, setFilterChannel] = useState('')
  const [filterKind, setFilterKind] = useState('')
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [broadcastText, setBroadcastText] = useState('')
  const [sending, setSending] = useState(false)
  const [results, setResults] = useState<BroadcastResult[] | null>(null)

  const load = async () => {
    setLoading(true)
    try {
      const params: { channel_type?: string; chat_kind?: string } = {}
      if (filterChannel) params.channel_type = filterChannel
      if (filterKind) params.chat_kind = filterKind
      setChats(await api.listChats(params))
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to load chats')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [filterChannel, filterKind])

  const toggleSelect = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleAll = () => {
    if (selected.size === chats.length) setSelected(new Set())
    else setSelected(new Set(chats.map((c) => c.id)))
  }

  const handleBroadcast = async () => {
    if (!broadcastText.trim() || selected.size === 0) return
    setSending(true)
    setResults(null)
    try {
      setResults(await api.broadcast(Array.from(selected), broadcastText.trim()))
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Broadcast failed')
    } finally {
      setSending(false)
    }
  }

  const sentCount = results?.filter((r) => r.status === 'sent').length ?? 0
  const totalCount = results?.length ?? 0

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Чаты</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Управление чатами и рассылка сообщений
        </p>
      </div>

      {/* Filters in a Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-wrap gap-6 items-end">
            <div className="space-y-2">
              <Label htmlFor="filter-channel">Мессенджер</Label>
              <Select
                value={filterChannel}
                onValueChange={(v) => setFilterChannel(v === '_all' ? '' : v)}
              >
                <SelectTrigger id="filter-channel" className="w-[180px]">
                  <SelectValue placeholder="Все" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="_all">Все</SelectItem>
                  {CHANNEL_TYPES.filter(Boolean).map((t) => (
                    <SelectItem key={t} value={t}>
                      {channelLabel[t] ?? t}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="filter-kind">Тип чата</Label>
              <Select
                value={filterKind}
                onValueChange={(v) => setFilterKind(v === '_all' ? '' : v)}
              >
                <SelectTrigger id="filter-kind" className="w-[180px]">
                  <SelectValue placeholder="Все" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="_all">Все</SelectItem>
                  {CHAT_KINDS.filter(Boolean).map((k) => (
                    <SelectItem key={k} value={k}>
                      {kindLabel[k] ?? k}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Chat table */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">Список чатов</CardTitle>
            {selected.size > 0 && (
              <Badge variant="secondary">
                Выбрано: {selected.size}
              </Badge>
            )}
          </div>
          {!loading && chats.length > 0 && (
            <CardDescription>
              Найдено {chats.length} {chats.length === 1 ? 'чат' : chats.length < 5 ? 'чата' : 'чатов'}
            </CardDescription>
          )}
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[40px] pl-4">
                  <Checkbox
                    checked={chats.length > 0 && selected.size === chats.length}
                    onCheckedChange={toggleAll}
                    disabled={loading || chats.length === 0}
                  />
                </TableHead>
                <TableHead>ID</TableHead>
                <TableHead>Название</TableHead>
                <TableHead>Мессенджер</TableHead>
                <TableHead>Тип</TableHead>
                <TableHead>Platform ID</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                // Skeleton loading rows
                Array.from({ length: 5 }).map((_, i) => (
                  <TableRow key={`skeleton-${i}`}>
                    <TableCell className="pl-4">
                      <Skeleton className="h-4 w-4 rounded" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-4 w-10" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-4 w-32" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-5 w-20 rounded-full" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-5 w-16 rounded-full" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-4 w-24" />
                    </TableCell>
                  </TableRow>
                ))
              ) : chats.length === 0 ? (
                // Empty state
                <TableRow>
                  <TableCell colSpan={6} className="h-48">
                    <div className="flex flex-col items-center justify-center gap-3 text-center">
                      <div className="rounded-full bg-muted p-3">
                        <MessageSquare className="h-6 w-6 text-muted-foreground" />
                      </div>
                      <div>
                        <p className="text-sm font-medium">Чаты не найдены</p>
                        <p className="text-sm text-muted-foreground mt-1">
                          Попробуйте изменить параметры фильтрации или добавьте бота в чат
                        </p>
                      </div>
                    </div>
                  </TableCell>
                </TableRow>
              ) : (
                chats.map((chat) => (
                  <TableRow
                    key={chat.id}
                    className={cn('cursor-pointer', selected.has(chat.id) && 'bg-muted/50')}
                    data-state={selected.has(chat.id) ? 'selected' : undefined}
                    onClick={() => toggleSelect(chat.id)}
                  >
                    <TableCell className="pl-4">
                      <Checkbox
                        checked={selected.has(chat.id)}
                        onCheckedChange={() => toggleSelect(chat.id)}
                        onClick={(e) => e.stopPropagation()}
                      />
                    </TableCell>
                    <TableCell className="text-sm">{chat.id}</TableCell>
                    <TableCell className="text-sm font-medium">
                      {chat.title || <span className="text-muted-foreground italic">Без названия</span>}
                    </TableCell>
                    <TableCell>
                      <Badge variant={channelVariant[chat.channel_type] ?? 'outline'}>
                        {channelLabel[chat.channel_type] ?? chat.channel_type}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {kindLabel[chat.chat_kind] ?? chat.chat_kind}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground font-mono">
                      {chat.platform_chat_id}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Broadcast panel */}
      {selected.size > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg flex items-center gap-2">
              <Send className="h-4 w-4" />
              Рассылка ({selected.size} {selected.size === 1 ? 'чат' : selected.size < 5 ? 'чата' : 'чатов'})
            </CardTitle>
            <CardDescription>
              Сообщение будет отправлено во все выбранные чаты
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Textarea
                placeholder="Введите текст сообщения..."
                className="min-h-[120px]"
                value={broadcastText}
                onChange={(e) => setBroadcastText(e.target.value)}
                maxLength={MAX_BROADCAST_LENGTH}
              />
              <div className="flex justify-end">
                <span
                  className={cn(
                    'text-xs tabular-nums',
                    broadcastText.length > MAX_BROADCAST_LENGTH * 0.9
                      ? 'text-destructive'
                      : 'text-muted-foreground',
                  )}
                >
                  {broadcastText.length} / {MAX_BROADCAST_LENGTH}
                </span>
              </div>
            </div>

            {results && (
              <>
                <Separator />
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <h3 className="text-sm font-medium">Результат рассылки</h3>
                    <Badge variant={sentCount === totalCount ? 'default' : 'secondary'}>
                      Отправлено {sentCount} из {totalCount}
                    </Badge>
                  </div>
                  <div className="space-y-1">
                    {results.map((r, i) => (
                      <div
                        key={i}
                        className={cn(
                          'flex items-center gap-2 text-sm px-3 py-2 rounded-md',
                          r.status === 'sent'
                            ? 'bg-green-50 text-green-700 dark:bg-green-950/30 dark:text-green-400'
                            : 'bg-red-50 text-red-700 dark:bg-red-950/30 dark:text-red-400',
                        )}
                      >
                        {r.status === 'sent' ? (
                          <CheckCircle2 className="h-4 w-4 shrink-0" />
                        ) : (
                          <XCircle className="h-4 w-4 shrink-0" />
                        )}
                        <span>
                          Chat #{r.chat_id} ({r.channel_type}):{' '}
                          {r.status === 'sent' ? 'Отправлено' : `Ошибка: ${r.error}`}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              </>
            )}
          </CardContent>
          <CardFooter className="gap-3">
            <Button
              disabled={!broadcastText.trim() || sending}
              onClick={handleBroadcast}
            >
              <Send className="h-4 w-4 mr-2" />
              {sending ? 'Отправка...' : 'Отправить'}
            </Button>
            <Button
              variant="ghost"
              onClick={() => {
                setSelected(new Set())
                setResults(null)
              }}
            >
              <X className="h-4 w-4 mr-2" />
              Отменить выбор
            </Button>
          </CardFooter>
        </Card>
      )}
    </div>
  )
}
