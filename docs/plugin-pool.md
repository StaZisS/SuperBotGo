# Пул плагинов (ModulePool)

## Общая идея

`ModulePool` — это **семафорный пул**, который ограничивает количество одновременно выполняющихся экземпляров одного WASM-плагина. Он **не хранит заранее созданные экземпляры модулей**: каждый вызов создаёт новый экземпляр WASM-модуля через `wazero.InstantiateModule`, а пул лишь контролирует, сколько таких экземпляров могут работать параллельно.

## 1. Выполнение действия через пул (Execute)

Диаграмма: [seq-pool-execute.mmd](seq-pool-execute.mmd)

```mermaid
sequenceDiagram
    title Выполнение действия плагина через ModulePool

    participant Caller as Вызывающий код<br/>(adapter, trigger, etc.)
    participant CM as CompiledModule<br/>(runtime/module.go)
    participant Pool as ModulePool<br/>(runtime/pool.go)
    participant Sem as Semaphore<br/>(chan struct{}, size=N)
    participant Wazero as wazero.Runtime
    participant WASM as WASM Instance<br/>(новый экземпляр)
    participant Metrics as Prometheus Metrics

    Caller->>CM: RunActionWithConfig(ctx, action, input, configJSON)
    CM->>CM: pool != nil?
    Note right of CM: Если пул включён —<br/>делегируем в pool.Execute()

    CM->>Pool: Execute(ctx, action, input, configJSON)
    Pool->>Pool: closed.Load() == false?

    Pool->>Pool: ctx = WithTimeout(ctx, timeoutSec)
    Pool->>Pool: ctx = WithValue(ctx, PluginIDKey, pluginID)
    Pool->>Pool: contextHooks(ctx, pluginID)

    rect rgb(255, 248, 230)
        Note over Pool, Sem: Ожидание свободного слота в семафоре
        Pool->>Sem: <-sem (захват токена)

        alt Слот доступен
            Sem-->>Pool: struct{} (токен получен)
        else Все N слотов заняты
            Note over Pool, Sem: Горутина блокируется до<br/>освобождения слота
            Sem-->>Pool: struct{} (токен получен после ожидания)
        else ctx.Done() (таймаут)
            Pool-->>Caller: error: "context cancelled waiting for pool slot"
        end
    end

    Pool->>Pool: activeExecutions.Add(1)
    Pool->>Pool: totalExecutions.Add(1)

    rect rgb(240, 248, 255)
        Note over Pool, WASM: Создание и выполнение нового экземпляра модуля

        Pool->>Pool: Подготовка ModuleConfig:<br/>env PLUGIN_ACTION=action<br/>env PLUGIN_CONFIG=configJSON<br/>stdin=input, stdout=buffer

        Pool->>Wazero: InstantiateModule(ctx, compiled, moduleConfig)
        Note right of Wazero: Создаётся новый экземпляр<br/>WASM модуля (изолированный)
        Wazero->>WASM: Инициализация + вызов _start
        WASM->>WASM: Читает stdin, выполняет action
        WASM->>WASM: Пишет результат в stdout

        alt ExitCode == 0
            WASM-->>Wazero: sys.ExitError{code: 0}
            Wazero-->>Pool: err (ExitError)
            Pool->>Pool: stdout.Bytes() → результат
        else ExitCode != 0
            WASM-->>Wazero: sys.ExitError{code: N}
            Pool-->>Caller: error: "wasm module exited with code N"
        else Другая ошибка
            Wazero-->>Pool: err
            Pool-->>Caller: error: "instantiate wasm module: ..."
        end
    end

    Note over WASM: Экземпляр уничтожен<br/>(память освобождена)

    Pool->>Pool: activeExecutions.Add(-1)
    Pool->>Sem: sem <- struct{} (возврат токена)

    Pool->>Metrics: PluginActionDuration.Observe(duration)
    Pool->>Metrics: PluginActionTotal.Inc(pluginID, action, status)

    Pool-->>CM: []byte (результат), nil
    CM-->>Caller: []byte, nil
```

