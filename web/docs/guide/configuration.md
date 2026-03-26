# Конфигурация

Плагины определяют типизированную схему конфигурации. Хост использует её для валидации значений, а админ-панель генерирует форму на её основе.

## Описание схемы {#schema}

Схема задаётся в поле `Config` структуры `Plugin` с помощью `ConfigFields`:

```go
import wasmplugin "github.com/superbot/wasmplugin"

wasmplugin.Plugin{
    // ...
    Config: wasmplugin.ConfigFields(
        wasmplugin.String("api_key", "API-ключ внешнего сервиса").Required().Sensitive(),
        wasmplugin.String("greeting", "Приветственное сообщение").Default("Привет!"),
        wasmplugin.Integer("timeout", "Таймаут в секундах").Default(30).Min(1).Max(300),
        wasmplugin.Number("rate", "Множитель скорости").Default(1.0).Min(0.1).Max(10.0),
        wasmplugin.Bool("verbose", "Подробное логирование"),
        wasmplugin.Enum("theme", "Цветовая тема", "light", "dark", "auto"),
    ),
}
```

## Типы полей {#field-types}

| Конструктор | JSON Schema тип | Описание |
|---|---|---|
| `String(key, desc)` | `"string"` | Строковое значение |
| `Integer(key, desc)` | `"integer"` | Целое число |
| `Number(key, desc)` | `"number"` | Число с плавающей точкой |
| `Bool(key, desc)` | `"boolean"` | Булево значение |
| `Enum(key, desc, values...)` | `"string"` + `enum` | Одно из перечисленных значений |

Каждый конструктор принимает `key` (ключ для доступа из кода) и `desc` (описание, отображаемое в UI). `Enum` дополнительно принимает список допустимых значений.

## Модификаторы {#modifiers}

Модификаторы вызываются цепочкой (chaining) и уточняют правила валидации:

| Метод | Применимость | Описание |
|---|---|---|
| `.Default(v)` | Все типы | Значение по умолчанию |
| `.Required()` | Все типы | Обязательное поле |
| `.Min(n)` | `Integer`, `Number` | Минимальное значение |
| `.Max(n)` | `Integer`, `Number` | Максимальное значение |
| `.MinLen(n)` | `String` | Минимальная длина строки |
| `.MaxLen(n)` | `String` | Максимальная длина строки |
| `.Pattern(re)` | `String` | Валидация по регулярному выражению |
| `.Sensitive()` | `String` | Отображать как пароль в UI, не логировать значение |

### Примеры

```go
// Обязательная строка длиной от 3 до 100 символов
wasmplugin.String("name", "Название проекта").Required().MinLen(3).MaxLen(100)

// Целое число от 1 до 60 со значением по умолчанию
wasmplugin.Integer("interval", "Интервал в минутах").Default(5).Min(1).Max(60)

// Строка, валидируемая по regex
wasmplugin.String("email", "Email администратора").
    Pattern(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).
    Required()

// Секретный ключ - скрыт в UI и логах
wasmplugin.String("token", "OAuth-токен").Required().Sensitive()
```

## Чтение конфигурации в обработчиках {#reading}

Используйте `ctx.Config(key, fallback)` в любом обработчике триггера:

```go
Handler: func(ctx *wasmplugin.EventContext) error {
    greeting := ctx.Config("greeting", "Привет!")
    timeout := ctx.Config("timeout", "30")
    verbose := ctx.Config("verbose", "false")

    ctx.Reply(greeting)
    return nil
}
```

::: info Значения возвращаются как строки
`ctx.Config()` всегда возвращает `string`. При необходимости парсите в нужный тип:
```go
timeout, _ := strconv.Atoi(ctx.Config("timeout", "30"))
```
:::

Второй аргумент `fallback` - значение, возвращаемое если ключ не задан в конфигурации.

## Чтение конфигурации в CallbackContext {#callback}

Конфигурация также доступна в callback-функциях шагов (валидация, динамические опции, пагинация, условия):

```go
wasmplugin.NewStep("mode").
    DynamicOptions("Режим:", func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
        theme := ctx.Config("theme", "light")
        opts := []wasmplugin.Option{
            wasmplugin.Opt("Стандартный", "standard"),
        }
        if theme == "dark" {
            opts = append(opts, wasmplugin.Opt("Ночной", "night"))
        }
        return opts
    })
```

Сигнатура та же: `ctx.Config(key, fallback) string`.

## Callback при конфигурации (OnConfigure) {#on-configure}

`OnConfigure` вызывается при установке или обновлении конфигурации администратором. Используйте для дополнительной валидации или подготовки данных:

```go
import "encoding/json"

wasmplugin.Plugin{
    // ...
    OnConfigure: func(config []byte) error {
        var cfg struct {
            APIKey  string `json:"api_key"`
            Timeout int    `json:"timeout"`
        }
        if err := json.Unmarshal(config, &cfg); err != nil {
            return err
        }
        if cfg.APIKey != "" && len(cfg.APIKey) < 10 {
            return fmt.Errorf("api_key слишком короткий")
        }
        return nil
    },
}
```

- Аргумент `config` - сырой JSON с полями конфигурации.
- Если функция возвращает ошибку, конфигурация **отклоняется** и не сохраняется.
- Если `OnConfigure` не задан, конфигурация сохраняется без дополнительных проверок (только валидация по схеме).
