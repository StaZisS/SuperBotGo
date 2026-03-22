# Plugin Sequence Diagrams

## 1. Добавление нового плагина (Upload + Install)

```mermaid
sequenceDiagram
    actor Admin
    participant UI as Admin UI
    participant API as AdminHandler
    participant Blob as BlobStore
    participant RT as WasmRuntime
    participant Loader as WasmLoader
    participant Mgr as PluginManager
    participant HostAPI as HostAPI
    participant Triggers as TriggerRegistry
    participant Store as PluginStore
    participant Versions as VersionStore
    participant Bus as EventBus

    Note over Admin, Bus: Фаза 1: Upload (загрузка и извлечение метаданных)

    Admin->>UI: Загружает .wasm файл
    UI->>API: POST /api/admin/plugins/upload (multipart)
    API->>RT: CompileModule(wasmBytes)
    RT-->>API: CompiledModule

    API->>HostAPI: ForPlugin("_upload_probe")
    HostAPI-->>API: hostModule (песочница для проверки)

    API->>RT: compiled.CallMeta()
    Note right of RT: Вызов экспорта meta()<br/>из WASM модуля
    RT-->>API: PluginMeta (ID, Name, Version,<br/>Commands, Triggers, Permissions,<br/>ConfigSchema)

    API->>API: SHA256(wasmBytes)
    API->>Blob: Save(pluginID_version.wasm)
    Blob-->>API: wasmKey

    API->>RT: compiled.Close()
    API-->>UI: 200 OK {meta, wasmKey, hash}
    UI-->>Admin: Показывает метаданные плагина,<br/>форму конфигурации и список разрешений

    Note over Admin, Bus: Фаза 2: Install (установка и активация)

    Admin->>UI: Подтверждает установку,<br/>задаёт config и permissions
    UI->>API: POST /api/admin/plugins/{id}/install<br/>{wasmKey, config, permissions}

    API->>Blob: Get(wasmKey)
    Blob-->>API: wasmBytes

    API->>Loader: LoadPluginFromBytes(id, wasmBytes, config)
    Loader->>RT: CompileModule(wasmBytes)
    RT-->>Loader: CompiledModule

    Loader->>RT: compiled.CallMeta()
    RT-->>Loader: PluginMeta

    alt config предоставлен
        Loader->>RT: compiled.CallConfigure(configJSON)
        RT-->>Loader: OK
    end

    Loader->>Loader: Создаёт WasmPlugin{compiled, meta, config}

    Loader->>HostAPI: Grant(pluginID, permissions)
    Note right of HostAPI: Сохраняет разрешения:<br/>db:read, db:write,<br/>network:read, triggers:http и т.д.

    Loader->>Triggers: RegisterTriggers(pluginID, triggers)
    Note right of Triggers: Регистрирует HTTP маршруты,<br/>Cron расписания,<br/>Event подписки

    Loader-->>API: WasmPlugin

    API->>Mgr: Register(plugin)
    Note right of Mgr: Добавляет в map[pluginID]Plugin

    API->>API: registerPluginCommands(plugin)
    Note right of API: Регистрирует каждую команду<br/>в StateManager

    API->>Store: SavePlugin(PluginRecord)
    Note right of Store: Сохраняет: ID, Name, Version,<br/>WasmKey, Hash, Config, Enabled=true

    API->>Versions: SaveVersion(VersionRecord)
    Note right of Versions: version=1, wasmKey, hash,<br/>timestamp

    API->>Bus: Publish(EventPluginInstalled)

    API-->>UI: 200 OK {plugin info}
    UI-->>Admin: Плагин установлен и активен ✓
```

## 2. Обновление существующего плагина

