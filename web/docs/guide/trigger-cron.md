# Cron

Cron-триггер выполняет плагин по расписанию. Расписание задаётся в стандартном cron-формате.

## Регистрация

```go
wasmplugin.Trigger{
    Name:        "daily_report",
    Type:        wasmplugin.TriggerCron,
    Description: "Ежедневный отчёт в 9:00",
    Schedule:    "0 9 * * *",
    Handler: func(ctx *wasmplugin.EventContext) error {
        report := generateReport()
        ctx.SendMessage("CHAT_ID", report)
        return nil
    },
}
```

## Формат расписания

Стандартный cron с 5 полями:

```
┌───────────── минута (0-59)
│ ┌───────────── час (0-23)
│ │ ┌───────────── день месяца (1-31)
│ │ │ ┌───────────── месяц (1-12)
│ │ │ │ ┌───────────── день недели (0-6, 0 = воскресенье)
│ │ │ │ │
* * * * *
```

## Поля ctx.Cron

| Поле | Тип | Описание |
|---|---|---|
| `ScheduleName` | `string` | Имя расписания (совпадает с `Name` триггера) |
| `FireTime` | `int64` | Unix timestamp момента срабатывания |

::: warning ctx.Reply() не работает в cron-триггерах
`ctx.Reply()` доступен только в messenger-триггерах. Для отправки сообщений из cron используйте `ctx.SendMessage(chatID, text)` с явным ID чата.
:::
