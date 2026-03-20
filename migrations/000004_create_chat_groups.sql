-- +goose Up
CREATE TABLE chat_groups (
    id BIGSERIAL PRIMARY KEY,
    channel_type VARCHAR(50) NOT NULL,
    root_chat_id BIGINT NOT NULL,
    CONSTRAINT fk_chat_groups_root_chat FOREIGN KEY (root_chat_id) REFERENCES chat_references(id)
);

CREATE TABLE chat_group_children (
    chat_group_id BIGINT NOT NULL,
    chat_reference_id BIGINT NOT NULL,
    CONSTRAINT pk_chat_group_children PRIMARY KEY (chat_group_id, chat_reference_id),
    CONSTRAINT fk_chat_group_children_group FOREIGN KEY (chat_group_id) REFERENCES chat_groups(id),
    CONSTRAINT fk_chat_group_children_chat_ref FOREIGN KEY (chat_reference_id) REFERENCES chat_references(id)
);

-- +goose Down
DROP TABLE IF EXISTS chat_group_children;
DROP TABLE IF EXISTS chat_groups;
