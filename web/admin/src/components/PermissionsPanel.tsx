import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

interface Permission {
  key: string
  description: string
  required: boolean
}

interface Props {
  permissions: Permission[]
  selected: string[]
  onChange: (selected: string[]) => void
  readOnly?: boolean
}

export default function PermissionsPanel({ permissions, selected, onChange, readOnly }: Props) {
  const isChecked = (key: string, required: boolean) =>
    required || selected.includes(key)

  const toggle = (key: string, required: boolean) => {
    if (required || readOnly) return
    if (selected.includes(key)) {
      onChange(selected.filter((k) => k !== key))
    } else {
      onChange([...selected, key])
    }
  }

  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium text-gray-700">Разрешения</h3>
      {permissions.length === 0 && (
        <p className="text-sm text-muted-foreground">Разрешения не требуются.</p>
      )}
      {permissions.map((p) => (
        <Label
          key={p.key}
          htmlFor={`perm-${p.key}`}
          className={cn(
            'flex items-start gap-3 p-2 rounded font-normal',
            readOnly || p.required ? 'opacity-75' : 'hover:bg-gray-50 cursor-pointer',
          )}
        >
          <Checkbox
            id={`perm-${p.key}`}
            checked={isChecked(p.key, p.required)}
            disabled={p.required || readOnly}
            onCheckedChange={() => toggle(p.key, p.required)}
            className="mt-0.5"
          />
          <div className="min-w-0">
            <div className="text-sm font-mono break-all">
              {p.key}
              {p.required && (
                <Badge variant="destructive" className="ml-2 font-sans text-xs">
                  Обязательно
                </Badge>
              )}
              {!p.required && (
                <Badge variant="secondary" className="ml-2 font-sans text-xs">
                  Опционально
                </Badge>
              )}
            </div>
            {p.description && (
              <div className="text-xs text-muted-foreground mt-0.5">{p.description}</div>
            )}
          </div>
        </Label>
      ))}
    </div>
  )
}
