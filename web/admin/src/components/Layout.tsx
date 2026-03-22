import { Outlet, Link, useLocation } from 'react-router-dom'
import { Bot, Package, Upload, MessageSquare } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Toaster } from '@/components/ui/sonner'
import { cn } from '@/lib/utils'

const navItems = [
  { to: '/admin/plugins', label: 'Плагины', icon: Package, exact: true },
  { to: '/admin/plugins/upload', label: 'Загрузка', icon: Upload, exact: true },
  { to: '/admin/chats', label: 'Чаты', icon: MessageSquare, exact: true },
]

export default function Layout() {
  const { pathname } = useLocation()

  const isActive = (to: string, exact: boolean) =>
    exact ? pathname === to : pathname.startsWith(to)

  return (
    <div className="min-h-screen bg-background text-foreground flex flex-col">
      <Toaster />
      <header className="border-b bg-background px-4 sm:px-6">
        <div className="max-w-6xl mx-auto flex items-center justify-between">
          <Link
            to="/admin/plugins"
            className="flex items-center gap-2 text-xl font-semibold tracking-tight hover:opacity-80 transition-opacity py-3"
          >
            <Bot className="h-6 w-6 text-primary" />
            SuperBot Админ
          </Link>
          <nav className="flex items-center gap-1">
            {navItems.map((item) => {
              const active = isActive(item.to, item.exact)
              const Icon = item.icon
              return (
                <Button
                  key={item.to}
                  variant="ghost"
                  size="sm"
                  asChild
                  className={cn(
                    'relative rounded-none py-5',
                    active
                      ? 'text-primary after:absolute after:bottom-0 after:left-0 after:right-0 after:h-0.5 after:bg-primary'
                      : 'text-muted-foreground hover:text-foreground',
                  )}
                >
                  <Link to={item.to}>
                    <Icon className="h-4 w-4 sm:mr-1.5" />
                    <span className="hidden sm:inline">{item.label}</span>
                  </Link>
                </Button>
              )
            })}
          </nav>
        </div>
      </header>
      <main className="max-w-6xl mx-auto w-full px-4 sm:px-6 py-8 flex-1">
        <Outlet />
      </main>
    </div>
  )
}
