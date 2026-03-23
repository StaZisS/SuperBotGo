# Команды

Команды — это slash-команды в мессенджере (Telegram, Discord и др.). Пользователь вводит команду в чат, бот отвечает.

## Простая команда (без шагов)

```go
wasmplugin.Command{
    Name:        "ping",
    Description: "Проверить, жив ли бот",
    Handler: func(ctx *wasmplugin.EventContext) error {
        ctx.Reply("pong!")
        return nil
    },
}
```

Обработчик получает [EventContext](/api/context) и вызывает `ctx.Reply()` для ответа.

## Команда с шагами

Шаги последовательно собирают параметры у пользователя. Хост запрашивает каждый шаг, затем вызывает `Handler` с заполненными параметрами.

```go
wasmplugin.Command{
    Name:        "greet",
    Description: "Поприветствовать",
    Steps: []wasmplugin.Step{
        {
            Param:  "name",
            Prompt: "Введите имя:",
        },
        {
            Param:  "style",
            Prompt: "Выберите стиль:",
            Options: []wasmplugin.Option{
                {Label: "Формально", Value: "formal"},
                {Label: "Неформально", Value: "casual"},
            },
        },
    },
    Handler: func(ctx *wasmplugin.EventContext) error {
        name := ctx.Param("name")
        style := ctx.Param("style")
        if style == "formal" {
            ctx.Reply("Добрый день, " + name + ".")
        } else {
            ctx.Reply("Привет, " + name + "!")
        }
        return nil
    },
}
```

### Поля Step

| Поле | Тип | Описание |
|---|---|---|
| `Param` | `string` | Ключ параметра |
| `Prompt` | `string` | Текст для пользователя |
| `Options` | `[]Option` | Предопределённые варианты (кнопки) |
| `Validation` | `string` | Regex-паттерн для валидации ввода |

## Ограничение по роли

```go
wasmplugin.Command{
    Name:    "admin-panel",
    MinRole: "admin",  // только для роли admin и выше
    Handler: handler,
}
```

## Продвинутый режим: Nodes

Для ветвления, пагинации, динамических опций и условной видимости используйте `Nodes` вместо `Steps`:

```go
wasmplugin.Command{
    Name: "search",
    Nodes: []wasmplugin.Node{
        wasmplugin.NewStep("mode").Options("Режим:",
            wasmplugin.Opt("Быстрый", "quick"),
            wasmplugin.Opt("Расширенный", "advanced"),
        ),
        wasmplugin.BranchOn("mode",
            wasmplugin.Case("advanced",
                wasmplugin.NewStep("date").
                    Validate(`^\d{4}-\d{2}-\d{2}$`),
            ),
        ),
    },
    Handler: handler,
}
```

::: tip
Если задан `Nodes`, поле `Steps` игнорируется. Подробнее — в разделе [Node Builder](/advanced/node-builder).
:::
