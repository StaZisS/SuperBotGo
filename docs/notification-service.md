# Сервис нотификаций

## Архитектура

Система доставки сообщений состоит из трёх уровней:

```
┌──────────────────────────────────────────────────────┐
│                    NotifyAPI                          │
│          (приоритеты, преференции, mentions)          │
├──────────────────────────────────────────────────────┤
│                    SenderAPI                          │
│            (маршрутизация по каналам)                 │
├──────────────────────────────────────────────────────┤
│               AdapterRegistry                        │
│       (retry, silent mode, strip mentions)            │
├────────────────┬─────────────────────────────────────┤
│ TelegramAdapter│ DiscordAdapter │  ... (другие)      │
│  (Bot API)     │ (Gateway API)  │                    │
└────────────────┴────────────────┴────────────────────┘
```

| Уровень | Пакет | Ответственность |
|---|---|---|
| **NotifyAPI** | `notification/notify.go` | Приоритеты, преференции пользователя, рабочие часы, auto-mention |
| **SenderAPI** | `plugin/sender.go` | Маршрутизация: reply, send-to-user, broadcast по каналам/проектам |
| **AdapterRegistry** | `channel/registry.go` | Retry с backoff, silent mode, strip mentions, выбор адаптера |
| **ChannelAdapter** | `channel/telegram/`, `channel/discord/` | Рендеринг Message → платформенный формат, отправка через API |

## 1. NotifyUser — уведомление пользователя

Диаграмма: [seq-notify-user.mmd](seq-notify-user.mmd)

```mermaid
sequenceDiagram
    title NotifyUser — уведомление пользователя с учётом приоритета

    participant Caller as Вызывающий код<br/>(plugin, system, etc.)
    participant Notify as NotifyAPI<br/>(notification/notify.go)
    participant Users as UserService
    participant Prefs as PrefsRepository<br/>(PgPrefsRepo / PlaceholderPrefsRepo)
    participant AR as AdapterRegistry<br/>(channel/registry.go)
    participant Adapter as ChannelAdapter<br/>(telegram / discord)

    Caller->>Notify: NotifyUser(ctx, userID, msg, priority)

    Notify->>Users: GetUser(ctx, userID)
    Users-->>Notify: GlobalUser{Accounts, PrimaryChannel}

    Notify->>Prefs: GetPrefs(ctx, userID)
    alt Преференции найдены
        Prefs-->>Notify: NotificationPrefs
    else Преференции не найдены
        Prefs-->>Notify: nil
        Notify->>Notify: defaultPrefs(userID, primaryChannel)
        Note right of Notify: ChannelPriority=[primaryChannel]<br/>MuteMentions=false<br/>Timezone="UTC"
    end

    Notify->>Notify: buildSendOptions(prefs, priority)
    Note right of Notify: Silent: PriorityLow +<br/>вне рабочих часов<br/>StripMentions: MuteMentions +<br/>priority < Critical

    alt priority == Critical
        rect rgb(255, 235, 235)
            Note over Notify, Adapter: Критический приоритет → отправка во ВСЕ каналы
            loop Для каждого аккаунта пользователя
                Notify->>Notify: maybeInjectMention(msg, platformUserID, prefs, priority)
                Note right of Notify: Critical всегда добавляет mention<br/>(даже если MuteMentions=true)
                Notify->>AR: SendToUserWithOpts(ctx, channelType, platformUserID, msg, opts)
                AR->>Adapter: SendToUser / SendToUserSilent
            end
        end
    else priority < Critical (Low / Normal / High)
        rect rgb(235, 245, 255)
            Note over Notify, Adapter: Обычный приоритет → один канал по преференциям
            Notify->>Notify: resolveChannel(user, prefs)
            Note right of Notify: 1. ChannelPriority из prefs<br/>2. PrimaryChannel<br/>3. Первый аккаунт
            Notify->>Notify: maybeInjectMention(msg, platformUserID, prefs, priority)
            Notify->>AR: SendToUserWithOpts(ctx, targetChannel, platformUserID, msg, opts)
            AR->>Adapter: SendToUser / SendToUserSilent
        end
    end

    Notify-->>Caller: error / nil
```

### Уровни приоритета

| Приоритет | Константа | Поведение |
|---|---|---|
| **Low** | `PriorityLow` | Silent вне рабочих часов, без mention |
| **Normal** | `PriorityNormal` | Стандартное уведомление со звуком |
| **High** | `PriorityHigh` | Auto-mention пользователя (если не MuteMentions) |
| **Critical** | `PriorityCritical` | Mention + отправка во **все** каналы, никогда не silent |

### Преференции пользователя (NotificationPrefs)

