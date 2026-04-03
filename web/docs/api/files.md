# Файлы

Файловое API позволяет плагинам принимать файлы от пользователей (фото, документы, аудио, видео), сохранять новые файлы и отправлять их обратно в чат.

Плагины работают **только со ссылками** (`FileRef`) — содержимое файлов не передаётся в event data. Для чтения и записи содержимого используются host-функции.

## Приём файлов

Когда пользователь отправляет файл с командой, плагин получает массив `FileRef` в событии:

```go
Handler: func(ctx *wasmplugin.EventContext) error {
    if !ctx.HasFiles() {
        ctx.Reply(wasmplugin.NewMessage("Отправьте файл вместе с командой"))
        return nil
    }

    for _, f := range ctx.Files() {
        ctx.Log(fmt.Sprintf("Файл: %s, тип: %s, размер: %d",
            f.Name, f.FileType, f.Size))
    }
    return nil
}
```

### FileRef

| Поле | Тип | Описание |
|---|---|---|
| `ID` | `string` | Уникальный идентификатор в FileStore |
| `Name` | `string` | Имя файла (`photo.jpg`, `report.pdf`) |
| `MIMEType` | `string` | MIME-тип (`image/jpeg`, `application/pdf`) |
| `Size` | `int64` | Размер в байтах |
| `FileType` | `string` | Категория: `photo`, `document`, `audio`, `video`, `voice`, `sticker` |

### Методы проверки

```go
ctx.HasFiles() bool     // есть ли прикреплённые файлы
ctx.Files() []FileRef   // список файлов
```

## Чтение содержимого

### `ctx.FileReadAll(fileID string) ([]byte, error)`

Читает весь файл в память. Подходит для файлов до нескольких МБ.

```go
data, err := ctx.FileReadAll(file.ID)
if err != nil {
    ctx.LogError("не удалось прочитать файл: " + err.Error())
    return nil
}
// data — содержимое файла ([]byte)
```

### `ctx.FileRead(fileID string, offset, maxBytes int64) ([]byte, bool, error)`

Чтение чанками. Полезно для больших файлов. Возвращает данные, флаг EOF и ошибку.

```go
var buf bytes.Buffer
var offset int64
for {
    chunk, eof, err := ctx.FileRead(file.ID, offset, 256*1024) // 256 KB
    if err != nil {
        return err
    }
    buf.Write(chunk)
    if eof {
        break
    }
    offset += int64(len(chunk))
}
```

::: info Лимит чанка
Максимальный размер одного чанка — **1 МБ**. При `maxBytes == 0` читается до 1 МБ.
:::

## Метаданные

### `ctx.FileMeta(fileID string) (*FileRef, error)`

Получает метаданные файла без скачивания содержимого:

```go
meta, err := ctx.FileMeta(fileID)
if err != nil {
    return err
}
fmt.Printf("%s (%d байт, %s)\n", meta.Name, meta.Size, meta.MIMEType)
```

## Сохранение файлов

### `ctx.FileStore(name, mimeType, fileType string, data []byte) (*FileRef, error)`

Сохраняет новый файл в FileStore:

```go
pdfData := generateReport()
ref, err := ctx.FileStore("report.pdf", "application/pdf", "document", pdfData)
if err != nil {
    return err
}
// ref.ID — можно использовать для отправки или хранения
```

### `ctx.FileStoreWithTTL(..., ttl time.Duration) (*FileRef, error)`

Сохраняет файл с ограниченным временем жизни:

```go
ref, err := ctx.FileStoreWithTTL("temp.png", "image/png", "photo", imgData, 1*time.Hour)
```

## Отправка файлов

Файлы отправляются через тип `Message` с методом `.File(ref, caption)`:

```go
ref, _ := ctx.FileStore("export.csv", "text/csv", "document", csvData)
ctx.Reply(wasmplugin.NewMessage("").File(*ref, ""))
```

Можно отправлять несколько файлов и комбинировать с текстом в одном вызове:

```go
ctx.Reply(wasmplugin.NewMessage("Вот ваш отчёт:").File(*pdfRef, "").File(*csvRef, ""))
```

Метод `.File(ref, caption)` принимает `FileRef` и необязательный caption (подпись к файлу). Можно вызывать цепочкой для отправки нескольких файлов.

