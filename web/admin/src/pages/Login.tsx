import { useState, type FormEvent } from 'react'
import { Bot, KeyRound } from 'lucide-react'
import { useLocation } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useAuth } from '@/hooks/useAuth'
import { Separator } from '@/components/ui/separator'

function getAuthErrorMessage(search: string): string | null {
  const code = new URLSearchParams(search).get('auth_error')
  if (code === 'admin_required') {
    return 'У вашей учётной записи нет доступа к админке.'
  }
  return null
}

export default function Login() {
  const { login, loginWithTSU, tsuEnabled } = useAuth()
  const location = useLocation()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const authError = getAuthErrorMessage(location.search)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    const err = await login(email, password)
    if (err) {
      setError(err)
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-background flex items-center justify-center px-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <div className="flex justify-center mb-2">
            <Bot className="h-10 w-10 text-primary" />
          </div>
          <CardTitle className="text-xl">SuperBot Admin</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {tsuEnabled && (
              <>
                <Button
                  type="button"
                  variant="outline"
                  className="w-full"
                  onClick={loginWithTSU}
                  disabled={submitting}
                >
                  <Bot className="h-4 w-4 mr-2" />
                  Войти через ТГУ
                </Button>
                <div className="flex items-center gap-3">
                  <Separator className="flex-1" />
                  <span className="text-xs text-muted-foreground">или</span>
                  <Separator className="flex-1" />
                </div>
              </>
            )}
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="admin@example.com"
                autoComplete="email"
                autoFocus
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Password"
                autoComplete="current-password"
                required
              />
            </div>
            {(error ?? authError) && (
              <p className="text-sm text-destructive">{error ?? authError}</p>
            )}
            <Button type="submit" className="w-full" disabled={submitting}>
              <KeyRound className="h-4 w-4 mr-2" />
              {submitting ? 'Вход...' : 'Войти по почте'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
