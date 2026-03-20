-- +goose Up

-- ============================================================
-- Таблица авторизационных кортежей (ReBAC на PostgreSQL)
-- Формат: object_type:object_id#relation@subject_type:subject_id
-- Совместима с миграцией на OpenFGA / SpiceDB
-- ============================================================

CREATE TABLE authorization_tuples (
    id              BIGSERIAL    PRIMARY KEY,
    object_type     VARCHAR(100) NOT NULL,    -- тип объекта (faculty, department, stream, group, ...)
    object_id       VARCHAR(255) NOT NULL,    -- id объекта (код сущности)
    relation        VARCHAR(100) NOT NULL,    -- тип связи (parent, member, teacher, dean, ...)
    subject_type    VARCHAR(100) NOT NULL,    -- тип субъекта (user, faculty, department, ...)
    subject_id      VARCHAR(255) NOT NULL,    -- id субъекта (external_id пользователя или код сущности)
    condition_name  VARCHAR(255),             -- имя условия (аналог SpiceDB Caveats)
    condition_ctx   JSONB,                    -- контекст условия
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Уникальный индекс: предотвращение дублирующих кортежей
CREATE UNIQUE INDEX idx_auth_tuples_unique
    ON authorization_tuples(object_type, object_id, relation, subject_type, subject_id);

-- Поиск по объекту: «все связи этого объекта»
CREATE INDEX idx_auth_tuples_object
    ON authorization_tuples(object_type, object_id);

-- Поиск по субъекту: «все связи этого субъекта»
CREATE INDEX idx_auth_tuples_subject
    ON authorization_tuples(subject_type, subject_id);

-- Поиск по relation: «все кортежи с данным типом связи»
CREATE INDEX idx_auth_tuples_relation
    ON authorization_tuples(relation);

-- Составной индекс для обхода parent-связей
CREATE INDEX idx_auth_tuples_parent_lookup
    ON authorization_tuples(object_type, relation, subject_type)
    WHERE relation = 'parent';

-- Составной индекс для поиска членов объекта
CREATE INDEX idx_auth_tuples_members
    ON authorization_tuples(object_type, object_id, relation, subject_type)
    WHERE subject_type = 'user';

COMMENT ON TABLE authorization_tuples IS 'ReBAC-кортежи авторизации (object#relation@subject)';
COMMENT ON COLUMN authorization_tuples.object_type IS 'Тип объекта: faculty, department, program, stream, group, subgroup, nationality_category';
COMMENT ON COLUMN authorization_tuples.relation IS 'Связь: parent, member, teacher, foreign_teacher, dean, head, director, curator';
COMMENT ON COLUMN authorization_tuples.condition_name IS 'Имя условия (аналог SpiceDB Caveat), опционально';
COMMENT ON COLUMN authorization_tuples.condition_ctx IS 'JSONB-контекст условия, опционально';

-- +goose Down
DROP TABLE IF EXISTS authorization_tuples;
