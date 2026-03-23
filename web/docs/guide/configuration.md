# Конфигурация

Плагины могут определять типизированную схему конфигурации. Хост использует её для валидации, а админ-панель генерирует форму на её основе.

## Описание схемы

```go
Config: wasmplugin.ConfigFields(
    wasmplugin.String("api_key", "API-ключ внешнего сервиса").Required().Sensitive(),
    wasmplugin.String("greeting", "Приветственное сообщение").Default("Привет!"),
    wasmplugin.Integer("timeout", "Таймаут в секундах").Default(30).Min(1).Max(300),
    wasmplugin.Number("rate", "Множитель скорости").Default(1.0).Min(0.1).Max(10.0),
    wasmplugin.Bool("verbose", "Подробное логирование"),
    wasmplugin.Enum("theme", "Цветовая тема", "light", "dark", "auto"),
),
```

## Типы полей

| Конструктор | JSON Schema тип | Описание |
|---|---|---|
| `String(key, desc)` | `"string"` | Строка |
| `Integer(key, desc)` | `"integer"` | Целое число |
| `Number(key, desc)` | `"number"` | Число с плавающей точкой |
| `Bool(key, desc)` | `"boolean"` | Булево значение |
| `Enum(key, desc, values...)` | `"string"` + `enum` | Перечисление |

## Модификаторы

Все модификаторы — chainable:

| Метод | Описание |
|---|---|
| `.Default(v)` | Значение по умолчанию |
| `.Required()` | Обязательное поле |
| `.Min(n)` | Минимум (Number/Integer) |
| `.Max(n)` | Максимум (Number/Integer) |
| `.MinLen(n)` | Минимальная длина строки |
| `.MaxLen(n)` | Максимальная длина строки |
| `.Pattern(re)` | Regex-паттерн для валидации |
| `.Sensitive()` | Отображать как пароль в UI |

## Чтение конфигурации в обработчиках

Используйте `ctx.Config(key, fallback)`:

```go
Handler: func(ctx *wasmplugin.EventContext) error {
    greeting := ctx.Config("greeting", "Привет!")  // ключ, fallback
    timeout  := ctx.Config("timeout", "30")
    // ...
}
```

::: info
Значения конфигурации возвращаются как строки. При необходимости парсите в нужный тип.
:::

Конфигурация также доступна в [CallbackContext](/advanced/node-builder#callbackcontext) для callback-функций шагов:

```go
wasmplugin.NewStep("mode").DynamicOptions("Режим:", func(ctx *wasmplugin.CallbackContext) []wasmplugin.Option {
    theme := ctx.Config("theme", "light")
    // ...
})
```

## Callback при конфигурации

Вызывается при установке или обновлении конфига администратором:

```go
OnConfigure: func(config []byte) error {
    // config — raw JSON от администратора
    // Верните error, чтобы отклонить конфигурацию
    return nil
},
```
