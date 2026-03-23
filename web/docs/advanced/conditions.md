# Условия

Декларативные условия выполняются **на стороне хоста** без WASM-callback'ов. Используйте их для видимости шагов и предикатов ветвления.

## Базовые условия

| Конструктор | Описание | Пример |
|---|---|---|
| `ParamEq(param, value)` | Равно | `ParamEq("mode", "advanced")` |
| `ParamNeq(param, value)` | Не равно | `ParamNeq("mode", "quick")` |
| `ParamMatch(param, regex)` | Соответствует regex | `ParamMatch("email", "^.+@.+$")` |
| `ParamSet(param)` | Был заполнен | `ParamSet("date")` |

## Комбинаторы

| Комбинатор | Описание |
|---|---|
| `And(cond1, cond2, ...)` | Все условия истинны |
| `Or(cond1, cond2, ...)` | Хотя бы одно истинно |
| `Not(cond)` | Отрицание |

## Комбинирование условий

```go
wasmplugin.And(
    wasmplugin.ParamEq("mode", "advanced"),
    wasmplugin.Or(
        wasmplugin.ParamEq("building", "1"),
        wasmplugin.ParamEq("building", "3"),
    ),
    wasmplugin.Not(wasmplugin.ParamSet("skip")),
)
```

## Использование в видимости шагов

```go
wasmplugin.NewStep("extra").
    Text("Дополнительные параметры:", wasmplugin.StylePlain).
    VisibleWhen(wasmplugin.And(
        wasmplugin.ParamEq("mode", "advanced"),
        wasmplugin.ParamSet("building"),
    ))
```

## Использование в условных ветках

```go
wasmplugin.ConditionalBranch(
    wasmplugin.When(
        wasmplugin.ParamEq("building", "3"),
        wasmplugin.NewStep("wing").Options("Крыло:",
            wasmplugin.Opt("Восточное", "east"),
            wasmplugin.Opt("Западное", "west"),
        ),
    ),
    wasmplugin.Otherwise(
        // выбор крыла не нужен
    ),
)
```

## Декларативные vs callback-условия

| | Декларативные | Callback |
|---|---|---|
| **Синтаксис** | `VisibleWhen(cond)` / `When(cond, ...)` | `VisibleWhenFunc(fn)` / `WhenFunc(fn, ...)` |
| **Выполнение** | На хосте, без вызова WASM | WASM-callback |
| **Производительность** | Быстрее | Медленнее (создаётся экземпляр модуля) |
| **Гибкость** | Только сравнение параметров | Произвольная логика на Go |

::: tip Предпочитайте декларативные
Используйте декларативные условия, когда возможно — они быстрее, потому что хост вычисляет их без запуска WASM-экземпляра. Callback-условия нужны только когда требуется логика за пределами простого сравнения параметров.
:::
