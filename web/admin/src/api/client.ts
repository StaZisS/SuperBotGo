const BASE = '/api/admin'

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(`${BASE}${url}`, {
      ...init,
      headers: {
        ...(init?.headers ?? {}),
        ...(init?.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
      },
    })
  } catch {
    throw new ApiError('Network error: unable to reach server', 0)
  }

  let data: unknown
  const contentType = res.headers.get('content-type') ?? ''
  if (contentType.includes('application/json')) {
    try {
      data = await res.json()
    } catch {
      throw new ApiError('Invalid JSON response from server', res.status)
    }
  } else {
    const text = await res.text()
    if (!res.ok) {
      throw new ApiError(text || `HTTP ${res.status}`, res.status)
    }
    try {
      data = JSON.parse(text)
    } catch {
      throw new ApiError(`Unexpected response format (HTTP ${res.status})`, res.status)
    }
  }

  if (!res.ok) {
    const msg =
      (data && typeof data === 'object' && 'error' in data && typeof (data as Record<string, unknown>).error === 'string')
        ? (data as Record<string, string>).error
        : `HTTP ${res.status}`
    throw new ApiError(msg, res.status)
  }

  return data as T
}

export interface PluginInfo {
  id: string
  name: string
  version: string
  type: 'go' | 'wasm'
  status: 'active' | 'disabled' | 'error'
  triggers: number
}

export interface TriggerDef {
  name: string
  type: 'cron' | 'http' | 'event' | 'messenger'
  description?: string
  min_role?: string
  schedule?: string
  path?: string
  methods?: string[]
  topic?: string
}

export interface PluginMeta {
  id: string
  name: string
  version: string
  triggers: TriggerDef[]
  requirements: { type: string; description: string; target?: string }[]
  config_schema: Record<string, unknown> | null
  wasm_key: string
  wasm_hash: string
  existing_version?: string
}

export interface PluginDetail {
  id: string
  name?: string
  version?: string
  type?: string
  status?: string
  commands?: { name: string; description: string; min_role?: string }[]
  meta?: PluginMeta
  config?: unknown
  permissions?: string[]
  wasm_hash?: string
  installed_at?: string
  updated_at?: string
}

export interface CommandSetting {
  id: number
  plugin_id: string
  command_name: string
  enabled: boolean
  policy_expression: string
  created_at: string
  updated_at: string
}

export interface RuleParamOption {
  value: string
  label: string
}

export interface RuleParam {
  name: string
  label: string
  type: 'select' | 'text' | 'text_or_select'
  placeholder?: string
  options?: RuleParamOption[]
  depends_on?: string
}

export interface RuleConditionType {
  id: string
  label: string
  template: string
  params: RuleParam[]
}

export interface RuleSchema {
  condition_types: RuleConditionType[]
  field_values: Record<string, RuleParamOption[]>
}

export interface RequirementInfo {
  type: string
  description: string
  target?: string
}

export interface PluginRequirementsDetail {
  requirements: RequirementInfo[]
}

export interface VersionInfo {
  id: number
  plugin_id: string
  version: string
  wasm_key: string
  wasm_hash: string
  config_json: unknown
  permissions: string[]
  changelog: string
  created_at: string
}

export interface ChannelStatus {
  name: string
  type: string
  status: 'connected' | 'disconnected' | 'not_configured'
}

export interface ChatReference {
  id: number
  channel_type: string
  platform_chat_id: string
  chat_kind: string
  title: string
}

export interface BroadcastResult {
  chat_id: number
  channel_type: string
  status: 'sent' | 'error'
  error?: string
}

export interface AccountBrief {
  channel_type: string
  username?: string
}

export interface UserListItem {
  id: number
  locale: string
  role: string
  person_name?: string
  accounts: AccountBrief[]
  created_at?: string
}

export interface UserDetail {
  id: number
  primary_channel: string
  locale: string
  role: string
  person_name?: string
  accounts: AccountInfo[]
  created_at?: string
}

export interface AccountInfo {
  id: number
  channel_type: string
  channel_user_id: string
  username?: string
  linked_at: string
}

