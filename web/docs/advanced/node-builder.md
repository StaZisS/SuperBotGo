# Node Builder

Для команд с ветвлением, пагинацией и динамическими опциями используйте `Nodes` вместо `Steps`.

## NewStep

```go
wasmplugin.NewStep("param_name").
    Text("Заголовок", wasmplugin.StyleHeader).
    Text("Описание", wasmplugin.StylePlain).
    Options("Выберите:",
        wasmplugin.Opt("Вариант A", "a"),
        wasmplugin.Opt("Вариант B", "b"),
    )
```

### Стили текста

| Константа | Отображение |
|---|---|
| `StylePlain` | Обычный текст |
| `StyleHeader` | Заголовок |
| `StyleSubheader` | Подзаголовок |
| `StyleCode` | Моноширинный |
| `StyleQuote` | Цитата |

## Блоки шага

Шаг может содержать **несколько блоков**, отображаемых последовательно:

```go
wasmplugin.NewStep("action").
    Text("Панель управления", wasmplugin.StyleHeader). // заголовок
    Text("Выберите действие:", wasmplugin.StylePlain).  // текст
    Link("https://docs.example.com", "Документация").  // ссылка
    Image("https://example.com/banner.png").            // картинка
    Options("Действие:",                                // кнопки
        wasmplugin.Opt("Создать", "create"),
        wasmplugin.Opt("Удалить", "delete"),
    )
```

## Валидация ввода

::: code-group

```go [Regex]
wasmplugin.NewStep("email").
    Text("Введите email:", wasmplugin.StylePlain).
    Validate(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
```

```go [Функция]
wasmplugin.NewStep("room").
    Text("Номер аудитории:", wasmplugin.StylePlain).
    ValidateFunc(func(ctx *wasmplugin.CallbackContext) bool {
        return len(ctx.Input) <= 4 && isDigits(ctx.Input)
    })
```

:::

::: info Приоритет
`ValidateFunc` имеет приоритет над `Validate` (regex), если заданы оба.
:::

## Динамические опции

Опции, вычисляемые при отображении шага через WASM-callback:

```go
wasmplugin.NewStep("teacher").
    Text("Выберите преподавателя:", wasmplugin.StylePlain).
    DynamicOptions("Преподаватель:", func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
        // ctx.Params содержит уже собранные параметры
        building := ctx.Params["building"]
        names := getTeachers(building)
        opts := make([]wasmplugin.Option, len(names))
        for i, name := range names {
            opts[i] = wasmplugin.Option{Label: name, Value: name}
        }
        return opts
    })
```

## Пагинация

Для больших списков опций с постраничной навигацией:

```go
wasmplugin.NewStep("building").
    PaginatedOptions("Корпус:", 5, func(ctx *wasmplugin.CallbackContext) wasmplugin.OptionsPage {
        all := getAllBuildings()
        pageSize := 5
        start := ctx.Page * pageSize
        if start >= len(all) {
            return wasmplugin.OptionsPage{}
        }
        end := min(start+pageSize, len(all))
        return wasmplugin.OptionsPage{
            Options: all[start:end],
            HasMore: end < len(all),
        }
    })
```

| Поле | Тип | Описание |
|---|---|---|
| `Options` | `[]Option` | Опции текущей страницы |
| `HasMore` | `bool` | Есть ещё страницы |

## Ветвление по значению (BranchOn)

Показывает разные шаги в зависимости от ранее собранного параметра:

```go
wasmplugin.BranchOn("mode",
    wasmplugin.Case("quick",
        // шаги для быстрого режима
    ),
    wasmplugin.Case("advanced",
        wasmplugin.NewStep("date").
            Text("Введите дату:", wasmplugin.StylePlain).
            Validate(`^\d{4}-\d{2}-\d{2}$`),
    ),
    wasmplugin.DefaultCase(
        // шаги по умолчанию
    ),
)
```

## Условное ветвление (ConditionalBranch)

Ветвление по условиям вместо точного совпадения значений:

```go
wasmplugin.ConditionalBranch(
    // Декларативное условие (выполняется на хосте, без WASM-callback)
    wasmplugin.When(
        wasmplugin.ParamEq("building", "3"),
        wasmplugin.NewStep("wing").Options("Крыло:",
            wasmplugin.Opt("Восточное", "east"),
            wasmplugin.Opt("Западное", "west"),
        ),
    ),
    // Callback-условие (вызов WASM)
    wasmplugin.WhenFunc(
        func(ctx *wasmplugin.CallbackContext) bool {
            return ctx.Params["type"] == "special"
        },
        wasmplugin.NewStep("extra").Text("Доп. информация:", wasmplugin.StylePlain),
    ),
    // Fallback
    wasmplugin.Otherwise(
        wasmplugin.NewStep("default").Text("Путь по умолчанию", wasmplugin.StylePlain),
    ),
)
```

Подробнее об условиях — в разделе [Условия](/advanced/conditions).

## Видимость шага

Условное отображение/скрытие шагов:

```go
wasmplugin.NewStep("notify").
    Options("Включить уведомления?",
        wasmplugin.Opt("Да", "yes"),
        wasmplugin.Opt("Нет", "no"),
    ).
    VisibleWhen(wasmplugin.ParamNeq("mode", "quick"))
```

Также доступен `VisibleWhenFunc(fn)` для callback-условий видимости.

## CallbackContext {#callbackcontext}

Доступен во всех callback-функциях (валидация, динамические опции, пагинация, условия):

| Поле | Тип | Описание |
|---|---|---|
| `UserID` | `int64` | ID пользователя |
| `Locale` | `string` | Локаль пользователя |
| `Params` | `map[string]string` | Уже собранные параметры |
| `Page` | `int` | Текущая страница (пагинация) |
| `Input` | `string` | Ввод пользователя (валидация) |

Метод `ctx.Config(key, fallback)` также доступен для чтения конфигурации плагина.

## Полный пример

```go
func searchCommand() wasmplugin.Command {
    return wasmplugin.Command{
        Name:        "search",
        Description: "Поиск по разным критериям",
        Nodes: []wasmplugin.Node{
            // Шаг 1: выбор критерия поиска
            wasmplugin.NewStep("what").
                Text("Поиск", wasmplugin.StyleHeader).
                Options("Искать по:",
                    wasmplugin.Opt("По преподавателю", "teacher"),
                    wasmplugin.Opt("По предмету", "subject"),
                ),

            // Шаг 2: ветвление по выбранному критерию
            wasmplugin.BranchOn("what",
                wasmplugin.Case("teacher",
                    wasmplugin.NewStep("building").
                        PaginatedOptions("Корпус:", 5, buildingPages),
                    wasmplugin.NewStep("teacher").
                        DynamicOptions("Преподаватель:", teacherOptions),
                ),
                wasmplugin.Case("subject",
                    wasmplugin.NewStep("subject").
                        PaginatedOptions("Предмет:", 10, subjectPages),
                ),
            ),

            // Шаг 3: условный шаг после ветки
            wasmplugin.NewStep("notify").
                Options("Уведомлять об изменениях?",
                    wasmplugin.Opt("Да", "yes"),
                    wasmplugin.Opt("Нет", "no"),
                ).
                VisibleWhen(wasmplugin.ParamEq("what", "teacher")),
        },
        Handler: func(ctx *wasmplugin.EventContext) error {
            ctx.Reply("Результаты для: " + ctx.Param("what"))
            return nil
        },
    }
}
```
