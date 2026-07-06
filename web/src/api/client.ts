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

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Wipe the stale token so the next /api/v1/* call doesn't keep
      // shipping an Authorization header for an invalid session. The
      // caller (typically LoginPage or an authenticated page) decides
      // what to do: LoginPage will display its red banner, an
      // authenticated page will end up at /login via ProtectedRoute
      // when the next render checks `useAuthStore().token === null`.
      //
      // We deliberately do NOT `window.location.href = '/login'` here.
      // That used to force a full page reload, which clobbered any
      // local error state the LoginPage was about to display — so
      // users saw the page "do nothing" after typing a wrong password.
      const hadToken = !!useAuthStore.getState().token
      useAuthStore.getState().logout()
      // Only the navigation-relevant paths should redirect to /login;
      // for now we let the route guard do it on the next render.
      // (If you ever wire a global toast, this is the place to call it
      //  when hadToken is true — i.e. a previously-authenticated user
      //  got a 401 from a protected endpoint.)
      void hadToken
    }
    return Promise.reject(error)
  }
)

export default api