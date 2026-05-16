package runtime

import "SuperBotGo/internal/wasm/protocol"

type PluginIDKey struct{}
type HTTPAuthDataKey struct{}

const MaxSupportedSDKVersion = protocol.MaxSupportedSDKVersion

const ActionMigrate = protocol.ActionMigrate
const ActionReconfigure = protocol.ActionReconfigure
const ActionHandleRPC = protocol.ActionHandleRPC

// Protocol DTO aliases are kept here so existing runtime/adapter call sites can
// move to internal/wasm/protocol gradually.
type PluginMeta = protocol.PluginMeta
type ReconfigureRequest = protocol.ReconfigureRequest
type MigrateRequest = protocol.MigrateRequest
type MigrateResponse = protocol.MigrateResponse
type RPCMethodDef = protocol.RPCMethodDef
type RPCRequest = protocol.RPCRequest
type RPCResponse = protocol.RPCResponse
type DependencyDef = protocol.DependencyDef
type MigrationDef = protocol.MigrationDef
type TriggerDef = protocol.TriggerDef
type OptionDef = protocol.OptionDef
type RequirementDef = protocol.RequirementDef
type NodeDef = protocol.NodeDef
type BlockDef = protocol.BlockDef
type ConditionDef = protocol.ConditionDef
type PaginationNodeDef = protocol.PaginationNodeDef
type CondCaseDef = protocol.CondCaseDef
type StepCallbackRequest = protocol.StepCallbackRequest
type StepCallbackResponse = protocol.StepCallbackResponse