### Отправка входящего файла обратно

```go
for _, f := range ctx.Files() {
    ctx.Reply(wasmplugin.NewMessage("").File(f, ""))  // эхо — отправить обратно
}
```

## Получение URL

### `ctx.FileURL(fileID string) (string, error)`

Возвращает временный URL для скачивания файла. Полезно при S3-бэкенде для передачи ссылки внешним системам.

```go
url, err := ctx.FileURL(file.ID)
if url != "" {
    ctx.Reply(wasmplugin.NewMessage("Скачать: " + url))
}
```

::: warning LocalFS
При использовании локального хранилища (LocalFS) метод возвращает пустую строку — прямые URL не поддерживаются.
:::

## Справочник API

| Метод | Сигнатура | Описание |
|---|---|---|
| `HasFiles` | `() bool` | Есть ли файлы во входящем сообщении |
| `Files` | `() []FileRef` | Список прикреплённых файлов |
| `FileMeta` | `(fileID string) (*FileRef, error)` | Метаданные файла |
| `FileRead` | `(fileID string, offset, maxBytes int64) ([]byte, bool, error)` | Чтение чанками |
| `FileReadAll` | `(fileID string) ([]byte, error)` | Чтение целиком |
| `FileURL` | `(fileID string) (string, error)` | Временный URL |
| `FileStore` | `(name, mimeType, fileType string, data []byte) (*FileRef, error)` | Сохранение |
| `FileStoreWithTTL` | `(name, mimeType, fileType string, data []byte, ttl time.Duration) (*FileRef, error)` | Сохранение с TTL |
| `Reply` (с файлом) | `(msg Message)` | Отправка файла через `wasmplugin.NewMessage("").File(ref, caption)` |

**Необходимое требование:**

```go
wasmplugin.File("Описание для чего нужны файлы")
```

## Полный пример: галерея фото

```go
wasmplugin.Plugin{
    ID:      "gallery",
    Name:    "Photo Gallery",
    Version: "1.0.0",
    Requirements: []wasmplugin.Requirement{
        wasmplugin.File("Приём и хранение фото").Build(),
        wasmplugin.KV("Хранение списков фото").Build(),
    },
    Triggers: []wasmplugin.Trigger{
        {
            Name:    "save",
            Type:    wasmplugin.TriggerMessenger,
            Handler: func(ctx *wasmplugin.EventContext) error {
                if !ctx.HasFiles() {
                    ctx.Reply(wasmplugin.NewMessage("Отправьте фото вместе с командой /save"))
                    return nil
                }

                for _, f := range ctx.Files() {
                    // Прочитать содержимое
                    data, err := ctx.FileReadAll(f.ID)
                    if err != nil {
                        ctx.LogError("чтение: " + err.Error())
                        continue
                    }

                    // Сохранить в FileStore
                    stored, err := ctx.FileStore(f.Name, f.MIMEType, f.FileType, data)
                    if err != nil {
                        ctx.LogError("сохранение: " + err.Error())
                        continue
                    }

                    // Запомнить ID в KV
                    key := fmt.Sprintf("user:%d:photos", ctx.Messenger.UserID)
                    existing, _, _ := ctx.KVGet(key)
                    if existing != "" {
                        existing += ","
                    }
                    ctx.KVSet(key, existing+stored.ID)
                }

                ctx.Reply(wasmplugin.NewMessage("Фото сохранены!"))
                return nil
            },
        },
        {
            Name:    "show",
            Type:    wasmplugin.TriggerMessenger,
            Handler: func(ctx *wasmplugin.EventContext) error {
                key := fmt.Sprintf("user:%d:photos", ctx.Messenger.UserID)
                val, found, _ := ctx.KVGet(key)
                if !found || val == "" {
                    ctx.Reply(wasmplugin.NewMessage("Нет сохранённых фото"))
                    return nil
                }

                ids := strings.Split(val, ",")
                msg := wasmplugin.NewMessage(fmt.Sprintf("Ваши фото (%d):", len(ids)))
                for _, id := range ids {
                    meta, err := ctx.FileMeta(id)
                    if err != nil {
                        continue
                    }
                    msg = msg.File(*meta, "")
                }
                ctx.Reply(msg)
                return nil
            },
        },
    },
}
```
