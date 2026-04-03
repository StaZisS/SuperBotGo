# Файловая подсистема

Файловая подсистема отвечает за приём, хранение, чтение и отправку файлов пользователей через мессенджеры. Она интегрирована на всех уровнях: от адаптеров каналов до WASM host API.

## Типы модели

### Входящие файлы

```
internal/model/file.go     — FileRef, FileType
internal/model/input.go    — FileInput (implements UserInput)
```

`FileInput` реализует интерфейс `UserInput`. Если caption начинается с `/` — роутится как команда с прикреплёнными файлами. Если caption пустой и нет активного диалога — игнорируется.

```go
type FileInput struct {
    Caption string       // текст/caption сообщения
    Files   []FileRef    // прикреплённые файлы
}

type FileRef struct {
    ID       string      // UUID в FileStore
    Name     string      // оригинальное имя файла
    MIMEType string      // MIME-тип
    Size     int64       // размер в байтах
    FileType FileType    // photo, document, audio, video, voice, sticker
}
```

### Исходящие файлы

```
internal/model/message.go  — FileBlock (implements ContentBlock)
```

`FileBlock` — блок контента для отправки файлов в сообщении:

```go
type FileBlock struct {
    FileRef FileRef
    Caption string
}
```

### Event data

`MessengerTriggerData.Files` и `CommandRequest.Files` — пробрасывают `[]FileRef` от адаптера до плагина. `EventResponse.ReplyFiles` — файлы, которые плагин хочет отправить обратно.

## FileStore

```
internal/filestore/filestore.go  — интерфейс
internal/filestore/s3.go         — S3 реализация
```

Отдельная абстракция (не переиспользует admin `BlobStore`), т.к. нужны метаданные, TTL и cleanup.

```go
type FileStore interface {
    Store(ctx, meta FileMeta, data io.Reader) (FileRef, error)
    Get(ctx, id string) (io.ReadCloser, *FileMeta, error)
    Meta(ctx, id string) (*FileMeta, error)
    Delete(ctx, id string) error
    URL(ctx, id string, expiry time.Duration) (string, error)
    Cleanup(ctx) (int, error)
}
```

### FileMeta

```go
type FileMeta struct {
    ID        string         // генерируется при Store
    Name      string
    MIMEType  string
    Size      int64
    FileType  model.FileType
    PluginID  string         // кто сохранил ("" для входящих)
    CreatedAt time.Time
    ExpiresAt *time.Time     // TTL (nil = без ограничения)
}
```

### S3

Файлы хранятся в S3-совместимом хранилище (AWS S3, MinIO) как два объекта:

```
<prefix><id>.data       — содержимое
<prefix><id>.meta.json  — метаданные (JSON)
```

- `URL()` — возвращает presigned GET URL для прямого скачивания из S3
- `Cleanup()` — листинг `.meta.json` объектов, проверка `ExpiresAt`, удаление просроченных. Запускается горутиной каждый час

## Поток данных: входящий файл

```mermaid
sequenceDiagram
    actor User
    participant TG as Telegram API
    participant Bot as Telegram Bot
    participant FS as FileStore
    participant CM as ChannelManager
    participant TR as TriggerRouter
    participant P as WASM Plugin

    User->>TG: Фото + caption "/photo"
    TG->>Bot: OnPhoto callback
    Bot->>TG: bot.File(teleFile) — скачивание
    TG-->>Bot: io.ReadCloser
    Bot->>FS: Store(FileMeta, reader)
    FS-->>Bot: FileRef{ID, Name, Size}
    Bot->>CM: Update{Input: FileInput{Caption: "/photo", Files: [ref]}}
    CM->>CM: FileInput.IsCommand() → true
    CM->>CM: extractFiles(input) → []FileRef
    CM->>TR: RouteEvent(Event{Files: [ref]})
    TR->>P: HandleEvent(ctx)
    Note over P: ctx.HasFiles() == true<br/>ctx.Files()[0].ID = "abc123"
```

## Поток данных: исходящий файл

```mermaid
sequenceDiagram
    participant P as WASM Plugin
    participant HA as Host API
    participant FS as FileStore
    participant WP as WasmPlugin adapter
    participant Bot as Telegram Adapter
    participant TG as Telegram API

    P->>HA: file_store(name, mime, data)
    HA->>FS: Store(FileMeta, bytes.Reader)
    FS-->>HA: FileRef{ID: "xyz789"}
    HA-->>P: FileRef
    P->>P: ctx.Reply(wasmplugin.NewMessage("").File(ref, ""))
    Note over P: response.ReplyFiles = [ref]
    P-->>WP: EventResponse{ReplyFiles: [ref]}
    WP->>Bot: SendToChat(FileBlock{ref})
    Bot->>FS: Get("xyz789")
    FS-->>Bot: io.ReadCloser + meta
    Bot->>TG: bot.Send(tele.Document{FromReader})
    TG-->>Bot: ok
```

## Конфигурация

```yaml
filestore:
  default_ttl: 24h          # TTL по умолчанию
  max_file_size: 52428800   # 50 МБ
  s3:
    bucket: my-bucket
    region: eu-central-1
    endpoint: http://localhost:9000  # MinIO для dev
    access_key: minioadmin
    secret_key: minioadmin
    prefix: files/
```
