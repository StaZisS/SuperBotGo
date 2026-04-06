import { useEffect, useState } from 'react'
import { api, AdminCredentialInfo } from '@/api/client'
import { toast } from 'sonner'
import { ShieldCheck, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  AlertDialog, AlertDialogTrigger, AlertDialogContent, AlertDialogHeader,
  AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogAction, AlertDialogCancel,
} from '@/components/ui/alert-dialog'
import { getErrorMessage } from '@/lib/utils'

export default function AdminAccessCard({ userId }: { userId: number }) {
  const [cred, setCred] = useState<AdminCredentialInfo | null>(null)
  const [hasAccess, setHasAccess] = useState(false)
  const [loading, setLoading] = useState(true)

  // Form for granting access
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [saving, setSaving] = useState(false)

  // Form for resetting password
  const [newPassword, setNewPassword] = useState('')

  useEffect(() => {
    api.getAdminCredential(userId)
      .then((res) => {
        setHasAccess(res.has_access)
        setCred(res.credential ?? null)
      })
      .catch(() => setHasAccess(false))
      .finally(() => setLoading(false))
  }, [userId])

  const handleGrant = async () => {
    if (!email || !password) return
    setSaving(true)
    try {
      const created = await api.createAdminCredential({
        global_user_id: userId,
        email,
        password,
      })
      setCred(created)
      setHasAccess(true)
      setEmail('')
      setPassword('')
      toast.success('Доступ в админку предоставлен')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setSaving(false)
    }
  }

  const handleResetPassword = async () => {
    if (!newPassword) return
    try {
      await api.updateAdminPassword(userId, newPassword)
      setNewPassword('')
      toast.success('Пароль сброшен')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    }
  }

  const handleRevoke = async () => {
    try {
      await api.deleteAdminCredential(userId)
      setHasAccess(false)
      setCred(null)
      toast.success('Доступ в админку отозван')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
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

            <div className="flex items-end gap-2">
              <div className="space-y-2 flex-1">
                <Label htmlFor="resetPw">Новый временный пароль</Label>
                <Input
                  id="resetPw"
                  type="text"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="Минимум 8 символов"
                />
              </div>
              <Button
                size="sm"
                onClick={handleResetPassword}
                disabled={newPassword.length < 8}
              >
                Сбросить пароль
              </Button>
            </div>

            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="destructive" size="sm">
                  <Trash2 className="mr-1.5 h-4 w-4" />Отозвать доступ
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Отозвать доступ в админку</AlertDialogTitle>
                  <AlertDialogDescription>
                    Пользователь больше не сможет входить в панель администрирования.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Отмена</AlertDialogCancel>
                  <AlertDialogAction onClick={handleRevoke} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                    Подтвердить
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        ) : (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              У пользователя нет доступа в админку. Назначьте email и временный пароль для входа.
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
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
              <div className="space-y-2">
                <Label htmlFor="adminPw">Временный пароль</Label>
                <Input
                  id="adminPw"
                  type="text"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Минимум 8 символов"
                />
              </div>
            </div>
            <Button onClick={handleGrant} disabled={saving || !email || password.length < 8} size="sm">
              {saving ? 'Сохранение...' : 'Предоставить доступ'}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
