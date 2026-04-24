import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react'

interface AuthContextType {
  authenticated: boolean
  loading: boolean
  tsuEnabled: boolean
  userId: number | null
  login: (email: string, password: string) => Promise<string | null>
  loginWithTSU: () => void
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [authenticated, setAuthenticated] = useState(false)
  const [loading, setLoading] = useState(true)
  const [tsuEnabled, setTsuEnabled] = useState(false)
  const [userId, setUserId] = useState<number | null>(null)

  const checkAuth = useCallback(async () => {
    try {
      const res = await fetch('/api/admin/auth/check')
      const data = await res.json()
      setAuthenticated(data.authenticated === true)
      setTsuEnabled(data.tsu_enabled === true)
      setUserId(typeof data.user_id === 'number' ? data.user_id : null)
    } catch {
      setAuthenticated(false)
      setTsuEnabled(false)
      setUserId(null)
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
        await checkAuth()
        return null
      }
      const data = await res.json()
      return data.error ?? 'Authentication failed'
    } catch {
      return 'Network error'
    }
  }, [checkAuth])

  const logout = useCallback(async () => {
    try {
      await fetch('/api/admin/auth/logout', { method: 'POST' })
    } finally {
      setAuthenticated(false)
      setUserId(null)
    }
  }, [])

  return (
    <AuthContext.Provider value={{ authenticated, loading, tsuEnabled, userId, login, loginWithTSU, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
