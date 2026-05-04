import { useState, type KeyboardEvent } from 'react'
import { Link } from 'react-router-dom'
import {
  ArrowRight,
  BookOpen,
  CheckCircle2,
  ChevronRight,
  FileUp,
  Package,
  ShieldCheck,
  Users,
  type LucideIcon,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { HelpTooltip, docsHref } from '@/components/AdminHelp'
import { cn } from '@/lib/utils'

interface ActionItem {
  id: string
  title: string
  summary: string
  details: string[]
  result: string
  to?: string
}

interface Scenario {
  title: string
  description: string
  icon: LucideIcon
  actions: ActionItem[]
}

interface GlossaryItem {
  term: string
  context: string
  explanation: string
}

interface AdminSection {
  title: string
  route: string
  purpose: string
  useWhen: string
}

const scenarios: Scenario[] = [
  {
    title: 'Запустить плагин',
    description: 'От готового `.wasm` файла до активных команд, HTTP-адресов или задач по расписанию.',
    icon: Package,
    actions: [
      {
        id: 'upload-wasm',
        title: 'Загрузить `.wasm`',
        summary: 'Проверяет файл и показывает метаданные до установки.',
        details: [
          'Перед установкой видно имя, ID, версию, точки запуска, запрошенные ресурсы и обязательные настройки.',
          'Если плагин с таким ID уже есть, админка покажет, что это обновление или конфликт версии.',
          'До нажатия “Установить” текущий установленный плагин не меняется.',
        ],
        result: 'Появляется карточка проверки с точками запуска, ресурсами и формой настроек.',
        to: '/admin/plugins/upload',
      },
      {
        id: 'fill-config',
        title: 'Заполнить конфигурацию',
        summary: 'Форма строится из схемы, которую объявил плагин.',
        details: [
          'Админка показывает только те поля, которые нужны конкретному плагину.',
          'Секретные поля отображаются как пароли и не должны копироваться в открытые чаты или задачи.',
          'Если обязательные поля не заполнены, установка или сохранение настроек будет остановлено.',
        ],
        result: 'Плагин получает рабочие параметры и может быть установлен или переконфигурирован.',
        to: '/admin/plugins',
      },
      {
        id: 'check-triggers',
        title: 'Проверить точки запуска',
        summary: 'Показывает, какими способами плагин будет запускаться.',
        details: [
          'Команда в мессенджере будет доступна пользователям подключённых каналов.',
          'HTTP-точка запуска нужна внешним системам и может требовать пользовательскую сессию или сервисный ключ.',
          'Расписание и события запускают плагин без ручного действия пользователя.',
        ],
        result: 'Понятно, где ждать эффект от установленного плагина.',
        to: '/admin/plugins',
      },
    ],
  },
  {
    title: 'Ограничить доступ',
    description: 'Сделать точку запуска доступной только нужным пользователям или внешним сервисам.',
    icon: ShieldCheck,
    actions: [
      {
        id: 'trigger-enabled',
        title: 'Включить или выключить точку запуска',
        summary: 'Основной переключатель доступности конкретной команды или HTTP-адреса.',
        details: [
          'Если точка запуска выключена, она не должна исполняться даже при корректных параметрах.',
          'Это удобно для временного отключения проблемной команды без удаления всего плагина.',
          'Настройка хранится отдельно от WASM-файла, поэтому переживает обновления плагина.',
        ],
        result: 'Точка запуска остаётся в списке, но меняет доступность для пользователей и интеграций.',
        to: '/admin/plugins',
      },
      {
        id: 'policy-expression',
        title: 'Задать политику доступа',
        summary: 'Тонкое правило доступа на роли, группы, канал и атрибуты пользователя.',
        details: [
          'Политика пишется как выражение и должна возвращать `true` или `false`.',
          'В выражении доступны `user.*`, роли и функции вроде `has_role()` или `check()`.',
          'Пустая политика означает, что дополнительных ограничений нет, если точка запуска включена.',
        ],
        result: 'Команда запускается только для пользователей, которые проходят политику.',
        to: '/admin/plugins',
      },
      {
        id: 'service-key',
        title: 'Создать сервисный ключ',
        summary: 'Bearer-токен для вызова HTTP-точки запуска внешней системой.',
        details: [
          'Область действия ключа задаётся на конкретную HTTP-точку запуска, а не на всю админку.',
          'HTTP-точка запуска должна разрешать сервисные ключи на странице прав.',
          'Токен показывается один раз после создания, поэтому его нужно сразу передать интеграции.',
        ],
        result: 'Внешняя система получает ограниченный доступ к нужному HTTP-адресу.',
        to: '/admin/http/service-keys',
      },
    ],
  },
  {
    title: 'Подготовить аудиторию',
    description: 'Синхронизировать пользователей, роли и университетскую структуру для политик доступа.',
    icon: Users,
    actions: [
      {
        id: 'users-import',
        title: 'Импортировать студентов',
        summary: 'Создаёт или обновляет учебные данные из Excel-шаблона.',
        details: [
          'Импорт связывает студентов с программой, потоком, группой и подгруппами.',
          'Эти данные потом используются в проверках доступа к командам и HTTP-точкам запуска.',
          'Ошибки импорта показываются по строкам, чтобы исправлять файл итеративно.',
        ],
        result: 'В админке появляются пользователи и учебные позиции для авторизации.',
        to: '/admin/users',
      },
      {
        id: 'admin-access',
        title: 'Назначить администратора',
        summary: 'Роль ADMIN в пользователе ещё не равна доступу к системной админке.',
        details: [
          'Доступ к админке выдаётся отдельно в карточке пользователя.',
          'После выдачи доступа пользователь сможет войти в админку доступным для него способом.',
        ],
        result: 'Пользователь получает управляемый вход в системную админку.',
        to: '/admin/users',
      },
      {
        id: 'university-tree',
        title: 'Поддерживать структуру университета',
        summary: 'Справочники питают связи и проверки доступа.',
        details: [
          'Иерархия: факультет, кафедра, программа, поток, группа, подгруппа.',
          'Позиции студентов, преподавателей и администраторов превращаются в данные для проверок доступа.',
          'Если структура неполная, политика может работать формально правильно, но не находить нужных пользователей.',
        ],
        result: 'Политика получает корректный контекст групп, ролей и принадлежности.',
        to: '/admin/university',
      },
    ],
  },
]

const glossary: GlossaryItem[] = [
  {
    term: 'ID плагина',
    context: 'Показывается при загрузке и в URL страницы плагина.',
    explanation: 'Стабильный идентификатор плагина. По нему хранятся настройки, версии, права точек запуска и межплагинные вызовы.',
  },
  {
    term: 'Точка запуска',
    context: 'Команда, HTTP-адрес, расписание или событие.',
    explanation: 'Способ запустить плагин. Один WASM-плагин может объявить несколько точек запуска разных типов.',
  },
  {
    term: 'Схема настроек',
    context: 'Форма, которая появляется при установке или настройке плагина.',
    explanation: 'Типизированное описание полей. Админка использует его для UI и базовой валидации.',
  },
  {
    term: 'Требование ресурса',
    context: 'Список ресурсов на странице проверки или требований плагина.',
    explanation: 'Явная заявка плагина на ресурсы платформы: базу данных, HTTP, KV, файлы, уведомления, события или другой плагин.',
  },
  {
    term: 'Политика доступа',
    context: 'Поле в правах точки запуска.',
    explanation: 'Выражение, которое решает, может ли пользователь вызвать команду или HTTP-точку запуска. Пустое значение снимает дополнительное ограничение.',
  },
  {
    term: 'Область действия сервисного ключа',
    context: 'Выбор HTTP-точки запуска при создании ключа.',
    explanation: 'Граница доступа для внешнего сервиса. Ключ должен быть привязан только к тем HTTP-точкам запуска, которые реально нужны интеграции.',
  },
]

const sections: AdminSection[] = [
  {
    title: 'Плагины',
    route: '/admin/plugins',
    purpose: 'Установка, статус, версии, настройки и требования WASM-плагинов.',
    useWhen: 'Нужно добавить функциональность или понять, почему команда или HTTP-адрес не работает.',
  },
  {
    title: 'Права точек запуска',
    route: '/admin/plugins',
    purpose: 'Включение точек запуска, доступ по пользовательской сессии, сервисным ключам и политике.',
    useWhen: 'Нужно ограничить команду, закрыть HTTP-адрес или выдать доступ внешней системе.',
  },
  {
    title: 'HTTP-сервисные ключи',
    route: '/admin/http/service-keys',
    purpose: 'Создание токенов для внешних интеграций.',
    useWhen: 'Внешний сервис должен вызвать HTTP-точку запуска без пользовательской сессии в браузере.',
  },
  {
    title: 'Пользователи',
    route: '/admin/users',
    purpose: 'Глобальные пользователи, канальные аккаунты, роли, импорт студентов и доступ к админке.',
    useWhen: 'Нужно найти человека, назначить доступ или проверить его привязки.',
  },
  {
    title: 'Структура',
    route: '/admin/university',
    purpose: 'Факультеты, кафедры, программы, потоки, группы и подгруппы.',
    useWhen: 'Политика зависит от учебной принадлежности или нужно поправить справочники.',
  },
]

const allActions = scenarios.flatMap((scenario) => scenario.actions)

function ActionRow({
  action,
  active,
  onSelect,
}: {
  action: ActionItem
  active: boolean
  onSelect: () => void
}) {
  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key !== 'Enter' && event.key !== ' ') return
    event.preventDefault()
    onSelect()
  }

  return (
    <div
      className={cn(
        'group rounded-lg border p-4 transition-colors hover:border-foreground/30 hover:bg-muted/40',
        active && 'border-primary bg-primary/5',
      )}
    >
      <button
        type="button"
        aria-expanded={active}
        onClick={onSelect}
        onKeyDown={handleKeyDown}
        className="flex w-full items-start justify-between gap-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h4 className="font-medium leading-tight">{action.title}</h4>
          </div>
          <p className="mt-1 text-sm text-muted-foreground">{action.summary}</p>
        </div>
        <ChevronRight
          className={cn(
            'mt-0.5 h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5',
            active && 'rotate-90 text-primary',
          )}
        />
      </button>
      {active && (
        <div className="mt-4 space-y-3 rounded-lg border bg-background/80 p-3">
          <div className="space-y-2">
            {action.details.map((detail) => (
              <div key={detail} className="flex gap-2 text-sm">
                <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />
                <span>{detail}</span>
              </div>
            ))}
          </div>
          <div className="rounded-md bg-muted/50 px-3 py-2 text-sm">
            <span className="font-medium">Итог: </span>
            <span className="text-muted-foreground">{action.result}</span>
          </div>
          {action.to && (
            <Button
              variant="link"
              size="sm"
              asChild
              className="h-auto p-0"
            >
              <Link to={action.to}>
                Перейти
                <ArrowRight className="ml-1.5 h-3.5 w-3.5" />
              </Link>
            </Button>
          )}
        </div>
      )}
    </div>
  )
}

