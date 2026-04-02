import { useState, useEffect, useCallback, useMemo } from 'react'
import { api, RuleSchema, RuleParam, RuleParamOption } from '@/api/client'
import { cn } from '@/lib/utils'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from '@/components/ui/collapsible'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { X, Copy, AlertTriangle, CheckCircle2, Zap } from 'lucide-react'

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

let idSeq = 0
function newId() { return `r_${Date.now()}_${++idSeq}` }

function newCondition(schema: RuleSchema): Condition {
  const firstType = schema.condition_types[0]
  const values: Record<string, string> = {}
  for (const p of firstType?.params ?? []) values[p.name] = ''
  return { kind: 'condition', id: newId(), typeId: firstType?.id ?? '', values }
}

function newGroup(schema: RuleSchema, logic: Logic = 'AND'): RuleGroup {
  return { kind: 'group', id: newId(), logic, items: [newCondition(schema)] }
}

function renderTemplate(template: string, values: Record<string, string>): string {
  let result = template
  for (const [key, val] of Object.entries(values)) {
    if (val === '') return ''
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

function splitTopLevel(expr: string, op: string): string[] {
  let depth = 0
  const parts: string[] = []
  let current = ''
  for (let i = 0; i < expr.length; i++) {
    if (expr[i] === '(') depth++
    else if (expr[i] === ')') depth--
    else if (depth === 0 && expr.substring(i, i + op.length) === op) {
      parts.push(current.trim())
      current = ''
      i += op.length - 1
      continue
    }
    current += expr[i]
  }
  parts.push(current.trim())
  return parts
}

function stripOuterParens(expr: string): string {
  if (!expr.startsWith('(') || !expr.endsWith(')')) return expr
  let depth = 0
  for (let i = 0; i < expr.length - 1; i++) {
    if (expr[i] === '(') depth++
    else if (expr[i] === ')') depth--
    if (depth === 0) return expr
  }
  return expr.slice(1, -1)
}

function tryParseCondition(expr: string): Condition | null {
  const s = expr.trim()

  const attr = s.match(/^user\.(\w+)\s*(==|!=)\s*"([^"]*)"$/)
  if (attr) {
    return { kind: 'condition', id: newId(), typeId: 'attribute', values: { field: attr[1], operator: attr[2], value: attr[3] } }
  }

  const graph = s.match(/^check\("([^"]*)",\s*"([^"]*)",\s*"([^"]*)"\)$/)
  if (graph) {
    return { kind: 'condition', id: newId(), typeId: 'graph', values: { relation: graph[1], objectType: graph[2], objectId: graph[3] } }
  }

  const role = s.match(/^has_role\("([^"]*)"\)$/)
  if (role) {
    return { kind: 'condition', id: newId(), typeId: 'role', values: { roleName: role[1] } }
  }

  return null
}

function parseExpr(expr: string): RuleItem | null {
  const trimmed = stripOuterParens(expr.trim())
  if (!trimmed) return null

  const orParts = splitTopLevel(trimmed, ' || ')
  if (orParts.length > 1) {
    const items: RuleItem[] = []
    for (const part of orParts) {
      const item = parseExpr(part)
      if (!item) return null
      items.push(item)
    }
    return { kind: 'group', id: newId(), logic: 'OR', items }
  }

  const andParts = splitTopLevel(trimmed, ' && ')
  if (andParts.length > 1) {
    const items: RuleItem[] = []
    for (const part of andParts) {
      const item = parseExpr(part)
      if (!item) return null
      items.push(item)
    }
    return { kind: 'group', id: newId(), logic: 'AND', items }
  }

  return tryParseCondition(trimmed)
}

function parseExpression(expr: string): RuleGroup | null {
  if (!expr.trim()) return null
  const item = parseExpr(expr.trim())
  if (!item) return null
  if (item.kind === 'group') return item
  return { kind: 'group', id: newId(), logic: 'AND', items: [item] }
}

function isConditionComplete(condition: Condition, schema: RuleSchema): boolean {
  const ct = schema.condition_types.find((t) => t.id === condition.typeId)
  if (!ct) return false
  for (const p of ct.params) {
    if (!condition.values[p.name]) return false
  }
  return true
}

function isGroupComplete(group: RuleGroup, schema: RuleSchema): boolean {
  if (group.items.length === 0) return false
  return group.items.every((item) =>
    item.kind === 'condition'
      ? isConditionComplete(item, schema)
      : isGroupComplete(item, schema),
  )
}

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
  const [initialized, setInitialized] = useState(false)

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

  // Initialize: parse existing expression into builder, or start empty
  useEffect(() => {
    if (!schema || initialized) return
    setInitialized(true)

    if (expression) {
      const parsed = parseExpression(expression)
      if (parsed) {
        setRoot(parsed)
        setRawExpr(expression)
      } else {
        setRawExpr(expression)
        setRawMode(true)
      }
    } else {
      setRoot(newGroup(schema, 'AND'))
    }
  }, [schema, initialized, expression])

  // Builder -> expression sync
  useEffect(() => {
    if (!rawMode && schema && root && initialized) {
      const expr = buildExpression(root, schema)
      setRawExpr(expr)
      onChange(expr)
    }
  }, [root, rawMode, schema, initialized, onChange])

  // Raw -> expression sync
  useEffect(() => {
    if (rawMode && initialized) onChange(rawExpr)
  }, [rawExpr, rawMode, initialized, onChange])

  const switchToBuilder = useCallback(() => {
    setRawMode(false)
    if (!schema) return

    if (rawExpr) {
      const parsed = parseExpression(rawExpr)
      if (parsed) {
        setRoot(parsed)
        return
      }
    }

    if (!root) {
      setRoot(newGroup(schema, 'AND'))
    }
  }, [schema, root, rawExpr])

  const updateRoot = useCallback((updater: (prev: RuleGroup) => RuleGroup) => {
    setRoot((prev) => prev ? updater(prev) : prev)
  }, [])

  const applyPreset = useCallback((expr: string) => {
    const parsed = parseExpression(expr)
    if (parsed) {
      setRoot(parsed)
      setRawMode(false)
    }
  }, [])

  if (!schema || !initialized) {
    return <div className="text-sm text-muted-foreground">Загрузка схемы...</div>
  }

  const activeTab = rawMode ? 'expression' : 'builder'

  return (
    <Tabs
      value={activeTab}
      onValueChange={(val) => {
        if (val === 'builder') {
          switchToBuilder()
        } else {
          setRawMode(true)
        }
      }}
    >
      <TabsList className="mb-3">
        <TabsTrigger value="builder">Конструктор</TabsTrigger>
        <TabsTrigger value="expression">Выражение</TabsTrigger>
      </TabsList>

      <TabsContent value="expression">
        <Textarea
          value={rawExpr}
          onChange={(e) => setRawExpr(e.target.value)}
          placeholder='напр. check("member", "faculty", "engineering") || has_role("ADMIN")'
          rows={3}
          className="font-mono resize-y"
        />
        <HelpBlock />
      </TabsContent>

      <TabsContent value="builder">
        {root ? (
          <div>
            <PresetsBar schema={schema} onApply={applyPreset} />

            <GroupEditor
              group={root}
              schema={schema}
              fieldValueMap={fieldValueMap}
              onChange={(g) => updateRoot(() => g)}
              isRoot
            />

            <ExpressionPreview root={root} schema={schema} />
          </div>
        ) : null}
      </TabsContent>
    </Tabs>
  )
}

