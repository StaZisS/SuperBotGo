import { useEffect, useState, useRef, useCallback } from 'react'
import { api, UserListItem } from '@/api/client'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Search, Trash2, X, Loader2, Users } from 'lucide-react'
import { Link } from 'react-router-dom'

const CHANNEL_SHORT: Record<string, string> = { TELEGRAM: 'TG', DISCORD: 'DC', VK: 'VK', MATTERMOST: 'MM' }

const CHANNEL_FILTERS = [
    { value: '', label: 'Все каналы' },
    { value: 'TELEGRAM', label: 'Telegram' },
    { value: 'DISCORD', label: 'Discord' },
    { value: 'VK', label: 'VK' },
    { value: 'MATTERMOST', label: 'Mattermost' },
]

const ROLE_FILTERS = [
    { value: '', label: 'Все роли' },
    { value: 'USER', label: 'USER' },
    { value: 'ADMIN', label: 'ADMIN' },
]

export default function UserList() {
    const [users, setUsers] = useState<UserListItem[]>([])
    const [total, setTotal] = useState(0)
    const [loading, setLoading] = useState(true)
    const [inputValue, setInputValue] = useState('')
    const [search, setSearch] = useState('')
    const [channel, setChannel] = useState('')
    const [role, setRole] = useState('')
    const debounceRef = useRef<ReturnType<typeof setTimeout>>()

    const handleInputChange = useCallback((value: string) => {
        setInputValue(value)
        clearTimeout(debounceRef.current)
        debounceRef.current = setTimeout(() => setSearch(value), 300)
    }, [])

    const clearSearch = () => {
        setInputValue('')
        setSearch('')
    }

    useEffect(() => {
        return () => clearTimeout(debounceRef.current)
    }, [])

    useEffect(() => {
        setLoading(true)
        api.listUsers({ search, channel, role })
            .then(data => {
                setUsers(data.users || [])
                setTotal(data.total ?? data.users?.length ?? 0)
            })
            .catch(e => toast.error(e.message))
            .finally(() => setLoading(false))
    }, [search, channel, role])

    const handleDelete = async (id: number) => {
        if (!confirm('Удалить пользователя?')) return
        try {
            await api.deleteUser(id)
            setUsers(prev => prev.filter(u => u.id !== id))
            setTotal(prev => prev - 1)
            toast.success('Пользователь удалён')
        } catch (e: any) {
            toast.error(e.message)
        }
    }

    const hasFilters = search || channel || role

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-2xl font-bold">Пользователи</h1>
            </div>

            {/* Search & Filters */}
            <div className="flex flex-wrap items-center gap-3">
                <div className="relative w-72">
                    {loading ? (
                        <Loader2 className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground animate-spin" />
                    ) : (
                        <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                    )}
                    <Input
                        placeholder="ФИО, @username или ID..."
                        value={inputValue}
                        onChange={e => handleInputChange(e.target.value)}
                        className="pl-8 pr-8"
                    />
                    {inputValue && (
                        <button onClick={clearSearch} className="absolute right-2 top-2.5 text-muted-foreground hover:text-foreground">
                            <X className="h-4 w-4" />
                        </button>
                    )}
                </div>

                <div className="flex gap-1.5">
                    {CHANNEL_FILTERS.map(f => (
                        <Button
                            key={f.value}
                            variant={channel === f.value ? 'default' : 'outline'}
                            size="sm"
                            onClick={() => setChannel(f.value)}
                            className="h-8 text-xs"
                        >
                            {f.label}
                        </Button>
                    ))}
                </div>

                <div className="flex gap-1.5">
                    {ROLE_FILTERS.map(f => (
                        <Button
                            key={f.value}
                            variant={role === f.value ? 'default' : 'outline'}
                            size="sm"
                            onClick={() => setRole(f.value)}
                            className="h-8 text-xs"
                        >
                            {f.label}
                        </Button>
                    ))}
                </div>

                {hasFilters && (
                    <span className="text-sm text-muted-foreground">
                        Найдено: {total}
                    </span>
                )}
            </div>

            <Card>
                <CardHeader>
                    <CardTitle className="text-base">
                        {!hasFilters && `Всего: ${total}`}
                    </CardTitle>
                </CardHeader>
                <CardContent>
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead className="w-20">ID</TableHead>
                                <TableHead>ФИО</TableHead>
                                <TableHead>Аккаунты</TableHead>
                                <TableHead className="w-16 text-right">Действия</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {loading ? (
                                Array.from({ length: 5 }).map((_, i) => (
                                    <TableRow key={i}>
                                        <TableCell><div className="h-4 w-10 bg-muted animate-pulse rounded" /></TableCell>
                                        <TableCell><div className="h-4 w-32 bg-muted animate-pulse rounded" /></TableCell>
                                        <TableCell><div className="h-4 w-24 bg-muted animate-pulse rounded" /></TableCell>
                                        <TableCell />
                                    </TableRow>
                                ))
                            ) : users.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={4}>
                                        <div className="flex flex-col items-center py-10 text-center">
                                            <Users className="h-10 w-10 text-muted-foreground/40 mb-3" />
                                            {hasFilters ? (
                                                <>
                                                    <p className="text-sm font-medium">Ничего не найдено</p>
                                                    <p className="text-xs text-muted-foreground mt-1">
                                                        Попробуйте изменить запрос или сбросить фильтры
                                                    </p>
                                                    <Button variant="outline" size="sm" className="mt-3" onClick={() => { clearSearch(); setChannel(''); setRole('') }}>
                                                        Сбросить фильтры
                                                    </Button>
                                                </>
                                            ) : (
                                                <p className="text-sm text-muted-foreground">Пользователей пока нет</p>
                                            )}
                                        </div>
                                    </TableCell>
                                </TableRow>
                            ) : (
                                users.map(user => (
                                    <TableRow key={user.id}>
                                        <TableCell>
                                            <Link to={`/admin/users/${user.id}`} className="text-primary hover:underline font-mono text-sm">
                                                {user.id}
                                            </Link>
                                        </TableCell>
                                        <TableCell>
                                            {user.person_name || <span className="text-muted-foreground">-</span>}
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex flex-wrap gap-1.5">
                                                {(user.accounts || []).filter(acc => acc.channel_type).map((acc, i) => (
                                                    <Badge key={i} variant="outline" className="font-normal">
                                                        {CHANNEL_SHORT[acc.channel_type] || acc.channel_type}
                                                        {acc.username && <span className="ml-1 font-medium">@{acc.username}</span>}
                                                    </Badge>
                                                ))}
                                                {(!user.accounts || user.accounts.length === 0) && (
                                                    <span className="text-muted-foreground text-sm">-</span>
                                                )}
                                            </div>
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <Button variant="ghost" size="icon" onClick={() => handleDelete(user.id)}>
                                                <Trash2 className="h-4 w-4 text-destructive" />
                                            </Button>
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                </CardContent>
            </Card>
        </div>
    )
}
