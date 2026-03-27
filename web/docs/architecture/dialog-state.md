# Стейт-машина диалогов

Система многошаговых диалогов для мессенджеров. Позволяет строить интерактивные команды
с ветвлениями, пагинацией и валидацией — через декларативный DSL, одинаковый для native- и WASM-плагинов.

## Диаграмма классов

```mermaid
classDiagram
    direction TB

    class Manager {
        -storage DialogStorage
        -commands map~string, *CommandDefinition~
        -handlers map~string, StateHandler~
        -mu RWMutex
        +RegisterCommand(def)
        +UnregisterCommand(name)
        +StartCommand(ctx, userID, chatID, name, locale) Message
        +ProcessInput(ctx, userID, chatID, input, locale) Message, *CommandRequest
        +GetCurrentStepMessage(ctx, userID, locale) *Message, string
        +RelocateDialog(ctx, userID, chatID)
        +CancelCommand(ctx, userID)
        +HasActiveDialog(ctx, userID) bool
        +IsCommandImmediate(name) bool
        +IsPreservesDialog(name) bool
    }

    class DialogStorage {
        <<interface>>
        +Save(ctx, userID, state)
        +Load(ctx, userID) *DialogState
        +Delete(ctx, userID)
    }

    class RedisStorage {
        -client *redis.Client
        -ttl Duration
        +Save(ctx, userID, state)
        +Load(ctx, userID) *DialogState
        +Delete(ctx, userID)
    }

    class DialogState {
        +UserID GlobalUserID
        +ChatID string
        +CommandName string
        +Params OptionMap
        +PageState map~string, int~
        +CreatedAt int64
    }

    class CommandDefinition {
        +Name string
        +Description string
        +Requirements *RoleRequirements
        +Nodes []CommandNode
        +PreservesDialog bool
        +ResolveActiveSteps(params) []StepNode
        +CurrentStep(params) *StepNode
        +IsComplete(params) bool
    }

    class CommandNode {
        <<interface>>
        +commandNode()
    }

    class StepNode {
        +ParamName string
        +MessageBuilder func(StepContext) Message
        +Validate func(UserInput) bool
        +Condition func(OptionMap) bool
        +Pagination *PaginationConfig
    }

    class BranchNode {
        +OnParam string
        +Cases map~string, []CommandNode~
        +Default []CommandNode
    }

    class ConditionalBranchNode {
        +Cases []ConditionalCase
        +Default []CommandNode
    }

    class ConditionalCase {
        +Predicate func(OptionMap) bool
        +Nodes []CommandNode
    }

    class StateHandler {
        <<interface>>
        +CreateNewState(commandName) State
        +RestoreState(ds) State
        +PersistState(s) DialogState
        +ProcessInput(userID, s, input) State, StepOutcome
        +BuildStepMessage(s, locale) Message
    }

    class State {
        <<interface>>
        +IsComplete() bool
        +FinalParams() OptionMap
    }

    class DslStateHandler {
        -command *CommandDefinition
        +CreateNewState(commandName) State
        +RestoreState(ds) State
        +PersistState(s) DialogState
        +ProcessInput(userID, s, input) State, StepOutcome
        +BuildStepMessage(s, locale) Message
    }

    class DslState {
        +Command *CommandDefinition
        +Params OptionMap
        +PageState map~string, int~
        +IsComplete() bool
        +FinalParams() OptionMap
    }

    class StepOutcome {
        +Message Message
        +CommandName string
        +IsComplete bool
        +Params OptionMap
    }

    class StepContext {
        +UserID GlobalUserID
        +Locale string
        +Params OptionMap
        +Page int
    }

    class PaginationConfig {
        +Prompt string
        +Prompts map~string, string~
        +PageSize int
        +PageProvider func(StepContext, int) OptionsPage
    }

    class StateManagerAdapter {
        -mgr *Manager
        +StartCommand(ctx, userID, channelType, chatID, name, locale) *StateResult
        +ProcessInput(ctx, userID, channelType, chatID, input, locale) *StateResult
        +CancelCommand(ctx, userID, channelType)
        +GetCurrentStepMessage(ctx, userID, locale) *Message, string
        +RelocateDialog(ctx, userID, chatID)
        +IsPreservesDialog(name) bool
    }

    %% Relationships
    Manager --> DialogStorage : storage
    Manager --> CommandDefinition : commands
    Manager --> StateHandler : handlers
    RedisStorage ..|> DialogStorage
    RedisStorage ..> DialogState : serialize/deserialize

    CommandDefinition *-- CommandNode : Nodes
    StepNode ..|> CommandNode
    BranchNode ..|> CommandNode
    ConditionalBranchNode ..|> CommandNode
    ConditionalBranchNode *-- ConditionalCase
    BranchNode o-- CommandNode : Cases / Default
    ConditionalCase o-- CommandNode : Nodes

    StepNode --> PaginationConfig : Pagination
    StepNode ..> StepContext : MessageBuilder arg

    DslStateHandler ..|> StateHandler
    DslStateHandler --> CommandDefinition
    DslState ..|> State
    DslState --> CommandDefinition
    DslStateHandler ..> DslState : create / restore
    DslStateHandler ..> StepOutcome : returns from ProcessInput

    StateManagerAdapter --> Manager : mgr
```

