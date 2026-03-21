import { useState, type ReactNode } from 'react'

interface SchemaProperty {
  type?: string
  description?: string
  enum?: string[]
  default?: unknown
  minimum?: number
  maximum?: number
  minLength?: number
  maxLength?: number
  pattern?: string
  properties?: Record<string, SchemaProperty>
  items?: SchemaProperty
  required?: string[]
}

interface Schema extends SchemaProperty {
  required?: string[]
}

interface Props {
  schema: Schema
  value: Record<string, unknown>
  onChange: (value: Record<string, unknown>) => void
  readOnly?: boolean
  errors?: Record<string, string>
}

export function validateField(key: string, prop: SchemaProperty, value: unknown, isRequired: boolean): string | null {
  if (isRequired && (value === undefined || value === null || value === '')) {
    return `${key} is required`
  }

  if (value === undefined || value === null || value === '') return null

  if ((prop.type === 'number' || prop.type === 'integer') && typeof value === 'number') {
    if (prop.minimum !== undefined && value < prop.minimum) {
      return `Minimum value is ${prop.minimum}`
    }
    if (prop.maximum !== undefined && value > prop.maximum) {
      return `Maximum value is ${prop.maximum}`
    }
    if (prop.type === 'integer' && !Number.isInteger(value)) {
      return 'Must be an integer'
    }
  }

  if (prop.type === 'string' && typeof value === 'string') {
    if (prop.minLength !== undefined && value.length < prop.minLength) {
      return `Minimum length is ${prop.minLength}`
    }
    if (prop.maxLength !== undefined && value.length > prop.maxLength) {
      return `Maximum length is ${prop.maxLength}`
    }
    if (prop.pattern) {
      try {
        if (!new RegExp(prop.pattern).test(value)) {
          return `Must match pattern: ${prop.pattern}`
        }
      } catch {
      }
    }
  }

  return null
}

export function validateSchema(schema: Schema, value: Record<string, unknown>): Record<string, string> {
  const errors: Record<string, string> = {}
  const properties = schema.properties ?? {}
  const required = schema.required ?? []

  for (const [key, prop] of Object.entries(properties)) {
    const err = validateField(key, prop, value[key], required.includes(key))
    if (err) errors[key] = err

    if (prop.type === 'object' && prop.properties && value[key] && typeof value[key] === 'object') {
      const nested = validateSchema(prop as Schema, value[key] as Record<string, unknown>)
      for (const [nk, nv] of Object.entries(nested)) {
        errors[`${key}.${nk}`] = nv
      }
    }
  }

  return errors
}

export default function JsonSchemaForm({ schema, value, onChange, readOnly, errors }: Props) {
  const properties = schema.properties ?? {}
  const required = schema.required ?? []

  return (
    <div className="space-y-4">
      {Object.entries(properties).map(([key, prop]) => (
        <FieldRenderer
          key={key}
          name={key}
          prop={prop}
          value={value[key]}
          required={required.includes(key)}
          readOnly={readOnly}
          onChange={(v) => onChange({ ...value, [key]: v })}
          error={errors?.[key]}
        />
      ))}
    </div>
  )
}

