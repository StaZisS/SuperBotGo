# База данных

Для работы с базой данных используется стандартный пакет `database/sql`. Плагин открывает соединение через драйвер `"superbot"`:

```go
import "database/sql"

db, err := sql.Open("superbot", "")
if err != nil {
    return err
}
defer db.Close()
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

```go
wasmplugin.Database("Чтение данных пользователей")
```
