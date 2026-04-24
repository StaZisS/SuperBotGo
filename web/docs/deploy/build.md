# Сборка и установка

## Сборка WASM

Плагин компилируется в один `.wasm` файл через стандартный Go-тулчейн:

```bash
GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm .
```

## Оптимизация размера

Флаги `-ldflags="-s -w"` убирают таблицу символов и отладочную информацию, уменьшая размер бинарника:

```bash
GOOS=wasip1 GOARCH=wasm go build -ldflags="-s -w" -o plugin.wasm .
```

::: tip
Для продакшн-сборок всегда используйте `-ldflags="-s -w"`. Разница в размере может составлять 30-50%.
:::

## Установка через Admin API

Установка проходит в два этапа: загрузка файла и активация плагина.

### Загрузка

```bash
curl -X POST http://host/api/admin/plugins/upload \
  -F "file=@plugin.wasm"
```

Ответ содержит `id` и метаданные, извлечённые из бинарника (ID, имя, версия, requirements).

### Активация

```bash
curl -X POST http://host/api/admin/plugins/{id}/install \
  -H "Content-Type: application/json" \
  -d '{
    "wasm_key": "...",
    "config": {}
  }'
```

При установке host сам вычисляет внутренние permissions из `Requirements`, объявленных в манифесте плагина.

## Разрешения {#permissions}

Каждый вызов Host API проверяет разрешения в рантайме. Отсутствие нужного разрешения приводит к ошибке.

| Ключ | Описание | Какие функции требуют | Requirement builder |
|---|---|---|---|
| `sql` | Доступ к базе данных | `sql.Open("superbot", name)` | `Database(desc)` |
| `kv` | Чтение/запись KV Store | `KVGet`, `KVSet`, `KVDelete`, `KVList` | `KV(desc)` |
| `network` | Исходящие HTTP-запросы | `HTTPRequest`, `HTTPGet`, `HTTPPost` | `HTTP(desc)` |
| `notify` | Отправка уведомлений | `NotifyUser`, `NotifyChat`, `NotifyStudents` | `NotifyReq(desc)` |
| `events` | Публикация событий | `PublishEvent` | `EventsReq(desc)` |
| `plugins:call:<target>` | Вызов другого плагина | `CallPlugin`, `CallPluginInto`, `CallPluginRaw` | `PluginDep(target, desc)` |
| `file` | Работа с файловым хранилищем | `FileMeta`, `FileRead`, `FileReadAll`, `FileURL`, `FileStore` | `File(desc)` |

## Объявление Requirements {#requirements}

Плагин декларирует необходимые ресурсы через builder-паттерн. Хост показывает эти требования администратору при установке.

```go
import wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"

wasmplugin.Plugin{
    ID:      "analytics",
    Name:    "Analytics Plugin",
    Version: "1.0.0",
    Requirements: []wasmplugin.Requirement{
        wasmplugin.Database("Хранение метрик").Build(),
        wasmplugin.HTTP("Отправка данных во внешнюю систему").Build(),
        wasmplugin.KV("Кеширование агрегатов").Build(),
        wasmplugin.NotifyReq("Оповещения при превышении порогов").Build(),
        wasmplugin.EventsReq("Публикация событий аналитики").Build(),
        wasmplugin.PluginDep("users", "Чтение профилей пользователей").Build(),
    },
    // ...
}
```

Каждый конструктор принимает строку-описание, объясняющую зачем плагину нужен этот ресурс. Описание отображается администратору. `.Build()` финализирует требование.

Для `Database` можно задать логическое имя через `.Name()`, чтобы плагин мог использовать несколько баз данных:

```go
Requirements: []wasmplugin.Requirement{
    wasmplugin.Database("Основное хранилище").Build(),                     // "default"
    wasmplugin.Database("Аналитика (read replica)").Name("analytics").Build(),
},
```

Строки подключения администратор указывает в секции `databases` конфигурации:

```json
{
  "databases": {
    "default": "postgres://user:pass@host/main",
    "analytics": "postgres://user:pass@host/analytics"
  }
}
```

::: warning
Если плагин вызывает Host API без соответствующего разрешения, вызов вернёт ошибку `permission denied`. Объявляйте все необходимые Requirements заранее.
:::

### WithConfig

Для requirements, требующих дополнительной конфигурации, используйте `.WithConfig()`:

```go
Requirements: []wasmplugin.Requirement{
    wasmplugin.HTTP("Внешний API").WithConfig(
        wasmplugin.ConfigFields(
            wasmplugin.String("api_url", "URL платёжного API").Required(),
            wasmplugin.String("api_key", "API-ключ").Required().Sensitive(),
        ),
    ).Build(),
},
```

## Ограничения среды выполнения {#limits}

Каждый экземпляр WASM-плагина выполняется в строгой песочнице:

| Ограничение | Значение | Описание |
|---|---|---|
| Память | 512 МБ | 8192 страниц линейной памяти WASM в текущем bootstrap |
| Таймаут | 5 секунд | Максимальное время одного выполнения |
| Конкурентность | 8 | Макс. одновременных выполнений на плагин |
| Arena | 64 КБ initial | Буфер SDK, растущий по требованию в пределах memory limit |
| Файловая система | Нет | Полная песочница, доступ к ФС отсутствует |
| Сеть | Только Host API | Через `HTTPRequest` с разрешением `network` |

::: info Конкурентность
Хост использует семафорный пул для ограничения одновременных выполнений плагина. Пул реализован как буферизованный канал-семафор: каждый `Execute()` захватывает токен, создаёт новый экземпляр WASM, выполняет его и возвращает токен. Если все 8 слотов заняты, новые запросы ожидают освобождения (до таймаута в 5 секунд).
:::

## Что дальше?

- [Миграции](/deploy/migrations) - управление данными при обновлении версий
- [KV Store](/api/kv-store) - хранение данных между вызовами
- [Host API](/api/host-api) - полный список доступных функций
