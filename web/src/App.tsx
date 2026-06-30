import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './lib/auth'
import Layout from './components/Layout'
import LoginPage from './pages/auth/LoginPage'
import BidsPage from './pages/bids/BidsPage'
import BidDetailPage from './pages/bids/BidDetailPage'
import OutlinePage from './pages/bids/OutlinePage'
import ChaptersPage from './pages/bids/ChaptersPage'
import AuditPage from './pages/bids/AuditPage'
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
        <Route path="bids/:id" element={<BidDetailPage />} />
        <Route path="bids/:id/outline" element={<OutlinePage />} />
        <Route path="bids/:id/chapters" element={<ChaptersPage />} />
        <Route path="bids/:id/audit" element={<AuditPage />} />
        <Route path="knowledge" element={<KnowledgePage />} />
      </Route>
    </Routes>
  )
}