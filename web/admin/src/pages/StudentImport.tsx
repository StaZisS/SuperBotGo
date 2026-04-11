// web/admin/src/pages/StudentImport.tsx
import { useState, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api, ImportResult } from '@/api/client'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardHeader, CardTitle, CardContent, CardFooter } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import WasmUploader from '@/components/WasmUploader'
import { Download, CheckCircle2, ArrowLeft, FileSpreadsheet } from 'lucide-react'
import { cn } from '@/lib/utils'

export default function StudentImport() {
    const [file, setFile] = useState<File | null>(null)
    const [result, setResult] = useState<ImportResult | null>(null)
    const [loading, setLoading] = useState(false)

    const handleFile = useCallback(async (f: File) => {
        setFile(f)
        setResult(null)
        setLoading(true)

        try {
            const res = await api.importStudents(f)
            setResult(res)
            if (res.errors.length === 0) {
                toast.success(`Импорт завершён: ${res.created} создано, ${res.updated} обновлено`)
            } else {
                toast.warning(`Импорт завершён с ошибками (${res.errors.length})`)
            }
        } catch (e: unknown) {
            toast.error(e instanceof Error ? e.message : 'Ошибка импорта')
            setFile(null)
        } finally {
            setLoading(false)
        }
    }, [])

    const handleDownloadTemplate = async () => {
        try {
            await api.downloadImportTemplate()
            toast.success('Шаблон скачан')
        } catch {
            toast.error('Не удалось скачать шаблон')
        }
    }

    const reset = () => {
        setFile(null)
        setResult(null)
    }
    return (
        <div className="space-y-6">
            {/* Header */}
            <div>
                <Button variant="ghost" size="sm" asChild className="mb-2 -ml-2">
                    <Link to="/admin/university">
                        <ArrowLeft className="mr-1 h-4 w-4" />Назад
                    </Link>
                </Button>
                <h2 className="text-lg font-semibold">Импорт студентов</h2>
                <p className="text-sm text-muted-foreground">
                    Массовый импорт студентов из Excel-файла
                </p>
            </div>

            {/* Template */}
            <Card>
                <CardContent className="pt-6">
                    <div className="flex items-center gap-4">
                        <div className="rounded-lg bg-muted p-3">
                            <FileSpreadsheet className="h-6 w-6 text-muted-foreground" />
                        </div>
                        <div className="flex-1">
                            <h3 className="font-medium">Шаблон для импорта</h3>
                            <p className="text-sm text-muted-foreground">
                                Скачайте шаблон Excel с примером заполнения
                            </p>
                        </div>
                        <Button variant="outline" onClick={handleDownloadTemplate}>
                            <Download className="mr-1.5 h-4 w-4" />
                            Скачать
                        </Button>
                    </div>
                </CardContent>
            </Card>

            {/* Result */}
            {result && (
                <Card className="border-l-4 border-l-green-500">
                    <CardHeader>
                        <CardTitle className="text-base flex items-center gap-2">
                            <CheckCircle2 className="h-5 w-5 text-green-500" />
                            Импорт завершён
                        </CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="grid grid-cols-4 gap-4 text-center">
                            <div className="p-4 bg-muted/50 rounded-lg">
                                <div className="text-2xl font-bold">{result.total}</div>
                                <div className="text-xs text-muted-foreground">Всего</div>
                            </div>
                            <div className="p-4 bg-green-50 dark:bg-green-950/30 rounded-lg">
                                <div className="text-2xl font-bold text-green-600">{result.created}</div>
                                <div className="text-xs text-muted-foreground">Создано</div>
                            </div>
                            <div className="p-4 bg-blue-50 dark:bg-blue-950/30 rounded-lg">
                                <div className="text-2xl font-bold text-blue-600">{result.updated}</div>
                                <div className="text-xs text-muted-foreground">Обновлено</div>
                            </div>
                            <div className={cn(
                                'p-4 rounded-lg',
                                result.skipped > 0 ? 'bg-amber-50 dark:bg-amber-950/30' : 'bg-muted/50'
                            )}>
                                <div className={cn(
                                    'text-2xl font-bold',
                                    result.skipped > 0 ? 'text-amber-600' : ''
                                )}>{result.skipped}</div>
                                <div className="text-xs text-muted-foreground">Пропущено</div>
                            </div>
                        </div>

                        {result.errors.length > 0 && (
                            <div>
                                <h4 className="text-sm font-medium mb-2">Ошибки ({result.errors.length})</h4>
                                <div className="max-h-48 overflow-y-auto border rounded-md">
                                    <Table>
                                        <TableHeader>
                                            <TableRow>
                                                <TableHead className="w-16">Строка</TableHead>
                                                <TableHead className="w-32">Поле</TableHead>
                                                <TableHead>Ошибка</TableHead>
                                            </TableRow>
                                        </TableHeader>
                                        <TableBody>
                                            {result.errors.slice(0, 50).map((err, i) => (
                                                <TableRow key={i}>
                                                    <TableCell className="font-mono text-sm">{err.row}</TableCell>
                                                    <TableCell>
                                                        {err.field && <Badge variant="outline">{err.field}</Badge>}
                                                    </TableCell>
                                                    <TableCell className="text-sm text-destructive">{err.message}</TableCell>
                                                </TableRow>
                                            ))}
                                        </TableBody>
                                    </Table>
                                </div>
                            </div>
                        )}
                    </CardContent>
                    <CardFooter>
                        <Button variant="outline" onClick={reset}>
                            Загрузить другой файл
                        </Button>
                    </CardFooter>
                </Card>
            )}

            {/* Upload */}
            {!result && (
                <WasmUploader
                    onFile={handleFile}
                    loading={loading}
                    accept=".xlsx,.xls"
                />
            )}
        </div>
    )
}