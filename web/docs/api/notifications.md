# Уведомления

Система уведомлений позволяет плагинам отправлять сообщения пользователям, чатам и проектам. В отличие от `ctx.Reply()`, уведомления учитывают предпочтения пользователя: рабочие часы, приоритетный канал доставки и настройки упоминаний.

## Приоритеты

Каждое уведомление имеет уровень приоритета, определяющий поведение доставки:

| Константа | Значение | Описание |
|---|---|---|
| `PriorityLow` | `0` | Информационное - без звука вне рабочих часов |
| `PriorityNormal` | `1` | Стандартное - со звуком |
| `PriorityHigh` | `2` | Важное - автоматическое упоминание пользователя |
| `PriorityCritical` | `3` | Срочное - упоминание, все каналы, никогда не молчит |

### Поведение по приоритетам

| Правило | Low | Normal | High | Critical |
|---|---|---|---|---|
| Звук/вибрация | Только в раб. часы | Да | Да | Да |
| Упоминание пользователя | Нет | Нет | Да | Да |
| Выбор канала | Предпочтительный | Предпочтительный | Предпочтительный | **Все каналы** |
| Учитывает MuteMentions | - | - | Да | **Нет** |

## Методы EventContext

### `ctx.NotifyUser(userID, text, priority)` {#notify-user}

Отправляет уведомление конкретному пользователю. Хост автоматически выбирает канал доставки по предпочтениям пользователя.

```go
// Информационное уведомление -без звука вне рабочих часов
ctx.NotifyUser(userID, "Сборка завершена", wasmplugin.PriorityLow)

// Срочное -отправится во все каналы пользователя
ctx.NotifyUser(userID, "Сервер недоступен!", wasmplugin.PriorityCritical)
```

| Параметр | Тип | Описание |
|---|---|---|
| `userID` | `int64` | Глобальный ID пользователя |
| `text` | `string` | Текст уведомления |
| `priority` | `int` | Уровень приоритета (0--3) |

### `ctx.NotifyChat(channelType, chatID, text, priority)` {#notify-chat}

Отправляет уведомление в конкретный чат.

```go
ctx.NotifyChat("telegram", "123456789", "Новый заказ!", wasmplugin.PriorityNormal)
```

| Параметр | Тип | Описание |
|---|---|---|
| `channelType` | `string` | Тип канала (`"telegram"`, `"discord"`, ...) |
| `chatID` | `string` | ID чата |
| `text` | `string` | Текст уведомления |
| `priority` | `int` | Уровень приоритета (0--3) |

### `ctx.NotifyProject(projectID, text, priority)` {#notify-project}

Отправляет уведомление во **все чаты**, привязанные к проекту.

```go
ctx.NotifyProject(42, "Релиз v2.0 опубликован", wasmplugin.PriorityHigh)
```

| Параметр | Тип | Описание |
|---|---|---|
| `projectID` | `int64` | ID проекта |
| `text` | `string` | Текст уведомления |
| `priority` | `int` | Уровень приоритета (0--3) |

**Необходимое разрешение:** `notify`

**Необходимое требование:**

```go
wasmplugin.NotifyReq("уведомления мониторинга")
```

::: info Одно разрешение на все методы
Все три метода (`NotifyUser`, `NotifyChat`, `NotifyProject`) используют единое разрешение `notify`. Одного вызова `NotifyReq(desc)` в `Requirements` достаточно.
:::

## Reply vs Notify

| | `ctx.Reply` | `ctx.Notify*` |
|---|---|---|
| **Назначение** | Прямой ответ в чат триггера | Отправка в произвольные чаты/пользователям |
| **Канал доставки** | Текущий чат | Определяется хостом по предпочтениям |
| **Рабочие часы** | Не учитываются | Учитываются (PriorityLow) |
| **Упоминания** | Нет | Автоматически (PriorityHigh+) |
| **Разрешение** | - | `notify` |

::: tip Когда что использовать
Используйте `ctx.Reply()` для ответов на команды пользователя в messenger-триггерах. Используйте `ctx.Notify*()` для отправки сообщений из cron/event-триггеров и для фоновых оповещений, где важна приоритетность и выбор канала доставки.
:::

## Полный пример

```go
package main

import wasmplugin "github.com/superbot/wasmplugin"

func main() {
    wasmplugin.Run(wasmplugin.Plugin{
        ID:          "monitor",
        Name:        "Мониторинг",
        Description: "Мониторинг и оповещения",
        Version:     "1.0.0",
        Requirements: wasmplugin.NotifyReq("уведомления мониторинга").
            HTTP("проверка здоровья API").
            Build(),

        Triggers: []wasmplugin.Trigger{
            {
                Name:     "health_check",
                Type:     wasmplugin.TriggerCron,
                Schedule: "*/5 * * * *",
                Handler: func(ctx *wasmplugin.EventContext) error {
                    resp, err := wasmplugin.HTTPGet("https://api.example.com/health")
                    if err != nil {
                        return err
                    }

                    if resp.StatusCode != 200 {
                        // Срочное уведомление -все каналы
                        ctx.NotifyProject(1, "API недоступен!", wasmplugin.PriorityCritical)
                        return nil
                    }

                    // Тихое информационное уведомление
                    ctx.NotifyUser(100, "Health check OK", wasmplugin.PriorityLow)
                    return nil
                },
            },
            {
                Name:        "notify",
                Type:        wasmplugin.TriggerMessenger,
                Description: "Отправить уведомление в проект",
                Handler: func(ctx *wasmplugin.EventContext) error {
                    text := ctx.Param("text")
                    if text == "" {
                        text = "Тестовое уведомление"
                    }
                    return ctx.NotifyProject(1, text, wasmplugin.PriorityNormal)
                },
            },
        },
    })
}
```
