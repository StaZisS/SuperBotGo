import { useEffect, useState, useCallback } from 'react'
import {
  api, PersonInfo, AllPositions,
  StudentPositionInfo, TeacherPositionInfo, AdminAppointmentInfo,
} from '@/api/client'
import { toast } from 'sonner'
import { Plus, Trash2, Pencil, UserPlus, Search, Link } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Dialog, DialogTrigger, DialogContent, DialogHeader,
  DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import CascadingSelect, { CascadeLevel } from './CascadingSelect'
import { getErrorMessage } from '@/lib/utils'

// ---- Label maps ----

const STATUS_LABELS: Record<string, string> = { active: 'Активен', suspended: 'Приостановлен', ended: 'Завершён' }
const NATIONALITY_LABELS: Record<string, string> = { domestic: 'РФ', foreign: 'Иностранный' }
const FUNDING_LABELS: Record<string, string> = { budget: 'Бюджет', contract: 'Контракт' }
const EDUCATION_LABELS: Record<string, string> = { full_time: 'Очная', part_time: 'Заочная', remote: 'Дистант' }
const EMPLOYMENT_LABELS: Record<string, string> = { full_time: 'Штатный', part_time: 'Совместитель', hourly: 'Почасовик' }
const APPOINTMENT_LABELS: Record<string, string> = {
  dean: 'Декан', dept_head: 'Завкафедрой', program_director: 'Руководитель направления',
  stream_curator: 'Куратор потока', group_curator: 'Куратор группы', foreign_student_curator: 'Куратор иностр. студентов',
}
const SCOPE_LABELS: Record<string, string> = {
  university_wide: 'Университет', faculty: 'Факультет', department: 'Кафедра',
  program: 'Направление', stream: 'Поток', group: 'Группа',
}

function statusVariant(s: string): 'default' | 'secondary' | 'destructive' {
  if (s === 'active') return 'default'
  if (s === 'ended') return 'destructive'
  return 'secondary'
}

// ---- Cascade configs ----

const FULL_CASCADE: CascadeLevel[] = [
  { key: 'faculty', label: 'Факультет', fetchFn: () => api.listFaculties() },
  { key: 'department', label: 'Кафедра', fetchFn: (pid) => api.listDepartments(pid!) },
  { key: 'program', label: 'Направление', fetchFn: (pid) => api.listPrograms(pid!) },
  { key: 'stream', label: 'Поток', fetchFn: (pid) => api.listStreams(pid!) },
  { key: 'group', label: 'Группа', fetchFn: (pid) => api.listGroups(pid!) },
]

function cascadeLevelsForScope(scopeType: string): CascadeLevel[] {
  const depth: Record<string, number> = { university_wide: 0, faculty: 1, department: 2, program: 3, stream: 4, group: 5 }
  return FULL_CASCADE.slice(0, depth[scopeType] ?? 0)
}

// ---- Main component ----

interface Props {
  userId: number
}