| Поле | Тип | Описание |
|---|---|---|
| `ChannelPriority` | `[]ChannelType` | Порядок предпочтения каналов (telegram, discord, ...) |
| `MuteMentions` | `bool` | Не добавлять auto-mention (кроме Critical) |
| `WorkHoursStart` | `*int` | Начало рабочих часов (0-23) |
| `WorkHoursEnd` | `*int` | Конец рабочих часов (0-23) |
| `Timezone` | `string` | Таймзона (IANA, по умолчанию `"UTC"`) |

Преференции хранятся в таблице `notification_prefs` (PostgreSQL) или in-memory (`PlaceholderPrefsRepo`).

### Логика resolveChannel

Выбор канала для отправки (при priority < Critical):

1. Перебор `ChannelPriority` из преференций — первый канал, на котором у пользователя есть аккаунт
2. `PrimaryChannel` пользователя
3. Первый аккаунт из списка (fallback)

### Логика maybeInjectMention

- `priority < High` → mention не добавляется
- `priority >= High` и `MuteMentions == true` и `priority < Critical` → mention не добавляется
- Если mention для данного `platformUserID` уже есть в блоках → пропуск
- Иначе → `MentionBlock{UserID}` добавляется в начало блоков сообщения

## 2. NotifyProject — рассылка по проекту

Диаграмма: [seq-notify-project.mmd](seq-notify-project.mmd)

```mermaid
sequenceDiagram
    title NotifyProject — рассылка уведомлений по проекту

    participant Caller as Вызывающий код
    participant Notify as NotifyAPI<br/>(notification/notify.go)
    participant ChatReg as ChatRegistry
    participant AR as AdapterRegistry<br/>(channel/registry.go)
    participant Adapter as ChannelAdapter<br/>(telegram / discord)

    Caller->>Notify: NotifyProject(ctx, projectID, msg, priority)

    Notify->>ChatReg: FindChatsByProject(ctx, projectID)
    ChatReg-->>Notify: []ChatReference{ChannelType, PlatformChatID}

    Notify->>Notify: opts = SendOptions{Silent: priority == Low}

    loop Для каждого чата проекта
        Notify->>AR: SendToChatWithOpts(ctx, channelType, chatID, msg, opts)
        AR->>Adapter: SendToChat / SendToChatSilent

        alt Ошибка
            Adapter-->>Notify: error
            Notify->>Notify: Накопить, продолжить
        else Успех
            Adapter-->>Notify: nil
        end
    end

    alt Были ошибки
        Notify-->>Caller: error: "project N broadcast failed on X/Y chats"
    else Все успешно
        Notify-->>Caller: nil
    end
```

Отправка продолжается даже при ошибках в отдельных чатах — ошибки накапливаются и возвращаются как `errors.Join(...)`.

## 3. SenderAPI — отправка из плагинов

Диаграмма: [seq-sender-api.mmd](seq-sender-api.mmd)

```mermaid
sequenceDiagram
    title SenderAPI — отправка сообщений из плагинов

    participant Plugin as WasmPlugin<br/>(wasm/adapter.go)
    participant Sender as SenderAPI<br/>(plugin/sender.go)
    participant Users as UserService
    participant ChatReg as ChatRegistry
    participant AR as AdapterRegistry<br/>(channel/registry.go)
    participant Adapter as ChannelAdapter

    Note over Plugin, Adapter: Reply — ответ в текущий чат
    Plugin->>Sender: Reply(ctx, messengerData, msg)
    Sender->>AR: SendToChat(ctx, channelType, chatID, msg)
    AR->>Adapter: SendToChat(...)

    Note over Plugin, Adapter: SendToUser — отправка пользователю (primary channel)
    Plugin->>Sender: SendToUser(ctx, userID, msg)
    Sender->>Users: GetUser(ctx, userID)
    Sender->>AR: SendToUser(ctx, primaryChannel, platformUserID, msg)

    Note over Plugin, Adapter: SendToAllChannels — broadcast во все каналы
    Plugin->>Sender: SendToAllChannels(ctx, userID, msg)
    Sender->>Users: GetUser(ctx, userID)
    loop Для каждого аккаунта
        Sender->>AR: SendToUser(ctx, channelType, channelUserID, msg)
    end

    Note over Plugin, Adapter: SendToProject — broadcast по чатам проекта
    Plugin->>Sender: SendToProject(ctx, projectID, msg)
    Sender->>ChatReg: FindChatsByProject(ctx, projectID)
    loop Для каждого чата
        Sender->>AR: SendToChat(ctx, channelType, chatID, msg)
    end
```

