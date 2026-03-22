import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

interface Props {
  status: string
}

const dotColors: Record<string, string> = {
  active: 'bg-green-500',
  disabled: 'bg-gray-400',
  error: 'bg-red-500',
}

const variantMap: Record<string, 'success' | 'secondary' | 'destructive'> = {
  active: 'success',
  disabled: 'secondary',
  error: 'destructive',
}

export default function PluginStatusBadge({ status }: Props) {
  const variant = variantMap[status] ?? 'secondary'
  const dot = dotColors[status] ?? dotColors.disabled

  return (
    <Badge variant={variant} className="inline-flex items-center gap-1.5">
      <span className={cn('h-1.5 w-1.5 rounded-full', dot)} />
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </Badge>
  )
}
