import { useEffect, useState } from 'react'
import { api, UserListItem } from '@/api/client'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Search, Trash2 } from 'lucide-react'
import { Link } from 'react-router-dom'

export default function UserList() {
    const [users, setUsers] = useState<UserListItem[]>([])
    const [loading, setLoading] = useState(true)
    const [search, setSearch] = useState('')

    useEffect(() => {
        setLoading(true)
        api.listUsers({ search })
            .then(data => setUsers(data.users))
            .catch(e => toast.error(e.message))
            .finally(() => setLoading(false))
    }, [search])

    const handleDelete = async (id: number) => {
        if (!confirm('Удалить пользователя?')) return
        try {
            await api.deleteUser(id)
            setUsers(prev => prev.filter(u => u.id !== id))
            toast.success('Пользователь удалён')
        } catch (e: any) {
            toast.error(e.message)
        }
    }

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-2xl font-bold">Пользователи</h1>
                <div className="flex items-center gap-2">
                    <div className="relative w-64">
                        <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                        <Input
                            placeholder="Поиск..."
                            value={search}
                            onChange={e => setSearch(e.target.value)}
                            className="pl-8"
                        />
                    </div>
                </div>
            </div>

            <Card>
                <CardHeader>
                    <CardTitle>Список пользователей</CardTitle>
                </CardHeader>
                <CardContent>
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>ID</TableHead>
                                <TableHead>Канал</TableHead>
                                <TableHead>Роль</TableHead>
                                <TableHead>Аккаунтов</TableHead>
                                <TableHead className="text-right">Действия</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {loading ? (
                                <TableRow><TableCell colSpan={5}>Загрузка...</TableCell></TableRow>
                            ) : users.length === 0 ? (
                                <TableRow><TableCell colSpan={5}>Пользователи не найдены</TableCell></TableRow>
                            ) : (
                                users.map(user => (
                                    <TableRow key={user.id}>
                                        <TableCell>
                                            <Link to={`/admin/users/${user.id}`} className="text-primary hover:underline">
                                                {user.id}
                                            </Link>
                                        </TableCell>
                                        <TableCell>
                                            <Badge variant="outline">{user.primary_channel}</Badge>
                                        </TableCell>
                                        <TableCell>{user.role}</TableCell>
                                        <TableCell>{user.account_count}</TableCell>
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