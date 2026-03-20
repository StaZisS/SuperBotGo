-- +goose Up
CREATE TABLE chat_bindings (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL,
    chat_reference_id BIGINT NOT NULL,
    CONSTRAINT fk_chat_bindings_project FOREIGN KEY (project_id) REFERENCES projects(id),
    CONSTRAINT fk_chat_bindings_chat_reference FOREIGN KEY (chat_reference_id) REFERENCES chat_references(id)
);

CREATE INDEX idx_chat_bindings_project_id ON chat_bindings(project_id);

-- +goose Down
DROP TABLE IF EXISTS chat_bindings;
