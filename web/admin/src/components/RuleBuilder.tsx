import { useState, useEffect, useCallback } from 'react'
import { api, RuleSchema, RuleParam, RuleParamOption } from '../api/client'

// ── Модель ──

interface Condition {
  kind: 'condition'
  id: string
  typeId: string
  values: Record<string, string>
}

interface RuleGroup {
  kind: 'group'
  id: string
  logic: 'AND' | 'OR'
  items: RuleItem[]
}

type RuleItem = Condition | RuleGroup

type Logic = 'AND' | 'OR'

// ── ID генератор ──

let idSeq = 0
function newId() { return `r_${++idSeq}` }

function newCondition(schema: RuleSchema): Condition {
  const firstType = schema.condition_types[0]
  const values: Record<string, string> = {}
  for (const p of firstType?.params ?? []) values[p.name] = ''
  return { kind: 'condition', id: newId(), typeId: firstType?.id ?? '', values }
}

function newGroup(schema: RuleSchema, logic: Logic = 'AND'): RuleGroup {
  return { kind: 'group', id: newId(), logic, items: [newCondition(schema)] }
}

// ── Генерация выражения ──

function renderTemplate(template: string, values: Record<string, string>): string {
  let result = template
  for (const [key, val] of Object.entries(values)) {
    result = result.split(`{${key}}`).join(val)
  }
  if (/\{[a-zA-Z]+\}/.test(result)) return ''
  return result
}

function itemToExpr(item: RuleItem, schema: RuleSchema): string {
  if (item.kind === 'condition') {
    const ct = schema.condition_types.find((t) => t.id === item.typeId)
    if (!ct) return ''
    return renderTemplate(ct.template, item.values)
  }
  // group
  const parts = item.items.map((i) => itemToExpr(i, schema)).filter(Boolean)
  if (parts.length === 0) return ''
  if (parts.length === 1) return parts[0]
  const sep = item.logic === 'AND' ? ' && ' : ' || '
  return '(' + parts.join(sep) + ')'
}

function buildExpression(root: RuleGroup, schema: RuleSchema): string {
  const parts = root.items.map((i) => itemToExpr(i, schema)).filter(Boolean)
  if (parts.length === 0) return ''
  if (parts.length === 1) return parts[0]
  const sep = root.logic === 'AND' ? ' && ' : ' || '
  return parts.join(sep)
}

// ── Хелпер для options ──

function getOptionsForParam(
  param: RuleParam,
  values: Record<string, string>,
  fieldValueMap: Map<string, RuleParamOption[]>,
): RuleParamOption[] {
  if (param.depends_on) {
    const depValue = values[param.depends_on]
    if (depValue && fieldValueMap.has(depValue)) {
      return fieldValueMap.get(depValue)!
    }
    return []
  }
  return param.options ?? []
}

// ── Компонент ──

