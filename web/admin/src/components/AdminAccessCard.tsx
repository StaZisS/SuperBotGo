import { useCallback, useEffect, useState } from 'react'
import { api, AdminCredentialInfo, ApiError } from '@/api/client'
import { toast } from 'sonner'
import { ShieldCheck } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { getErrorMessage } from '@/lib/utils'

export default function AdminAccessCard({ userId }: { userId: number }) {
  const [cred, setCred] = useState<AdminCredentialInfo | null>(null)
  const [hasAccess, setHasAccess] = useState(false)
  const [loading, setLoading] = useState(true)

  // Form for granting access
  const [email, setEmail] = useState('')
  const [saving, setSaving] = useState(false)

  const loadCredential = useCallback(async () => {
    setLoading(true)
    try {
      const res = await api.getAdminCredential(userId)
      setHasAccess(res.has_access)
      setCred(res.credential ?? null)
    } catch {
      setHasAccess(false)
      setCred(null)
    } finally {
      setLoading(false)
    }
  }, [userId])

  useEffect(() => {
    loadCredential()
  }, [loadCredential])

  const handleGrant = async () => {
    if (!email) return
    setSaving(true)
    try {
      const created = await api.createAdminCredential({
        global_user_id: userId,
        email,
      })
      setCred(created)
      setHasAccess(true)
      setEmail('')
      toast.success('Доступ в админку предоставлен, временный пароль отправлен на email')
    } catch (e: unknown) {
      if (e instanceof ApiError && e.status === 409 && e.message === 'доступ в админку уже выдан этому пользователю') {
        await loadCredential()
      }
      toast.error(getErrorMessage(e))
    } finally {
      setSaving(false)
    }
  }

  if (loading) return null

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base flex items-center gap-2">
          <ShieldCheck className="h-4 w-4" />
          Доступ в админку
          {hasAccess && <Badge variant="secondary">Активен</Badge>}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {hasAccess && cred ? (
          <div className="space-y-4">
            <div className="text-sm space-y-1">
              <p><span className="text-muted-foreground">Email:</span> {cred.email}</p>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              У пользователя нет доступа в админку. Укажите email: сервер сгенерирует временный пароль и отправит его по SMTP.
            </p>
            <div className="grid grid-cols-1 gap-4">
              <div className="space-y-2">
                <Label htmlFor="adminEmail">Email</Label>
                <Input
                  id="adminEmail"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="user@example.com"
                />
              </div>
            </div>
            <Button onClick={handleGrant} disabled={saving || !email} size="sm">
              {saving ? 'Сохранение...' : 'Предоставить доступ и отправить пароль'}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
