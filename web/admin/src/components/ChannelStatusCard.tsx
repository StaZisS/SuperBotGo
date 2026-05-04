import { useEffect, useState } from 'react'
import { api, ChannelStatus } from '@/api/client'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { Radio } from 'lucide-react'
import { HelpTooltip } from '@/components/AdminHelp'

const statusConfig: Record<
  ChannelStatus['status'],
  { label: string; variant: 'success' | 'destructive' | 'secondary'; dot: string }
> = {
  connected: { label: 'Подключён', variant: 'success', dot: 'bg-green-500' },
  disconnected: { label: 'Отключён', variant: 'destructive', dot: 'bg-red-500' },
  not_configured: { label: 'Не настроен', variant: 'secondary', dot: 'bg-gray-400' },
}

export default function ChannelStatusCard() {
  const [channels, setChannels] = useState<ChannelStatus[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api
      .listChannelStatus()
      .then(setChannels)
      .catch((err) => console.error('Failed to load channel status:', err))
      .finally(() => setLoading(false))
  }, [])

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base flex items-center gap-2">
          <Radio className="h-4 w-4" />
          Каналы связи
          <HelpTooltip>
            Статус подключений к мессенджерам. Если канал не подключён, команды
            плагинов в этом канале недоступны пользователям.
          </HelpTooltip>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="flex gap-4">
            <Skeleton className="h-8 w-36" />
            <Skeleton className="h-8 w-36" />
          </div>
        ) : (
          <div className="flex flex-wrap gap-4">
            {channels.map((ch) => {
              const cfg = statusConfig[ch.status] ?? statusConfig.not_configured
              return (
                <div
                  key={ch.type}
                  className="flex items-center gap-2.5 rounded-lg border px-4 py-2"
                >
                  <span className="text-sm font-medium">{ch.name}</span>
                  <Badge variant={cfg.variant} className="inline-flex items-center gap-1.5">
                    <span className={cn('h-1.5 w-1.5 rounded-full', cfg.dot)} />
                    {cfg.label}
                  </Badge>
                </div>
              )
            })}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
