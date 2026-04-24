import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api, UserDetail as UserDetailType } from '@/api/client'
import { toast } from 'sonner'
import { ArrowLeft, Trash2, Unlink, Save, User } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import {
  AlertDialog, AlertDialogTrigger, AlertDialogContent, AlertDialogHeader,
  AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogAction, AlertDialogCancel,
} from '@/components/ui/alert-dialog'
import { formatDate, getErrorMessage } from '@/lib/utils'
import UserPositions from '@/components/UserPositions'
import AdminAccessCard from '@/components/AdminAccessCard'

export default function UserDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [user, setUser] = useState<UserDetailType | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [locale, setLocale] = useState('')
  const [role, setRole] = useState('')

  useEffect(() => {
    if (!id) return
    const uid = Number(id)
    setLoading(true)
    api.getUser(uid)
      .then((u) => {
        setUser(u)
        setLocale(u.locale || '')
        setRole(u.role || '')
      })
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [id])

  const handleSave = async () => {
    if (!id) return
    setSaving(true)
    try {
      await api.updateUser(Number(id), { locale, role })
      setUser(prev => prev ? { ...prev, locale, role } : prev)
      toast.success('Пользователь обновлён')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!id) return
    try {
      await api.deleteUser(Number(id))
      toast.success('Пользователь удалён')
      navigate('/admin/users')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    }
  }

  const handleUnlink = async (accountId: number) => {
    if (!id) return
    try {
      await api.unlinkAccount(Number(id), accountId)
      setUser(prev => prev ? { ...prev, accounts: prev.accounts.filter(a => a.id !== accountId) } : prev)
      toast.success('Аккаунт отвязан')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    }
  }

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-20" />
        <Skeleton className="h-6 w-48" />
        <Card><CardContent className="py-6"><Skeleton className="h-32 w-full" /></CardContent></Card>
      </div>
    )
  }

  if (!user) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <User className="h-12 w-12 text-muted-foreground/50 mb-4" />
        <h3 className="text-lg font-semibold mb-1">Пользователь не найден</h3>
        <Button variant="outline" asChild className="mt-4">
          <Link to="/admin/users"><ArrowLeft className="mr-1.5 h-4 w-4" />Назад</Link>
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <Button variant="ghost" size="sm" asChild className="mb-2 -ml-2">
            <Link to="/admin/users"><ArrowLeft className="mr-1 h-4 w-4" />Назад</Link>
          </Button>
          <h2 className="text-lg font-semibold">
            {user.person_name || `Пользователь #${user.id}`}
          </h2>
          {user.created_at && (
            <p className="text-sm text-muted-foreground">Создан {formatDate(user.created_at)}</p>
          )}
        </div>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="destructive" size="sm">
              <Trash2 className="mr-1.5 h-4 w-4" />Удалить
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Удалить пользователя</AlertDialogTitle>
              <AlertDialogDescription>
                Вы уверены, что хотите удалить пользователя <strong>#{user.id}</strong>? Это действие нельзя отменить.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Отмена</AlertDialogCancel>
              <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                Подтвердить
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>

      {/* Edit card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Основные данные</CardTitle>
          {user.accounts?.length > 0 && (
            <CardDescription>Канал: <Badge variant="outline">{user.primary_channel}</Badge></CardDescription>
          )}
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="locale">Локаль</Label>
              <Select value={locale} onValueChange={setLocale}>
                <SelectTrigger id="locale">
                  <SelectValue placeholder="Выберите локаль" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="ru">ru — Русский</SelectItem>
                  <SelectItem value="en">en — English</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="role">Роль</Label>
              <Select value={role} onValueChange={setRole}>
                <SelectTrigger id="role">
                  <SelectValue placeholder="Выберите роль" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="USER">USER</SelectItem>
                  <SelectItem value="ADMIN">ADMIN</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>TSU Account ID</Label>
              <div className="min-h-10 rounded-md border bg-muted/30 px-3 py-2 text-sm break-all">
                {user.tsu_accounts_id || 'Не привязан'}
              </div>
            </div>
          </div>
          <Button onClick={handleSave} disabled={saving} size="sm">
            <Save className="mr-1.5 h-4 w-4" />{saving ? 'Сохранение...' : 'Сохранить'}
          </Button>
        </CardContent>
      </Card>

      {/* Admin access — only for users with ADMIN role */}
      {user.role === 'ADMIN' && <AdminAccessCard userId={Number(id)} />}

      {/* Positions */}
      <UserPositions userId={Number(id)} />

      {/* Accounts */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Привязанные аккаунты ({user.accounts?.length || 0})</CardTitle>
        </CardHeader>
        <CardContent>
          {!user.accounts || user.accounts.length === 0 ? (
            <p className="text-sm text-muted-foreground">Нет привязанных аккаунтов</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Канал</TableHead>
                  <TableHead>ID</TableHead>
                  <TableHead>Username</TableHead>
                  <TableHead>Привязан</TableHead>
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {user.accounts.map(acc => (
                  <TableRow key={acc.id}>
                    <TableCell><Badge variant="outline">{acc.channel_type}</Badge></TableCell>
                    <TableCell className="font-mono text-sm">{acc.channel_user_id}</TableCell>
                    <TableCell>{acc.username || '-'}</TableCell>
                    <TableCell>{formatDate(acc.linked_at)}</TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon" onClick={() => handleUnlink(acc.id)}>
                        <Unlink className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

    </div>
  )
}
