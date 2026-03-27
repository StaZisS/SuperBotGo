//go:build wasip1

package wasmplugin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"math"
)

// ---------------------------------------------------------------------------
// WASM imports — SQL host functions
// ---------------------------------------------------------------------------

//go:wasmimport env sql_open
func _sql_open(ptr uint32, length uint32) uint64

//go:wasmimport env sql_close
func _sql_close(ptr uint32, length uint32) uint64

//go:wasmimport env sql_exec
func _sql_exec(ptr uint32, length uint32) uint64

//go:wasmimport env sql_query
func _sql_query(ptr uint32, length uint32) uint64

//go:wasmimport env sql_next
func _sql_next(ptr uint32, length uint32) uint64

//go:wasmimport env sql_rows_close
func _sql_rows_close(ptr uint32, length uint32) uint64

//go:wasmimport env sql_begin
func _sql_begin(ptr uint32, length uint32) uint64

//go:wasmimport env sql_end
func _sql_end(ptr uint32, length uint32) uint64

// ---------------------------------------------------------------------------
// Request / response types
// ---------------------------------------------------------------------------

type sqlOpenReq struct {
	Name string `msgpack:"name,omitempty"`
}

type sqlOpenResp struct {
	Handle uint32 `msgpack:"handle"`
}

type sqlCloseReq struct {
	Handle uint32 `msgpack:"handle"`
}

type sqlExecReq struct {
	Handle uint32 `msgpack:"handle"`
	SQL    string `msgpack:"sql"`
	Args   []any  `msgpack:"args,omitempty"`
}

type sqlExecResp struct {
	LastID   int64 `msgpack:"last_id"`
	Affected int64 `msgpack:"affected"`
}

type sqlQueryReq struct {
	Handle uint32 `msgpack:"handle"`
	SQL    string `msgpack:"sql"`
	Args   []any  `msgpack:"args,omitempty"`
}

type sqlQueryResp struct {
	Cursor  uint32   `msgpack:"cursor"`
	Columns []string `msgpack:"columns"`
}

type sqlNextReq struct {
	Cursor uint32 `msgpack:"cursor"`
}

type sqlNextResp struct {
	Row  []any `msgpack:"row,omitempty"`
	Done bool  `msgpack:"done"`
}

type sqlRowsCloseReq struct {
	Cursor uint32 `msgpack:"cursor"`
}

type sqlBeginReq struct {
	Handle    uint32 `msgpack:"handle"`
	ReadOnly  bool   `msgpack:"read_only,omitempty"`
	Isolation string `msgpack:"isolation,omitempty"`
}

type sqlBeginResp struct {
	TX uint32 `msgpack:"tx"`
}

type sqlEndReq struct {
	TX     uint32 `msgpack:"tx"`
	Commit bool   `msgpack:"commit"`
}

// ---------------------------------------------------------------------------
// Driver registration
// ---------------------------------------------------------------------------

func init() {
	sql.Register("superbot", &wasmDriver{})
}

// ---------------------------------------------------------------------------
// driver.Driver
// ---------------------------------------------------------------------------

type wasmDriver struct{}

func (d *wasmDriver) Open(name string) (driver.Conn, error) {
	if name == "" {
		name = "default"
	}
	var resp sqlOpenResp
	if err := callHostWithResult(_sql_open, sqlOpenReq{Name: name}, &resp); err != nil {
		return nil, err
	}
	return &wasmConn{handle: resp.Handle}, nil
}

// ---------------------------------------------------------------------------
// driver.Conn + driver.ConnBeginTx + driver.QueryerContext + driver.ExecerContext
// ---------------------------------------------------------------------------

type wasmConn struct {
	handle uint32
	closed bool
}

func (c *wasmConn) Prepare(query string) (driver.Stmt, error) {
	return &wasmStmt{conn: c, query: query}, nil
}

func (c *wasmConn) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return callHostWithResult(_sql_close, sqlCloseReq{Handle: c.handle}, nil)
}

func (c *wasmConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *wasmConn) BeginTx(_ context.Context, opts driver.TxOptions) (driver.Tx, error) {
	req := sqlBeginReq{
		Handle:   c.handle,
		ReadOnly: opts.ReadOnly,
	}
	switch sql.IsolationLevel(opts.Isolation) {
	case sql.LevelReadCommitted:
		req.Isolation = "read_committed"
	case sql.LevelRepeatableRead:
		req.Isolation = "repeatable_read"
	case sql.LevelSerializable:
		req.Isolation = "serializable"
	}

	var resp sqlBeginResp
	if err := callHostWithResult(_sql_begin, req, &resp); err != nil {
		return nil, err
	}
	return &wasmTx{txHandle: resp.TX, connHandle: c.handle}, nil
}

