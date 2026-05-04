# Триггеры

Триггер - это точка входа в плагин. Каждый триггер описывается структурой `Trigger` с указанием типа, и все типы триггеров проходят через единую систему маршрутизации платформы.

```go
wasmplugin.Trigger{
    Name: "имя",
    Type: wasmplugin.TriggerMessenger, // или TriggerHTTP, TriggerCron, TriggerEvent
    Descriptions: map[string]string{
        "ru": "описание",
        "en": "description",
    },
    Handler: func(ctx *wasmplugin.EventContext) error { ... },
}
```

Один плагин может зарегистрировать несколько триггеров разных типов.

## Типы триггеров

| Тип | Константа | Назначение | Обязательные поля |
|---|---|---|---|
| [Messenger](/guide/trigger-messenger) | `TriggerMessenger` | Slash-команды в мессенджере | `Name` |
| [HTTP](/guide/trigger-http) | `TriggerHTTP` | HTTP-эндпоинты для внешних систем | `Path`, `Methods` |
| [Cron](/guide/trigger-cron) | `TriggerCron` | Действия по расписанию | `Schedule` |
| [Event](/guide/trigger-event) | `TriggerEvent` | Подписка на события от других плагинов | `Topic` |

У всех триггеров доступны общие поля: `Name`, `Type`, `Descriptions`, `Handler`. `Description` оставлено как deprecated fallback для старых плагинов.

## Данные контекста

Каждый тип триггера заполняет своё поле в [EventContext](/api/context):

| Тип | Поле контекста | Структура данных |
|---|---|---|
| Messenger | `ctx.Messenger` | `MessengerData` |
| HTTP | `ctx.HTTP` | `HTTPEventData` |
| Cron | `ctx.Cron` | `CronEventData` |
| Event | `ctx.Event` | `EventBusData` |

## Fallback-обработчик {#fallback}

Если у триггера нет собственного `Handler`, платформа вызывает `Plugin.OnEvent`:

```go
wasmplugin.Plugin{
    Triggers: []wasmplugin.Trigger{
        {Name: "hook1", Type: wasmplugin.TriggerHTTP, Path: "/a", Methods: []string{"POST"}},
        {Name: "hook2", Type: wasmplugin.TriggerHTTP, Path: "/b", Methods: []string{"POST"}},
    },
    OnEvent: func(ctx *wasmplugin.EventContext) error {
        switch ctx.TriggerName {
        case "hook1":
            // ...
        case "hook2":
            // ...
        }
        return nil
    },
}
```

Это удобно, когда один обработчик обслуживает несколько триггеров.
