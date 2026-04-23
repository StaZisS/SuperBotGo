import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react'

interface AuthContextType {
  authenticated: boolean
  loading: boolean
  tsuEnabled: boolean
  login: (email: string, password: string) => Promise<string | null>
  loginWithTSU: () => void
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [authenticated, setAuthenticated] = useState(false)
  const [loading, setLoading] = useState(true)
  const [tsuEnabled, setTsuEnabled] = useState(false)

  const checkAuth = useCallback(async () => {
    try {
      const res = await fetch('/api/admin/auth/check')
      const data = await res.json()
      setAuthenticated(data.authenticated === true)
      setTsuEnabled(data.tsu_enabled === true)
    } catch {
      setAuthenticated(false)
      setTsuEnabled(false)
    } finally {
      setLoading(false)
    }
  }, [])

  const loginWithTSU = useCallback(() => {
    window.location.href = `/api/auth/tsu/start?return_to=${encodeURIComponent('/admin/plugins')}`
  }, [])

  useEffect(() => {
    checkAuth()
  }, [checkAuth])

  const login = useCallback(async (email: string, password: string): Promise<string | null> => {
    try {
      const res = await fetch('/api/admin/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      })
      if (res.ok) {
        setAuthenticated(true)
        return null
      }
      const data = await res.json()
      return data.error ?? 'Authentication failed'
    } catch {
      return 'Network error'
    }
  }, [])

  const logout = useCallback(async () => {
    try {
      await fetch('/api/admin/auth/logout', { method: 'POST' })
    } finally {
      setAuthenticated(false)
    }
  }, [])

  return (
    <AuthContext.Provider value={{ authenticated, loading, tsuEnabled, login, loginWithTSU, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
