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
    // Try parsing as JSON anyway (some servers don't set content-type)
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
  commands: number
}

export interface PluginMeta {
  id: string
  name: string
  version: string
  commands: { name: string; description: string; min_role?: string }[]
  permissions: { key: string; description: string; required: boolean }[]
  config_schema: Record<string, unknown> | null
  wasm_key: string
  wasm_hash: string
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

export const api = {
  listPlugins: () => request<PluginInfo[]>('/plugins'),

  getPlugin: (id: string) => request<PluginDetail>(`/plugins/${encodeURIComponent(id)}`),

  uploadPlugin: (file: File) => {
    const form = new FormData()
    form.append('wasm', file)
    return request<PluginMeta>('/plugins/upload', { method: 'POST', body: form })
  },

  installPlugin: (id: string, body: { wasm_key: string; config: unknown; permissions: string[] }) =>
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
}