export default function UserPositions({ userId }: Props) {
  const [person, setPerson] = useState<PersonInfo | null | undefined>(undefined) // undefined = loading
  const [positions, setPositions] = useState<AllPositions | null>(null)
  const [createPersonOpen, setCreatePersonOpen] = useState(false)
  const [linkSearchOpen, setLinkSearchOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<PersonInfo[]>([])
  const [searching, setSearching] = useState(false)
  const [personForm, setPersonForm] = useState<Omit<PersonInfo, 'id'>>({
    external_id: '', last_name: '', first_name: '', middle_name: '', email: '', phone: '',
  })

  const loadPerson = useCallback(async () => {
    try {
      const p = await api.getUserPerson(userId)
      setPerson(p)
      if (p) {
        const pos = await api.getUserPositions(userId)
        setPositions(pos)
      }
    } catch {
      setPerson(null)
    }
  }, [userId])

  useEffect(() => { loadPerson() }, [loadPerson])

  const handleCreatePerson = async () => {
    try {
      await api.createUserPerson(userId, personForm)
      toast.success('Профиль создан')
      setCreatePersonOpen(false)
      loadPerson()
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    }
  }

  const handleSearch = async () => {
    if (!searchQuery.trim()) return
    setSearching(true)
    try {
      const results = await api.searchUnlinkedPersons(searchQuery.trim())
      setSearchResults(results || [])
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setSearching(false)
    }
  }

  const handleLinkPerson = async (personId: number) => {
    try {
      await api.linkPersonToUser(userId, personId)
      toast.success('Профиль привязан')
      setLinkSearchOpen(false)
      setSearchQuery('')
      setSearchResults([])
      loadPerson()
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    }
  }

  // Loading
  if (person === undefined) {
    return <Card><CardContent className="py-6"><Skeleton className="h-24 w-full" /></CardContent></Card>
  }

  // No person linked
  if (!person) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Позиции</CardTitle>
          <CardDescription>У пользователя нет привязанного университетского профиля</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-2">
            {/* Search & Link existing person */}
            <Dialog open={linkSearchOpen} onOpenChange={(open) => { setLinkSearchOpen(open); if (!open) { setSearchQuery(''); setSearchResults([]) } }}>
              <DialogTrigger asChild>
                <Button variant="outline" size="sm"><Search className="mr-1.5 h-4 w-4" />Найти и привязать</Button>
              </DialogTrigger>
              <DialogContent className="max-w-lg">
                <DialogHeader><DialogTitle>Привязать университетский профиль</DialogTitle></DialogHeader>
                <div className="flex gap-2">
                  <Input
                    placeholder="Поиск по ФИО, external ID, email..."
                    value={searchQuery}
                    onChange={e => setSearchQuery(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && handleSearch()}
                  />
                  <Button onClick={handleSearch} disabled={searching || !searchQuery.trim()} size="sm">
                    {searching ? 'Поиск...' : 'Найти'}
                  </Button>
                </div>
                {searchResults.length > 0 && (
                  <div className="max-h-64 overflow-y-auto border rounded-md">
                    {searchResults.map(p => (
                      <div key={p.id} className="flex items-center justify-between p-3 border-b last:border-b-0 hover:bg-muted/50">
                        <div>
                          <div className="font-medium text-sm">{p.last_name} {p.first_name} {p.middle_name}</div>
                          <div className="text-xs text-muted-foreground">
                            {p.external_id && <span className="font-mono mr-2">{p.external_id}</span>}
                            {p.email && <span>{p.email}</span>}
                          </div>
                        </div>
                        <Button variant="outline" size="sm" onClick={() => handleLinkPerson(p.id)}>
                          <Link className="mr-1 h-3 w-3" />Привязать
                        </Button>
                      </div>
                    ))}
                  </div>
                )}
                {searchResults.length === 0 && searchQuery && !searching && (
                  <p className="text-sm text-muted-foreground text-center py-4">Свободные профили не найдены</p>
                )}
              </DialogContent>
            </Dialog>

            {/* Create new person */}
            <Dialog open={createPersonOpen} onOpenChange={setCreatePersonOpen}>
              <DialogTrigger asChild>
                <Button variant="ghost" size="sm" onClick={() => setPersonForm({
                  external_id: '', last_name: '', first_name: '', middle_name: '', email: '', phone: '',
                })}>
                  <UserPlus className="mr-1.5 h-4 w-4" />Создать новый
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader><DialogTitle>Создать университетский профиль</DialogTitle></DialogHeader>
                <div className="grid gap-3">
                  <div className="grid grid-cols-3 gap-3">
                    <div className="space-y-1.5">
                      <Label className="text-xs">Фамилия *</Label>
                      <Input value={personForm.last_name} onChange={e => setPersonForm(p => ({ ...p, last_name: e.target.value }))} />
                    </div>
                    <div className="space-y-1.5">
                      <Label className="text-xs">Имя *</Label>
                      <Input value={personForm.first_name} onChange={e => setPersonForm(p => ({ ...p, first_name: e.target.value }))} />
                    </div>
                    <div className="space-y-1.5">
                      <Label className="text-xs">Отчество</Label>
                      <Input value={personForm.middle_name || ''} onChange={e => setPersonForm(p => ({ ...p, middle_name: e.target.value }))} />
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div className="space-y-1.5">
                      <Label className="text-xs">External ID</Label>
                      <Input value={personForm.external_id || ''} onChange={e => setPersonForm(p => ({ ...p, external_id: e.target.value }))} placeholder="SSO логин" />
                    </div>
                    <div className="space-y-1.5">
                      <Label className="text-xs">Email</Label>
                      <Input value={personForm.email || ''} onChange={e => setPersonForm(p => ({ ...p, email: e.target.value }))} />
                    </div>
                  </div>
                </div>
                <DialogFooter>
                  <Button onClick={handleCreatePerson} disabled={!personForm.last_name || !personForm.first_name}>Создать</Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Позиции</CardTitle>
        <CardDescription>
          {person.last_name} {person.first_name} {person.middle_name}
          {person.external_id && <span className="ml-2 font-mono text-xs">({person.external_id})</span>}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue="student">
          <TabsList className="mb-4">
            <TabsTrigger value="student">Студент ({positions?.student.length || 0})</TabsTrigger>
            <TabsTrigger value="teacher">Преподаватель ({positions?.teacher.length || 0})</TabsTrigger>
            <TabsTrigger value="admin">Администратор ({positions?.admin.length || 0})</TabsTrigger>
          </TabsList>

          <TabsContent value="student">
            <StudentTab userId={userId} items={positions?.student || []} onReload={loadPerson} />
          </TabsContent>
          <TabsContent value="teacher">
            <TeacherTab userId={userId} items={positions?.teacher || []} onReload={loadPerson} />
          </TabsContent>
          <TabsContent value="admin">
            <AdminTab userId={userId} items={positions?.admin || []} onReload={loadPerson} />
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}

// ============================================================
// Student Tab
// ============================================================

function StudentTab({ userId, items, onReload }: { userId: number; items: StudentPositionInfo[]; onReload: () => void }) {
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<StudentPositionInfo | null>(null)
  const [cascade, setCascade] = useState<Record<string, number | undefined>>({})
  const [status, setStatus] = useState('active')
  const [nationality, setNationality] = useState('domestic')
  const [funding, setFunding] = useState('budget')
  const [education, setEducation] = useState('full_time')

  const openAdd = () => {
    setEditing(null)
    setCascade({})
    setStatus('active'); setNationality('domestic'); setFunding('budget'); setEducation('full_time')
    setOpen(true)
  }

  const openEdit = (item: StudentPositionInfo) => {
    setEditing(item)
    setCascade({
      faculty: item.faculty_id, department: item.department_id,
      program: item.program_id ?? undefined, stream: item.stream_id ?? undefined, group: item.study_group_id ?? undefined,
    })
    setStatus(item.status); setNationality(item.nationality_type); setFunding(item.funding_type); setEducation(item.education_form)
    setOpen(true)
  }

  const handleSave = async () => {
    const data = {
      program_id: cascade.program, stream_id: cascade.stream, study_group_id: cascade.group,
      status, nationality_type: nationality, funding_type: funding, education_form: education,
    }
    try {
      if (editing) {
        await api.updateStudentPosition(userId, editing.id, data)
        toast.success('Позиция обновлена')
      } else {
        await api.createStudentPosition(userId, data)
        toast.success('Позиция создана')
      }
      setOpen(false)
      onReload()
    } catch (e: unknown) { toast.error(getErrorMessage(e)) }
  }

  const handleDelete = async (id: number) => {
    try {
      await api.deleteStudentPosition(userId, id)
      toast.success('Позиция удалена')
      onReload()
    } catch (e: unknown) { toast.error(getErrorMessage(e)) }
  }

  return (
    <div>
      <div className="flex justify-end mb-3">
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button variant="outline" size="sm" onClick={openAdd}><Plus className="mr-1 h-4 w-4" />Добавить</Button>
          </DialogTrigger>
          <DialogContent className="max-w-lg">
            <DialogHeader><DialogTitle>{editing ? 'Редактировать' : 'Добавить'} студенческую позицию</DialogTitle></DialogHeader>
            <CascadingSelect levels={FULL_CASCADE} values={cascade} onChange={setCascade} />
            <div className="grid grid-cols-2 gap-3 mt-3">
              <EnumSelect label="Статус" value={status} onChange={setStatus} options={STATUS_LABELS} />
              <EnumSelect label="Гражданство" value={nationality} onChange={setNationality} options={NATIONALITY_LABELS} />
              <EnumSelect label="Финансирование" value={funding} onChange={setFunding} options={FUNDING_LABELS} />
              <EnumSelect label="Форма обучения" value={education} onChange={setEducation} options={EDUCATION_LABELS} />
            </div>
            <DialogFooter><Button onClick={handleSave}>Сохранить</Button></DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">Нет студенческих позиций</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Направление</TableHead>
              <TableHead>Поток</TableHead>
              <TableHead>Группа</TableHead>
              <TableHead>Статус</TableHead>
              <TableHead>Гражд.</TableHead>
              <TableHead>Финанс.</TableHead>
              <TableHead>Форма</TableHead>
              <TableHead className="text-right">Действия</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map(item => (
              <TableRow key={item.id}>
                <TableCell>{item.program_name || '-'}</TableCell>
                <TableCell>{item.stream_name || '-'}</TableCell>
                <TableCell>{item.study_group_name || '-'}</TableCell>
                <TableCell><Badge variant={statusVariant(item.status)}>{STATUS_LABELS[item.status] || item.status}</Badge></TableCell>
                <TableCell>{NATIONALITY_LABELS[item.nationality_type] || item.nationality_type}</TableCell>
                <TableCell>{FUNDING_LABELS[item.funding_type] || item.funding_type}</TableCell>
                <TableCell>{EDUCATION_LABELS[item.education_form] || item.education_form}</TableCell>
                <TableCell className="text-right space-x-1">
                  <Button variant="ghost" size="icon" onClick={() => openEdit(item)}><Pencil className="h-4 w-4" /></Button>
                  <Button variant="ghost" size="icon" onClick={() => handleDelete(item.id)}><Trash2 className="h-4 w-4 text-destructive" /></Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

// ============================================================
// Teacher Tab
// ============================================================

const TEACHER_CASCADE: CascadeLevel[] = FULL_CASCADE.slice(0, 2)

function TeacherTab({ userId, items, onReload }: { userId: number; items: TeacherPositionInfo[]; onReload: () => void }) {
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<TeacherPositionInfo | null>(null)
  const [cascade, setCascade] = useState<Record<string, number | undefined>>({})
  const [title, setTitle] = useState('')
  const [employment, setEmployment] = useState('full_time')
  const [status, setStatus] = useState('active')

  const openAdd = () => {
    setEditing(null); setCascade({}); setTitle(''); setEmployment('full_time'); setStatus('active')
    setOpen(true)
  }

  const openEdit = (item: TeacherPositionInfo) => {
    setEditing(item)
    setCascade({ faculty: item.faculty_id, department: item.department_id ?? undefined })
    setTitle(item.position_title); setEmployment(item.employment_type); setStatus(item.status)
    setOpen(true)
  }

  const handleSave = async () => {
    const data = { department_id: cascade.department, position_title: title, employment_type: employment, status }
    try {
      if (editing) {
        await api.updateTeacherPosition(userId, editing.id, data)
        toast.success('Позиция обновлена')
      } else {
        await api.createTeacherPosition(userId, data)
        toast.success('Позиция создана')
      }
      setOpen(false); onReload()
    } catch (e: unknown) { toast.error(getErrorMessage(e)) }
  }

  const handleDelete = async (id: number) => {
    try { await api.deleteTeacherPosition(userId, id); toast.success('Позиция удалена'); onReload() }
    catch (e: unknown) { toast.error(getErrorMessage(e)) }
  }

  return (
    <div>
      <div className="flex justify-end mb-3">
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button variant="outline" size="sm" onClick={openAdd}><Plus className="mr-1 h-4 w-4" />Добавить</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader><DialogTitle>{editing ? 'Редактировать' : 'Добавить'} позицию преподавателя</DialogTitle></DialogHeader>
            <CascadingSelect levels={TEACHER_CASCADE} values={cascade} onChange={setCascade} />
            <div className="space-y-1.5 mt-3">
              <Label className="text-xs">Должность</Label>
              <Input value={title} onChange={e => setTitle(e.target.value)} placeholder="доцент, профессор..." />
            </div>
            <div className="grid grid-cols-2 gap-3 mt-3">
              <EnumSelect label="Тип занятости" value={employment} onChange={setEmployment} options={EMPLOYMENT_LABELS} />
              <EnumSelect label="Статус" value={status} onChange={setStatus} options={STATUS_LABELS} />
            </div>
            <DialogFooter><Button onClick={handleSave} disabled={!title}>Сохранить</Button></DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">Нет преподавательских позиций</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Кафедра</TableHead>
              <TableHead>Должность</TableHead>
              <TableHead>Тип занятости</TableHead>
              <TableHead>Статус</TableHead>
              <TableHead className="text-right">Действия</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map(item => (
              <TableRow key={item.id}>
                <TableCell>{item.department_name || '-'}</TableCell>
                <TableCell>{item.position_title}</TableCell>
                <TableCell>{EMPLOYMENT_LABELS[item.employment_type] || item.employment_type}</TableCell>
                <TableCell><Badge variant={statusVariant(item.status)}>{STATUS_LABELS[item.status] || item.status}</Badge></TableCell>
                <TableCell className="text-right space-x-1">
                  <Button variant="ghost" size="icon" onClick={() => openEdit(item)}><Pencil className="h-4 w-4" /></Button>
                  <Button variant="ghost" size="icon" onClick={() => handleDelete(item.id)}><Trash2 className="h-4 w-4 text-destructive" /></Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

// ============================================================
// Admin Appointment Tab
// ============================================================

function AdminTab({ userId, items, onReload }: { userId: number; items: AdminAppointmentInfo[]; onReload: () => void }) {
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<AdminAppointmentInfo | null>(null)
  const [appointmentType, setAppointmentType] = useState('dean')
  const [scopeType, setScopeType] = useState('faculty')
  const [cascade, setCascade] = useState<Record<string, number | undefined>>({})
  const [status, setStatus] = useState('active')

  const scopeLevels = cascadeLevelsForScope(scopeType)

  const openAdd = () => {
    setEditing(null); setAppointmentType('dean'); setScopeType('faculty'); setCascade({}); setStatus('active')
    setOpen(true)
  }

  const openEdit = (item: AdminAppointmentInfo) => {
    setEditing(item)
    setAppointmentType(item.appointment_type); setScopeType(item.scope_type)
    setStatus(item.status)
    // We can't fully reconstruct the cascade for edit without parent IDs.
    // Set the last level's value; admin will need to re-select the cascade.
    setCascade({})
    setOpen(true)
  }

  const handleScopeTypeChange = (val: string) => {
    setScopeType(val)
    setCascade({})
  }

  const getScopeId = (): number | undefined => {
    if (scopeType === 'university_wide') return undefined
    const scopeKeys: Record<string, string> = {
      faculty: 'faculty', department: 'department', program: 'program', stream: 'stream', group: 'group',
    }
    const key = scopeKeys[scopeType]
    return key ? cascade[key] : undefined
  }

  const handleSave = async () => {
    const data = {
      appointment_type: appointmentType, scope_type: scopeType,
      scope_id: getScopeId(), status,
    }
    try {
      if (editing) {
        await api.updateAdminAppointment(userId, editing.id, data)
        toast.success('Назначение обновлено')
      } else {
        await api.createAdminAppointment(userId, data)
        toast.success('Назначение создано')
      }
      setOpen(false); onReload()
    } catch (e: unknown) { toast.error(getErrorMessage(e)) }
  }

  const handleDelete = async (id: number) => {
    try { await api.deleteAdminAppointment(userId, id); toast.success('Назначение удалено'); onReload() }
    catch (e: unknown) { toast.error(getErrorMessage(e)) }
  }

  return (
    <div>
      <div className="flex justify-end mb-3">
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button variant="outline" size="sm" onClick={openAdd}><Plus className="mr-1 h-4 w-4" />Добавить</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader><DialogTitle>{editing ? 'Редактировать' : 'Добавить'} назначение</DialogTitle></DialogHeader>
            <div className="grid grid-cols-2 gap-3">
              <EnumSelect label="Тип назначения" value={appointmentType} onChange={setAppointmentType} options={APPOINTMENT_LABELS} />
              <EnumSelect label="Тип области" value={scopeType} onChange={handleScopeTypeChange} options={SCOPE_LABELS} />
            </div>
            {scopeLevels.length > 0 && (
              <div className="mt-3">
                <CascadingSelect levels={scopeLevels} values={cascade} onChange={setCascade} />
              </div>
            )}
            <div className="mt-3">
              <EnumSelect label="Статус" value={status} onChange={setStatus} options={STATUS_LABELS} />
            </div>
            <DialogFooter><Button onClick={handleSave}>Сохранить</Button></DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">Нет административных назначений</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Тип назначения</TableHead>
              <TableHead>Тип области</TableHead>
              <TableHead>Область</TableHead>
              <TableHead>Статус</TableHead>
              <TableHead className="text-right">Действия</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map(item => (
              <TableRow key={item.id}>
                <TableCell>{APPOINTMENT_LABELS[item.appointment_type] || item.appointment_type}</TableCell>
                <TableCell>{SCOPE_LABELS[item.scope_type] || item.scope_type}</TableCell>
                <TableCell>{item.scope_name || '-'}</TableCell>
                <TableCell><Badge variant={statusVariant(item.status)}>{STATUS_LABELS[item.status] || item.status}</Badge></TableCell>
                <TableCell className="text-right space-x-1">
                  <Button variant="ghost" size="icon" onClick={() => openEdit(item)}><Pencil className="h-4 w-4" /></Button>
                  <Button variant="ghost" size="icon" onClick={() => handleDelete(item.id)}><Trash2 className="h-4 w-4 text-destructive" /></Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

// ============================================================
// Shared enum select
// ============================================================

function EnumSelect({ label, value, onChange, options }: {
  label: string; value: string; onChange: (v: string) => void; options: Record<string, string>
}) {
  return (
    <div className="space-y-1.5">
      <Label className="text-xs">{label}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger><SelectValue /></SelectTrigger>
        <SelectContent>
          {Object.entries(options).map(([k, v]) => (
            <SelectItem key={k} value={k}>{v}</SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
