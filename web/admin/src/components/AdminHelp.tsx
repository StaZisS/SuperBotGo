import type { ReactNode } from 'react'
import { CircleHelp, ExternalLink } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

const DOCS_BASE = 'https://staziss.github.io/SuperBotGo'

export function docsHref(path: string) {
  return `${DOCS_BASE}${path}`
}

export function HelpTooltip({
  children,
  className,
  label = 'Показать пояснение',
}: {
  children: ReactNode
  className?: string
  label?: string
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          className={cn(
            'inline-flex h-7 w-7 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            className,
          )}
          aria-label={label}
        >
          <CircleHelp className="h-4 w-4" />
        </button>
      </TooltipTrigger>
      <TooltipContent>{children}</TooltipContent>
    </Tooltip>
  )
}

export function DocsLink({
  path,
  label = 'Документация',
  className,
  variant = 'ghost',
}: {
  path: string
  label?: string
  className?: string
  variant?: 'ghost' | 'outline' | 'link'
}) {
  return (
    <Button variant={variant} size="sm" asChild className={className}>
      <a href={docsHref(path)} target="_blank" rel="noreferrer">
        {label}
        <ExternalLink className="ml-1.5 h-3.5 w-3.5" />
      </a>
    </Button>
  )
}
