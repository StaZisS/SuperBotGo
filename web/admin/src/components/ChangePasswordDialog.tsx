import { useEffect, useState } from 'react'
import { Copy, KeyRound, Trash2 } from 'lucide-react'
import { api, type CreatedUserToken, type UserTokenInfo } from '@/api/client'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog, DialogTrigger, DialogContent, DialogHeader,
  DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getErrorMessage } from '@/lib/utils'

function formatDate(value?: string) {
  if (!value) return 'никогда'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('ru-RU')
}

export default function ChangePasswordDialog() {
  const [open, setOpen] = useState(false)
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [passwordSaving, setPasswordSaving] = useState(false)
  const [tokensLoading, setTokensLoading] = useState(false)
  const [tokenSaving, setTokenSaving] = useState(false)
  const [tokenName, setTokenName] = useState('')
  const [tokenExpiresAt, setTokenExpiresAt] = useState('')
  const [tokens, setTokens] = useState<UserTokenInfo[]>([])
  const [createdToken, setCreatedToken] = useState<CreatedUserToken | null>(null)

  useEffect(() => {
    if (!open) return
    void loadTokens()
  }, [open])

  async function loadTokens() {
    setTokensLoading(true)
    try {
      setTokens(await api.listOwnTokens())
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setTokensLoading(false)
    }
  }

  const handleSubmit = async () => {
    setPasswordSaving(true)
    try {
      await api.changeOwnPassword(currentPassword, newPassword)
      toast.success('Пароль изменён')
      setOpen(false)
      setCurrentPassword('')
      setNewPassword('')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setPasswordSaving(false)
    }
  }

  async function handleCreateToken() {
    setTokenSaving(true)
    try {
      const token = await api.createOwnToken({
        name: tokenName,
        expires_at: tokenExpiresAt ? new Date(tokenExpiresAt).toISOString() : undefined,
      })
      setCreatedToken(token)
      setTokenName('')
      setTokenExpiresAt('')
      setTokens((current) => [token, ...current])
      toast.success('Токен выпущен')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setTokenSaving(false)
    }
  }

  async function handleDeleteToken(id: number) {
    try {
      await api.deleteOwnToken(id)
      setTokens((current) => current.filter((token) => token.id !== id))
      toast.success('Токен отозван')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    }
  }

  async function handleCopy(value: string) {
    try {
      await navigator.clipboard.writeText(value)
      toast.success('Скопировано')
    } catch {
      toast.error('Не удалось скопировать')
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) {
          setCurrentPassword('')
          setNewPassword('')
          setTokenName('')
          setTokenExpiresAt('')
          setCreatedToken(null)
        }
      }}
    >
      <DialogTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="text-muted-foreground hover:text-foreground rounded-none py-5"
        >
          <KeyRound className="h-4 w-4" />
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Аккаунт</DialogTitle>
          <DialogDescription>Настройки входа и пользовательские токены.</DialogDescription>
        </DialogHeader>
        <Tabs defaultValue="password" className="py-2">
          <TabsList>
            <TabsTrigger value="password">Пароль</TabsTrigger>
            <TabsTrigger value="tokens">Токены</TabsTrigger>
          </TabsList>

          <TabsContent value="password" className="space-y-4 pt-2">
            <div className="space-y-2">
              <Label htmlFor="curPw">Текущий пароль</Label>
              <Input
                id="curPw"
                type="password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                autoComplete="current-password"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="newPw">Новый пароль</Label>
              <Input
                id="newPw"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="Минимум 8 символов"
                autoComplete="new-password"
              />
            </div>
            <DialogFooter>
              <Button
                onClick={handleSubmit}
                disabled={passwordSaving || !currentPassword || newPassword.length < 8}
              >
                {passwordSaving ? 'Сохранение...' : 'Сменить пароль'}
              </Button>
            </DialogFooter>
          </TabsContent>

          <TabsContent value="tokens" className="space-y-6 pt-2">
            <div className="grid gap-3 rounded-md border p-4">
              <div className="grid gap-2">
                <Label htmlFor="tokenName">Название</Label>
                <Input
                  id="tokenName"
                  value={tokenName}
                  onChange={(e) => setTokenName(e.target.value)}
                  placeholder="Например: CLI"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="tokenExpires">Истекает</Label>
                <Input
                  id="tokenExpires"
                  type="datetime-local"
                  value={tokenExpiresAt}
                  onChange={(e) => setTokenExpiresAt(e.target.value)}
                />
              </div>
              <div>
                <Button onClick={handleCreateToken} disabled={tokenSaving || !tokenName.trim()}>
                  {tokenSaving ? 'Выпуск...' : 'Выписать токен'}
                </Button>
              </div>
            </div>

            {createdToken && (
              <div className="grid gap-2 rounded-md border border-primary/30 bg-primary/5 p-4">
                <div className="text-sm font-medium">Новый токен</div>
                <div className="flex gap-2">
                  <Input readOnly value={createdToken.token} />
                  <Button type="button" variant="outline" onClick={() => handleCopy(createdToken.token)}>
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
                <div className="text-xs text-muted-foreground">
                  Секрет показывается только один раз.
                </div>
              </div>
            )}

            <div className="grid gap-3">
              <div className="text-sm font-medium">Активные токены</div>
              {tokensLoading && (
                <div className="text-sm text-muted-foreground">Загрузка...</div>
              )}
              {!tokensLoading && tokens.length === 0 && (
                <div className="text-sm text-muted-foreground">Токенов пока нет.</div>
              )}
              {!tokensLoading && tokens.map((token) => (
                <div key={token.id} className="flex items-start justify-between gap-4 rounded-md border p-4">
                  <div className="min-w-0 space-y-1">
                    <div className="font-medium">{token.name}</div>
                    <div className="text-xs text-muted-foreground break-all">
                      открытый ID: {token.public_id}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      создан: {formatDate(token.created_at)}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      последнее использование: {formatDate(token.last_used_at)}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      истекает: {formatDate(token.expires_at)}
                    </div>
                  </div>
                  <Button type="button" variant="outline" size="icon" onClick={() => handleDeleteToken(token.id)}>
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