export default function RuleBuilder({
  expression,
  onChange,
}: {
  expression: string
  onChange: (expr: string) => void
}) {
  const [schema, setSchema] = useState<RuleSchema | null>(null)
  const [root, setRoot] = useState<RuleGroup | null>(null)
  const [rawMode, setRawMode] = useState(false)
  const [rawExpr, setRawExpr] = useState(expression)
  const [fieldValueMap, setFieldValueMap] = useState<Map<string, RuleParamOption[]>>(new Map())

  // Загрузка схемы
  useEffect(() => {
    api.getRuleSchema().then((s) => {
      setSchema(s)
      if (s.field_values) {
        const map = new Map<string, RuleParamOption[]>()
        for (const [field, opts] of Object.entries(s.field_values)) {
          map.set(field, opts)
        }
        setFieldValueMap(map)
      }
    }).catch(() => {
      setSchema({ condition_types: [], field_values: {} })
    })
  }, [])

  // Если выражение уже задано — raw mode
  useEffect(() => {
    if (expression && !root) {
      setRawExpr(expression)
      setRawMode(true)
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Генерация выражения из визуального билдера
  useEffect(() => {
    if (!rawMode && schema && root) {
      const expr = buildExpression(root, schema)
      setRawExpr(expr)
      onChange(expr)
    }
  }, [root, rawMode, schema]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (rawMode) onChange(rawExpr)
  }, [rawExpr, rawMode]) // eslint-disable-line react-hooks/exhaustive-deps

  const switchToBuilder = useCallback(() => {
    setRawMode(false)
    if (schema && !root) {
      setRoot(newGroup(schema, 'AND'))
    }
  }, [schema, root])

  const updateRoot = useCallback((updater: (prev: RuleGroup) => RuleGroup) => {
    setRoot((prev) => prev ? updater(prev) : prev)
  }, [])

  if (!schema) {
    return <div className="text-sm text-gray-400">Загрузка схемы...</div>
  }

  return (
    <div>
      <div className="flex gap-2 mb-3">
        <button
          onClick={switchToBuilder}
          className={`px-3 py-1 text-xs rounded-lg border ${!rawMode ? 'bg-purple-50 border-purple-300 text-purple-700' : 'border-gray-300 text-gray-600 hover:bg-gray-50'}`}
        >
          Конструктор
        </button>
        <button
          onClick={() => setRawMode(true)}
          className={`px-3 py-1 text-xs rounded-lg border ${rawMode ? 'bg-purple-50 border-purple-300 text-purple-700' : 'border-gray-300 text-gray-600 hover:bg-gray-50'}`}
        >
          Выражение
        </button>
      </div>

      {rawMode ? (
        <div>
          <textarea
            value={rawExpr}
            onChange={(e) => setRawExpr(e.target.value)}
            placeholder='напр. check("member", "faculty", "engineering") || has_role("ADMIN")'
            rows={3}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono focus:ring-2 focus:ring-purple-500 resize-y"
          />
          <HelpBlock />
        </div>
      ) : root ? (
        <div>
          <GroupEditor
            group={root}
            schema={schema}
            fieldValueMap={fieldValueMap}
            onChange={(g) => updateRoot(() => g)}
            isRoot
          />

          {/* Превью */}
          <div className="mt-3 p-2 bg-gray-50 rounded-lg">
            <div className="text-xs text-gray-400 mb-1">Сгенерированное выражение:</div>
            <code className="text-xs font-mono text-purple-700 break-all">
              {buildExpression(root, schema) || '(пусто)'}
            </code>
          </div>
        </div>
      ) : null}
    </div>
  )
}

// ── Редактор группы ──

function GroupEditor({
  group,
  schema,
  fieldValueMap,
  onChange,
  onRemove,
  isRoot,
}: {
  group: RuleGroup
  schema: RuleSchema
  fieldValueMap: Map<string, RuleParamOption[]>
  onChange: (g: RuleGroup) => void
  onRemove?: () => void
  isRoot?: boolean
}) {
  const updateItem = (id: string, updated: RuleItem) => {
    onChange({ ...group, items: group.items.map((i) => (i.id === id ? updated : i)) })
  }

  const removeItem = (id: string) => {
    const filtered = group.items.filter((i) => i.id !== id)
    onChange({ ...group, items: filtered })
  }

  const addCondition = () => {
    onChange({ ...group, items: [...group.items, newCondition(schema)] })
  }

  const addSubGroup = () => {
    const subLogic: Logic = group.logic === 'AND' ? 'OR' : 'AND'
    onChange({ ...group, items: [...group.items, newGroup(schema, subLogic)] })
  }

  const borderColor = group.logic === 'AND' ? 'border-blue-200' : 'border-orange-200'
  const bgColor = group.logic === 'AND' ? 'bg-blue-50/30' : 'bg-orange-50/30'
  const badgeColor = group.logic === 'AND' ? 'bg-blue-100 text-blue-700' : 'bg-orange-100 text-orange-700'

  return (
    <div className={`border ${borderColor} ${bgColor} rounded-lg p-3 ${isRoot ? '' : 'ml-2'}`}>
      {/* Заголовок группы */}
      <div className="flex items-center gap-2 mb-2">
        <select
          value={group.logic}
          onChange={(e) => onChange({ ...group, logic: e.target.value as Logic })}
          className={`px-2 py-0.5 rounded text-xs font-semibold border-0 ${badgeColor}`}
        >
          <option value="AND">И (AND)</option>
          <option value="OR">ИЛИ (OR)</option>
        </select>
        <span className="text-xs text-gray-400">
          {group.logic === 'AND' ? 'все условия должны выполняться' : 'хотя бы одно условие'}
        </span>
        {!isRoot && onRemove && (
          <button onClick={onRemove} className="ml-auto text-gray-300 hover:text-red-500 text-sm">
            &times;
          </button>
        )}
      </div>

      {/* Элементы */}
      <div className="space-y-1.5">
        {group.items.map((item, i) => (
          <div key={item.id}>
            {i > 0 && (
              <div className="text-center py-0.5">
                <span className={`text-xs px-1.5 py-0.5 rounded ${badgeColor}`}>
                  {group.logic === 'AND' ? 'И' : 'ИЛИ'}
                </span>
              </div>
            )}
            {item.kind === 'condition' ? (
              <ConditionRow
                condition={item}
                schema={schema}
                fieldValueMap={fieldValueMap}
                onChangeType={(typeId) => {
                  const ct = schema.condition_types.find((t) => t.id === typeId)
                  if (!ct) return
                  const values: Record<string, string> = {}
                  for (const p of ct.params) values[p.name] = ''
                  updateItem(item.id, { ...item, typeId, values })
                }}
                onChangeValues={(values) => updateItem(item.id, { ...item, values })}
                onRemove={group.items.length > 1 ? () => removeItem(item.id) : undefined}
              />
            ) : (
              <GroupEditor
                group={item}
                schema={schema}
                fieldValueMap={fieldValueMap}
                onChange={(g) => updateItem(item.id, g)}
                onRemove={group.items.length > 1 ? () => removeItem(item.id) : undefined}
              />
            )}
          </div>
        ))}
      </div>

      {/* Кнопки добавления */}
      <div className="flex gap-2 mt-2">
        <button
          onClick={addCondition}
          className="px-2.5 py-1 text-xs border border-dashed border-gray-300 text-gray-500 rounded hover:border-gray-400 hover:text-gray-700"
        >
          + Условие
        </button>
        <button
          onClick={addSubGroup}
          className="px-2.5 py-1 text-xs border border-dashed border-gray-300 text-gray-500 rounded hover:border-gray-400 hover:text-gray-700"
        >
          + Группа ({group.logic === 'AND' ? 'ИЛИ' : 'И'})
        </button>
      </div>
    </div>
  )
}

// ── Строка условия ──

function ConditionRow({
  condition,
  schema,
  fieldValueMap,
  onChangeType,
  onChangeValues,
  onRemove,
}: {
  condition: Condition
  schema: RuleSchema
  fieldValueMap: Map<string, RuleParamOption[]>
  onChangeType: (typeId: string) => void
  onChangeValues: (values: Record<string, string>) => void
  onRemove?: () => void
}) {
  const ct = schema.condition_types.find((t) => t.id === condition.typeId)

  const setValue = (name: string, value: string) => {
    onChangeValues({ ...condition.values, [name]: value })
  }

  return (
    <div className="flex items-center gap-2 p-2 bg-white rounded border border-gray-200 flex-wrap">
      <select
        value={condition.typeId}
        onChange={(e) => onChangeType(e.target.value)}
        className="px-2 py-1.5 border border-gray-300 rounded text-xs bg-white shrink-0"
      >
        {schema.condition_types.map((t) => (
          <option key={t.id} value={t.id}>{t.label}</option>
        ))}
      </select>

      {ct?.params.map((param) => (
        <ParamInput
          key={param.name}
          param={param}
          value={condition.values[param.name] ?? ''}
          allValues={condition.values}
          fieldValueMap={fieldValueMap}
          onChange={(v) => setValue(param.name, v)}
        />
      ))}

      {onRemove && (
        <button
          onClick={onRemove}
          className="text-gray-300 hover:text-red-500 text-lg leading-none shrink-0 px-1"
        >
          &times;
        </button>
      )}
    </div>
  )
}

// ── Поле параметра ──

function ParamInput({
  param,
  value,
  allValues,
  fieldValueMap,
  onChange,
}: {
  param: RuleParam
  value: string
  allValues: Record<string, string>
  fieldValueMap: Map<string, RuleParamOption[]>
  onChange: (v: string) => void
}) {
  const options = getOptionsForParam(param, allValues, fieldValueMap)

  if (param.type === 'select') {
    return (
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="px-2 py-1.5 border border-gray-300 rounded text-xs bg-white"
      >
        <option value="">— {param.label} —</option>
        {options.map((o) => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    )
  }

  if (param.type === 'text_or_select' && options.length > 0) {
    return (
      <div className="flex gap-1 flex-1 min-w-0">
        <select
          value={options.some((o) => o.value === value) ? value : ''}
          onChange={(e) => { if (e.target.value) onChange(e.target.value) }}
          className="px-2 py-1.5 border border-gray-300 rounded text-xs bg-white"
        >
          <option value="">— {param.label} —</option>
          {options.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={param.placeholder ?? ''}
          className="px-2 py-1.5 border border-gray-300 rounded text-xs flex-1 min-w-0"
        />
      </div>
    )
  }

  return (
    <input
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={param.placeholder ?? param.label}
      className="px-2 py-1.5 border border-gray-300 rounded text-xs flex-1 min-w-0"
    />
  )
}

// ── Справка ──

function HelpBlock() {
  return (
    <details className="text-xs text-gray-400 mt-2">
      <summary className="cursor-pointer hover:text-gray-600">Справка по синтаксису</summary>
      <div className="mt-2 p-3 bg-gray-50 rounded-lg space-y-1 font-mono text-xs">
        <div><span className="text-purple-600">user.nationality_type</span>, <span className="text-purple-600">user.funding_type</span>, <span className="text-purple-600">user.education_form</span></div>
        <div><span className="text-purple-600">user.groups</span>, <span className="text-purple-600">user.roles</span>, <span className="text-purple-600">user.external_id</span></div>
        <div><span className="text-blue-600">check(relation, obj_type, obj_id)</span> — обход графа</div>
        <div><span className="text-blue-600">is_member(obj_type, obj_id)</span> — проверка членства</div>
        <div><span className="text-blue-600">has_role(name)</span>, <span className="text-blue-600">has_any_role(n1, n2)</span></div>
        <div className="text-gray-500">Операторы: &amp;&amp;, ||, !, ==, !=, in</div>
        <div className="text-gray-500">Скобки для приоритета: (A &amp;&amp; B) || C</div>
      </div>
    </details>
  )
}
