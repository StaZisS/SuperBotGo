import { useEffect, useState, useCallback } from 'react'
import { api, RefItem } from '@/api/client'
import { toast } from 'sonner'
import { Plus, Pencil, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import CascadingSelect, { CascadeLevel } from '@/components/CascadingSelect'

// ---- Cascade levels ----

const CASCADE_FACULTY: CascadeLevel[] = [
  { key: 'faculty', label: 'Факультет', fetchFn: () => api.listFaculties() },
]
const CASCADE_DEPT: CascadeLevel[] = [
  ...CASCADE_FACULTY,
  { key: 'department', label: 'Кафедра', fetchFn: (pid) => api.listDepartments(pid!) },
]
const CASCADE_PROG: CascadeLevel[] = [
  ...CASCADE_DEPT,
  { key: 'program', label: 'Направление', fetchFn: (pid) => api.listPrograms(pid!) },
]
const CASCADE_STREAM: CascadeLevel[] = [
  ...CASCADE_PROG,
  { key: 'stream', label: 'Поток', fetchFn: (pid) => api.listStreams(pid!) },
]
const CASCADE_GROUP: CascadeLevel[] = [
  ...CASCADE_STREAM,
  { key: 'group', label: 'Группа', fetchFn: (pid) => api.listGroups(pid!) },
]

// ---- Enum labels ----

const DEGREE_LABELS: Record<string, string> = { bachelor: 'Бакалавриат', master: 'Магистратура', specialist: 'Специалитет', phd: 'Аспирантура' }
const SUBGROUP_TYPE_LABELS: Record<string, string> = { language: 'Языковая', physical_education: 'Физкультура', elective: 'По выбору', lab: 'Лабораторная' }

// ---- Generic entity tab ----

interface FieldDef {
  key: string
  label: string
  type: 'text' | 'number' | 'select'
  options?: Record<string, string>
  required?: boolean
}

interface EntityTabConfig {
  parentCascade?: CascadeLevel[]
  parentKey?: string    // key in cascade that is the direct parent
  parentField?: string  // field name sent to API (e.g. "faculty_id")
  fields: FieldDef[]
  listFn: (parentId?: number) => Promise<RefItem[]>
  createFn: (data: any) => Promise<any>
  updateFn: (id: number, data: any) => Promise<any>
  deleteFn: (id: number) => Promise<any>
}

function EntityTab({ config }: { config: EntityTabConfig }) {
  const [items, setItems] = useState<RefItem[]>([])
  const [loading, setLoading] = useState(false)
  const [cascade, setCascade] = useState<Record<string, number | undefined>>({})
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form, setForm] = useState<Record<string, any>>({})

  const parentId = config.parentKey ? cascade[config.parentKey] : undefined
  const canList = !config.parentKey || parentId != null

  const load = useCallback(async () => {
    if (!canList) { setItems([]); return }
    setLoading(true)
    try {
      const data = await config.listFn(parentId)
      setItems(data || [])
    } catch (e: unknown) { toast.error((e as Error).message) }
    finally { setLoading(false) }
  }, [canList, parentId, config])

  useEffect(() => { load() }, [load])

  const openAdd = () => {
    setEditingId(null)
    const defaults: Record<string, any> = {}
    config.fields.forEach(f => { defaults[f.key] = f.type === 'number' ? '' : '' })
    if (config.parentField && parentId) defaults[config.parentField] = parentId
    setForm(defaults)
    setDialogOpen(true)
  }

  const openEdit = (item: RefItem) => {
    setEditingId(item.id)
    setForm({ code: item.code, name: item.name })
    if (config.parentField && parentId) setForm(prev => ({ ...prev, [config.parentField!]: parentId }))
    setDialogOpen(true)
  }

  const handleSave = async () => {
    const data = { ...form }
    config.fields.forEach(f => {
      if (f.type === 'number' && data[f.key] !== '') data[f.key] = Number(data[f.key])
      if (f.type === 'number' && data[f.key] === '') data[f.key] = null
    })
    if (config.parentField && parentId) data[config.parentField] = parentId
    try {
      if (editingId) {
        await config.updateFn(editingId, data)
        toast.success('Обновлено')
      } else {
        await config.createFn(data)
        toast.success('Создано')
      }
      setDialogOpen(false)
      load()
    } catch (e: unknown) { toast.error((e as Error).message) }
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Удалить запись?')) return
    try {
      await config.deleteFn(id)
      toast.success('Удалено')
      load()
    } catch (e: unknown) { toast.error((e as Error).message) }
  }

  const setField = (key: string, value: any) => setForm(prev => ({ ...prev, [key]: value }))

  return (
    <div className="space-y-4">
      {config.parentCascade && (
        <CascadingSelect levels={config.parentCascade} values={cascade} onChange={setCascade} />
      )}

      {canList && (
        <>
          <div className="flex justify-end">
            <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
              <Button variant="outline" size="sm" onClick={openAdd}>
                <Plus className="mr-1 h-4 w-4" />Добавить
              </Button>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>{editingId ? 'Редактировать' : 'Создать'}</DialogTitle>
                </DialogHeader>
                <div className="grid gap-3">
                  {config.fields.map(f => (
                    <div key={f.key} className="space-y-1.5">
                      <Label className="text-xs">{f.label}{f.required && ' *'}</Label>
                      {f.type === 'select' && f.options ? (
                        <Select value={form[f.key] || ''} onValueChange={v => setField(f.key, v)}>
                          <SelectTrigger><SelectValue placeholder="Выберите" /></SelectTrigger>
                          <SelectContent>
                            {Object.entries(f.options).map(([k, v]) => (
                              <SelectItem key={k} value={k}>{v}</SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      ) : (
                        <Input
                          type={f.type === 'number' ? 'number' : 'text'}
                          value={form[f.key] ?? ''}
                          onChange={e => setField(f.key, e.target.value)}
                        />
                      )}
                    </div>
                  ))}
                </div>
                <DialogFooter>
                  <Button onClick={handleSave}>Сохранить</Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>

          {loading ? (
            <p className="text-sm text-muted-foreground">Загрузка...</p>
          ) : items.length === 0 ? (
            <p className="text-sm text-muted-foreground">Нет записей</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-20">ID</TableHead>
                  <TableHead>Код</TableHead>
                  <TableHead>Название</TableHead>
                  <TableHead className="w-24 text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map(item => (
                  <TableRow key={item.id}>
                    <TableCell className="font-mono text-sm">{item.id}</TableCell>
                    <TableCell className="font-mono">{item.code}</TableCell>
                    <TableCell>{item.name}</TableCell>
                    <TableCell className="text-right space-x-1">
                      <Button variant="ghost" size="icon" onClick={() => openEdit(item)}>
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" onClick={() => handleDelete(item.id)}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </>
      )}

      {!canList && (
        <p className="text-sm text-muted-foreground py-4">Выберите родительский элемент в фильтре выше</p>
      )}
    </div>
  )
}

// ---- Tab configs ----

const TABS: { key: string; label: string; config: EntityTabConfig }[] = [
  {
    key: 'faculties', label: 'Факультеты',
    config: {
      fields: [
        { key: 'code', label: 'Код', type: 'text', required: true },
        { key: 'name', label: 'Название', type: 'text', required: true },
        { key: 'short_name', label: 'Сокращение', type: 'text' },
      ],
      listFn: () => api.listFaculties(),
      createFn: (d) => api.createFaculty(d),
      updateFn: (id, d) => api.updateFaculty(id, d),
      deleteFn: (id) => api.deleteFaculty(id),
    },
  },
  {
    key: 'departments', label: 'Кафедры',
    config: {
      parentCascade: CASCADE_FACULTY, parentKey: 'faculty', parentField: 'faculty_id',
      fields: [
        { key: 'code', label: 'Код', type: 'text', required: true },
        { key: 'name', label: 'Название', type: 'text', required: true },
        { key: 'short_name', label: 'Сокращение', type: 'text' },
      ],
      listFn: (pid) => api.listDepartments(pid!),
      createFn: (d) => api.createDepartment(d),
      updateFn: (id, d) => api.updateDepartment(id, d),
      deleteFn: (id) => api.deleteDepartment(id),
    },
  },
  {
    key: 'programs', label: 'Направления',
    config: {
      parentCascade: CASCADE_DEPT, parentKey: 'department', parentField: 'department_id',
      fields: [
        { key: 'code', label: 'Код', type: 'text', required: true },
        { key: 'name', label: 'Название', type: 'text', required: true },
        { key: 'degree_level', label: 'Уровень', type: 'select', options: DEGREE_LABELS, required: true },
      ],
      listFn: (pid) => api.listPrograms(pid!),
      createFn: (d) => api.createProgram(d),
      updateFn: (id, d) => api.updateProgram(id, d),
      deleteFn: (id) => api.deleteProgram(id),
    },
  },
  {
    key: 'streams', label: 'Потоки',
    config: {
      parentCascade: CASCADE_PROG, parentKey: 'program', parentField: 'program_id',
      fields: [
        { key: 'code', label: 'Код', type: 'text', required: true },
        { key: 'name', label: 'Название', type: 'text' },
        { key: 'year_started', label: 'Год начала', type: 'number' },
      ],
      listFn: (pid) => api.listStreams(pid!),
      createFn: (d) => api.createStream(d),
      updateFn: (id, d) => api.updateStream(id, d),
      deleteFn: (id) => api.deleteStream(id),
    },
  },
  {
    key: 'groups', label: 'Группы',
    config: {
      parentCascade: CASCADE_STREAM, parentKey: 'stream', parentField: 'stream_id',
      fields: [
        { key: 'code', label: 'Код', type: 'text', required: true },
        { key: 'name', label: 'Название', type: 'text' },
      ],
      listFn: (pid) => api.listGroups(pid!),
      createFn: (d) => api.createGroup(d),
      updateFn: (id, d) => api.updateGroup(id, d),
      deleteFn: (id) => api.deleteGroup(id),
    },
  },
  {
    key: 'subgroups', label: 'Подгруппы',
    config: {
      parentCascade: CASCADE_GROUP, parentKey: 'group', parentField: 'study_group_id',
      fields: [
        { key: 'code', label: 'Код', type: 'text', required: true },
        { key: 'name', label: 'Название', type: 'text' },
        { key: 'subgroup_type', label: 'Тип', type: 'select', options: SUBGROUP_TYPE_LABELS, required: true },
      ],
      listFn: (pid) => api.listSubgroups(pid!),
      createFn: (d) => api.createSubgroup(d),
      updateFn: (id, d) => api.updateSubgroup(id, d),
      deleteFn: (id) => api.deleteSubgroup(id),
    },
  },
  {
    key: 'courses', label: 'Курсы',
    config: {
      fields: [
        { key: 'code', label: 'Код', type: 'text', required: true },
        { key: 'name', label: 'Название', type: 'text', required: true },
      ],
      listFn: () => api.listCourses(),
      createFn: (d) => api.createCourse(d),
      updateFn: (id, d) => api.updateCourse(id, d),
      deleteFn: (id) => api.deleteCourse(id),
    },
  },
]

// ---- Page ----

export default function UniversityHierarchy() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Структура университета</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Управление иерархией</CardTitle>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="faculties">
            <TabsList className="flex flex-wrap h-auto gap-1 mb-4">
              {TABS.map(t => (
                <TabsTrigger key={t.key} value={t.key} className="text-xs">{t.label}</TabsTrigger>
              ))}
            </TabsList>
            {TABS.map(t => (
              <TabsContent key={t.key} value={t.key}>
                <EntityTab config={t.config} />
              </TabsContent>
            ))}
          </Tabs>
        </CardContent>
      </Card>
    </div>
  )
}
