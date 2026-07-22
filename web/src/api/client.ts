import axios from 'axios'
import { useAuthStore } from '../lib/auth'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// In-flight refresh promise. Concurrent 401s share a single refresh
// round-trip so we don't hammer /auth/refresh with N parallel calls.
let refreshing: Promise<string> | null = null

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const status = error.response?.status
    const original = error.config
    // Only attempt a refresh on 401, for a retriable request, when we
    // still hold a refresh token, and never for the refresh call itself.
    if (
      status === 401 &&
      original &&
      !original._retried &&
      useAuthStore.getState().refreshToken &&
      !String(original.url || '').includes('/auth/refresh')
    ) {
      original._retried = true
      try {
        if (!refreshing) {
          const { refreshToken, setTokens } = useAuthStore.getState()
          refreshing = (async () => {
            const res = await axios.post('/api/v1/auth/refresh', {
              refresh_token: refreshToken,
            })
            const next = res.data as { access_token: string; refresh_token: string }
            setTokens(next.access_token, next.refresh_token)
            return next.access_token
          })().finally(() => {
            refreshing = null
          })
        }
        const newToken = await refreshing
        original.headers.Authorization = `Bearer ${newToken}`
        return api(original)
      } catch (e) {
        useAuthStore.getState().logout()
        return Promise.reject(e)
      }
    }
    // Any other 401 (no refresh token, refresh failed, already retried)
    // clears the stale session; the route guard redirects to /login.
    if (status === 401) {
      useAuthStore.getState().logout()
    }
    return Promise.reject(error)
  }
)

export default api
