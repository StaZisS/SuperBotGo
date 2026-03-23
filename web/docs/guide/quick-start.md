# Быстрый старт

## Минимальный плагин

```go
package main

import "github.com/superbot/wasmplugin"

func main() {
    wasmplugin.Run(wasmplugin.Plugin{
        ID:      "hello",
        Name:    "Hello Plugin",
        Version: "1.0.0",
        Commands: []wasmplugin.Command{
            {
                Name:        "hello",
                Description: "Say hello",
                Handler: func(ctx *wasmplugin.EventContext) error {
                    ctx.Reply("Hello, world!")
                    return nil
                },
            },
        },
    })
}
```

## Инициализация проекта

```bash
# Создание проекта
mkdir my-plugin && cd my-plugin
go mod init my-plugin
go mod edit -require github.com/superbot/wasmplugin@v0.0.0
go mod edit -replace github.com/superbot/wasmplugin=../../sdk/go-plugin
```

## Сборка

```bash
GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm .
```

::: tip Оптимизированная сборка
Используйте `-ldflags="-s -w"` чтобы убрать отладочную информацию и уменьшить размер бинарника:
```bash
GOOS=wasip1 GOARCH=wasm go build -ldflags="-s -w" -o plugin.wasm .
```
:::

## Структура проекта

```
my-plugin/
├── go.mod          # модуль + replace для SDK
├── go.sum
├── main.go         # wasmplugin.Run(...)
├── handlers.go     # обработчики команд и триггеров
└── data.go         # данные, хелперы
```

## go.mod

```
module my-plugin

go 1.24.0

require github.com/superbot/wasmplugin v0.0.0

replace github.com/superbot/wasmplugin => ../../sdk/go-plugin
```

## Что дальше?

- [Структура плагина](/guide/plugin-structure) — структура `Plugin` и жизненный цикл
- [Команды](/guide/commands) — добавление slash-команд для мессенджера
- [Триггеры](/guide/triggers) — обработка HTTP, Cron и Event триггеров
