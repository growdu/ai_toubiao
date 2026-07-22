import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './lib/auth'
import Layout from './components/Layout'
import { ToastContainer } from './components/ToastContainer'
import { useThemeStore } from './lib/theme'
import { useEffect } from 'react'
import { GlobalErrorBoundary, RouteErrorBoundary } from './components/ErrorBoundary'
import LandingPage from './pages/LandingPage'
import LoginPage from './pages/auth/LoginPage'
import RegisterPage from './pages/auth/RegisterPage'
import BidsPage from './pages/bids/BidsPage'
import BidWorkspaceWrapper from './pages/bids/BidWorkspaceWrapper'
import ExportPage from './pages/bids/ExportPage'
import KnowledgePage from './pages/knowledge/KnowledgePage'
import SettingsPage from './pages/settings/SettingsPage'

const PUBLIC_LANDING = '/welcome'

const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const { token } = useAuthStore()
  if (!token) {
    return <Navigate to={PUBLIC_LANDING} replace />
  }
  return <>{children}</>
}

const PublicOnlyRoute = ({ children }: { children: React.ReactNode }) => {
  const { token } = useAuthStore()
  if (token) {
    return <Navigate to='/bids' replace />
  }
  return <>{children}</>
}

function LoginElement() {
  return <PublicOnlyRoute><LoginPage /></PublicOnlyRoute>
}
function RegisterElement() {
  return <PublicOnlyRoute><RegisterPage /></PublicOnlyRoute>
}
function ProtectedElement() {
  return <ProtectedRoute><Layout /></ProtectedRoute>
}

export default function App() {
  const applyTheme = useThemeStore(s => s.apply)
  useEffect(() => { applyTheme() }, [applyTheme])
  return (
    <GlobalErrorBoundary>
      <Routes>
        <Route path='/welcome' element={<LandingPage />} />
        <Route path='/login' element={<LoginElement />} />
        <Route path='/register' element={<RegisterElement />} />
        <Route element={<ProtectedElement />}>
          <Route path='/bids' element={<RouteErrorBoundary><BidsPage /></RouteErrorBoundary>} />
          <Route path='/bids/:id' element={<RouteErrorBoundary><BidWorkspaceWrapper /></RouteErrorBoundary>} />
          <Route path='/bids/:id/export' element={<RouteErrorBoundary><ExportPage /></RouteErrorBoundary>} />
          <Route path='/knowledge' element={<RouteErrorBoundary><KnowledgePage /></RouteErrorBoundary>} />
          <Route path='/settings' element={<RouteErrorBoundary><SettingsPage /></RouteErrorBoundary>} />
        </Route>
        <Route path='/' element={<Navigate to={PUBLIC_LANDING} replace />} />
        <Route path='*' element={<Navigate to={PUBLIC_LANDING} replace />} />
      </Routes>
      <ToastContainer />
    </GlobalErrorBoundary>
  )
}