## 2. Конкурентное выполнение (семафор)

Диаграмма: [seq-pool-concurrency.mmd](seq-pool-concurrency.mmd)

```mermaid
sequenceDiagram
    title Конкурентное выполнение с семафором (N=3)

    participant G1 as Горутина 1
    participant G2 as Горутина 2
    participant G3 as Горутина 3
    participant G4 as Горутина 4
    participant Pool as ModulePool
    participant Sem as Semaphore<br/>(capacity=3, tokens=3)

    Note over Sem: Начальное состояние:<br/>3 токена доступно

    G1->>Pool: Execute(action)
    Pool->>Sem: <-sem
    Sem-->>Pool: токен (осталось: 2)
    Note over G1: Слот 1 занят

    G2->>Pool: Execute(action)
    Pool->>Sem: <-sem
    Sem-->>Pool: токен (осталось: 1)
    Note over G2: Слот 2 занят

    G3->>Pool: Execute(action)
    Pool->>Sem: <-sem
    Sem-->>Pool: токен (осталось: 0)
    Note over G3: Слот 3 занят

    G4->>Pool: Execute(action)
    Pool->>Sem: <-sem
    Note over G4, Sem: Канал пуст — G4 заблокирована

    G1->>Pool: Выполнение завершено
    Pool->>Sem: sem <- struct{} (возврат)
    Note over Sem: Токенов: 1

    Sem-->>Pool: токен для G4
    Note over G4: G4 разблокирована,<br/>начинает выполнение

    G2->>Pool: Выполнение завершено
    Pool->>Sem: sem <- struct{} (возврат)

    G3->>Pool: Выполнение завершено
    Pool->>Sem: sem <- struct{} (возврат)

    G4->>Pool: Выполнение завершено
    Pool->>Sem: sem <- struct{} (возврат)

    Note over Sem: Все 3 токена возвращены
```

## 3. Жизненный цикл пула

Диаграмма: [seq-pool-lifecycle.mmd](seq-pool-lifecycle.mmd)

```mermaid
sequenceDiagram
    title Жизненный цикл ModulePool

    participant Loader as Loader<br/>(adapter/loader.go)
    participant RT as Runtime<br/>(runtime/runtime.go)
    participant CM as CompiledModule<br/>(runtime/module.go)
    participant Pool as ModulePool<br/>(runtime/pool.go)
    participant Sem as Semaphore

    Note over Loader, Sem: Фаза 1: Создание пула при загрузке плагина

    Loader->>RT: CompileModule(wasmBytes)
    RT-->>Loader: CompiledModule

    Loader->>CM: CallMeta(ctx)
    CM-->>Loader: PluginMeta

    Loader->>CM: CallConfigure(ctx, config)
    CM-->>Loader: OK

    Loader->>CM: EnablePool(poolConfig)
    CM->>Pool: NewModulePool(cm, cfg)
    Pool->>Pool: size = cfg.MaxConcurrency ?? 8
    Pool->>Sem: make(chan struct{}, size)
    Pool->>Sem: Заполняет size токенами

    Note right of Pool: Пул готов к работе

    Note over Loader, Sem: Фаза 2: Работа (многократные вызовы Execute)

    loop Каждый входящий запрос
        Loader->>CM: RunActionWithConfig(ctx, action, input, config)
        CM->>Pool: Execute(ctx, action, input, config)
        Pool->>Sem: <-sem (захват)
        Note over Pool: InstantiateModule → выполнение
        Pool->>Sem: sem <- {} (возврат)
        Pool-->>CM: result
        CM-->>Loader: result
    end

    Note over Loader, Sem: Фаза 3: Остановка (Unload / Reload / Shutdown)

    Loader->>Loader: drainPlugin(lp, pluginID)
    Note right of Loader: Ждёт завершения activeRequests<br/>таймаут: 10 секунд

    Loader->>CM: Close(ctx)
    CM->>Pool: Close()
    Pool->>Pool: closed.Store(true)
    Pool->>Sem: Забирает все N токенов
    Note right of Pool: Дожидается завершения<br/>всех текущих Execute()

    CM->>RT: compiled.Close(ctx)
    Note right of RT: Освобождение<br/>скомпилированного модуля
```

