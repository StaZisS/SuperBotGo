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
import Layout from './components/Layout'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/admin/plugins" element={<PluginList />} />
          <Route path="/admin/plugins/upload" element={<PluginUpload />} />
          <Route path="/admin/plugins/:id" element={<PluginDetail />} />
          <Route path="/admin/plugins/:id/config" element={<PluginConfig />} />
          <Route path="/admin/plugins/:id/permissions" element={<PluginCommandPermissions />} />
          <Route path="/admin/plugins/:id/plugin-permissions" element={<PluginPermissionsPage />} />
          <Route path="*" element={<Navigate to="/admin/plugins" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </React.StrictMode>,
)
