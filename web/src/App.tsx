import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './lib/auth'
import Layout from './components/Layout'
import { ToastContainer } from './components/ToastContainer'
import { useThemeStore } from './lib/theme'
import { useEffect } from 'react'
import LandingPage from './pages/LandingPage'
import LoginPage from './pages/auth/LoginPage'
import RegisterPage from './pages/auth/RegisterPage'
import BidsPage from './pages/bids/BidsPage'
import BidWorkspace from './pages/bids/BidWorkspace'
import ExportPage from './pages/bids/ExportPage'
import KnowledgePage from './pages/knowledge/KnowledgePage'
import SettingsPage from './pages/settings/SettingsPage'

const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const { token } = useAuthStore()
  if (!token) {
    // Send unauthenticated visitors to the marketing landing page rather
    // than straight to /login — gives them a chance to read what the
    // product does before being asked to authenticate. The landing page
    // itself has a "登录" button that jumps to /login.
    return <Navigate to="/" replace />
  }
  return <>{children}</>
}

// PublicOnlyRoute: only renders if the visitor is NOT logged in. Used for
// /login and /register so a returning user doesn't see them again after
// signing in (they'd be bounced back to /bids).
const PublicOnlyRoute = ({ children }: { children: React.ReactNode }) => {
  const { token } = useAuthStore()
  if (token) {
    return <Navigate to="/bids" replace />
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
        {/* Public marketing/auth routes. /login + /register are gated on
            PublicOnlyRoute so a logged-in user is bounced into the app. */}
        <Route path="/" element={<LandingPage />} />
        <Route
          path="/login"
          element={
            <PublicOnlyRoute>
              <LoginPage />
            </PublicOnlyRoute>
          }
        />
        <Route
          path="/register"
          element={
            <PublicOnlyRoute>
              <RegisterPage />
            </PublicOnlyRoute>
          }
        />

        {/* Authenticated app. */}
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <Layout />
            </ProtectedRoute>
          }
        >
          <Route path="bids" element={<BidsPage />} />
          <Route path="bids/:id" element={<BidWorkspace />} />
          <Route path="bids/:id/export" element={<ExportPage />} />
          <Route path="knowledge" element={<KnowledgePage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>

        {/* Anything else: send to landing. */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <ToastContainer />
    </>
  )
}