## 4. Graceful shutdown при обновлении плагина

Диаграмма: [seq-pool-reload.mmd](seq-pool-reload.mmd)

```mermaid
sequenceDiagram
    title Graceful Shutdown пула при обновлении плагина

    participant Req as Активные запросы
    participant Loader as Loader
    participant OldPool as Старый ModulePool
    participant OldCM as Старый CompiledModule
    participant NewCM as Новый CompiledModule
    participant NewPool as Новый ModulePool

    Note over Loader: ReloadPluginFromBytes()

    Loader->>Loader: old.draining.Store(true)
    Note right of Loader: Новые запросы к старому<br/>плагину отклоняются

    rect rgb(240, 255, 240)
        Note over Loader, NewPool: Загрузка нового модуля

        Loader->>NewCM: CompileModule(newWasmBytes)
        Loader->>NewCM: CallMeta() + CallConfigure()
        Loader->>NewCM: EnablePool(poolConfig)
        NewCM->>NewPool: NewModulePool(cm, cfg)
        Note right of NewPool: Новый пул готов,<br/>запросы идут сюда
    end

    Loader->>Loader: plugins[id] = newPlugin
    Note right of Loader: Новые запросы идут<br/>в новый пул

    rect rgb(255, 240, 240)
        Note over Req, OldPool: Дренирование старого пула

        Req->>OldPool: Execute() (всё ещё выполняется)
        Note over Loader: drainPlugin(): ждёт<br/>activeRequests == 0

        alt Все запросы завершились
            Req-->>OldPool: done
            OldPool-->>Loader: drained
            Note over Loader: Плагин слит мгновенно
        else Таймаут 10 секунд
            Note over Loader: Force close
        end
    end

    Loader->>OldPool: Close()
    OldPool->>OldPool: Забирает все токены из семафора

    Loader->>OldCM: Close(ctx)
    Note over OldCM: Память освобождена
```

## Конфигурация

| Параметр | По умолчанию | Описание |
|---|---|---|
| `PoolMaxConcurrency` | `8` | Максимум одновременных выполнений на один плагин |
| `PoolMaxConcurrency = -1` | — | Безлимитный режим: семафор отключён, `sem = nil` |
| `PoolMaxConcurrency = 0` | `8` | Используется значение по умолчанию |

Параметр задаётся в `Config.PoolMaxConcurrency` при создании `Runtime`.

## Почему не пул экземпляров

WASM-модули в wazero с AOT-компиляцией инстанцируются очень быстро (доли миллисекунды). Держать заранее созданные экземпляры невыгодно:

- **Каждый экземпляр потребляет память** (линейная память WASM, стек, таблицы).
- **Нет общего состояния** — плагины работают как чистые функции (stdin -> stdout), повторное использование экземпляра не даёт преимуществ.
- **Изоляция** — каждый вызов получает чистое окружение без побочных эффектов от предыдущих вызовов.

## Статистика

Пул собирает метрики для мониторинга:

| Метрика | Описание |
|---|---|
| `PoolSize` | Размер пула (макс. одновременных выполнений) |
| `ActiveExecutions` | Сколько выполняется прямо сейчас |
| `TotalExecutions` | Всего выполнений за время жизни пула |
| `AvgWaitTimeMs` | Среднее время ожидания свободного слота (мс) |
| `AvgInstantiateMs` | Среднее время инстанцирования WASM модуля (мс) |

Если `AvgWaitTimeMs` растёт — стоит увеличить `PoolMaxConcurrency`. Если нагрузка невелика — можно уменьшить для экономии ресурсов.