export interface UserRole {
  id: number
  role_name: string
  role_type: string
  scope?: string
  granted_at: string
}

// === POSITIONS ===

export interface RefItem {
  id: number
  code: string
  name: string
}

export interface PersonInfo {
  id: number
  external_id?: string
  last_name: string
  first_name: string
  middle_name?: string
  email?: string
  phone?: string
}

export interface StudentPositionInfo {
  id: number
  program_id?: number
  program_name?: string
  stream_id?: number
  stream_name?: string
  study_group_id?: number
  study_group_name?: string
  status: string
  nationality_type: string
  funding_type: string
  education_form: string
  department_id?: number
  faculty_id?: number
}

export interface TeacherPositionInfo {
  id: number
  department_id?: number
  department_name?: string
  position_title: string
  employment_type: string
  status: string
  faculty_id?: number
}

export interface AdminAppointmentInfo {
  id: number
  appointment_type: string
  scope_type: string
  scope_id?: number
  scope_name?: string
  status: string
}

export interface AllPositions {
  student: StudentPositionInfo[]
  teacher: TeacherPositionInfo[]
  admin: AdminAppointmentInfo[]
}

// Helpers for university reference CRUD — reduce per-entity boilerplate.
const refList = (res: string, param?: string) => (parentId?: number) => {
  const qs = param && parentId != null ? `?${param}=${parentId}` : ''
  return request<RefItem[]>(`/university/${res}${qs}`)
}
const refCreate = (res: string) => (data: Record<string, unknown>) =>
  request<{ id: number }>(`/university/manage/${res}`, { method: 'POST', body: JSON.stringify(data) })
const refUpdate = (res: string) => (id: number, data: Record<string, unknown>) =>
  request<{ status: string }>(`/university/manage/${res}/${id}`, { method: 'PUT', body: JSON.stringify(data) })
const refDelete = (res: string) => (id: number) =>
  request<{ status: string }>(`/university/manage/${res}/${id}`, { method: 'DELETE' })

