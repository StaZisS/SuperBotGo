-- +goose Up
CREATE TABLE authz_outbox (
    id            BIGSERIAL     PRIMARY KEY,
    operation     VARCHAR(30)   NOT NULL
                  CHECK (operation IN (
                      'TOUCH', 'DELETE', 'DELETE_BY_OBJECT',
                      'DELETE_BY_SUBJECT', 'REPLACE'
                  )),
    payload       JSONB         NOT NULL,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    processed_at  TIMESTAMPTZ,
    attempts      INT           NOT NULL DEFAULT 0,
    last_error    TEXT,
    locked_until  TIMESTAMPTZ
);

CREATE INDEX idx_authz_outbox_pending
    ON authz_outbox (id)
    WHERE processed_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS authz_outbox;
