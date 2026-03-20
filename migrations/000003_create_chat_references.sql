-- +goose Up
CREATE TABLE chat_references (
    id BIGSERIAL PRIMARY KEY,
    channel_type VARCHAR(50) NOT NULL,
    platform_chat_id VARCHAR(255) NOT NULL,
    chat_kind VARCHAR(50) NOT NULL,
    parent_chat_id VARCHAR(255),
    title VARCHAR(500)
);

CREATE TABLE chat_reference_metadata (
    chat_reference_id BIGINT NOT NULL,
    meta_key VARCHAR(255) NOT NULL,
    meta_value VARCHAR(1000),
    CONSTRAINT pk_chat_reference_metadata PRIMARY KEY (chat_reference_id, meta_key),
    CONSTRAINT fk_chat_ref_metadata_chat_reference FOREIGN KEY (chat_reference_id) REFERENCES chat_references(id)
);

-- +goose Down
DROP TABLE IF EXISTS chat_reference_metadata;
DROP TABLE IF EXISTS chat_references;
