package hostapi

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tetratelabs/wazero/api"
	"github.com/vmihailenco/msgpack/v5"
)

const wasmSQLMaxTimeout = 4 * time.Second

type sqlCallContext struct {
	pluginID string
	execID   string
}

// sqlPreamble extracts common fields, checks permissions, reads and unmarshals the payload,
// and verifies that sqlStore is available. Returns nil request on failure (error already written to stack).
func sqlPreamble[T any](h *HostAPI, ctx context.Context, mod api.Module, stack []uint64) (*sqlCallContext, *T) {
	sc := &sqlCallContext{
		pluginID: pluginIDFromContext(ctx),
		execID:   TraceIDFromContext(ctx),
	}
	offset := uint32(stack[0])
	length := uint32(stack[1])

	if err := h.perms.CheckPermission(sc.pluginID, "sql"); err != nil {
		returnError(ctx, mod, stack, err)
		return nil, nil
	}

	data, err := readPayload(mod, offset, length)
	if err != nil {
		returnError(ctx, mod, stack, err)
		return nil, nil
	}

	var req T
	if err := msgpack.Unmarshal(data, &req); err != nil {
		returnError(ctx, mod, stack, err)
		return nil, nil
	}

	if h.sqlStore == nil {
		returnError(ctx, mod, stack, errDepNotAvailable("SQLStore"))
		return nil, nil
	}

	return sc, &req
}

// ── sql_open ──────────────────────────────────────────────────────────

func (h *HostAPI) sqlOpenFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		sc, _ := sqlPreamble[sqlOpenRequest](h, ctx, mod, stack)
		if sc == nil {
			return
		}

		sqlCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmSQLMaxTimeout))
		defer cancel()

		pool, err := h.sqlStore.getOrCreatePool(sqlCtx, sc.pluginID)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		conn, err := pool.Acquire(sqlCtx)
		if err != nil {
			returnError(ctx, mod, stack, fmt.Errorf("acquire connection: %w", err))
			return
		}

		handle, err := h.sqlStore.Alloc(sc.pluginID, sc.execID, &sqlHandle{
			kind: handleConn,
			conn: conn,
		})
		if err != nil {
			conn.Release()
			returnError(ctx, mod, stack, err)
			return
		}

		writeResult(ctx, mod, stack, sqlOpenResponse{Handle: handle})
	}
}

// ── sql_close ─────────────────────────────────────────────────────────

func (h *HostAPI) sqlCloseFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		sc, req := sqlPreamble[sqlCloseRequest](h, ctx, mod, stack)
		if sc == nil {
			return
		}

		sh, err := h.sqlStore.Remove(sc.pluginID, sc.execID, req.Handle)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if sh.kind == handleConn && sh.conn != nil {
			sh.conn.Release()
		}

		writeResult(ctx, mod, stack, sqlCloseResponse{OK: true})
	}
}

// ── sql_exec ──────────────────────────────────────────────────────────

func (h *HostAPI) sqlExecFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		sc, req := sqlPreamble[sqlExecRequest](h, ctx, mod, stack)
		if sc == nil {
			return
		}

		sh, err := h.sqlStore.Get(sc.pluginID, sc.execID, req.Handle)
		if err != nil {
			returnError(ctx, mod, stack, err)
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
			returnError(ctx, mod, stack, fmt.Errorf("handle %d is not a connection or transaction", req.Handle))
			return
		}
		if err != nil {
			returnError(ctx, mod, stack, fmt.Errorf("sql exec: %w", err))
			return
		}

		writeResult(ctx, mod, stack, sqlExecResponse{
			Affected: tag.RowsAffected(),
		})
	}
}

// ── sql_query ─────────────────────────────────────────────────────────