function PresetsBar({
  schema,
  onApply,
}: {
  schema: RuleSchema
  onApply: (expr: string) => void
}) {
  const presets = useMemo(() => {
    const list: { label: string; expr: string }[] = []
    if (schema.condition_types.some((t) => t.id === 'role')) {
      list.push({ label: 'Только админы', expr: 'has_role("ADMIN")' })
    }
    return list
  }, [schema])

  if (presets.length === 0) return null

  return (
    <div className="flex items-center gap-2 mb-3 flex-wrap">
      <span className="text-xs text-muted-foreground flex items-center gap-1">
        <Zap className="h-3 w-3" />
        Шаблоны:
      </span>
      {presets.map((p) => (
        <Button
          key={p.label}
          variant="outline"
          size="sm"
          className="h-7 text-xs border-dashed"
          onClick={() => onApply(p.expr)}
        >
          {p.label}
        </Button>
      ))}
    </div>
  )
}

function ExpressionPreview({
  root,
  schema,
}: {
  root: RuleGroup
  schema: RuleSchema
}) {
  const expr = buildExpression(root, schema)
  const complete = isGroupComplete(root, schema)

  return (
    <div
      className={cn(
        'mt-3 p-3 rounded-lg border-l-4',
        complete
          ? 'bg-green-50/50 border-l-green-500'
          : 'bg-amber-50/50 border-l-amber-500',
      )}
    >
      <div className="flex items-center gap-1.5 mb-1">
        {complete ? (
          <CheckCircle2 className="h-3.5 w-3.5 text-green-600 shrink-0" />
        ) : (
          <AlertTriangle className="h-3.5 w-3.5 text-amber-600 shrink-0" />
        )}
        <span className={cn('text-xs font-medium', complete ? 'text-green-700' : 'text-amber-700')}>
          {complete ? 'Выражение готово' : 'Заполните все поля условий'}
        </span>
      </div>
      <code className="text-xs font-mono text-foreground/80 break-all">
        {expr || '\u2014'}
      </code>
    </div>
  )
}

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

  const duplicateItem = (id: string) => {
    const idx = group.items.findIndex((i) => i.id === id)
    if (idx === -1) return
    const item = group.items[idx]
    if (item.kind !== 'condition') return
    const clone: Condition = { ...item, id: newId(), values: { ...item.values } }
    const newItems = [...group.items]
    newItems.splice(idx + 1, 0, clone)
    onChange({ ...group, items: newItems })
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
  const badgeVariant = group.logic === 'AND' ? 'default' : 'warning'

  return (
    <Card className={cn('p-3', borderColor, bgColor, !isRoot && 'ml-2')}>
      <div className="flex items-center gap-2 mb-2">
        <Select
          value={group.logic}
          onValueChange={(v) => onChange({ ...group, logic: v as Logic })}
        >
          <SelectTrigger className="h-7 w-auto min-w-[120px] text-xs font-semibold">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="AND" className="text-xs">И (AND)</SelectItem>
            <SelectItem value="OR" className="text-xs">ИЛИ (OR)</SelectItem>
          </SelectContent>
        </Select>
        <span className="text-xs text-muted-foreground">
          {group.logic === 'AND' ? 'все условия должны выполняться' : 'хотя бы одно условие'}
        </span>
        {!isRoot && onRemove && (
          <Button
            variant="ghost"
            size="icon"
            onClick={onRemove}
            className="ml-auto h-6 w-6 text-muted-foreground hover:text-destructive"
          >
            <X className="h-3.5 w-3.5" />
          </Button>
        )}
      </div>

      <div className="space-y-1.5">
        {group.items.map((item, i) => (
          <div key={item.id}>
            {i > 0 && (
              <div className="text-center py-0.5">
                <Badge variant={badgeVariant} className="text-xs">
                  {group.logic === 'AND' ? 'И' : 'ИЛИ'}
                </Badge>
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
                onDuplicate={() => duplicateItem(item.id)}
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

      <div className="flex gap-2 mt-2">
        <Button variant="outline" size="sm" onClick={addCondition} className="border-dashed text-xs">
          + Условие
        </Button>
        <Button variant="outline" size="sm" onClick={addSubGroup} className="border-dashed text-xs">
          + Группа ({group.logic === 'AND' ? 'ИЛИ' : 'И'})
        </Button>
      </div>
    </Card>
  )
}

function ConditionRow({
  condition,
  schema,
  fieldValueMap,
  onChangeType,
  onChangeValues,
  onRemove,
  onDuplicate,
}: {
  condition: Condition
  schema: RuleSchema
  fieldValueMap: Map<string, RuleParamOption[]>
  onChangeType: (typeId: string) => void
  onChangeValues: (values: Record<string, string>) => void
  onRemove?: () => void
  onDuplicate: () => void
}) {
  const ct = schema.condition_types.find((t) => t.id === condition.typeId)
  const complete = isConditionComplete(condition, schema)

  const setValue = (name: string, value: string) => {
    onChangeValues({ ...condition.values, [name]: value })
  }

  return (
    <div
      className={cn(
        'flex items-center gap-2 p-2 bg-background rounded-md border flex-wrap transition-colors',
        complete ? 'border-border' : 'border-amber-300 ring-1 ring-amber-200',
      )}
    >
      <Select value={condition.typeId} onValueChange={onChangeType}>
        <SelectTrigger className="h-8 w-auto min-w-[130px] text-xs shrink-0">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {schema.condition_types.map((t) => (
            <SelectItem key={t.id} value={t.id} className="text-xs">
              {t.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

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

      <div className="flex items-center gap-0.5 shrink-0 ml-auto">
        <Button
          variant="ghost"
          size="icon"
          onClick={onDuplicate}
          className="h-7 w-7 text-muted-foreground hover:text-foreground"
          title="Дублировать условие"
        >
          <Copy className="h-3.5 w-3.5" />
        </Button>
        {onRemove && (
          <Button
            variant="ghost"
            size="icon"
            onClick={onRemove}
            className="h-7 w-7 text-muted-foreground hover:text-destructive"
            title="Удалить условие"
          >
            <X className="h-3.5 w-3.5" />
          </Button>
        )}
      </div>
    </div>
  )
}

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
      <Select value={value || undefined} onValueChange={onChange}>
        <SelectTrigger
          className={cn(
            'h-8 w-auto min-w-[100px] text-xs',
            !value && 'text-muted-foreground',
          )}
        >
          <SelectValue placeholder={`\u2014 ${param.label} \u2014`} />
        </SelectTrigger>
        <SelectContent>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value} className="text-xs">
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    )
  }

  if (param.type === 'text_or_select' && options.length > 0) {
    return (
      <div className="flex items-center gap-1 flex-1 min-w-0">
        <Select
          value={options.some((o) => o.value === value) ? value : undefined}
          onValueChange={onChange}
        >
          <SelectTrigger className="h-8 w-auto min-w-[100px] text-xs shrink-0">
            <SelectValue placeholder={`\u2014 ${param.label} \u2014`} />
          </SelectTrigger>
          <SelectContent>
            {options.map((o) => (
              <SelectItem key={o.value} value={o.value} className="text-xs">
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <span className="text-xs text-muted-foreground shrink-0">или</span>
        <Input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={param.placeholder ?? 'своё значение'}
          className="h-8 text-xs flex-1 min-w-0"
        />
      </div>
    )
  }

  return (
    <Input
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={param.placeholder ?? param.label}
      className={cn('h-8 text-xs flex-1 min-w-0', !value && 'border-amber-300')}
    />
  )
}

function HelpBlock() {
  const [open, setOpen] = useState(false)

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="mt-2">
      <CollapsibleTrigger className="text-xs text-muted-foreground hover:text-foreground cursor-pointer">
        {open ? '- ' : '+ '}Справка по синтаксису
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="mt-2 p-3 bg-muted rounded-lg space-y-1 font-mono text-xs">
          <div><span className="text-primary">user.nationality_type</span>, <span className="text-primary">user.funding_type</span>, <span className="text-primary">user.education_form</span></div>
          <div><span className="text-primary">user.groups</span>, <span className="text-primary">user.roles</span>, <span className="text-primary">user.external_id</span></div>
          <div><span className="text-blue-600">check(relation, obj_type, obj_id)</span> — обход графа</div>
          <div><span className="text-blue-600">is_member(obj_type, obj_id)</span> — проверка членства</div>
          <div><span className="text-blue-600">has_role(name)</span>, <span className="text-blue-600">has_any_role(n1, n2)</span></div>
          <div className="text-muted-foreground">Операторы: &amp;&amp;, ||, !, ==, !=, in</div>
          <div className="text-muted-foreground">Скобки для приоритета: (A &amp;&amp; B) || C</div>
        </div>
      </CollapsibleContent>
    </Collapsible>
  )
}