## Граф узлов команды

Каждая команда — это дерево из `CommandNode`. При обработке ввода дерево линеаризуется в список
активных шагов (`flattenNodes`) с учётом уже собранных параметров.

```mermaid
graph TD
    subgraph "CommandDefinition (find)"
        S1["StepNode<br/><b>what</b><br/>options: teacher, subject, room"]

        BR["BranchNode<br/>OnParam: <b>what</b>"]

        S1 --> BR

        subgraph "case: teacher"
            S2["StepNode<br/><b>building</b><br/>paginated"]
            S3["StepNode<br/><b>teacher</b><br/>dynamic options"]
            S2 --> S3
        end

        subgraph "case: subject"
            S4["StepNode<br/><b>subject</b><br/>paginated"]
        end

        subgraph "case: room"
            S5["StepNode<br/><b>building</b><br/>paginated"]
            S6["StepNode<br/><b>floor</b><br/>validate: 1-9"]
            CBR["ConditionalBranchNode<br/>building == 3"]
            S7["StepNode<br/><b>wing</b><br/>east / west"]
            S5 --> S6 --> CBR --> S7
        end

        BR --> S2
        BR --> S4
        BR --> S5

        NOTIFY["StepNode<br/><b>notify</b><br/>yes / no<br/>visibleWhen: what != room"]

        S3 --> NOTIFY
        S4 --> NOTIFY
        S7 --> NOTIFY
    end

    classDef step fill:#e8f5e9,stroke:#2e7d32,color:#1b5e20
    classDef branch fill:#fff3e0,stroke:#ef6c00,color:#e65100
    classDef cond fill:#fce4ec,stroke:#c62828,color:#b71c1c
    class S1,S2,S3,S4,S5,S6,S7,NOTIFY step
    class BR branch
    class CBR cond
```

## Жизненный цикл диалога

```mermaid
stateDiagram-v2
    [*] --> Idle

    Idle --> Active : /command (StartCommand)
    Active --> Complete : все шаги заполнены
    Active --> Relocated : /resume из другого чата
    Relocated --> Active : ввод в новом чате
    Active --> Cancelled : новая команда без PreservesDialog
    Active --> Expired : Redis TTL (30 мин)

    Complete --> [*] : Handler вызван, state удалён
    Cancelled --> [*] : state удалён
    Expired --> [*] : state удалён Redis
```

## Хранение состояния

Состояние диалога хранится в Redis с ключом `dialog:state:{GlobalUserID}`.
У каждого пользователя может быть **один** активный диалог.

```
┌──────────────────────────────────────┐
│  Redis key: dialog:state:42          │
│  TTL: 30 min (обновляется при Save)  │
├──────────────────────────────────────┤
│  {                                   │
│    "user_id": 42,                    │
│    "chat_id": "930733076",           │
│    "command_name": "find",           │
│    "params": {                       │
│      "what": "teacher",              │
│      "building": "2"                 │
│    },                                │
│    "page_state": {                   │
│      "building": 1                   │
│    },                                │
│    "created_at": 1711526400          │
│  }                                   │
└──────────────────────────────────────┘
```

### Привязка к чату

- `ChatID` сохраняется при старте команды
- `ProcessInput` игнорирует ввод из чужого чата (тихий пропуск, без ошибки)
- `/resume` переносит диалог в текущий чат через `RelocateDialog`
- Это позволяет начать команду в Telegram и продолжить в Discord (при связанных аккаунтах)

## Обработка ввода (ProcessInput)

```mermaid
flowchart TD
    A[ProcessInput] --> B{Есть DialogState?}
    B -- нет --> Z1[ErrNoActiveDialog]
    B -- да --> C{ChatID совпадает?}
    C -- нет --> Z2["nil, nil, nil<br/>(тихий пропуск)"]
    C -- да --> D[RestoreState]
    D --> E[handler.ProcessInput]
    E --> F{Validate?}
    F -- fail --> G["Params не меняется<br/>повтор шага"]
    F -- ok --> H["Params[step] = input"]
    G --> I[BuildStepMessage]
    H --> I
    I --> J{IsComplete?}
    J -- нет --> K[Save state → Redis]
    J -- да --> L[Delete state → Redis]
    L --> M["return CommandRequest<br/>(→ RouteEvent → Plugin)"]
```
