# Messenger

Messenger-триггер регистрирует slash-команду в мессенджере. Пользователь вводит команду в чат, бот собирает параметры через шаги и вызывает обработчик.

Имя триггера становится slash-командой: `Name: "hello"` регистрирует `/hello`.

```go
wasmplugin.Trigger{
    Name:        "hello",
    Type:        wasmplugin.TriggerMessenger,
    Description: "Описание для списка команд",
    Nodes:       []wasmplugin.Node{...}, // шаги (опционально)
    Handler:     func(ctx *wasmplugin.EventContext) error { ... },
}
```

## Простая команда

Команда без шагов - обработчик вызывается сразу при вводе команды:

```go
wasmplugin.Trigger{
    Name:        "ping",
    Type:        wasmplugin.TriggerMessenger,
    Description: "Проверить, жив ли бот",
    Handler: func(ctx *wasmplugin.EventContext) error {
        ctx.Reply(wasmplugin.NewMessage("pong!"))
        return nil
    },
}
```

Обработчик получает [EventContext](/api/context) и вызывает `ctx.Reply()` для ответа в чат.

## Многошаговая команда {#steps}

Шаги последовательно собирают параметры у пользователя. Хост отображает каждый шаг, дожидается ответа и сохраняет значение. После сбора всех параметров вызывается `Handler`.

```go
wasmplugin.Trigger{
    Name:        "greet",
    Type:        wasmplugin.TriggerMessenger,
    Description: "Поприветствовать",
    Nodes: []wasmplugin.Node{
        wasmplugin.NewStep("name").
            Text("Как вас зовут?", wasmplugin.StylePlain),
        wasmplugin.NewStep("style").
            Options("Выберите стиль:",
                wasmplugin.Opt("Формально", "formal"),
                wasmplugin.Opt("Неформально", "casual"),
            ),
    },
    Handler: func(ctx *wasmplugin.EventContext) error {
        name := ctx.Param("name")
        if ctx.Param("style") == "formal" {
            ctx.Reply(wasmplugin.NewMessage("Добрый день, " + name + "."))
        } else {
            ctx.Reply(wasmplugin.NewMessage("Привет, " + name + "!"))
        }
        return nil
    },
}
```

`NewStep(param)` создаёт шаг, который сохранит ввод пользователя в параметр с указанным ключом. В обработчике значения доступны через `ctx.Param(key)`.

## Текстовые стили {#styles}

Шаг может содержать несколько текстовых блоков с разным оформлением:

```go
wasmplugin.NewStep("action").
    Text("Панель управления", wasmplugin.StyleHeader).
    Text("Выберите действие:", wasmplugin.StylePlain).
    Link("https://docs.example.com", "Документация").
    Image("https://example.com/banner.png").
    Options("Действие:",
        wasmplugin.Opt("Создать", "create"),
        wasmplugin.Opt("Удалить", "delete"),
    )
```

| Константа | Отображение |
|---|---|
| `StylePlain` | Обычный текст |
| `StyleHeader` | Заголовок |
| `StyleSubheader` | Подзаголовок |
| `StyleCode` | Моноширинный блок |
| `StyleQuote` | Цитата |

Дополнительные элементы:
- **`.Link(url, label)`** - кликабельная ссылка
- **`.Image(url)`** - изображение

## Опции {#options}

Статические опции отображаются как кнопки. Пользователь выбирает одну из них:

```go
wasmplugin.NewStep("color").
    Options("Выберите цвет:",
        wasmplugin.Opt("Красный", "red"),
        wasmplugin.Opt("Синий", "blue"),
        wasmplugin.Opt("Зелёный", "green"),
    )
```

`Opt(label, value)` создаёт опцию: `label` - текст кнопки, `value` - значение, которое сохранится в параметр.

## Валидация ввода {#validation}

### Валидация по регулярному выражению

```go
wasmplugin.NewStep("email").
    Text("Введите email:", wasmplugin.StylePlain).
    Validate(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
```

Если ввод не соответствует паттерну, хост повторно запрашивает шаг.

### Валидация через функцию

```go
wasmplugin.NewStep("age").
    Text("Введите возраст:", wasmplugin.StylePlain).
    ValidateFunc(func(ctx *wasmplugin.CallbackContext) bool {
        n, err := strconv.Atoi(ctx.Input)
        return err == nil && n >= 1 && n <= 150
    })
```

