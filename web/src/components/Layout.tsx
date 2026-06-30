import { Outlet, Link, useLocation } from 'react-router-dom'
import { useAuthStore } from '../lib/auth'

const navItems = [
  { path: '/bids', label: '标书管理' },
  { path: '/knowledge', label: '知识库' },
  { path: '/settings', label: '设置' },
]

export default function Layout() {
  const location = useLocation()
  const { logout, userId } = useAuthStore()

  return (
    <div className="min-h-screen flex">
      {/* Sidebar */}
      <aside className="w-64 bg-gray-900 text-white flex flex-col">
        <div className="p-4 text-xl font-bold border-b border-gray-700">
          AI 标书系统
        </div>
        <nav className="flex-1 p-4 space-y-1">
          {navItems.map((item) => (
            <Link
              key={item.path}
              to={item.path}
              className={`block px-4 py-2 rounded-lg ${
                location.pathname.startsWith(item.path)
                  ? 'bg-blue-600'
                  : 'hover:bg-gray-700'
              }`}
            >
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="p-4 border-t border-gray-700">
          <div className="text-sm text-gray-400 mb-2">用户: {userId}</div>
          <button
            onClick={logout}
            className="w-full px-4 py-2 text-sm bg-gray-700 hover:bg-gray-600 rounded-lg"
          >
            退出登录
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 bg-gray-50">
        <Outlet />
      </main>
    </div>
  )
}