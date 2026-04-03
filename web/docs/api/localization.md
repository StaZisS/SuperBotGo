# Локализация

SDK предоставляет систему `Catalog` для управления переводами. Каталог хранит строки на всех поддерживаемых языках и интегрируется с методами `EventContext` для автоматического выбора локали.

## Создание каталога

```go
import wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"

catalog := wasmplugin.NewCatalog("ru")
```

Аргумент - локаль по умолчанию. Если для запрошенной локали перевод не найден, используется значение из локали по умолчанию.

## Добавление переводов

### Программно

```go
catalog := wasmplugin.NewCatalog("ru").
    Add("ru", map[string]string{
        "greeting":   "Привет, {name}!",
        "task_done":  "Задача выполнена",
        "item_count": "Найдено {0} элементов",
    }).
    Add("en", map[string]string{
        "greeting":   "Hello, {name}!",
        "task_done":  "Task completed",
        "item_count": "Found {0} items",
    })
```

### Из TOML-файлов

Переводы можно хранить в `.toml` файлах и загружать через `embed.FS`:

```go
import "embed"

//go:embed locales
var locales embed.FS

catalog := wasmplugin.NewCatalog("ru").LoadFS(locales, "locales")
```

Структура каталога:

```
locales/
  ru.toml
  en.toml
```

Формат TOML-файла - плоский список `ключ = "значение"`:

```toml
# locales/ru.toml
greeting = "Привет, {name}!"
task_done = "Задача выполнена"
item_count = "Найдено {0} элементов"
```

```toml
# locales/en.toml
greeting = "Hello, {name}!"
task_done = "Task completed"
item_count = "Found {0} items"
```

Имя файла (без расширения) используется как код локали.

## Объединение каталогов

Метод `Merge` объединяет два каталога. Переводы из переданного каталога дополняют текущий:

```go
base := wasmplugin.NewCatalog("ru").LoadFS(baseLocales, "locales")
extra := wasmplugin.NewCatalog("ru").Add("ru", map[string]string{
    "new_key": "Новое значение",
})

combined := base.Merge(extra)
```

## Получение переводов

### Все локали: `.L(key, args...)`

Возвращает `map[string]string` со значениями для всех локалей. Используется с `wasmplugin.NewLocalizedMessage`.

```go
texts := catalog.L("greeting", "name", "Алиса")
// {"ru": "Привет, Алиса!", "en": "Hello, Алиса!"}
```

### Одна локаль: `.T(locale, key, args...)`

Возвращает строку для конкретной локали:

```go
text := catalog.T("ru", "greeting", "name", "Алиса")
// "Привет, Алиса!"
```

### Функция-переводчик: `.Tr(locale)`

Возвращает функцию `func(key string, args ...any) string`, привязанную к локали:

```go
t := catalog.Tr("en")
msg := t("greeting", "name", "Alice")
// "Hello, Alice!"
```

## Интерполяция

Поддерживаются два вида подстановок:

| Синтаксис | Описание | Пример |
|---|---|---|
| `{name}` | Именованный аргумент | `"Привет, {name}!"` - передать `"name", "Алиса"` |
| `{0}` | Позиционный аргумент | `"Найдено {0} из {1}"` - передать `"5", "10"` |

Именованные аргументы передаются парами `ключ, значение`:

```go
catalog.T("ru", "greeting", "name", "Алиса")
```

Позиционные аргументы передаются по порядку:

```go
catalog.T("ru", "item_count", "42")
```

## Использование в обработчиках

### Локализованный ответ

```go
Handler: func(ctx *wasmplugin.EventContext) error {
    ctx.Reply(wasmplugin.NewLocalizedMessage(catalog.L("task_done")))
    return nil
}
```

## Локализованные опции

Метод `.Opt(key, value, args...)` создаёт локализованную опцию для шагов команд:

```go
catalog := wasmplugin.NewCatalog("ru").
    Add("ru", map[string]string{
        "opt_yes": "Да",
        "opt_no":  "Нет",
        "prompt":  "Подтвердите действие:",
    }).
    Add("en", map[string]string{
        "opt_yes": "Yes",
        "opt_no":  "No",
        "prompt":  "Confirm action:",
    })

Nodes: []wasmplugin.Node{
    wasmplugin.NewStep("confirm").
        LocalizedOptions(catalog.L("prompt"),
            catalog.Opt("opt_yes", "yes"),
            catalog.Opt("opt_no", "no"),
        ),
}
```

## Локализованный Node Builder

Для шагов с локализованным текстом используйте `LocalizedText` и `LocalizedOptions`:

```go
wasmplugin.NewStep("info").
    LocalizedText(catalog.L("welcome_text"), wasmplugin.StyleHeader)

wasmplugin.NewStep("action").
    LocalizedOptions(catalog.L("choose_action"),
        catalog.Opt("opt_create", "create"),
        catalog.Opt("opt_delete", "delete"),
    )
```

## Полный пример

```go
package main

import (
    "embed"

    wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"
)

//go:embed locales
var locales embed.FS

var catalog = wasmplugin.NewCatalog("ru").LoadFS(locales, "locales")

func main() {
    wasmplugin.Run(wasmplugin.Plugin{
        ID:      "i18n-demo",
        Name:    "i18n Demo",
        Version: "1.0.0",

        Triggers: []wasmplugin.Trigger{
            {
                Name:        "hello",
                Type:        wasmplugin.TriggerMessenger,
                Description: "Приветствие",
                Nodes: []wasmplugin.Node{
                    wasmplugin.NewStep("name").
                        LocalizedText(catalog.L("enter_name"), wasmplugin.StylePlain),
                },
                Handler: func(ctx *wasmplugin.EventContext) error {
                    name := ctx.Param("name")
                    ctx.Reply(wasmplugin.NewLocalizedMessage(catalog.L("greeting", "name", name)))
                    return nil
                },
            },
        },
    })
}
```
