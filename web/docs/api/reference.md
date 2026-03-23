# Справочник API

Полный список всех публичных функций и типов пакета `wasmplugin`.

## Точка входа

| Функция | Описание |
|---|---|
| `Run(p Plugin)` | Точка входа — вызывать из `main()` |

## Функции Host API

| Функция | Описание | Разрешение |
|---|---|---|
| `DBQuery(query)` | Запрос к БД | `db:read` |
| `DBSave(record)` | Сохранение в БД | `db:write` |
| `HTTPRequest(method, url, headers, body)` | HTTP-запрос | `network:read` |
| `HTTPGet(url)` | HTTP GET (shortcut) | `network:read` |
| `HTTPPost(url, contentType, body)` | HTTP POST (shortcut) | `network:read` |
| `CallPlugin(target, method, params)` | Вызов другого плагина | `plugins:events` |
| `PublishEvent(topic, payload)` | Публикация события | `plugins:events` |

## Кодирование

| Функция | Описание |
|---|---|
| `UseMessagePack()` | Переключить на MessagePack |
| `UseJSON()` | Переключить на JSON (по умолчанию) |

## Конструкторы схемы конфигурации

| Функция | Описание |
|---|---|
| `ConfigFields(fields...)` | Создать схему конфигурации |
| `String(key, desc)` | Строковое поле |
| `Integer(key, desc)` | Целочисленное поле |
| `Number(key, desc)` | Числовое поле |
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

## Node-конструкторы

| Функция | Описание |
|---|---|
| `NewStep(param)` | Создать шаг команды |
| `Opt(label, value)` | Создать опцию |

### Методы StepBuilder

| Метод | Описание |
|---|---|
| `.Text(text, style)` | Добавить текстовый блок |
| `.Options(prompt, opts...)` | Статические опции |
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
| `BranchOn(param, cases...)` | Ветвление по значению |
| `Case(value, nodes...)` | Ветка для значения |
| `DefaultCase(nodes...)` | Ветка по умолчанию |
| `ConditionalBranch(cases...)` | Условное ветвление |
| `When(cond, nodes...)` | Декларативная условная ветка |
| `WhenFunc(fn, nodes...)` | Callback-условная ветка |
| `Otherwise(nodes...)` | Ветка по умолчанию |

### Условия

| Функция | Описание |
|---|---|
| `ParamEq(param, value)` | Параметр равен значению |
| `ParamNeq(param, value)` | Параметр не равен |
| `ParamMatch(param, regex)` | Параметр соответствует regex |
| `ParamSet(param)` | Параметр был заполнен |
| `And(conds...)` | Все условия истинны |
| `Or(conds...)` | Хотя бы одно истинно |
| `Not(cond)` | Отрицание |

## Константы

| Константа | Значение | Описание |
|---|---|---|
| `TriggerHTTP` | `"http"` | Тип HTTP-триггера |
| `TriggerCron` | `"cron"` | Тип Cron-триггера |
| `TriggerEvent` | `"event"` | Тип Event bus-триггера |
| `StylePlain` | `"plain"` | Обычный текст |
| `StyleHeader` | `"header"` | Заголовок |
| `StyleSubheader` | `"subheader"` | Подзаголовок |
| `StyleCode` | `"code"` | Моноширинный текст |
| `StyleQuote` | `"quote"` | Цитата |
| `ProtocolVersion` | `1` | Версия протокола SDK |
