# Быстрый старт

Пошаговое руководство: от пустой директории до работающего WASM-плагина.

## Требования

- **Go 1.24+** (с поддержкой `GOOS=wasip1`)

Проверьте версию:

```bash
go version
# go version go1.24.0 linux/amd64
```

## 1. Создайте проект

```bash
mkdir my-plugin && cd my-plugin
go mod init my-plugin
```

## 2. Добавьте SDK

SDK распространяется как Go-модуль из основного репозитория SuperBotGo. Установите последнюю версию:

```bash
go get github.com/StaZisS/SuperBotGo/sdk/go-plugin@latest
```

Или конкретную версию:

```bash
go get github.com/StaZisS/SuperBotGo/sdk/go-plugin@v0.1.0
```

В коде SDK импортируется с алиасом `wasmplugin`:

```go
import wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"
```

::: tip Локальная разработка
Для разработки с локальной копией SDK используйте `replace` в `go.mod`:
```
replace github.com/StaZisS/SuperBotGo/sdk/go-plugin => ../../sdk/go-plugin
```
:::

## 3. Напишите плагин

Создайте файл `main.go`:

```go
package main

import wasmplugin "github.com/StaZisS/SuperBotGo/sdk/go-plugin"

func main() {
    wasmplugin.Run(wasmplugin.Plugin{
        ID:      "hello",
        Name:    "Hello Plugin",
        Version: "1.0.0",
        Triggers: []wasmplugin.Trigger{
            {
                Name:        "hello",
                Type:        wasmplugin.TriggerMessenger,
                Description: "Приветствие",
                Handler: func(ctx *wasmplugin.EventContext) error {
                    ctx.Reply("Привет, мир!")
                    return nil
                },
            },
        },
    })
}
```

Пользователь введёт `/hello` в чат, и бот ответит `Привет, мир!`.

## 4. Соберите WASM-модуль

```bash
GOOS=wasip1 GOARCH=wasm go build -o my-plugin.wasm .
```

::: tip Оптимизированная сборка
Используйте `-ldflags="-s -w"` чтобы убрать отладочную информацию и уменьшить размер файла:
```bash
GOOS=wasip1 GOARCH=wasm go build -ldflags="-s -w" -o my-plugin.wasm .
```
:::

## Структура проекта

После сборки проект выглядит так:

```
my-plugin/
├── go.mod           # модуль и зависимости
├── go.sum
├── main.go          # wasmplugin.Run(...)
└── my-plugin.wasm   # собранный плагин
```

По мере роста плагина выделяйте обработчики и данные в отдельные файлы:

```
my-plugin/
├── go.mod
├── go.sum
├── main.go          # wasmplugin.Run(...)
├── handlers.go      # обработчики команд и триггеров
├── data.go          # хелперы и данные
└── my-plugin.wasm
```

## Что дальше?

- [Структура плагина](/guide/plugin-structure) - поля `Plugin`, жизненный цикл, требования
- [Триггеры](/guide/triggers) - Messenger-команды, HTTP, Cron, Event Bus
- [Конфигурация](/guide/configuration) - типизированная схема конфигурации