function FieldRenderer({
  name,
  prop,
  value,
  required,
  readOnly,
  onChange,
  error,
}: {
  name: string
  prop: SchemaProperty
  value: unknown
  required: boolean
  readOnly?: boolean
  onChange: (v: unknown) => void
  error?: string
}) {
  const label = (
    <label className="block text-sm font-medium text-gray-700 mb-1">
      {name}
      {required && <span className="text-red-500 ml-1">*</span>}
      {prop.description && <span className="text-xs text-gray-400 ml-2">{prop.description}</span>}
    </label>
  )

  const inputBase =
    'w-full px-3 py-2 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500'
  const inputClass = error
    ? `${inputBase} border-red-400 focus:ring-red-500`
    : `${inputBase} border-gray-300`

  const errorHint = error ? (
    <p className="mt-1 text-xs text-red-600">{error}</p>
  ) : null

  if (prop.enum) {
    return (
      <div>
        {label}
        <select
          value={(value as string) ?? ''}
          onChange={(e) => onChange(e.target.value)}
          disabled={readOnly}
          className={inputClass}
        >
          <option value="">Select...</option>
          {prop.enum.map((opt) => (
            <option key={opt} value={opt}>
              {opt}
            </option>
          ))}
        </select>
        {errorHint}
      </div>
    )
  }

  if (prop.type === 'boolean') {
    return (
      <div className="flex items-center gap-3">
        <button
          type="button"
          role="switch"
          aria-checked={!!value}
          disabled={readOnly}
          onClick={() => onChange(!value)}
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
            value ? 'bg-blue-600' : 'bg-gray-300'
          } ${readOnly ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}`}
        >
          <span
            className={`inline-block h-4 w-4 rounded-full bg-white transition-transform ${
              value ? 'translate-x-6' : 'translate-x-1'
            }`}
          />
        </button>
        <span className="text-sm text-gray-700">
          {name}
          {prop.description && <span className="text-xs text-gray-400 ml-2">{prop.description}</span>}
        </span>
      </div>
    )
  }

  if (prop.type === 'number' || prop.type === 'integer') {
    return (
      <div>
        {label}
        <input
          type="number"
          value={(value as number) ?? prop.default ?? ''}
          min={prop.minimum}
          max={prop.maximum}
          step={prop.type === 'integer' ? 1 : undefined}
          disabled={readOnly}
          onChange={(e) => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
          className={inputClass}
        />
        {errorHint}
      </div>
    )
  }

  if (prop.type === 'object' && prop.properties) {
    return (
      <fieldset className="border border-gray-200 rounded-lg p-4">
        <legend className="text-sm font-medium text-gray-700 px-2">{name}</legend>
        <JsonSchemaForm
          schema={prop as Schema}
          value={(value as Record<string, unknown>) ?? {}}
          onChange={(v) => onChange(v)}
          readOnly={readOnly}
        />
      </fieldset>
    )
  }

  if (prop.type === 'array' && prop.items?.type === 'string') {
    return (
      <ArrayStringField
        name={name}
        value={value}
        readOnly={readOnly}
        onChange={onChange}
        label={label}
        error={errorHint}
      />
    )
  }

  const isSensitive =
    /secret|password|token|api_key|apikey/i.test(name)

  return (
    <div>
      {label}
      <input
        type={isSensitive ? 'password' : 'text'}
        value={(value as string) ?? ''}
        disabled={readOnly}
        onChange={(e) => onChange(e.target.value)}
        className={inputClass}
        autoComplete={isSensitive ? 'off' : undefined}
      />
      {errorHint}
    </div>
  )
}

function ArrayStringField({
  name: _name,
  value,
  readOnly,
  onChange,
  label,
  error,
}: {
  name: string
  value: unknown
  readOnly?: boolean
  onChange: (v: unknown) => void
  label: ReactNode
  error: ReactNode
}) {
  const [input, setInput] = useState('')
  const arr = Array.isArray(value) ? (value as string[]) : []

  const addItem = () => {
    const trimmed = input.trim()
    if (!trimmed) return
    onChange([...arr, trimmed])
    setInput('')
  }

  return (
    <div>
      {label}
      <div className="flex flex-wrap gap-2 mb-2">
        {arr.map((item, i) => (
          <span key={i} className="inline-flex items-center gap-1 px-2 py-1 bg-gray-100 rounded text-sm">
            {item}
            {!readOnly && (
              <button
                type="button"
                onClick={() => onChange(arr.filter((_, j) => j !== i))}
                className="text-gray-400 hover:text-red-500 leading-none"
                aria-label={`Remove ${item}`}
              >
                &times;
              </button>
            )}
          </span>
        ))}
      </div>
      {!readOnly && (
        <div className="flex gap-2">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                addItem()
              }
            }}
            className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="Add item and press Enter"
          />
          <button
            type="button"
            onClick={addItem}
            className="px-3 py-2 border border-gray-300 rounded-lg text-sm hover:bg-gray-50"
          >
            Add
          </button>
        </div>
      )}
      {error}
    </div>
  )
}
