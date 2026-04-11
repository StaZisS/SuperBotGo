import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import './index.css'
import PluginList from './pages/PluginList'
import PluginUpload from './pages/PluginUpload'
import PluginDetail from './pages/PluginDetail'
import PluginConfig from './pages/PluginConfig'
import PluginCommandPermissions from './pages/PluginCommandPermissions'
import PluginPermissionsPage from './pages/PluginPermissionsPage'
import PluginVersions from './pages/PluginVersions'
import Layout from './components/Layout'
import UserList from './pages/UserList'
import UserDetail from './pages/UserDetail'
import UniversityHierarchy from './pages/UniversityHierarchy'
import Login from './pages/Login'
import { AuthProvider, useAuth } from './hooks/useAuth'

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { authenticated, loading } = useAuth()

  if (loading) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <div className="text-muted-foreground">Loading...</div>
      </div>
    )
  }

  if (!authenticated) {
    return <Login />
  }

  return <>{children}</>
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <RequireAuth>
          <Routes>
            <Route element={<Layout />}>
              <Route path="/admin/users" element={<UserList />} />
              <Route path="/admin/users/:id" element={<UserDetail />} />
              <Route path="/admin/plugins" element={<PluginList />} />
              <Route path="/admin/plugins/upload" element={<PluginUpload />} />
              <Route path="/admin/plugins/:id" element={<PluginDetail />} />
              <Route path="/admin/plugins/:id/config" element={<PluginConfig />} />
              <Route path="/admin/plugins/:id/permissions" element={<PluginCommandPermissions />} />
              <Route path="/admin/plugins/:id/plugin-permissions" element={<PluginPermissionsPage />} />
              <Route path="/admin/plugins/:id/versions" element={<PluginVersions />} />
              <Route path="/admin/university" element={<UniversityHierarchy />} />
              <Route path="*" element={<Navigate to="/admin/plugins" replace />} />
            </Route>
          </Routes>
        </RequireAuth>
      </AuthProvider>
    </BrowserRouter>
  </React.StrictMode>,
)
