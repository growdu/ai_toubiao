import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { authApi } from '../../api/auth'
import { useAuthStore } from '../../lib/auth'

export default function LoginPage() {
  const navigate = useNavigate()
  const { setAuth } = useAuthStore()
  const [tenantSlug, setTenantSlug] = useState('demo-a')
  const [email, setEmail] = useState('admin@demo-a.test')
  const [password, setPassword] = useState('admin123')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await authApi.login({ tenant_slug: tenantSlug, email, password })
      const { token, userId, tenantId } = authApi.extractAuthInfo(res)
      if (!token) {
        setError('登录返回无效，未收到 token')
        return
      }
      setAuth(token, userId, tenantId)
      navigate('/bids')
    } catch (err: any) {
      setError(err.response?.data?.message || err.response?.data?.error?.message || '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-100">
      <div className="bg-white p-8 rounded-xl shadow-md w-full max-w-md">
        <h1 className="text-2xl font-bold mb-6 text-center">AI 标书生成系统</h1>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              租户标识
            </label>
            <input
              type="text"
              value={tenantSlug}
              onChange={(e) => setTenantSlug(e.target.value)}
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="demo-a"
              required
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              邮箱
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="admin@example.com"
              required
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              密码
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="••••••••"
              required
            />
          </div>
          {error && <div className="text-red-500 text-sm">{error}</div>}
          <button
            type="submit"
            disabled={loading}
            className="w-full py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
          >
            {loading ? '登录中...' : '登录'}
          </button>
          <p className="text-xs text-gray-400 text-center">
            测试账号：demo-a / admin@demo-a.test / admin123
          </p>
        </form>
      </div>
    </div>
  )
}
