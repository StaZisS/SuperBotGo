package hostapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tetratelabs/wazero/api"
)

const wasmSQLMaxTimeout = 4 * time.Second

// ── request/response types ────────────────────────────────────────────

type sqlOpenRequest struct{}

type sqlOpenResponse struct {
	Handle uint32 `json:"handle" msgpack:"handle"`
}

type sqlCloseRequest struct {
	Handle uint32 `json:"handle" msgpack:"handle"`
}

type sqlCloseResponse struct {
	OK bool `json:"ok" msgpack:"ok"`
}

type sqlExecRequest struct {
	Handle uint32 `json:"handle" msgpack:"handle"`
	SQL    string `json:"sql" msgpack:"sql"`
	Args   []any  `json:"args,omitempty" msgpack:"args,omitempty"`
}

type sqlExecResponse struct {
	LastID   int64 `json:"last_id" msgpack:"last_id"`
	Affected int64 `json:"affected" msgpack:"affected"`
}

type sqlQueryRequest struct {
	Handle uint32 `json:"handle" msgpack:"handle"`
	SQL    string `json:"sql" msgpack:"sql"`
	Args   []any  `json:"args,omitempty" msgpack:"args,omitempty"`
}

type sqlQueryResponse struct {
	Cursor  uint32   `json:"cursor" msgpack:"cursor"`
	Columns []string `json:"columns" msgpack:"columns"`
}

type sqlNextRequest struct {
	Cursor uint32 `json:"cursor" msgpack:"cursor"`
}

type sqlNextResponse struct {
	Row  []any `json:"row,omitempty" msgpack:"row,omitempty"`
	Done bool  `json:"done" msgpack:"done"`
}

type sqlRowsCloseRequest struct {
	Cursor uint32 `json:"cursor" msgpack:"cursor"`
}

type sqlRowsCloseResponse struct {
	OK bool `json:"ok" msgpack:"ok"`
}

type sqlBeginRequest struct {
	Handle    uint32 `json:"handle" msgpack:"handle"`
	ReadOnly  bool   `json:"read_only,omitempty" msgpack:"read_only,omitempty"`
	Isolation string `json:"isolation,omitempty" msgpack:"isolation,omitempty"`
}

type sqlBeginResponse struct {
	TX uint32 `json:"tx" msgpack:"tx"`
}

type sqlEndRequest struct {
	TX     uint32 `json:"tx" msgpack:"tx"`
	Commit bool   `json:"commit" msgpack:"commit"`
}

type sqlEndResponse struct {
	OK bool `json:"ok" msgpack:"ok"`
}

// ── sql_open ──────────────────────────────────────────────────────────

func (h *HostAPI) sqlOpenFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		execID := TraceIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "sql"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		_, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.sqlStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("SQLStore"), enc)
			return
		}

		sqlCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmSQLMaxTimeout))
		defer cancel()

		pool, err := h.sqlStore.getOrCreatePool(sqlCtx, pluginID)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		conn, err := pool.Acquire(sqlCtx)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("acquire connection: %w", err), enc)
			return
		}

		handle, err := h.sqlStore.Alloc(pluginID, execID, &sqlHandle{
			kind: handleConn,
			conn: conn,
		})
		if err != nil {
			conn.Release()
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		writeEncodedResult(ctx, mod, stack, sqlOpenResponse{Handle: handle}, enc)
	}
}

// ── sql_close ─────────────────────────────────────────────────────────

func (h *HostAPI) sqlCloseFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		execID := TraceIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "sql"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req sqlCloseRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.sqlStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("SQLStore"), enc)
			return
		}

		sh, err := h.sqlStore.Remove(pluginID, execID, req.Handle)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if sh.kind == handleConn && sh.conn != nil {
			sh.conn.Release()
		}

		writeEncodedResult(ctx, mod, stack, sqlCloseResponse{OK: true}, enc)
	}
}

// ── sql_exec ──────────────────────────────────────────────────────────

func (h *HostAPI) sqlExecFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		execID := TraceIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "sql"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req sqlExecRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.sqlStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("SQLStore"), enc)
			return
		}

		sh, err := h.sqlStore.Get(pluginID, execID, req.Handle)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		sqlCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmSQLMaxTimeout))
		defer cancel()

		var tag pgconn.CommandTag
		switch sh.kind {
		case handleConn:
			tag, err = sh.conn.Exec(sqlCtx, req.SQL, req.Args...)
		case handleTx:
			tag, err = sh.tx.Exec(sqlCtx, req.SQL, req.Args...)
		default:
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("handle %d is not a connection or transaction", req.Handle), enc)
			return
		}
		if err != nil {
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("sql exec: %w", err), enc)
			return
		}

		writeEncodedResult(ctx, mod, stack, sqlExecResponse{
			Affected: tag.RowsAffected(),
		}, enc)
	}
}

// ── sql_query ─────────────────────────────────────────────────────────

func (h *HostAPI) sqlQueryFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		execID := TraceIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "sql"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req sqlQueryRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.sqlStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("SQLStore"), enc)
			return
		}

		sh, err := h.sqlStore.Get(pluginID, execID, req.Handle)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		sqlCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmSQLMaxTimeout))
		defer cancel()

		var rows pgx.Rows
		switch sh.kind {
		case handleConn:
			rows, err = sh.conn.Query(sqlCtx, req.SQL, req.Args...)
		case handleTx:
			rows, err = sh.tx.Query(sqlCtx, req.SQL, req.Args...)
		default:
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("handle %d is not a connection or transaction", req.Handle), enc)
			return
		}
		if err != nil {
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("sql query: %w", err), enc)
			return
		}

		cols := make([]string, len(rows.FieldDescriptions()))
		for i, fd := range rows.FieldDescriptions() {
			cols[i] = fd.Name
		}

		cursorID, err := h.sqlStore.Alloc(pluginID, execID, &sqlHandle{
			kind:       handleRows,
			rows:       rows,
			cols:       cols,
			connHandle: req.Handle,
		})
		if err != nil {
			rows.Close()
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		writeEncodedResult(ctx, mod, stack, sqlQueryResponse{
			Cursor:  cursorID,
			Columns: cols,
		}, enc)
	}
}

