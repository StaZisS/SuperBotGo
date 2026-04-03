import { Badge } from '@/components/ui/badge'
import { Database, Globe, HardDrive, Bell, Radio, Puzzle, Archive, FileUp } from 'lucide-react'

export interface Requirement {
  type: string
  description: string
  target?: string
}

interface Props {
  requirements: Requirement[]
}

export const TYPE_META: Record<string, { label: string; icon: React.ReactNode }> = {
  database: { label: 'База данных (SQL)', icon: <Database className="h-4 w-4" /> },
  db: { label: 'Legacy БД', icon: <Archive className="h-4 w-4" /> },
  http: { label: 'HTTP-запросы', icon: <Globe className="h-4 w-4" /> },
  kv: { label: 'Key-Value хранилище', icon: <HardDrive className="h-4 w-4" /> },
  notify: { label: 'Уведомления', icon: <Bell className="h-4 w-4" /> },
  events: { label: 'Публикация событий', icon: <Radio className="h-4 w-4" /> },
  plugin: { label: 'Вызов плагина', icon: <Puzzle className="h-4 w-4" /> },
  file: { label: 'Файловое хранилище', icon: <FileUp className="h-4 w-4" /> },
}

export default function RequirementsPanel({ requirements }: Props) {
  if (requirements.length === 0) {
    return (
      <div className="space-y-2">
        <h3 className="text-sm font-medium text-gray-700">Требования</h3>
        <p className="text-sm text-muted-foreground">Плагин не требует дополнительных ресурсов.</p>
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium text-gray-700">Требования</h3>
      {requirements.map((req, i) => {
        const meta = TYPE_META[req.type] || { label: req.type, icon: null }
        return (
          <div
            key={`${req.type}-${req.target || ''}-${i}`}
            className="flex items-start gap-3 p-2 rounded bg-muted/50"
          >
            <span className="mt-0.5 text-muted-foreground shrink-0">{meta.icon}</span>
            <div className="min-w-0">
              <div className="text-sm font-medium flex items-center gap-2">
                {meta.label}
                {req.target && (
                  <Badge variant="outline" className="font-mono text-xs">
                    {req.target}
                  </Badge>
                )}
                <Badge variant="destructive" className="text-xs">
                  Обязательно
                </Badge>
              </div>
              {req.description && (
                <div className="text-xs text-muted-foreground mt-0.5">{req.description}</div>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
