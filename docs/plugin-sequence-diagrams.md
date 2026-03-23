# Plugin Sequence Diagrams

## 1. Добавление нового плагина (Upload + Install)

```mermaid
sequenceDiagram
    actor Admin
    participant UI as Admin UI
    participant API as AdminHandler
    participant Blob as BlobStore
    participant RT as WasmRuntime
    participant Loader as WasmLoader<br/>(wasm/adapter/loader.go)
    participant Mgr as PluginManager
    participant HostAPI as HostAPI
    participant Reg as PluginRegistry<br/>(wasm/registry)
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
    RT-->>API: PluginMeta (ID, Name, Version,<br/>Commands, Triggers, Permissions,<br/>ConfigSchema, Dependencies, SDKVersion)

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

    API->>Loader: LoadPluginFromBytes(ctx, wasmBytes, config, permissions)

    Loader->>RT: CompileModule(wasmBytes)
    RT-->>Loader: CompiledModule

    Loader->>HostAPI: ForPlugin("_temp_probe", permissions)
    Loader->>RT: compiled.CallMeta()
    RT-->>Loader: PluginMeta
    Loader->>HostAPI: RevokePermissions("_temp_probe")

    rect rgb(240, 248, 255)
        Note over Loader, Reg: Проверка зависимостей и целостности

        alt Dependencies defined
            Loader->>Reg: ResolveDependencies(pluginID, version, installedPlugins)
            Reg-->>Loader: ok (или ошибка, если зависимости не выполнены)
        end

        alt Registry has version hash
            Loader->>Reg: GetVersion(pluginID, version)
            Loader->>Reg: VerifyOrError(wasmBytes, wasmHash)
            Note right of Reg: Проверка целостности<br/>WASM-модуля по хешу
            Reg-->>Loader: ok
        end
    end

    Loader->>HostAPI: ForPlugin(pluginID, permissions)
    Note right of HostAPI: Сохраняет разрешения:<br/>db:read, db:write, kv:read, kv:write,<br/>network:read, plugins:events и т.д.

    alt config предоставлен
        Loader->>Loader: ValidateConfigAgainstSchema(meta.ConfigSchema, config)
        Loader->>RT: compiled.CallConfigure(configJSON)
        RT-->>Loader: OK
    end

    Loader->>RT: compiled.EnablePool(poolConfig)
    Note right of RT: Включение пула модулей<br/>для конкурентного выполнения

    Loader->>Loader: Создаёт WasmPlugin{compiled, meta, config, send}
    Loader->>Loader: plugins[meta.ID] = loadedPlugin{plugin, compiled, config, perms}

    Loader->>Triggers: RegisterTriggers(pluginID, triggers)
    Note right of Triggers: Регистрирует HTTP маршруты,<br/>Cron расписания

    Loader->>Reg: Register(PluginEntry{ID, Name, Dependencies, Signature, Versions})
    Note right of Reg: Сохраняет в реестр:<br/>хеш, зависимости, версии

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
    UI-->>Admin: Плагин установлен и активен
```

## 2. Обновление существующего плагина