export const api = {
  listPlugins: () => request<PluginInfo[]>('/plugins'),

  getPlugin: (id: string) => request<PluginDetail>(`/plugins/${encodeURIComponent(id)}`),

  uploadPlugin: (file: File) => {
    const form = new FormData()
    form.append('wasm', file)
    return request<PluginMeta>('/plugins/upload', { method: 'POST', body: form })
  },

  installPlugin: (id: string, body: { wasm_key: string; config: unknown }) =>
    request<{ id: string; status: string }>(`/plugins/${encodeURIComponent(id)}/install`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  updateConfig: (id: string, config: unknown) =>
    request<{ status: string }>(`/plugins/${encodeURIComponent(id)}/config`, {
      method: 'PUT',
      body: JSON.stringify({ config }),
    }),

  updatePlugin: (id: string, file: File) => {
    const form = new FormData()
    form.append('wasm', file)
    return request<{ status: string }>(`/plugins/${encodeURIComponent(id)}/update`, {
      method: 'POST',
      body: form,
    })
  },

  disablePlugin: (id: string) =>
    request<{ status: string }>(`/plugins/${encodeURIComponent(id)}/disable`, { method: 'POST' }),

  enablePlugin: (id: string) =>
    request<{ status: string }>(`/plugins/${encodeURIComponent(id)}/enable`, { method: 'POST' }),

  deletePlugin: (id: string) =>
    request<{ status: string }>(`/plugins/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  getRuleSchema: () => request<RuleSchema>('/rule-schema'),

  listCommandSettings: (pluginId: string) =>
    request<CommandSetting[]>(`/plugins/${encodeURIComponent(pluginId)}/commands/settings`),

  setCommandEnabled: (pluginId: string, commandName: string, enabled: boolean) =>
    request<{ status: string }>(
      `/plugins/${encodeURIComponent(pluginId)}/commands/${encodeURIComponent(commandName)}/enabled`,
      { method: 'PUT', body: JSON.stringify({ enabled }) },
    ),

  setCommandPolicy: (pluginId: string, commandName: string, expression: string) =>
    request<{ status: string }>(
      `/plugins/${encodeURIComponent(pluginId)}/commands/${encodeURIComponent(commandName)}/policy`,
      { method: 'PUT', body: JSON.stringify({ expression }) },
    ),

  getPluginRequirements: (id: string) =>
    request<PluginRequirementsDetail>(`/plugins/${encodeURIComponent(id)}/requirements`),

  listVersions: (pluginId: string) =>
    request<VersionInfo[]>(`/plugins/${encodeURIComponent(pluginId)}/versions`),

  rollbackVersion: (pluginId: string, versionId: number) =>
    request<{ status: string; version: string; version_id: number }>(
      `/plugins/${encodeURIComponent(pluginId)}/versions/${versionId}/rollback`,
      { method: 'POST' },
    ),

  deleteVersion: (pluginId: string, versionId: number) =>
    request<{ status: string }>(
      `/plugins/${encodeURIComponent(pluginId)}/versions/${versionId}`,
      { method: 'DELETE' },
    ),

  listChannelStatus: () => request<ChannelStatus[]>('/channels/status'),

  listChats: (params?: { channel_type?: string; chat_kind?: string }) => {
    const q = new URLSearchParams()
    if (params?.channel_type) q.set('channel_type', params.channel_type)
    if (params?.chat_kind) q.set('chat_kind', params.chat_kind)
    const qs = q.toString()
    return request<ChatReference[]>(`/chats${qs ? `?${qs}` : ''}`)
  },

  broadcast: (chatIds: number[], text: string) =>
    request<BroadcastResult[]>('/broadcast', {
      method: 'POST',
      body: JSON.stringify({ chat_ids: chatIds, text }),
    }),

  // === USERS ===

  listUsers: (params?: { search?: string; role?: string; channel?: string; offset?: number; limit?: number }) => {
    const q = new URLSearchParams()
    if (params?.search) q.set('search', params.search)
    if (params?.role) q.set('role', params.role)
    if (params?.channel) q.set('channel', params.channel)
    if (params?.offset) q.set('offset', String(params.offset))
    if (params?.limit) q.set('limit', String(params.limit))
    return request<{ users: UserListItem[]; total: number }>(`/users${q.toString() ? '?' + q : ''}`)
  },

  getUser: (id: number) => request<UserDetail>(`/users/${id}`),

  updateUser: (id: number, data: { locale?: string; role?: string }) =>
      request<{ status: string }>(`/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

  deleteUser: (id: number) =>
      request<{ status: string }>(`/users/${id}`, { method: 'DELETE' }),

  getUserRoles: (userId: number) =>
      request<UserRole[]>(`/users/${userId}/roles`),

  removeUserRole: (userId: number, roleName: string, roleType: string) =>
      request<{ status: string }>(`/users/${userId}/roles`, {
        method: 'DELETE',
        body: JSON.stringify({ role_name: roleName, role_type: roleType }),
      }),

  unlinkAccount: (userId: number, accountId: number) =>
      request<{ status: string }>(`/users/${userId}/accounts/${accountId}`, { method: 'DELETE' }),

  // === UNIVERSITY REFERENCE DATA + CRUD ===

  listFaculties: refList('faculties'),
  createFaculty: refCreate('faculties'),
  updateFaculty: refUpdate('faculties'),
  deleteFaculty: refDelete('faculties'),

  listDepartments: refList('departments', 'faculty_id'),
  createDepartment: refCreate('departments'),
  updateDepartment: refUpdate('departments'),
  deleteDepartment: refDelete('departments'),

  listPrograms: refList('programs', 'department_id'),
  createProgram: refCreate('programs'),
  updateProgram: refUpdate('programs'),
  deleteProgram: refDelete('programs'),

  listStreams: refList('streams', 'program_id'),
  createStream: refCreate('streams'),
  updateStream: refUpdate('streams'),
  deleteStream: refDelete('streams'),

  listGroups: refList('groups', 'stream_id'),
  createGroup: refCreate('groups'),
  updateGroup: refUpdate('groups'),
  deleteGroup: refDelete('groups'),

  listSubgroups: refList('subgroups', 'study_group_id'),
  createSubgroup: refCreate('subgroups'),
  updateSubgroup: refUpdate('subgroups'),
  deleteSubgroup: refDelete('subgroups'),

  listCourses: refList('courses'),
  createCourse: refCreate('courses'),
  updateCourse: refUpdate('courses'),
  deleteCourse: refDelete('courses'),

  listSemesters: () => request<{ id: number; year: number; semester_type: string }[]>('/university/semesters'),
  createSemester: (data: { year: number; semester_type: string }) =>
      request<{ id: number }>('/university/manage/semesters', { method: 'POST', body: JSON.stringify(data) }),
  updateSemester: (id: number, data: { year: number; semester_type: string }) =>
      request<{ status: string }>(`/university/manage/semesters/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteSemester: (id: number) => request<{ status: string }>(`/university/manage/semesters/${id}`, { method: 'DELETE' }),

  // === PERSON & POSITIONS ===

  getUserPerson: (userId: number) => request<PersonInfo | null>(`/users/${userId}/person`),

  searchUnlinkedPersons: (query: string) => request<PersonInfo[]>(`/persons/search?q=${encodeURIComponent(query)}`),

  linkPersonToUser: (userId: number, personId: number) =>
      request<{ status: string }>(`/users/${userId}/person/link`, { method: 'POST', body: JSON.stringify({ person_id: personId }) }),

  createUserPerson: (userId: number, data: Omit<PersonInfo, 'id'>) =>
      request<PersonInfo>(`/users/${userId}/person`, { method: 'POST', body: JSON.stringify(data) }),

  getUserPositions: (userId: number) => request<AllPositions>(`/users/${userId}/positions`),

  createStudentPosition: (userId: number, data: Omit<StudentPositionInfo, 'id' | 'program_name' | 'stream_name' | 'study_group_name' | 'department_id' | 'faculty_id'>) =>
      request<StudentPositionInfo>(`/users/${userId}/positions/student`, { method: 'POST', body: JSON.stringify(data) }),
  updateStudentPosition: (userId: number, posId: number, data: Omit<StudentPositionInfo, 'id' | 'program_name' | 'stream_name' | 'study_group_name' | 'department_id' | 'faculty_id'>) =>
      request<{ status: string }>(`/users/${userId}/positions/student/${posId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteStudentPosition: (userId: number, posId: number) =>
      request<{ status: string }>(`/users/${userId}/positions/student/${posId}`, { method: 'DELETE' }),

  createTeacherPosition: (userId: number, data: Omit<TeacherPositionInfo, 'id' | 'department_name' | 'faculty_id'>) =>
      request<TeacherPositionInfo>(`/users/${userId}/positions/teacher`, { method: 'POST', body: JSON.stringify(data) }),
  updateTeacherPosition: (userId: number, posId: number, data: Omit<TeacherPositionInfo, 'id' | 'department_name' | 'faculty_id'>) =>
      request<{ status: string }>(`/users/${userId}/positions/teacher/${posId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTeacherPosition: (userId: number, posId: number) =>
      request<{ status: string }>(`/users/${userId}/positions/teacher/${posId}`, { method: 'DELETE' }),

  createAdminAppointment: (userId: number, data: Omit<AdminAppointmentInfo, 'id' | 'scope_name'>) =>
      request<AdminAppointmentInfo>(`/users/${userId}/positions/admin-appointment`, { method: 'POST', body: JSON.stringify(data) }),
  updateAdminAppointment: (userId: number, posId: number, data: Omit<AdminAppointmentInfo, 'id' | 'scope_name'>) =>
      request<{ status: string }>(`/users/${userId}/positions/admin-appointment/${posId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteAdminAppointment: (userId: number, posId: number) =>
      request<{ status: string }>(`/users/${userId}/positions/admin-appointment/${posId}`, { method: 'DELETE' }),
}