func (c *wasmConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	allocReset()
	var resp sqlQueryResp
	if err := callHostWithResult(_sql_query, sqlQueryReq{
		Handle: c.handle,
		SQL:    query,
		Args:   namedToArgs(args),
	}, &resp); err != nil {
		return nil, err
	}
	return &wasmRows{cursor: resp.Cursor, columns: resp.Columns}, nil
}

func (c *wasmConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	allocReset()
	var resp sqlExecResp
	if err := callHostWithResult(_sql_exec, sqlExecReq{
		Handle: c.handle,
		SQL:    query,
		Args:   namedToArgs(args),
	}, &resp); err != nil {
		return nil, err
	}
	return &wasmResult{lastID: resp.LastID, affected: resp.Affected}, nil
}

// ---------------------------------------------------------------------------
// driver.Tx
// ---------------------------------------------------------------------------

type wasmTx struct {
	txHandle   uint32
	connHandle uint32
	done       bool
}

func (t *wasmTx) Commit() error {
	if t.done {
		return fmt.Errorf("transaction already completed")
	}
	t.done = true
	return callHostWithResult(_sql_end, sqlEndReq{TX: t.txHandle, Commit: true}, nil)
}

func (t *wasmTx) Rollback() error {
	if t.done {
		return fmt.Errorf("transaction already completed")
	}
	t.done = true
	return callHostWithResult(_sql_end, sqlEndReq{TX: t.txHandle, Commit: false}, nil)
}

// ---------------------------------------------------------------------------
// driver.Rows
// ---------------------------------------------------------------------------

type wasmRows struct {
	cursor  uint32
	columns []string
	closed  bool
}

func (r *wasmRows) Columns() []string {
	return r.columns
}

func (r *wasmRows) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return callHostWithResult(_sql_rows_close, sqlRowsCloseReq{Cursor: r.cursor}, nil)
}

func (r *wasmRows) Next(dest []driver.Value) error {
	if r.closed {
		return io.EOF
	}

	allocReset()

	var resp sqlNextResp
	if err := callHostWithResult(_sql_next, sqlNextReq{Cursor: r.cursor}, &resp); err != nil {
		return err
	}
	if resp.Done {
		r.closed = true
		return io.EOF
	}

	for i, v := range resp.Row {
		if i >= len(dest) {
			break
		}
		dest[i] = normalizeDriverValue(v)
	}
	return nil
}

// ---------------------------------------------------------------------------
// driver.Result
// ---------------------------------------------------------------------------

type wasmResult struct {
	lastID   int64
	affected int64
}

func (r *wasmResult) LastInsertId() (int64, error) { return r.lastID, nil }
func (r *wasmResult) RowsAffected() (int64, error) { return r.affected, nil }

// ---------------------------------------------------------------------------
// driver.Stmt (thin wrapper for Prepare support)
// ---------------------------------------------------------------------------

type wasmStmt struct {
	conn  *wasmConn
	query string
}

func (s *wasmStmt) Close() error  { return nil }
func (s *wasmStmt) NumInput() int { return -1 }
func (s *wasmStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.conn.ExecContext(context.Background(), s.query, valuesToNamed(args))
}
func (s *wasmStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.conn.QueryContext(context.Background(), s.query, valuesToNamed(args))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func namedToArgs(named []driver.NamedValue) []any {
	if len(named) == 0 {
		return nil
	}
	args := make([]any, len(named))
	for i, nv := range named {
		args[i] = nv.Value
	}
	return args
}

func valuesToNamed(vals []driver.Value) []driver.NamedValue {
	named := make([]driver.NamedValue, len(vals))
	for i, v := range vals {
		named[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return named
}

// normalizeDriverValue converts msgpack-deserialized values to types that
// database/sql can scan.
func normalizeDriverValue(v any) driver.Value {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		// If it's a whole number, convert to int64.
		if val == math.Trunc(val) && val >= math.MinInt64 && val <= math.MaxInt64 {
			return int64(val)
		}
		return val
	case string:
		return val
	case []any:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
