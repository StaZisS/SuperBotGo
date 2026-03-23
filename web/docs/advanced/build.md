# Сборка и деплой

## Сборка WASM

```bash
GOOS=wasip1 GOARCH=wasm go build -o my-plugin.wasm .
```

### Оптимизированная сборка

Удаление отладочной информации для уменьшения размера:

```bash
GOOS=wasip1 GOARCH=wasm go build -ldflags="-s -w" -o my-plugin.wasm .
```

## Установка через Admin API

```bash
# Загрузка — извлечение метаданных
curl -X POST http://host/api/admin/plugins/upload \
  -F "file=@my-plugin.wasm"

# Установка — активация плагина
curl -X POST http://host/api/admin/plugins/{id}/install \
  -H "Content-Type: application/json" \
  -d '{"wasm_key": "...", "config": {...}, "permissions": [...]}'
```

## Разрешения {#permissions}

Каждый вызов Host API проверяет разрешения в рантайме. Отсутствие разрешения приводит к ошибке.

| Ключ | Описание | Для чего |
|---|---|---|
| `db:read` | Чтение из базы данных | `DBQuery` |
| `db:write` | Запись в базу данных | `DBSave` |
| `kv:read` | Чтение из KV Store | `KVGet`, `KVList` |
| `kv:write` | Запись в KV Store | `KVSet`, `KVDelete` |
| `network:read` | Исходящие HTTP-запросы | `HTTPRequest`, `HTTPGet`, `HTTPPost` |
| `plugins:events` | Межплагинное взаимодействие | `CallPlugin`, `PublishEvent` |
| `triggers:http` | Регистрация HTTP-эндпоинтов | HTTP-триггеры |

Объявление разрешений в плагине:

```go
Permissions: []wasmplugin.Permission{
    {Key: "db:read", Description: "Чтение данных пользователей", Required: true},
    {Key: "network:read", Description: "Запросы к внешнему API"},
},
```

## Ограничения среды выполнения {#limits}

| Ограничение | По умолчанию | Описание |
|---|---|---|
| Память | 16 МБ | 256 страниц линейной памяти WASM |
| Таймаут | 5 секунд | Таймаут на одно выполнение |
| Конкурентность | 8 | Макс. одновременных выполнений на плагин |
| Arena | 256 КБ | Heap для передачи данных между хостом и плагином |
| Файловая система | Нет | Полная песочница, без доступа к ФС |
| Сеть | Только Host API | Через `HTTPRequest` с разрешением `network:read` |

::: info Конкурентность
Хост использует семафорный пул для ограничения одновременных выполнений плагина. Если все слоты заняты, новые запросы ждут (до таймаута). Пул использует буферизованный канал как семафор — каждый `Execute()` захватывает токен, создаёт новый экземпляр WASM, выполняет его и возвращает токен.
:::