`ValidateFunc` получает [CallbackContext](#callbackcontext) с полем `Input` - текст, введённый пользователем. Возвращает `true`, если ввод корректен.

::: info Приоритет
Если заданы оба варианта, `ValidateFunc` имеет приоритет над `Validate` (regex).
:::

## Динамические опции {#dynamic-options}

Опции, вычисляемые на лету через WASM-callback. Используйте, когда список опций зависит от ранее собранных параметров или внешних данных:

```go
wasmplugin.NewStep("teacher").
    Text("Выберите преподавателя:", wasmplugin.StylePlain).
    DynamicOptions("Преподаватель:",
        func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
            building := ctx.Params["building"]
            teachers := getTeachersByBuilding(building)
            opts := make([]wasmplugin.Option, len(teachers))
            for i, t := range teachers {
                opts[i] = wasmplugin.Opt(t.Name, t.ID)
            }
            return opts
        },
    )
```

Callback вызывается каждый раз при отображении шага. `ctx.Params` содержит все уже собранные параметры.

## Пагинация {#pagination}

Для больших списков используйте `PaginatedOptions` с постраничной навигацией:

```go
wasmplugin.NewStep("city").
    PaginatedOptions("Город:", 5,
        func(ctx *wasmplugin.CallbackContext) wasmplugin.OptionsPage {
            all := getCities()
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
        },
    )
```

Структура `OptionsPage`:

| Поле | Тип | Описание |
|---|---|---|
| `Options` | `[]Option` | Опции текущей страницы |
| `HasMore` | `bool` | Если `true`, хост покажет кнопку «Далее» |

Поле `ctx.Page` содержит номер текущей страницы (начиная с 0).

## Ветвление {#branching}

### BranchOn - ветвление по значению параметра

Показывает разные шаги в зависимости от значения ранее собранного параметра:

```go
wasmplugin.NewStep("mode").
    Options("Режим поиска:",
        wasmplugin.Opt("Быстрый", "quick"),
        wasmplugin.Opt("Расширенный", "advanced"),
    ),

wasmplugin.BranchOn("mode",
    wasmplugin.Case("quick",
        wasmplugin.NewStep("query").
            Text("Введите запрос:", wasmplugin.StylePlain),
    ),
    wasmplugin.Case("advanced",
        wasmplugin.NewStep("date").
            Text("Дата (ГГГГ-ММ-ДД):", wasmplugin.StylePlain).
            Validate(`^\d{4}-\d{2}-\d{2}$`),
        wasmplugin.NewStep("query").
            Text("Введите запрос:", wasmplugin.StylePlain),
    ),
    wasmplugin.DefaultCase(
        wasmplugin.NewStep("query").
            Text("Введите запрос:", wasmplugin.StylePlain),
    ),
)
```

- **`Case(value, nodes...)`** - ветка для конкретного значения
- **`DefaultCase(nodes...)`** - ветка по умолчанию, если ни один `Case` не сработал

## Условное ветвление {#conditional-branching}

`ConditionalBranch` позволяет ветвиться по произвольным условиям, а не только по точному совпадению:

```go
wasmplugin.ConditionalBranch(
    // Декларативное условие (выполняется на хосте без WASM-callback)
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
        wasmplugin.NewStep("extra").
            Text("Дополнительная информация:", wasmplugin.StylePlain),
    ),

    // Fallback - если ни одно условие не сработало
    wasmplugin.Otherwise(
        wasmplugin.NewStep("default").
            Text("Стандартный путь", wasmplugin.StylePlain),
    ),
)
```

- **`When(cond, nodes...)`** - декларативное условие, выполняется на хосте
- **`WhenFunc(fn, nodes...)`** - callback-условие, вызывает WASM
- **`Otherwise(nodes...)`** - ветка по умолчанию

Условия проверяются сверху вниз. Выполняется первая сработавшая ветка.

## Условия {#conditions}

Декларативные условия выполняются на стороне хоста без WASM-callback. Они используются в `When()`, `VisibleWhen()` и других декларативных конструкциях.

### Базовые условия

| Конструктор | Описание | Пример |
|---|---|---|
| `ParamEq(param, value)` | Параметр равен значению | `ParamEq("mode", "advanced")` |
| `ParamNeq(param, value)` | Параметр не равен значению | `ParamNeq("mode", "quick")` |
| `ParamMatch(param, regex)` | Параметр соответствует regex | `ParamMatch("email", "^.+@.+$")` |
| `ParamSet(param)` | Параметр был заполнен | `ParamSet("date")` |

### Комбинаторы

| Комбинатор | Описание |
|---|---|
| `And(cond1, cond2, ...)` | Все условия истинны |
| `Or(cond1, cond2, ...)` | Хотя бы одно условие истинно |
| `Not(cond)` | Отрицание условия |

### Пример комбинирования

```go
wasmplugin.And(
    wasmplugin.ParamEq("mode", "advanced"),
    wasmplugin.Or(
        wasmplugin.ParamEq("building", "1"),
        wasmplugin.ParamEq("building", "3"),
    ),
    wasmplugin.Not(wasmplugin.ParamSet("skip")),
)
```

### Декларативные vs callback-условия

| | Декларативные | Callback |
|---|---|---|
| **Синтаксис** | `VisibleWhen(cond)` / `When(cond, ...)` | `VisibleWhenFunc(fn)` / `WhenFunc(fn, ...)` |
| **Выполнение** | На хосте, без вызова WASM | WASM-callback |
| **Производительность** | Быстрее | Медленнее (создаётся экземпляр модуля) |
| **Гибкость** | Только сравнение параметров | Произвольная логика на Go |

::: tip Предпочитайте декларативные условия
Декларативные условия быстрее, потому что хост вычисляет их без запуска WASM-экземпляра. Используйте callback-условия только когда нужна логика за пределами простого сравнения параметров.
:::

## Видимость шагов {#visibility}

Шаг можно показывать или скрывать в зависимости от условий:

### Декларативная видимость

```go
wasmplugin.NewStep("notify").
    Options("Включить уведомления?",
        wasmplugin.Opt("Да", "yes"),
        wasmplugin.Opt("Нет", "no"),
    ).
    VisibleWhen(wasmplugin.ParamNeq("mode", "quick"))
```

Шаг `notify` не будет показан, если пользователь выбрал режим `quick`.

### Callback-видимость

```go
wasmplugin.NewStep("premium_feature").
    Text("Настройте премиум-функцию:", wasmplugin.StylePlain).
    VisibleWhenFunc(func(ctx *wasmplugin.CallbackContext) bool {
        return ctx.Config("premium_enabled", "false") == "true"
    })
```

## CallbackContext {#callbackcontext}

`CallbackContext` доступен во всех callback-функциях: `ValidateFunc`, `DynamicOptions`, `PaginatedOptions`, `WhenFunc`, `VisibleWhenFunc`.

| Поле | Тип | Описание |
|---|---|---|
| `UserID` | `int64` | ID пользователя |
| `Locale` | `string` | Локаль пользователя |
| `Params` | `map[string]string` | Уже собранные параметры |
| `Page` | `int` | Текущая страница (для `PaginatedOptions`) |
| `Input` | `string` | Ввод пользователя (для `ValidateFunc`) |

Метод `ctx.Config(key, fallback)` позволяет читать конфигурацию плагина внутри callback.

## Полный пример {#full-example}

Команда `/search` с выбором критерия, ветвлением, пагинацией, динамическими опциями и условной видимостью:

```go
package main

import wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"

func searchCommand() wasmplugin.Trigger {
    return wasmplugin.Trigger{
        Name:        "search",
        Type:        wasmplugin.TriggerMessenger,
        Description: "Поиск по разным критериям",
        Nodes: []wasmplugin.Node{
            // 1. Выбор критерия поиска
            wasmplugin.NewStep("what").
                Text("Поиск", wasmplugin.StyleHeader).
                Options("Искать по:",
                    wasmplugin.Opt("По преподавателю", "teacher"),
                    wasmplugin.Opt("По предмету", "subject"),
                ),

            // 2. Ветвление: разные шаги для каждого критерия
            wasmplugin.BranchOn("what",
                wasmplugin.Case("teacher",
                    wasmplugin.NewStep("building").
                        PaginatedOptions("Корпус:", 5,
                            func(ctx *wasmplugin.CallbackContext) wasmplugin.OptionsPage {
                                all := getBuildings()
                                start := ctx.Page * 5
                                if start >= len(all) {
                                    return wasmplugin.OptionsPage{}
                                }
                                end := min(start+5, len(all))
                                return wasmplugin.OptionsPage{
                                    Options: all[start:end],
                                    HasMore: end < len(all),
                                }
                            },
                        ),
                    wasmplugin.NewStep("teacher").
                        DynamicOptions("Преподаватель:",
                            func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
                                teachers := getTeachers(ctx.Params["building"])
                                opts := make([]wasmplugin.Option, len(teachers))
                                for i, t := range teachers {
                                    opts[i] = wasmplugin.Opt(t, t)
                                }
                                return opts
                            },
                        ),
                ),
                wasmplugin.Case("subject",
                    wasmplugin.NewStep("subject").
                        PaginatedOptions("Предмет:", 10,
                            func(ctx *wasmplugin.CallbackContext) wasmplugin.OptionsPage {
                                all := getSubjects()
                                start := ctx.Page * 10
                                if start >= len(all) {
                                    return wasmplugin.OptionsPage{}
                                }
                                end := min(start+10, len(all))
                                return wasmplugin.OptionsPage{
                                    Options: all[start:end],
                                    HasMore: end < len(all),
                                }
                            },
                        ),
                ),
            ),

            // 3. Условный шаг: уведомления только для поиска по преподавателю
            wasmplugin.NewStep("notify").
                Options("Уведомлять об изменениях?",
                    wasmplugin.Opt("Да", "yes"),
                    wasmplugin.Opt("Нет", "no"),
                ).
                VisibleWhen(wasmplugin.ParamEq("what", "teacher")),
        },
        Handler: func(ctx *wasmplugin.EventContext) error {
            what := ctx.Param("what")
            switch what {
            case "teacher":
                ctx.Reply(wasmplugin.NewMessage("Результаты для преподавателя: " + ctx.Param("teacher")))
            case "subject":
                ctx.Reply(wasmplugin.NewMessage("Результаты для предмета: " + ctx.Param("subject")))
            }
            return nil
        },
    }
}
```
