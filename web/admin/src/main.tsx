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
import ChatList from './pages/ChatList'
import Layout from './components/Layout'
import UserList from './pages/UserList'


ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/admin/users" element={<UserList />} />
          <Route path="/admin/plugins" element={<PluginList />} />
          <Route path="/admin/plugins/upload" element={<PluginUpload />} />
          <Route path="/admin/plugins/:id" element={<PluginDetail />} />
          <Route path="/admin/plugins/:id/config" element={<PluginConfig />} />
          <Route path="/admin/plugins/:id/permissions" element={<PluginCommandPermissions />} />
          <Route path="/admin/plugins/:id/plugin-permissions" element={<PluginPermissionsPage />} />
          <Route path="/admin/plugins/:id/versions" element={<PluginVersions />} />
          <Route path="/admin/chats" element={<ChatList />} />
          <Route path="*" element={<Navigate to="/admin/plugins" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </React.StrictMode>,
)