function GlossaryCard({ item }: { item: GlossaryItem }) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <h3 className="font-medium">{item.term}</h3>
              <HelpTooltip>{item.explanation}</HelpTooltip>
            </div>
            <p className="mt-1 text-sm text-muted-foreground">{item.context}</p>
          </div>
        </div>
        <p className="mt-3 text-sm">{item.explanation}</p>
      </CardContent>
    </Card>
  )
}

function SectionCard({ section }: { section: AdminSection }) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h3 className="font-medium">{section.title}</h3>
            <p className="mt-1 text-sm text-muted-foreground">{section.purpose}</p>
          </div>
          <Button variant="outline" size="sm" asChild className="shrink-0">
            <Link to={section.route}>Открыть</Link>
          </Button>
        </div>
        <div className="mt-3 rounded-md bg-muted/50 px-3 py-2 text-sm">
          <span className="font-medium">Когда идти сюда: </span>
          <span className="text-muted-foreground">{section.useWhen}</span>
        </div>
      </CardContent>
    </Card>
  )
}

export default function Onboarding() {
  const [selectedActionId, setSelectedActionId] = useState(allActions[0]?.id ?? '')

  return (
    <div className="space-y-6">
      <section className="rounded-xl border bg-card p-6">
        <div className="max-w-3xl">
          <Badge variant="secondary" className="mb-3 rounded-md">
            Карта админки
          </Badge>
          <h1 className="text-2xl font-bold tracking-tight">Онбординг без лишней теории</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Короткая карта типовых задач, терминов и разделов админки.
          </p>
          <div className="mt-5 flex flex-wrap gap-3">
            <Button asChild>
              <Link to="/admin/plugins/upload">
                <FileUp className="mr-2 h-4 w-4" />
                Загрузить плагин
              </Link>
            </Button>
            <Button variant="outline" asChild>
              <a href={docsHref('/guide/overview')} target="_blank" rel="noreferrer">
                <BookOpen className="mr-2 h-4 w-4" />
                Полная документация
              </a>
            </Button>
          </div>
        </div>
      </section>

      <Tabs defaultValue="scenarios" className="space-y-4">
        <TabsList className="h-auto flex-wrap justify-start gap-1">
          <TabsTrigger value="scenarios">Сценарии</TabsTrigger>
          <TabsTrigger value="terms">Поля и термины</TabsTrigger>
          <TabsTrigger value="sections">Разделы админки</TabsTrigger>
        </TabsList>

        <TabsContent value="scenarios" className="space-y-4">
          <div className="space-y-4">
            {scenarios.map((scenario) => {
              const Icon = scenario.icon
              return (
                <Card key={scenario.title}>
                  <CardHeader className="pb-4">
                    <div className="flex items-start gap-3">
                      <div className="rounded-lg border bg-muted/50 p-2">
                        <Icon className="h-5 w-5" />
                      </div>
                      <div>
                        <CardTitle className="text-base">{scenario.title}</CardTitle>
                        <p className="mt-1 text-sm text-muted-foreground">
                          {scenario.description}
                        </p>
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    {scenario.actions.map((action) => (
                      <ActionRow
                        key={action.id}
                        action={action}
                        active={selectedActionId === action.id}
                        onSelect={() => setSelectedActionId(action.id)}
                      />
                    ))}
                  </CardContent>
                </Card>
              )
            })}
          </div>
        </TabsContent>

        <TabsContent value="terms">
          <div className="grid gap-3 lg:grid-cols-2">
            {glossary.map((item) => (
              <GlossaryCard key={item.term} item={item} />
            ))}
          </div>
        </TabsContent>

        <TabsContent value="sections">
          <div className="grid gap-3 lg:grid-cols-2">
            {sections.map((section) => (
              <SectionCard key={section.title} section={section} />
            ))}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  )
}
