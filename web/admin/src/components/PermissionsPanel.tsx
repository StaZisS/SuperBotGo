interface Permission {
  key: string
  description: string
  required: boolean
}

interface Props {
  permissions: Permission[]
  selected: string[]
  onChange: (selected: string[]) => void
  readOnly?: boolean
}

export default function PermissionsPanel({ permissions, selected, onChange, readOnly }: Props) {
  const isChecked = (key: string, required: boolean) =>
    required || selected.includes(key)

  const toggle = (key: string, required: boolean) => {
    if (required || readOnly) return
    if (selected.includes(key)) {
      onChange(selected.filter((k) => k !== key))
    } else {
      onChange([...selected, key])
    }
  }

  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium text-gray-700">Permissions</h3>
      {permissions.length === 0 && (
        <p className="text-sm text-gray-400">No permissions required.</p>
      )}
      {permissions.map((p) => (
        <label
          key={p.key}
          className={`flex items-start gap-3 p-2 rounded ${
            readOnly || p.required ? 'opacity-75' : 'hover:bg-gray-50 cursor-pointer'
          }`}
        >
          <input
            type="checkbox"
            checked={isChecked(p.key, p.required)}
            disabled={p.required || readOnly}
            onChange={() => toggle(p.key, p.required)}
            className="mt-0.5 h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
          />
          <div className="min-w-0">
            <div className="text-sm font-mono break-all">
              {p.key}
              {p.required && (
                <span className="ml-2 text-xs text-red-600 font-medium font-sans">Required</span>
              )}
              {!p.required && (
                <span className="ml-2 text-xs text-gray-400 font-sans">Optional</span>
              )}
            </div>
            {p.description && (
              <div className="text-xs text-gray-500 mt-0.5">{p.description}</div>
            )}
          </div>
        </label>
      ))}
    </div>
  )
}