// ── sql_next ──────────────────────────────────────────────────────────

func (h *HostAPI) sqlNextFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		execID := TraceIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "sql"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req sqlNextRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.sqlStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("SQLStore"), enc)
			return
		}

		sh, err := h.sqlStore.Get(pluginID, execID, req.Cursor)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}
		if sh.kind != handleRows {
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("handle %d is not a cursor", req.Cursor), enc)
			return
		}

		if !sh.rows.Next() {
			if err := sh.rows.Err(); err != nil {
				returnErrorEnc(ctx, mod, stack, fmt.Errorf("sql next: %w", err), enc)
				return
			}
			// Auto-close and remove the cursor handle.
			sh.rows.Close()
			_, _ = h.sqlStore.Remove(pluginID, execID, req.Cursor)
			writeEncodedResult(ctx, mod, stack, sqlNextResponse{Done: true}, enc)
			return
		}

		values, err := sh.rows.Values()
		if err != nil {
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("sql row values: %w", err), enc)
			return
		}

		row := make([]any, len(values))
		for i, v := range values {
			row[i] = normalizeValue(v)
		}

		writeEncodedResult(ctx, mod, stack, sqlNextResponse{Row: row}, enc)
	}
}

// ── sql_rows_close ────────────────────────────────────────────────────

func (h *HostAPI) sqlRowsCloseFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		execID := TraceIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "sql"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req sqlRowsCloseRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.sqlStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("SQLStore"), enc)
			return
		}

		sh, err := h.sqlStore.Remove(pluginID, execID, req.Cursor)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}
		if sh.kind == handleRows && sh.rows != nil {
			sh.rows.Close()
		}

		writeEncodedResult(ctx, mod, stack, sqlRowsCloseResponse{OK: true}, enc)
	}
}

// ── sql_begin ─────────────────────────────────────────────────────────

func (h *HostAPI) sqlBeginFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		execID := TraceIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "sql"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req sqlBeginRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.sqlStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("SQLStore"), enc)
			return
		}

		sh, err := h.sqlStore.Get(pluginID, execID, req.Handle)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}
		if sh.kind != handleConn {
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("handle %d is not a connection", req.Handle), enc)
			return
		}

		sqlCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmSQLMaxTimeout))
		defer cancel()

		txOpts := pgx.TxOptions{
			AccessMode: pgx.ReadWrite,
		}
		if req.ReadOnly {
			txOpts.AccessMode = pgx.ReadOnly
		}
		switch req.Isolation {
		case "read_committed":
			txOpts.IsoLevel = pgx.ReadCommitted
		case "repeatable_read":
			txOpts.IsoLevel = pgx.RepeatableRead
		case "serializable":
			txOpts.IsoLevel = pgx.Serializable
		}

		tx, err := sh.conn.BeginTx(sqlCtx, txOpts)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("begin tx: %w", err), enc)
			return
		}

		txHandle, err := h.sqlStore.Alloc(pluginID, execID, &sqlHandle{
			kind:       handleTx,
			tx:         tx,
			connHandle: req.Handle,
		})
		if err != nil {
			_ = tx.Rollback(ctx)
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		writeEncodedResult(ctx, mod, stack, sqlBeginResponse{TX: txHandle}, enc)
	}
}

// ── sql_end ───────────────────────────────────────────────────────────

func (h *HostAPI) sqlEndFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		execID := TraceIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "sql"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req sqlEndRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.sqlStore == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("SQLStore"), enc)
			return
		}

		sh, err := h.sqlStore.Remove(pluginID, execID, req.TX)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}
		if sh.kind != handleTx {
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("handle %d is not a transaction", req.TX), enc)
			return
		}

		sqlCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmSQLMaxTimeout))
		defer cancel()

		if req.Commit {
			err = sh.tx.Commit(sqlCtx)
		} else {
			err = sh.tx.Rollback(sqlCtx)
		}
		if err != nil {
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("sql end: %w", err), enc)
			return
		}

		writeEncodedResult(ctx, mod, stack, sqlEndResponse{OK: true}, enc)
	}
}

// ── value normalization ───────────────────────────────────────────────

// normalizeValue converts pgx types to JSON/msgpack-safe types.
func normalizeValue(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case bool:
		return val
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	case uint8:
		return int64(val)
	case uint16:
		return int64(val)
	case uint32:
		return int64(val)
	case uint64:
		if val <= math.MaxInt64 {
			return int64(val)
		}
		return float64(val)
	case float32:
		return float64(val)
	case float64:
		return val
	case string:
		return val
	case []byte:
		return base64.StdEncoding.EncodeToString(val)
	case time.Time:
		return val.Format(time.RFC3339Nano)
	case [16]byte:
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			val[0:4], val[4:6], val[6:8], val[8:10], val[10:16])
	case fmt.Stringer:
		return val.String()
	default:
		slog.Debug("sql: unknown value type, converting to string", "type", fmt.Sprintf("%T", v))
		return fmt.Sprintf("%v", v)
	}
}
