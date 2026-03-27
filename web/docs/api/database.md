# База данных

Для работы с базой данных используется стандартный пакет `database/sql`. Плагин открывает соединение через драйвер `"superbot"`, передавая имя базы данных:

```go
import "database/sql"

// Подключение к базе "default" (основная БД)
db, err := sql.Open("superbot", "")
if err != nil {
    return err
}
defer db.Close()
```

Пустая строка `""` эквивалентна `"default"`. Если плагин использует несколько баз данных, указывайте имя явно:

```go
// Основная БД
mainDB, _ := sql.Open("superbot", "")

// Именованная БД
analyticsDB, _ := sql.Open("superbot", "analytics")
```

## Запрос

```go
rows, err := db.Query("SELECT id, name FROM users WHERE active = $1", true)
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    var id int
    var name string
    rows.Scan(&id, &name)
}
```

## Сохранение

```go
_, err := db.Exec(
    "INSERT INTO logs (event, username) VALUES ($1, $2)",
    "login", "alice",
)
```

## Требование

Плагин обязан объявить `Database` requirement. Хост автоматически добавит секцию `databases` в конфиг-схему, куда администратор вводит строки подключения.

```go
// Одна БД (имя "default"):
wasmplugin.Database("Чтение данных пользователей").Build()

// Несколько БД:
wasmplugin.Database("Основное хранилище").Build(),
wasmplugin.Database("Аналитика (read replica)").Name("analytics").Build(),
```

Администратор при установке указывает конфиг вида:

```json
{
  "databases": {
    "default": "postgres://user:pass@host/main",
    "analytics": "postgres://user:pass@host/analytics"
  }
}
```

## Миграции

SQL-миграции запускаются автоматически перед `OnConfigure` по базе `"default"`. Подробнее см. [Миграции](/deploy/migrations).
