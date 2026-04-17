-- +goose Up
CREATE TABLE wasm_event_queue (
    id               BIGSERIAL    PRIMARY KEY,
    topic            TEXT         NOT NULL,
    payload          JSONB        NOT NULL,
    source_plugin_id VARCHAR(255) NOT NULL DEFAULT '',
    status           TEXT         NOT NULL DEFAULT 'pending',
    retry_count      INTEGER      NOT NULL DEFAULT 0,
    available_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    locked_until     TIMESTAMPTZ,
    claimed_by       VARCHAR(64),
    last_error       TEXT,
    dead_lettered_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_wasm_event_queue_pending
    ON wasm_event_queue (status, available_at, id);

CREATE INDEX idx_wasm_event_queue_processing
    ON wasm_event_queue (status, locked_until, id);

CREATE INDEX idx_wasm_event_queue_dead
    ON wasm_event_queue (status, dead_lettered_at, id);

-- +goose Down
DROP TABLE IF EXISTS wasm_event_queue;
