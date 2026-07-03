import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './lib/auth'
import Layout from './components/Layout'
import LoginPage from './pages/auth/LoginPage'
import BidsPage from './pages/bids/BidsPage'
import BidWorkspace from './pages/bids/BidWorkspace'
import ExportPage from './pages/bids/ExportPage'
import KnowledgePage from './pages/knowledge/KnowledgePage'

const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const { token } = useAuthStore()
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/*"
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
      </Route>
    </Routes>
  )
}
