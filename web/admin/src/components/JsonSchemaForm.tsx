import { useState, type ReactNode } from 'react'
import { X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

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
  const fieldId = `field-${name}`

  const label = (
    <Label htmlFor={fieldId} className="mb-1 block">
      {name}
      {required && <span className="text-destructive ml-1">*</span>}
      {prop.description && <span className="text-xs text-muted-foreground ml-2">{prop.description}</span>}
    </Label>
  )

  const errorHint = error ? (
    <p className="mt-1 text-xs text-destructive">{error}</p>
  ) : null

  if (prop.enum) {
    return (
      <div>
        {label}
        <select
          id={fieldId}
          value={(value as string) ?? ''}
          onChange={(e) => onChange(e.target.value)}
          disabled={readOnly}
          className={cn(
            'flex h-10 w-full rounded-md border bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
            error ? 'border-destructive focus-visible:ring-destructive' : 'border-input',
          )}
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
        <Switch
          id={fieldId}
          checked={!!value}
          onCheckedChange={(checked) => onChange(checked)}
          disabled={readOnly}
        />
        <Label htmlFor={fieldId} className="cursor-pointer">
          {name}
          {prop.description && <span className="text-xs text-muted-foreground ml-2">{prop.description}</span>}
        </Label>
      </div>
    )
  }

  if (prop.type === 'number' || prop.type === 'integer') {
    return (
      <div>
        {label}
        <Input
          id={fieldId}
          type="number"
          value={(value as number) ?? prop.default ?? ''}
          min={prop.minimum}
          max={prop.maximum}
          step={prop.type === 'integer' ? 1 : undefined}
          disabled={readOnly}
          onChange={(e) => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
          className={cn(error && 'border-destructive focus-visible:ring-destructive')}
        />
        {errorHint}
      </div>
    )
  }

  if (prop.type === 'object' && prop.properties) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-medium">{name}</CardTitle>
        </CardHeader>
        <CardContent>
          <JsonSchemaForm
            schema={prop as Schema}
            value={(value as Record<string, unknown>) ?? {}}
            onChange={(v) => onChange(v)}
            readOnly={readOnly}
          />
        </CardContent>
      </Card>
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
      <Input
        id={fieldId}
        type={isSensitive ? 'password' : 'text'}
        value={(value as string) ?? ''}
        disabled={readOnly}
        onChange={(e) => onChange(e.target.value)}
        className={cn(error && 'border-destructive focus-visible:ring-destructive')}
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
          <Badge key={i} variant="secondary" className="gap-1">
            {item}
            {!readOnly && (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="h-4 w-4 p-0 hover:bg-transparent hover:text-destructive"
                onClick={() => onChange(arr.filter((_, j) => j !== i))}
                aria-label={`Remove ${item}`}
              >
                <X className="h-3 w-3" />
              </Button>
            )}
          </Badge>
        ))}
      </div>
      {!readOnly && (
        <div className="flex gap-2">
          <Input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                addItem()
              }
            }}
            className="flex-1"
            placeholder="Add item and press Enter"
          />
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={addItem}
          >
            Add
          </Button>
        </div>
      )}
      {error}
    </div>
  )
}
