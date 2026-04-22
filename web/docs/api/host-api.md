# Host API

Плагины взаимодействуют с платформой через функции хоста. Каждая группа функций требует соответствующего [requirement](/guide/plugin-structure#requirements) в манифесте плагина.

| Группа | Описание | Требование |
|---|---|---|
| [База данных](/api/database) | SQL-запросы через `database/sql` | `Database(desc)` |
| [HTTP-клиент](/api/http) | Исходящие HTTP-запросы | `HTTP(desc)` |
| [KV Store](/api/kv-store) | Key-value хранилище с TTL | `KV(desc)` |
| [Уведомления](/api/notifications) | Отправка уведомлений пользователям и в чаты | `NotifyReq(desc)` |
| [Файлы](/api/files) | Приём, хранение и отправка файлов | `File(desc)` |
| [Межплагинное взаимодействие](/api/plugins) | Вызов методов других плагинов и публикация событий | `PluginDep` / `EventsReq` |

Дополнительно:

- HTTP host calls всегда проходят SSRF-защиту.
- При включённом `wasm.http_policy_enabled` host применяет requirement-driven HTTP policy из `requirements.http.<name>`.
- `call_plugin` использует отдельный RPC path и разрешает только методы, опубликованные в `RPCMethods`.
- Для файлов основной bulk-read ABI теперь `file_read_into`; старый `file_read` помечен как `deprecated` и сохранён только для обратной совместимости со старыми `.wasm`.
