import { Outlet, Link, useLocation } from 'react-router-dom'
import ToastContainer from './Toast'

const navItems = [
  { to: '/admin/plugins', label: 'Плагины', exact: true },
  { to: '/admin/plugins/upload', label: 'Загрузка', exact: true },
  { to: '/admin/chats', label: 'Чаты', exact: true },
]

export default function Layout() {
  const { pathname } = useLocation()

  const isActive = (to: string, exact: boolean) =>
    exact ? pathname === to : pathname.startsWith(to)

  return (
    <div className="min-h-screen bg-gray-50">
      <ToastContainer />
      <header className="bg-white border-b border-gray-200 px-4 sm:px-6 py-4">
        <div className="max-w-6xl mx-auto flex items-center justify-between">
          <Link to="/admin/plugins" className="text-xl font-semibold text-gray-900 hover:text-gray-700">
            SuperBot Админ
          </Link>
          <nav className="flex gap-4 text-sm">
            {navItems.map((item) => (
              <Link
                key={item.to}
                to={item.to}
                className={
                  isActive(item.to, item.exact)
                    ? 'text-blue-600 font-medium'
                    : 'text-gray-600 hover:text-gray-900'
                }
              >
                {item.label}
              </Link>
            ))}
          </nav>
        </div>
      </header>
      <main className="max-w-6xl mx-auto px-4 sm:px-6 py-8">
        <Outlet />
      </main>
    </div>
  )
}