| Метод | Описание |
|---|---|
| `Reply(ctx, messengerData, msg)` | Ответ в чат, из которого пришло сообщение |
| `ReplyToChat(ctx, channelType, chatID, msg)` | Отправка в конкретный чат по типу канала и ID |
| `SendToUser(ctx, userID, msg)` | Отправка пользователю через primary channel |
| `SendToAllChannels(ctx, userID, msg)` | Broadcast во все подключённые каналы пользователя |
| `SendToProject(ctx, projectID, msg)` | Broadcast во все чаты, привязанные к проекту |

## 4. Доставка с retry

Диаграмма: [seq-send-retry.mmd](seq-send-retry.mmd)

```mermaid
sequenceDiagram
    title Доставка сообщения с retry (channel/send.go)

    participant Caller as AdapterRegistry<br/>(channel/registry.go)
    participant Retry as withRetry()<br/>(channel/send.go)
    participant Adapter as ChannelAdapter<br/>(telegram / discord)
    participant API as Messenger API<br/>(Telegram Bot API /<br/>Discord Gateway)

    Caller->>Retry: withRetry(ctx, fn)

    loop attempt = 0..2 (maxRetries = 3)
        Retry->>Adapter: fn() → SendToChat / SendToUser
        Adapter->>Adapter: msg.IsEmpty()? → error
        Adapter->>Adapter: Render(msg) → text + media
        Adapter->>Adapter: Обрезка по лимиту платформы
        Note right of Adapter: Telegram: 4096 символов<br/>Discord: 2000 символов

        Adapter->>API: HTTP POST (send message)

        alt Успех (200 OK)
            API-->>Adapter: response
            Adapter-->>Retry: nil
            Retry-->>Caller: nil
        else Транзиентная ошибка
            API-->>Adapter: error
            Adapter-->>Retry: error
            Retry->>Retry: isTransient(err)? → true

            alt Не последняя попытка
                Retry->>Retry: backoffDelay(attempt)
                Note right of Retry: 500ms × 2^attempt<br/>+ jitter (±30%)<br/>max = 5s
                Retry->>Retry: slog.Warn("send failed, retrying")
            else Последняя попытка
                Retry-->>Caller: error
            end

        else Постоянная ошибка
            API-->>Adapter: error
            Adapter-->>Retry: error
            Retry->>Retry: isTransient(err)? → false
            Retry-->>Caller: error (немедленный возврат)
        end
    end
```

### Параметры retry

| Параметр | Значение | Описание |
|---|---|---|
| `maxRetries` | `3` | Максимум попыток |
| `baseDelay` | `500ms` | Базовая задержка |
| `maxDelay` | `5s` | Максимальная задержка |
| `jitterPercent` | `0.3` (±30%) | Случайное отклонение для предотвращения thundering herd |

### Транзиентные ошибки (повтор)

- `net.Error` (любая сетевая ошибка)
- `429 Too Many Requests` / `retry after`
- `timeout` / `EOF` / `connection reset` / `connection refused`
- `502 Bad Gateway` / `503 Service Unavailable` / `504 Gateway Timeout`
- `500 Internal Server Error` / `temporary failure`

### Постоянные ошибки (без повтора)

- `400 Bad Request` — невалидное сообщение
- `401 Unauthorized` / `403 Forbidden` — проблемы авторизации
- `404 Not Found` — чат/пользователь не существует
- Любые ошибки, не попавшие под паттерны выше

## 5. Silent mode и SilentSender

Интерфейс `SilentSender` — опциональное расширение для адаптеров, поддерживающих тихую доставку (без звуковых уведомлений на устройстве пользователя):

```go
type SilentSender interface {
    SendToUserSilent(ctx, platformUserID, msg, silent) error
    SendToChatSilent(ctx, chatID, msg, silent) error
}
```

| Адаптер | Реализует SilentSender | Механизм |
|---|---|---|
| **Telegram** | Да | `tele.SendOptions{DisableNotification: true}` |
| **Discord** | Да | `discordgo.MessageFlagsSuppressNotifications` |

Если адаптер не реализует `SilentSender`, сообщение отправляется обычным способом (fallback на `SendToChat`/`SendToUser`).

### Когда включается Silent

- `PriorityLow` + пользователь **вне рабочих часов** → `Silent: true`
- `NotifyChat` / `NotifyProject` с `PriorityLow` → `Silent: true`

## Разница между NotifyAPI и SenderAPI

| | NotifyAPI | SenderAPI |
|---|---|---|
| **Уровень** | Высокоуровневый | Низкоуровневый |
| **Приоритеты** | Да (Low → Critical) | Нет |
| **Преференции** | Да (канал, mentions, часы) | Нет |
| **Auto-mention** | Да (High, Critical) | Нет |
| **Silent mode** | Да (по приоритету + часам) | Нет |
| **Выбор канала** | По преференциям | Явный или primary |
| **Применение** | Системные уведомления | Ответы плагинов, broadcast |
