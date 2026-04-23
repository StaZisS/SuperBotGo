import { useEffect, useState } from 'react'
import { Copy, Key, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { api, type CreatedUserToken, type UserTokenInfo } from '@/api/client'
import { getErrorMessage } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger,
} from '@/components/ui/dialog'

function formatDate(value?: string) {
  if (!value) return 'never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

export default function UserTokensDialog() {
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [name, setName] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [tokens, setTokens] = useState<UserTokenInfo[]>([])
  const [createdToken, setCreatedToken] = useState<CreatedUserToken | null>(null)

  useEffect(() => {
    if (!open) return
    void loadTokens()
  }, [open])

  async function loadTokens() {
    setLoading(true)
    try {
      setTokens(await api.listOwnTokens())
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setLoading(false)
    }
  }

  async function handleCreate() {
    setSaving(true)
    try {
      const token = await api.createOwnToken({
        name,
        expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
      })
      setCreatedToken(token)
      setName('')
      setExpiresAt('')
      setTokens((current) => [token, ...current])
      toast.success('Токен выпущен')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: number) {
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
          setCreatedToken(null)
          setName('')
          setExpiresAt('')
        }
      }}
    >
      <DialogTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="text-muted-foreground hover:text-foreground rounded-none py-5"
        >
          <Key className="h-4 w-4" />
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>User Tokens</DialogTitle>
          <DialogDescription>
            Работают и после входа через ТГУ, и после входа по почте в админке.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-6 py-2">
          <div className="grid gap-3 rounded-md border p-4">
            <div className="grid gap-2">
              <Label htmlFor="tokenName">Название</Label>
              <Input
                id="tokenName"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="CLI token"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="tokenExpires">Истекает</Label>
              <Input
                id="tokenExpires"
                type="datetime-local"
                value={expiresAt}
                onChange={(e) => setExpiresAt(e.target.value)}
              />
            </div>
            <DialogFooter className="justify-start">
              <Button onClick={handleCreate} disabled={saving || !name.trim()}>
                {saving ? 'Выпуск...' : 'Выписать токен'}
              </Button>
            </DialogFooter>
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
            {loading && (
              <div className="text-sm text-muted-foreground">Загрузка...</div>
            )}
            {!loading && tokens.length === 0 && (
              <div className="text-sm text-muted-foreground">Токенов пока нет.</div>
            )}
            {!loading && tokens.map((token) => (
              <div key={token.id} className="flex items-start justify-between gap-4 rounded-md border p-4">
                <div className="min-w-0 space-y-1">
                  <div className="font-medium">{token.name}</div>
                  <div className="text-xs text-muted-foreground break-all">
                    public_id: {token.public_id}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    created: {formatDate(token.created_at)}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    last used: {formatDate(token.last_used_at)}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    expires: {formatDate(token.expires_at)}
                  </div>
                </div>
                <Button type="button" variant="outline" size="icon" onClick={() => handleDelete(token.id)}>
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            ))}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