func (h *HostAPI) sqlQueryFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		sc, req := sqlPreamble[sqlQueryRequest](h, ctx, mod, stack)
		if sc == nil {
			return
		}

		sh, err := h.sqlStore.Get(sc.pluginID, sc.execID, req.Handle)
		if err != nil {
			returnError(ctx, mod, stack, err)
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
			returnError(ctx, mod, stack, fmt.Errorf("handle %d is not a connection or transaction", req.Handle))
			return
		}
		if err != nil {
			returnError(ctx, mod, stack, fmt.Errorf("sql query: %w", err))
			return
		}

		cols := make([]string, len(rows.FieldDescriptions()))
		for i, fd := range rows.FieldDescriptions() {
			cols[i] = fd.Name
		}

		cursorID, err := h.sqlStore.Alloc(sc.pluginID, sc.execID, &sqlHandle{
			kind:       handleRows,
			rows:       rows,
			cols:       cols,
			connHandle: req.Handle,
		})
		if err != nil {
			rows.Close()
			returnError(ctx, mod, stack, err)
			return
		}

		writeResult(ctx, mod, stack, sqlQueryResponse{
			Cursor:  cursorID,
			Columns: cols,
		})
	}
}

// ── sql_next ──────────────────────────────────────────────────────────

func (h *HostAPI) sqlNextFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		sc, req := sqlPreamble[sqlNextRequest](h, ctx, mod, stack)
		if sc == nil {
			return
		}

		sh, err := h.sqlStore.Get(sc.pluginID, sc.execID, req.Cursor)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}
		if sh.kind != handleRows {
			returnError(ctx, mod, stack, fmt.Errorf("handle %d is not a cursor", req.Cursor))
			return
		}

		if !sh.rows.Next() {
			if err := sh.rows.Err(); err != nil {
				returnError(ctx, mod, stack, fmt.Errorf("sql next: %w", err))
				return
			}
			sh.rows.Close()
			_, _ = h.sqlStore.Remove(sc.pluginID, sc.execID, req.Cursor)
			writeResult(ctx, mod, stack, sqlNextResponse{Done: true})
			return
		}

		values, err := sh.rows.Values()
		if err != nil {
			returnError(ctx, mod, stack, fmt.Errorf("sql row values: %w", err))
			return
		}

		row := make([]any, len(values))
		for i, v := range values {
			row[i] = normalizeValue(v)
		}

		writeResult(ctx, mod, stack, sqlNextResponse{Row: row})
	}
}

// ── sql_rows_close ────────────────────────────────────────────────────

func (h *HostAPI) sqlRowsCloseFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		sc, req := sqlPreamble[sqlRowsCloseRequest](h, ctx, mod, stack)
		if sc == nil {
			return
		}

		sh, err := h.sqlStore.Remove(sc.pluginID, sc.execID, req.Cursor)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}
		if sh.kind == handleRows && sh.rows != nil {
			sh.rows.Close()
		}

		writeResult(ctx, mod, stack, sqlRowsCloseResponse{OK: true})
	}
}

// ── sql_begin ─────────────────────────────────────────────────────────

func (h *HostAPI) sqlBeginFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		sc, req := sqlPreamble[sqlBeginRequest](h, ctx, mod, stack)
		if sc == nil {
			return
		}

		sh, err := h.sqlStore.Get(sc.pluginID, sc.execID, req.Handle)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}
		if sh.kind != handleConn {
			returnError(ctx, mod, stack, fmt.Errorf("handle %d is not a connection", req.Handle))
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
			returnError(ctx, mod, stack, fmt.Errorf("begin tx: %w", err))
			return
		}

		txHandle, err := h.sqlStore.Alloc(sc.pluginID, sc.execID, &sqlHandle{
			kind:       handleTx,
			tx:         tx,
			connHandle: req.Handle,
		})
		if err != nil {
			_ = tx.Rollback(ctx)
			returnError(ctx, mod, stack, err)
			return
		}

		writeResult(ctx, mod, stack, sqlBeginResponse{TX: txHandle})
	}
}

// ── sql_end ───────────────────────────────────────────────────────────

func (h *HostAPI) sqlEndFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		sc, req := sqlPreamble[sqlEndRequest](h, ctx, mod, stack)
		if sc == nil {
			return
		}

		sh, err := h.sqlStore.Remove(sc.pluginID, sc.execID, req.TX)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}
		if sh.kind != handleTx {
			returnError(ctx, mod, stack, fmt.Errorf("handle %d is not a transaction", req.TX))
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
			returnError(ctx, mod, stack, fmt.Errorf("sql end: %w", err))
			return
		}

		writeResult(ctx, mod, stack, sqlEndResponse{OK: true})
	}
}
