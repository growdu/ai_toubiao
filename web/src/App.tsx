import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './lib/auth'
import Layout from './components/Layout'
import { ToastContainer } from './components/ToastContainer'
import { useThemeStore } from './lib/theme'
import { useEffect } from 'react'
import LoginPage from './pages/auth/LoginPage'
import BidsPage from './pages/bids/BidsPage'
import BidWorkspace from './pages/bids/BidWorkspace'
import ExportPage from './pages/bids/ExportPage'
import KnowledgePage from './pages/knowledge/KnowledgePage'
import SettingsPage from './pages/settings/SettingsPage'

const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const { token } = useAuthStore()
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

export default function App() {
  const applyTheme = useThemeStore(s => s.apply)

  // Ensure the resolved theme is applied after hydration on first mount.
  useEffect(() => { applyTheme() }, [applyTheme])

  return (
    <>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <Layout />
            </ProtectedRoute>
          }
        >
          <Route index element={<Navigate to="/bids" replace />} />
          <Route path="bids" element={<BidsPage />} />
          <Route path="bids/:id" element={<BidWorkspace />} />
          <Route path="bids/:id/export" element={<ExportPage />} />
          <Route path="knowledge" element={<KnowledgePage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Routes>
      <ToastContainer />
    </>
  )
}
