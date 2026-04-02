import { useEffect, useState, useCallback } from 'react'
import { RefItem } from '@/api/client'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export interface CascadeLevel {
  key: string
  label: string
  fetchFn: (parentId?: number) => Promise<RefItem[]>
}

interface Props {
  levels: CascadeLevel[]
  values: Record<string, number | undefined>
  onChange: (values: Record<string, number | undefined>) => void
}

export default function CascadingSelect({ levels, values, onChange }: Props) {
  const [options, setOptions] = useState<Record<string, RefItem[]>>({})
  const [loading, setLoading] = useState<Record<string, boolean>>({})

  const loadLevel = useCallback(async (index: number, parentId?: number) => {
    const level = levels[index]
    if (!level) return
    setLoading(prev => ({ ...prev, [level.key]: true }))
    try {
      const items = await level.fetchFn(parentId)
      setOptions(prev => ({ ...prev, [level.key]: items || [] }))
    } catch {
      setOptions(prev => ({ ...prev, [level.key]: [] }))
    } finally {
      setLoading(prev => ({ ...prev, [level.key]: false }))
    }
  }, [levels])

  // Load first level on mount
  useEffect(() => {
    if (levels.length > 0) {
      loadLevel(0)
    }
  }, [levels, loadLevel])

  // Load subsequent levels when parent values are set (for edit mode initialization)
  useEffect(() => {
    for (let i = 1; i < levels.length; i++) {
      const parentKey = levels[i - 1].key
      const parentVal = values[parentKey]
      const currentKey = levels[i].key
      if (parentVal && !options[currentKey]?.length && !loading[currentKey]) {
        loadLevel(i, parentVal)
      }
    }
  }, [levels, values, options, loading, loadLevel])

  const handleChange = (index: number, val: string) => {
    const level = levels[index]
    const numVal = val ? Number(val) : undefined
    const newValues = { ...values, [level.key]: numVal }
    // Clear all levels below
    for (let i = index + 1; i < levels.length; i++) {
      newValues[levels[i].key] = undefined
      setOptions(prev => ({ ...prev, [levels[i].key]: [] }))
    }
    onChange(newValues)
    // Load next level
    if (numVal && index + 1 < levels.length) {
      loadLevel(index + 1, numVal)
    }
  }

  return (
    <div className="grid grid-cols-1 gap-3">
      {levels.map((level, i) => {
        const parentKey = i > 0 ? levels[i - 1].key : null
        const disabled = parentKey ? !values[parentKey] : false
        const items = options[level.key] || []
        const isLoading = loading[level.key]

        return (
          <div key={level.key} className="space-y-1.5">
            <Label className="text-xs">{level.label}</Label>
            <Select
              value={values[level.key] != null ? String(values[level.key]) : ''}
              onValueChange={(v) => handleChange(i, v)}
              disabled={disabled}
            >
              <SelectTrigger>
                <SelectValue placeholder={isLoading ? 'Загрузка...' : `Выберите`} />
              </SelectTrigger>
              <SelectContent>
                {items.map(item => (
                  <SelectItem key={item.id} value={String(item.id)}>
                    {item.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )
      })}
    </div>
  )
}
