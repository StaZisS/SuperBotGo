package hostapi

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"math"
	"time"
)

type sqlOpenRequest struct {
	Name string `msgpack:"name,omitempty"` // logical database name; defaults to "default"
}

type sqlOpenResponse struct {
	Handle uint32 `msgpack:"handle"`
}

type sqlCloseRequest struct {
	Handle uint32 `msgpack:"handle"`
}

type sqlCloseResponse struct {
	OK bool `msgpack:"ok"`
}

type sqlExecRequest struct {
	Handle uint32 `msgpack:"handle"`
	SQL    string `msgpack:"sql"`
	Args   []any  `msgpack:"args,omitempty"`
}

type sqlExecResponse struct {
	LastID   int64 `msgpack:"last_id"`
	Affected int64 `msgpack:"affected"`
}

type sqlQueryRequest struct {
	Handle uint32 `msgpack:"handle"`
	SQL    string `msgpack:"sql"`
	Args   []any  `msgpack:"args,omitempty"`
}

type sqlQueryResponse struct {
	Cursor  uint32   `msgpack:"cursor"`
	Columns []string `msgpack:"columns"`
}

type sqlNextRequest struct {
	Cursor uint32 `msgpack:"cursor"`
}

type sqlNextResponse struct {
	Row  []any `msgpack:"row,omitempty"`
	Done bool  `msgpack:"done"`
}

type sqlRowsCloseRequest struct {
	Cursor uint32 `msgpack:"cursor"`
}

type sqlRowsCloseResponse struct {
	OK bool `msgpack:"ok"`
}

type sqlBeginRequest struct {
	Handle    uint32 `msgpack:"handle"`
	ReadOnly  bool   `msgpack:"read_only,omitempty"`
	Isolation string `msgpack:"isolation,omitempty"`
}

type sqlBeginResponse struct {
	TX uint32 `msgpack:"tx"`
}

type sqlEndRequest struct {
	TX     uint32 `msgpack:"tx"`
	Commit bool   `msgpack:"commit"`
}

type sqlEndResponse struct {
	OK bool `msgpack:"ok"`
}

// normalizeValue converts pgx types to msgpack-safe types.
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
