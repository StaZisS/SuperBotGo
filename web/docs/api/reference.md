# Справочник API

Полный список всех публичных функций и типов пакета `wasmplugin`.

## Точка входа

| Функция | Описание |
|---|---|
| `Run(p Plugin)` | Точка входа - вызывать из `main()` |

## Функции Host API

| Функция | Сигнатура | Требование |
|---|---|---|
| `sql.Open` | `("superbot", "") (*sql.DB, error)` | `Database(desc)` |
| `HTTPRequest` | `(method, url string, headers map[string]string, body string) (*HTTPResponse, error)` | `HTTP(desc)` |
| `HTTPGet` | `(url string) (*HTTPResponse, error)` | `HTTP(desc)` |
| `HTTPPost` | `(url, contentType, body string) (*HTTPResponse, error)` | `HTTP(desc)` |
| `CallPlugin` | `(target, method string, params interface{}) ([]byte, error)` | `PluginDep(target, desc)` |
| `PublishEvent` | `(topic string, payload interface{}) error` | `EventsReq(desc)` |

## Методы EventContext

### Сообщения

| Метод | Сигнатура |
|---|---|
| `Reply` | `(text string)` |
| `ReplyLocalized` | `(texts map[string]string)` |

### HTTP-ответы

| Метод | Сигнатура |
|---|---|
| `SetHTTPResponse` | `(statusCode int, headers map[string]string, body string)` |
| `JSON` | `(statusCode int, v interface{})` |

### Логирование

| Метод | Сигнатура |
|---|---|
| `Log` | `(msg string)` |
| `LogError` | `(msg string)` |

### Данные

| Метод | Сигнатура | Описание |
|---|---|---|
| `Config` | `(key, fallback string) string` | Значение конфигурации |
| `Param` | `(key string) string` | Параметр команды |
| `Locale` | `() string` | Локаль пользователя |

### Уведомления

| Метод | Сигнатура | Требование |
|---|---|---|
| `NotifyUser` | `(userID int64, text string, priority int) error` | `notify` |
| `NotifyChat` | `(channelType, chatID, text string, priority int) error` | `notify` |
| `NotifyProject` | `(projectID int64, text string, priority int) error` | `notify` |

### KV Store

| Метод | Сигнатура | Требование |
|---|---|---|
| `KVGet` | `(key string) (string, bool, error)` | `kv` |
| `KVSet` | `(key, value string) error` | `kv` |
| `KVSetWithTTL` | `(key, value string, ttl time.Duration) error` | `kv` |
| `KVDelete` | `(key string) error` | `kv` |
| `KVList` | `(prefix string) ([]string, error)` | `kv` |

## Конструкторы схемы конфигурации

| Функция | Описание |
|---|---|
| `ConfigFields(fields...)` | Создать схему конфигурации |
| `String(key, desc)` | Строковое поле |
| `Integer(key, desc)` | Целочисленное поле |
| `Number(key, desc)` | Числовое поле (float) |
| `Bool(key, desc)` | Булево поле |
| `Enum(key, desc, values...)` | Перечисление |

### Модификаторы полей

| Метод | Описание |
|---|---|
| `.Default(v)` | Значение по умолчанию |
| `.Required()` | Обязательное поле |
| `.Min(n)` / `.Max(n)` | Мин/макс значение (Number/Integer) |
| `.MinLen(n)` / `.MaxLen(n)` | Мин/макс длина строки |
| `.Pattern(re)` | Regex-валидация |
| `.Sensitive()` | Поле-пароль в UI |

## Конструкторы требований

| Функция | Разрешение | Описание |
|---|---|---|
| `Database(desc)` | `sql` | Доступ к базе данных |
| `HTTP(desc)` | `network` | HTTP-запросы |
| `KV(desc)` | `kv` | Key-value хранилище |
| `NotifyReq(desc)` | `notify` | Отправка уведомлений |
| `EventsReq(desc)` | `events` | Публикация событий |
| `PluginDep(target, desc)` | `plugin:<target>` | Вызов другого плагина |

## Node-конструкторы

