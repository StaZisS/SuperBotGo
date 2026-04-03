# Уведомления

Система уведомлений позволяет плагинам отправлять сообщения пользователям, чатам и студентам по университетской иерархии. В отличие от `ctx.Reply()`, уведомления учитывают предпочтения пользователя: рабочие часы, приоритетный канал доставки и настройки упоминаний.

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
// Информационное уведомление — без звука вне рабочих часов
ctx.NotifyUser(userID, "Сборка завершена", wasmplugin.PriorityLow)

// Срочное — отправится во все каналы пользователя
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

### `ctx.NotifyStudents()` {#notify-students}

Возвращает builder для отправки уведомления **всем студентам** в указанном уровне университетской иерархии. Каждый студент получает персональное уведомление через `NotifyUser` с учётом приоритетов и предпочтений.

#### Scope-методы (уровень иерархии)

| Метод | Описание |
|---|---|
| `.Faculty(id)` | Все студенты факультета |
| `.Department(id)` | Все студенты кафедры |
| `.Program(id)` | Все студенты направления подготовки |
| `.Stream(id)` | Все студенты потока |
| `.Group(id)` | Все студенты учебной группы |
| `.Subgroup(id)` | Все студенты подгруппы |

#### Остальные методы

| Метод | Описание |
|---|---|
| `.Message(msg)` | Сообщение ([Message](#message-type), обязательно) |
| `.Priority(p)` | Приоритет (по умолчанию `PriorityNormal`) |
| `.Send()` | Отправить уведомление |

### Тип `Message` {#message-type}

`Message` — rich-сообщение из блоков контента. Создаётся через `NewMessage(text)` и расширяется builder-методами.

| Конструктор / метод | Описание |
|---|---|
| `NewMessage(text)` | Создать сообщение с текстовым блоком |
| `.Text(text)` | Добавить текстовый блок (plain) |
| `.StyledText(text, style)` | Добавить текст со стилем (`StyleHeader`, `StyleCode`, ...) |
| `.Mention(userID)` | Добавить упоминание пользователя |
| `.File(ref, caption)` | Добавить файл-вложение |
| `.Link(url, label)` | Добавить ссылку |
| `.Image(url)` | Добавить изображение |

#### Примеры

```go
// Простое текстовое уведомление
ctx.NotifyStudents().
    Stream(streamID).
    Message(wasmplugin.NewMessage("Пары завтра отменены")).
    Priority(wasmplugin.PriorityHigh).
    Send()

// Уведомить группу — стандартный приоритет (по умолчанию)
ctx.NotifyStudents().
    Group(groupID).
    Message(wasmplugin.NewMessage("Сдача лабы перенесена на пятницу")).
    Send()

// Rich-сообщение с упоминанием и ссылкой
ctx.NotifyStudents().
    Subgroup(subgroupID).
    Message(
        wasmplugin.NewMessage("Замена преподавателя по английскому").
            Mention(newTeacherID).
            Link("https://schedule.university.ru/changes", "Подробности"),
    ).
    Send()

// Сообщение с файлом
ctx.NotifyStudents().
    Faculty(facultyID).
    Message(
        wasmplugin.NewMessage("Новое расписание на семестр").
            File(scheduleFile, "расписание.pdf"),
    ).
    Priority(wasmplugin.PriorityCritical).
    Send()
```

::: warning Валидация на стороне SDK
`Send()` вернёт ошибку, если не задан scope (не вызван ни один из методов `Faculty`, `Stream`, `Group` и т.д.) или не задан текст. Host-вызов не произойдёт.
:::

**Необходимое разрешение:** `notify`

**Необходимое требование:**

```go
wasmplugin.NotifyReq("рассылка студентам по расписанию").Build()
```

::: info Одно разрешение на все методы
Все методы (`NotifyUser`, `NotifyChat`, `NotifyStudents`) используют единое разрешение `notify`. Одного `NotifyReq(desc)` достаточно.
:::

## Reply vs Notify

| | `ctx.Reply` | `ctx.Notify*` |
|---|---|---|
| **Назначение** | Прямой ответ в чат триггера | Отправка в произвольные чаты/пользователям/студентам |
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

import wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"

func main() {
    wasmplugin.Run(wasmplugin.Plugin{
        ID:          "schedule-alerts",
        Name:        "Оповещения по расписанию",
        Description: "Уведомления студентов об изменениях в расписании",
        Version:     "1.0.0",
        Requirements: []wasmplugin.Requirement{
            wasmplugin.NotifyReq("рассылка студентам об изменениях расписания").Build(),
            wasmplugin.Database("Чтение расписания").Build(),
        },

        Triggers: []wasmplugin.Trigger{
            {
                Name:     "check_changes",
                Type:     wasmplugin.TriggerCron,
                Schedule: "*/30 * * * *",
                Handler: func(ctx *wasmplugin.EventContext) error {
                    // Проверяем изменения в расписании
                    // ...

                    // Уведомить поток об отмене пар
                    ctx.NotifyStudents().
                        Stream(streamID).
                        Message(wasmplugin.NewMessage("Лекция по математике завтра отменена")).
                        Priority(wasmplugin.PriorityHigh).
                        Send()

                    return nil
                },
            },
            {
                Name:        "notify_group",
                Type:        wasmplugin.TriggerMessenger,
                Description: "Отправить уведомление группе",
                Handler: func(ctx *wasmplugin.EventContext) error {
                    text := ctx.Param("text")
                    if text == "" {
                        text = "Тестовое уведомление"
                    }
                    return ctx.NotifyStudents().
                        Group(groupID).
                        Message(wasmplugin.NewMessage(text)).
                        Send()
                },
            },
        },
    })
}
```