```mermaid
sequenceDiagram
    actor Admin
    participant UI as Admin UI
    participant API as AdminHandler
    participant Blob as BlobStore
    participant RT as WasmRuntime
    participant Loader as WasmLoader
    participant Mgr as PluginManager
    participant HostAPI as HostAPI
    participant Triggers as TriggerRegistry
    participant Store as PluginStore
    participant Versions as VersionStore
    participant Bus as EventBus

    Admin->>UI: Загружает новый .wasm файл
    UI->>API: POST /api/admin/plugins/{id}/update (multipart)

    API->>Mgr: Get(pluginID)
    Mgr-->>API: oldPlugin
    API->>API: oldCommands = oldPlugin.Commands()

    API->>API: SHA256(newWasmBytes)
    API->>Blob: Save(pluginID_update_{timestamp}.wasm)
    Blob-->>API: newWasmKey

    API->>Loader: ReloadPluginFromBytes(pluginID, newWasmBytes)

    Note over Loader, RT: Загрузка нового модуля

    Loader->>Loader: oldPermissions = GetPermissions(pluginID)
    Loader->>Loader: oldConfig = oldPlugin.Config()

    Loader->>RT: CompileModule(newWasmBytes)
    RT-->>Loader: newCompiledModule

    Loader->>RT: newCompiled.CallMeta()
    RT-->>Loader: newPluginMeta

    alt config существует
        Loader->>RT: newCompiled.CallConfigure(oldConfig)
        RT-->>Loader: OK
    end

    Loader->>Loader: Создаёт newWasmPlugin

    Loader->>HostAPI: Grant(pluginID, oldPermissions)
    Note right of HostAPI: Переносит старые разрешения<br/>на новую версию

    Note over Loader, Triggers: Перерегистрация триггеров

    Loader->>Triggers: UnregisterTriggers(pluginID)
    Loader->>Triggers: RegisterTriggers(pluginID, newTriggers)

    Note over Loader, RT: Закрытие старого модуля

    Loader->>RT: oldCompiled.Close()
    Note right of RT: Освобождение памяти<br/>старого WASM модуля

    Loader-->>API: newWasmPlugin

    Note over API, Mgr: Обновление в менеджере

    API->>Mgr: Remove(pluginID)
    API->>Mgr: Register(newPlugin)

    Note over API, API: Синхронизация команд

    API->>API: registerPluginCommands(newPlugin)
    API->>API: syncCommandsOnUpdate(oldCommands, newCommands)

    Note right of API: Сравнение старых и новых команд:<br/>• Новые команды → регистрация<br/>• Удалённые → unregister + удаление<br/>  настроек разрешений команд

    Note over API, API: Синхронизация разрешений

    API->>API: syncPermissionsOnUpdate(oldMeta, newMeta)
    Note right of API: Новые required permissions<br/>автоматически выдаются плагину

    Note over API, Store: Персистентность

    API->>Store: UpdatePlugin(PluginRecord)
    Note right of Store: Обновляет: WasmKey, Hash,<br/>Version, UpdatedAt

    API->>Versions: SaveVersion(VersionRecord)
    Note right of Versions: version++, newWasmKey,<br/>newHash, timestamp

    API->>Bus: Publish(EventPluginUpdated)

    API-->>UI: 200 OK {updated plugin info}
    UI-->>Admin: Плагин обновлён ✓
```

## Ключевые отличия между добавлением и обновлением

| Аспект | Добавление (Install) | Обновление (Update) |
|--------|---------------------|---------------------|
| Фазы | 2 фазы: Upload → Install | 1 фаза: Upload + обновление |
| Метаданные | Извлекаются и показываются пользователю для подтверждения | Извлекаются автоматически |
| Конфигурация | Задаётся пользователем | Наследуется от старой версии |
| Разрешения | Задаются пользователем | Переносятся + автовыдача новых |
| Команды | Просто регистрируются | Синхронизация: добавление новых, удаление убранных |
| Триггеры | Регистрируются | Перерегистрация (unreg + reg) |
| Старый модуль | — | Закрывается (Close) |
| Событие | `EventPluginInstalled` | `EventPluginUpdated` |