```mermaid
sequenceDiagram
    actor Admin
    participant UI as Admin UI
    participant API as AdminHandler
    participant Blob as BlobStore
    participant Loader as WasmLoader<br/>(wasm/adapter/loader.go)
    participant RT as WasmRuntime
    participant HostAPI as HostAPI
    participant Reg as PluginRegistry<br/>(wasm/registry)
    participant Mgr as PluginManager
    participant Triggers as TriggerRegistry
    participant Store as PluginStore
    participant Versions as VersionStore
    participant Bus as EventBus
    participant Metrics as Metrics

    Admin->>UI: Загружает новый .wasm файл
    UI->>API: POST /api/admin/plugins/{id}/update (multipart)

    API->>Mgr: Get(pluginID)
    Mgr-->>API: oldPlugin
    API->>API: oldCommands = oldPlugin.Commands()

    API->>API: SHA256(newWasmBytes)
    API->>Blob: Save(pluginID_update_{timestamp}.wasm)
    Blob-->>API: newWasmKey

    API->>Loader: ReloadPluginFromBytes(ctx, pluginID, newWasmBytes, newConfig)

    Note over Loader, Metrics: Начало reload с отслеживанием метрик

    rect rgb(240, 248, 255)
        Note over Loader, RT: Подготовка к обновлению

        Loader->>Loader: old = plugins[pluginID]
        Loader->>Loader: perms = old.perms
        Loader->>Loader: config = newConfig ?? old.config
        Loader->>Loader: oldVersion = old.plugin.Version()
        Loader->>Loader: old.draining.Store(true)
        Note right of Loader: Помечает старый плагин<br/>как сливающийся (draining)
    end

    rect rgb(240, 255, 240)
        Note over Loader, Reg: Загрузка нового модуля (через LoadPluginFromBytes)

        Loader->>RT: CompileModule(newWasmBytes)
        RT-->>Loader: newCompiledModule

        Loader->>RT: newCompiled.CallMeta()
        RT-->>Loader: newPluginMeta

        alt Dependencies defined
            Loader->>Reg: ResolveDependencies(pluginID, newVersion, installedPlugins)
            Reg-->>Loader: ok
        end

        alt Registry has version hash
            Loader->>Reg: VerifyOrError(newWasmBytes, wasmHash)
            Reg-->>Loader: ok
        end

        Loader->>HostAPI: ForPlugin(pluginID, perms)
        Note right of HostAPI: Переносит старые разрешения<br/>на новую версию

        alt config существует
            Loader->>Loader: ValidateConfigAgainstSchema(meta.ConfigSchema, config)
            Loader->>RT: newCompiled.CallConfigure(config)
            RT-->>Loader: OK
        end

        Loader->>RT: newCompiled.EnablePool(poolConfig)
        Loader->>Loader: Создаёт newWasmPlugin
        Loader->>Triggers: RegisterTriggers(pluginID, newTriggers)
    end

    rect rgb(255, 248, 240)
        Note over Loader, RT: Миграция версии (если версия изменилась)

        alt oldVersion != newVersion
            Loader->>RT: newCompiled.CallMigrate(ctx, oldVersion, newVersion)
            Note right of RT: Вызов экспорта migrate()<br/>для миграции данных плагина
            RT-->>Loader: OK (или warn если ошибка)
        end
    end

    rect rgb(255, 240, 240)
        Note over Loader, RT: Graceful drain старого плагина

        Loader->>Loader: drainPlugin(old, pluginID)
        Note right of Loader: Ждёт завершения activeRequests<br/>или таймаут (10 секунд)

        alt activeRequests == 0
            Note over Loader: Плагин слит мгновенно
        else Таймаут 10s
            Note over Loader: Force close после таймаута
        end

        Loader->>RT: old.compiled.Close(ctx)
        Note right of RT: Освобождение памяти<br/>старого WASM модуля
    end

    Loader-->>API: nil (success)
    Loader->>Metrics: PluginReloadTotal.Inc(pluginID, "ok")
    Loader->>Metrics: PluginReloadDuration.Observe(duration)

    Note over API, Mgr: Обновление в менеджере

    API->>Mgr: Remove(pluginID)
    API->>Mgr: Register(newPlugin)

    Note over API, API: Синхронизация команд

    API->>API: registerPluginCommands(newPlugin)
    API->>API: syncCommandsOnUpdate(oldCommands, newCommands)

    Note right of API: Сравнение старых и новых команд:<br/>Новые команды → регистрация<br/>Удалённые → unregister + удаление<br/>настроек разрешений команд

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
    UI-->>Admin: Плагин обновлён
```

## Ключевые отличия между добавлением и обновлением

| Аспект | Добавление (Install) | Обновление (Update) |
|--------|---------------------|---------------------|
| Фазы | 2 фазы: Upload → Install | 1 фаза: Upload + обновление |
| Метаданные | Извлекаются и показываются пользователю для подтверждения | Извлекаются автоматически |
| Конфигурация | Задаётся пользователем | Наследуется от старой версии (если не задана новая) |
| Разрешения | Задаются пользователем | Переносятся + автовыдача новых |
| Зависимости | Проверяются ResolveDependencies | Проверяются ResolveDependencies |
| Целостность | Проверяется VerifyOrError | Проверяется VerifyOrError |
| Команды | Просто регистрируются | Синхронизация: добавление новых, удаление убранных |
| Триггеры | Регистрируются | Перерегистрация (unreg + reg) |
| Старый модуль | — | Graceful drain → Close |
| Миграция | — | CallMigrate(oldVersion, newVersion) |
| Метрики | — | PluginReloadTotal, PluginReloadDuration |
| Событие | `EventPluginInstalled` | `EventPluginUpdated` |
