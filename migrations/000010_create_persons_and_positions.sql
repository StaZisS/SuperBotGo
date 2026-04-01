-- +goose Up

-- ============================================================
-- Сущности идентичности и позиции
-- Person — единая точка идентичности
-- «Студент» и «Преподаватель» — позиции, которые человек занимает
-- ============================================================

-- Человек (идентичность)
CREATE TABLE persons (
    id              BIGSERIAL    PRIMARY KEY,
    external_id     VARCHAR(255) UNIQUE,                -- логин/SSO, используется как SubjectID в SpiceDB
    last_name       VARCHAR(255) NOT NULL,
    first_name      VARCHAR(255) NOT NULL,
    middle_name     VARCHAR(255),
    email           VARCHAR(500),
    phone           VARCHAR(100),
    global_user_id  BIGINT       REFERENCES global_users(id) ON DELETE SET NULL,  -- связь с ботом
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_persons_external_id ON persons(external_id);
CREATE INDEX idx_persons_global_user_id ON persons(global_user_id);
CREATE INDEX idx_persons_name ON persons(last_name, first_name);

COMMENT ON TABLE persons IS 'Люди — единая точка идентичности';
COMMENT ON COLUMN persons.external_id IS 'Логин/SSO ID, используется как subject_id (user:external_id) в авторизации';
COMMENT ON COLUMN persons.global_user_id IS 'Связь с аккаунтом бота (global_users), если есть';

-- ============================================================
-- Позиция студента
-- ============================================================

CREATE TABLE student_positions (
    id                BIGSERIAL    PRIMARY KEY,
    person_id         BIGINT       NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    program_id        BIGINT       REFERENCES programs(id) ON DELETE SET NULL,
    stream_id         BIGINT       REFERENCES streams(id) ON DELETE SET NULL,
    study_group_id    BIGINT       REFERENCES study_groups(id) ON DELETE SET NULL,
    status            VARCHAR(50)  NOT NULL DEFAULT 'active'
                                   CHECK (status IN ('active', 'suspended', 'ended')),
    nationality_type  VARCHAR(50)  NOT NULL DEFAULT 'domestic'
                                   CHECK (nationality_type IN ('domestic', 'foreign')),
    funding_type      VARCHAR(50)  NOT NULL DEFAULT 'budget'
                                   CHECK (funding_type IN ('budget', 'contract')),
    education_form    VARCHAR(50)  NOT NULL DEFAULT 'full_time'
                                   CHECK (education_form IN ('full_time', 'part_time', 'remote')),
    enrolled_at       TIMESTAMPTZ,
    graduated_at      TIMESTAMPTZ,
    started_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    ended_at          TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_student_positions_person_id ON student_positions(person_id);
CREATE INDEX idx_student_positions_program_id ON student_positions(program_id);
CREATE INDEX idx_student_positions_stream_id ON student_positions(stream_id);
CREATE INDEX idx_student_positions_study_group_id ON student_positions(study_group_id);
CREATE INDEX idx_student_positions_status ON student_positions(status);
CREATE INDEX idx_student_positions_nationality ON student_positions(nationality_type);
CREATE INDEX idx_student_positions_funding ON student_positions(funding_type);

COMMENT ON TABLE student_positions IS 'Позиция студента — привязка человека к группе/потоку/направлению';
COMMENT ON COLUMN student_positions.nationality_type IS 'Тип гражданства: domestic (РФ) / foreign (иностранец)';

-- Связь студент ↔ подгруппа (many-to-many)
CREATE TABLE student_subgroups (
    id                   BIGSERIAL PRIMARY KEY,
    student_position_id  BIGINT    NOT NULL REFERENCES student_positions(id) ON DELETE CASCADE,
    subgroup_id          BIGINT    NOT NULL REFERENCES subgroups(id) ON DELETE CASCADE,
    UNIQUE (student_position_id, subgroup_id)
);

CREATE INDEX idx_student_subgroups_position ON student_subgroups(student_position_id);
CREATE INDEX idx_student_subgroups_subgroup ON student_subgroups(subgroup_id);

COMMENT ON TABLE student_subgroups IS 'Принадлежность студента к подгруппам (язык, физра, выбор)';

-- ============================================================
-- Позиция преподавателя
-- ============================================================

CREATE TABLE teacher_positions (
    id               BIGSERIAL    PRIMARY KEY,
    person_id        BIGINT       NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    department_id    BIGINT       REFERENCES departments(id) ON DELETE SET NULL,
    position_title   VARCHAR(100) NOT NULL,  -- доцент, профессор, ассистент, старший преподаватель
    employment_type  VARCHAR(50)  NOT NULL DEFAULT 'full_time'
                                  CHECK (employment_type IN ('full_time', 'part_time', 'hourly')),
    status           VARCHAR(50)  NOT NULL DEFAULT 'active'
                                  CHECK (status IN ('active', 'suspended', 'ended')),
    started_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    ended_at         TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_teacher_positions_person_id ON teacher_positions(person_id);
CREATE INDEX idx_teacher_positions_department_id ON teacher_positions(department_id);
CREATE INDEX idx_teacher_positions_status ON teacher_positions(status);

COMMENT ON TABLE teacher_positions IS 'Позиция преподавателя — привязка человека к кафедре';

-- ============================================================
-- Назначения преподавателя (что именно ведёт)
-- ============================================================

CREATE TABLE teaching_assignments (
    id                   BIGSERIAL    PRIMARY KEY,
    teacher_position_id  BIGINT       NOT NULL REFERENCES teacher_positions(id) ON DELETE CASCADE,
    course_id            BIGINT       NOT NULL REFERENCES courses(id) ON DELETE RESTRICT,
    semester_id          BIGINT       NOT NULL REFERENCES semesters(id) ON DELETE RESTRICT,
    stream_id            BIGINT       REFERENCES streams(id) ON DELETE SET NULL,
    study_group_id       BIGINT       REFERENCES study_groups(id) ON DELETE SET NULL,
    assignment_type      VARCHAR(50)  NOT NULL
                                      CHECK (assignment_type IN ('lecturer', 'practice', 'supervisor', 'examiner')),
    student_scope        VARCHAR(50)  NOT NULL DEFAULT 'all'
                                      CHECK (student_scope IN ('all', 'foreign_only', 'specific_subgroup')),
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_teaching_assignments_teacher ON teaching_assignments(teacher_position_id);
CREATE INDEX idx_teaching_assignments_course ON teaching_assignments(course_id);
CREATE INDEX idx_teaching_assignments_semester ON teaching_assignments(semester_id);
CREATE INDEX idx_teaching_assignments_stream ON teaching_assignments(stream_id);
CREATE INDEX idx_teaching_assignments_group ON teaching_assignments(study_group_id);

COMMENT ON TABLE teaching_assignments IS 'Назначения преподавателя: какой курс, какому потоку/группе, в какой роли';
COMMENT ON COLUMN teaching_assignments.student_scope IS 'Определяет доступ к студентам: all/foreign_only/specific_subgroup';

-- ============================================================
-- Административные назначения
-- ============================================================

CREATE TABLE administrative_appointments (
    id                BIGSERIAL    PRIMARY KEY,
    person_id         BIGINT       NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    appointment_type  VARCHAR(100) NOT NULL
                                   CHECK (appointment_type IN (
                                       'dean', 'dept_head', 'program_director',
                                       'stream_curator', 'group_curator', 'foreign_student_curator'
                                   )),
    scope_type        VARCHAR(100) NOT NULL
                                   CHECK (scope_type IN (
                                       'university_wide', 'faculty', 'department',
                                       'program', 'stream', 'group'
                                   )),
    scope_id          BIGINT,                -- id сущности-скоупа (NULL для university_wide)
    student_filter    JSONB,                 -- опциональный фильтр, например {"nationality_type": "foreign"}
    status            VARCHAR(50)  NOT NULL DEFAULT 'active'
                                   CHECK (status IN ('active', 'suspended', 'ended')),
    started_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    ended_at          TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_admin_appointments_person_id ON administrative_appointments(person_id);
CREATE INDEX idx_admin_appointments_type ON administrative_appointments(appointment_type);
CREATE INDEX idx_admin_appointments_scope ON administrative_appointments(scope_type, scope_id);
CREATE INDEX idx_admin_appointments_status ON administrative_appointments(status);

COMMENT ON TABLE administrative_appointments IS 'Административные назначения (декан, завкаф, руководитель направления и т.д.)';
COMMENT ON COLUMN administrative_appointments.scope_type IS 'Область действия назначения';
COMMENT ON COLUMN administrative_appointments.student_filter IS 'JSONB-фильтр для сквозных ролей, например {"nationality_type": "foreign"}';

-- +goose Down
DROP TABLE IF EXISTS administrative_appointments;
DROP TABLE IF EXISTS teaching_assignments;
DROP TABLE IF EXISTS teacher_positions;
DROP TABLE IF EXISTS student_subgroups;
DROP TABLE IF EXISTS student_positions;
DROP TABLE IF EXISTS persons;
