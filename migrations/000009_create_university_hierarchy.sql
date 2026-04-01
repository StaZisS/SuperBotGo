-- +goose Up

-- ============================================================
-- Организационная иерархия университета
-- Шестиуровневая цепочка: Факультет → Кафедра → Направление → Поток → Группа → Подгруппа
-- ============================================================

-- Факультеты
CREATE TABLE faculties (
    id          BIGSERIAL    PRIMARY KEY,
    code        VARCHAR(100) NOT NULL UNIQUE,       -- краткий код, используется как ObjectID в SpiceDB
    name        VARCHAR(500) NOT NULL,
    short_name  VARCHAR(100),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE faculties IS 'Факультеты университета';
COMMENT ON COLUMN faculties.code IS 'Уникальный код факультета (например: engineering)';

-- Кафедры
CREATE TABLE departments (
    id          BIGSERIAL    PRIMARY KEY,
    faculty_id  BIGINT       NOT NULL REFERENCES faculties(id) ON DELETE RESTRICT,
    code        VARCHAR(100) NOT NULL UNIQUE,
    name        VARCHAR(500) NOT NULL,
    short_name  VARCHAR(100),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_departments_faculty_id ON departments(faculty_id);

COMMENT ON TABLE departments IS 'Кафедры факультетов';

-- Направления подготовки / специальности
CREATE TABLE programs (
    id              BIGSERIAL    PRIMARY KEY,
    department_id   BIGINT       NOT NULL REFERENCES departments(id) ON DELETE RESTRICT,
    code            VARCHAR(100) NOT NULL UNIQUE,
    name            VARCHAR(500) NOT NULL,
    degree_level    VARCHAR(50)  NOT NULL CHECK (degree_level IN ('bachelor', 'master', 'specialist', 'phd')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_programs_department_id ON programs(department_id);
CREATE INDEX idx_programs_degree_level ON programs(degree_level);

COMMENT ON TABLE programs IS 'Направления подготовки (специальности)';
COMMENT ON COLUMN programs.degree_level IS 'Уровень образования: bachelor/master/specialist/phd';

-- Потоки (например «9722»)
CREATE TABLE streams (
    id           BIGSERIAL    PRIMARY KEY,
    program_id   BIGINT       NOT NULL REFERENCES programs(id) ON DELETE RESTRICT,
    code         VARCHAR(100) NOT NULL UNIQUE,
    name         VARCHAR(500),
    year_started INT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_streams_program_id ON streams(program_id);

COMMENT ON TABLE streams IS 'Потоки (объединения групп одного направления и года)';

-- Учебные группы (например «972203»)
CREATE TABLE study_groups (
    id          BIGSERIAL    PRIMARY KEY,
    stream_id   BIGINT       NOT NULL REFERENCES streams(id) ON DELETE RESTRICT,
    code        VARCHAR(100) NOT NULL UNIQUE,
    name        VARCHAR(500),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_study_groups_stream_id ON study_groups(stream_id);

COMMENT ON TABLE study_groups IS 'Учебные группы';

-- Подгруппы (например «972203(1)» для английского)
CREATE TABLE subgroups (
    id              BIGSERIAL    PRIMARY KEY,
    study_group_id  BIGINT       NOT NULL REFERENCES study_groups(id) ON DELETE CASCADE,
    code            VARCHAR(100) NOT NULL UNIQUE,
    name            VARCHAR(500),
    subgroup_type   VARCHAR(50)  NOT NULL CHECK (subgroup_type IN ('language', 'physical_education', 'elective', 'lab')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_subgroups_study_group_id ON subgroups(study_group_id);
CREATE INDEX idx_subgroups_type ON subgroups(subgroup_type);

COMMENT ON TABLE subgroups IS 'Подгруппы (языковые, физкультурные, по выбору, лабораторные)';

-- ============================================================
-- Учебные сущности
-- ============================================================

-- Учебные курсы
CREATE TABLE courses (
    id          BIGSERIAL    PRIMARY KEY,
    code        VARCHAR(100) NOT NULL UNIQUE,
    name        VARCHAR(500) NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE courses IS 'Учебные курсы (дисциплины)';

-- Семестры
CREATE TABLE semesters (
    id              BIGSERIAL   PRIMARY KEY,
    year            INT         NOT NULL,
    semester_type   VARCHAR(20) NOT NULL CHECK (semester_type IN ('fall', 'spring')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (year, semester_type)
);

COMMENT ON TABLE semesters IS 'Семестры (учебные периоды)';

-- +goose Down
DROP TABLE IF EXISTS semesters;
DROP TABLE IF EXISTS courses;
DROP TABLE IF EXISTS subgroups;
DROP TABLE IF EXISTS study_groups;
DROP TABLE IF EXISTS streams;
DROP TABLE IF EXISTS programs;
DROP TABLE IF EXISTS departments;
DROP TABLE IF EXISTS faculties;
