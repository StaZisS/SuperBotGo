# Авторизация через ТГУ.Аккаунты

Интеграция с внешним сервисом аутентификации **ТГУ.Аккаунты**.
Используется для привязки учётной записи ТГУ к глобальному пользователю бота
и автоматической линковки с записью `person` из университетского синка.

## Общая схема

```mermaid
sequenceDiagram
    participant U as Пользователь
    participant B as Бот (Telegram/Discord)
    participant S as Веб-сервер (SuperBotGo)
    participant T as ТГУ.Аккаунты

    U->>B: /link → "ТГУ.Аккаунты"
    B->>S: генерация state (in-memory, TTL 10 мин)
    S-->>B: URL: /oauth/authorize?state=xxx
    B-->>U: кнопка-ссылка "Войти через ТГУ"

    U->>S: GET /oauth/authorize?state=xxx
    S->>S: Verify(state) — существует?
    S-->>U: Set-Cookie: tsu_auth_state=xxx
    S-->>U: 302 → accounts.tsu.ru/Account/Login2/?applicationId=...

    U->>T: логин / пароль
    T->>T: проверка
    T-->>U: 302 → /oauth/login?token=yyy

    U->>S: GET /oauth/login?token=yyy (+ cookie)
    S->>S: Consume(state из cookie) → userID
    S->>T: POST /api/Account/ {token, applicationId, secretKey}
    T-->>S: {accessToken, accountId}

    S->>S: linkAccount(userID, accountId)
    S->>S: autoLinkPerson(userID, accountId)
    S-->>U: HTML "Аккаунт привязан"
```

## Модель данных

```mermaid
erDiagram
    global_users {
        bigint id PK
        varchar tsu_accounts_id "AccountId (GUID) из ТГУ"
        varchar primary_channel
        varchar role
    }

    channel_accounts {
        bigint id PK
        varchar channel_type "TELEGRAM | DISCORD"
        varchar channel_user_id
        bigint global_user_id FK
    }

    persons {
        bigint id PK
        varchar external_id "== tsu_accounts_id"
        varchar last_name
        varchar first_name
        bigint global_user_id FK "автолинковка"
    }

    global_users ||--o{ channel_accounts : "has"
    global_users ||--o| persons : "linked via tsu_accounts_id = external_id"
```

## Конфигурация

```yaml
tsu_accounts:
  application_id: "12345"
  secret_key: "hoho..."
  callback_url: "https://bot.example.com/oauth/login"
  base_url: "https://accounts.kreosoft.space"
```

Env-переменные:
- `BOT_TSU__ACCOUNTS_APPLICATION__ID`
- `BOT_TSU__ACCOUNTS_SECRET__KEY`
- `BOT_TSU__ACCOUNTS_CALLBACK__URL`
- `BOT_TSU__ACCOUNTS_BASE__URL`

## HTTP-эндпоинты

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/oauth/authorize?state=...` | Проверяет state, ставит cookie, редирект на ТГУ |
| GET | `/oauth/login?token=...` | Callback от ТГУ: обмен token → AccountId, линковка |