| Функция | Описание |
|---|---|
| `NewStep(param)` | Создать шаг команды |
| `Opt(label, value)` | Создать опцию |

### Методы StepBuilder

| Метод | Описание |
|---|---|
| `.Text(text, style)` | Добавить текстовый блок |
| `.LocalizedText(texts, style)` | Локализованный текстовый блок |
| `.Options(prompt, opts...)` | Статические опции |
| `.LocalizedOptions(prompts, ...Option)` | Локализованные опции |
| `.DynamicOptions(prompt, fn)` | Динамические опции (callback) |
| `.PaginatedOptions(prompt, pageSize, fn)` | Пагинированные опции |
| `.Link(url, label)` | Ссылка |
| `.Image(url)` | Картинка |
| `.Validate(regex)` | Regex-валидация ввода |
| `.ValidateFunc(fn)` | Callback-валидация ввода |
| `.VisibleWhen(cond)` | Декларативное условие видимости |
| `.VisibleWhenFunc(fn)` | Callback-условие видимости |

### Ветвление

| Функция | Описание |
|---|---|
| `BranchOn(param, cases...)` | Ветвление по значению параметра |
| `Case(value, nodes...)` | Ветка для конкретного значения |
| `DefaultCase(nodes...)` | Ветка по умолчанию |
| `ConditionalBranch(cases...)` | Условное ветвление |
| `When(cond, nodes...)` | Декларативная условная ветка |
| `WhenFunc(fn, nodes...)` | Callback-условная ветка |
| `Otherwise(nodes...)` | Ветка по умолчанию (для ConditionalBranch) |

### Условия

| Функция | Описание |
|---|---|
| `ParamEq(param, value)` | Параметр равен значению |
| `ParamNeq(param, value)` | Параметр не равен значению |
| `ParamMatch(param, regex)` | Параметр соответствует regex |
| `ParamSet(param)` | Параметр был заполнен |
| `And(conds...)` | Все условия истинны |
| `Or(conds...)` | Хотя бы одно условие истинно |
| `Not(cond)` | Отрицание условия |

## Каталог переводов

| Метод | Сигнатура | Описание |
|---|---|---|
| `NewCatalog` | `(defaultLocale string) *Catalog` | Создать каталог |
| `.Add` | `(locale string, translations map[string]string) *Catalog` | Добавить переводы |
| `.Merge` | `(other *Catalog) *Catalog` | Объединить каталоги |
| `.LoadFS` | `(fsys fs.FS, dir string) *Catalog` | Загрузить TOML-файлы |
| `.L` | `(key string, args ...any) map[string]string` | Все локали для ключа |
| `.T` | `(locale, key string, args ...any) string` | Перевод для одной локали |
| `.Tr` | `(locale string) func(key string, args ...any) string` | Функция-переводчик |
| `.Opt` | `(key, value string, args ...any) Option` | Локализованная опция |

## Миграции

| Функция / тип | Описание |
|---|---|
| `MigrationsFromFS(fsys fs.FS, dir string) []SQLMigration` | Загрузить SQL-миграции из FS |
| `SQLMigration` | `Version int, Description string, Up string, Down string` |

## Константы

| Константа | Значение | Описание |
|---|---|---|
| `TriggerMessenger` | `"messenger"` | Тип messenger-триггера |
| `TriggerHTTP` | `"http"` | Тип HTTP-триггера |
| `TriggerCron` | `"cron"` | Тип Cron-триггера |
| `TriggerEvent` | `"event"` | Тип Event bus-триггера |
| `StylePlain` | `"plain"` | Обычный текст |
| `StyleHeader` | `"header"` | Заголовок |
| `StyleSubheader` | `"subheader"` | Подзаголовок |
| `StyleCode` | `"code"` | Моноширинный текст |
| `StyleQuote` | `"quote"` | Цитата |
| `PriorityLow` | `0` | Информационное уведомление |
| `PriorityNormal` | `1` | Стандартное уведомление |
| `PriorityHigh` | `2` | Важное - с упоминанием |
| `PriorityCritical` | `3` | Срочное - все каналы |
| `ProtocolVersion` | `1` | Версия протокола SDK |
