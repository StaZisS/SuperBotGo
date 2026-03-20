-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Триггерные функции синхронизации бизнес-таблиц → authorization_tuples
-- При изменении данных в бизнес-таблицах автоматически обновляются
-- соответствующие кортежи авторизации.
-- ============================================================

-- ============================================================
-- sync_hierarchy_tuples: синхронизация parent-кортежей организационной иерархии
-- Отдельная функция для каждого уровня иерархии
-- ============================================================

-- Кафедра → Факультет
CREATE OR REPLACE FUNCTION sync_department_hierarchy() RETURNS TRIGGER AS $$
DECLARE
    v_parent_code TEXT;
BEGIN
    -- Удаляем старый кортеж при UPDATE или DELETE
    IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
        DELETE FROM authorization_tuples
        WHERE object_type = 'department'
          AND object_id = OLD.code
          AND relation = 'parent';
    END IF;

    -- Создаём новый кортеж при INSERT или UPDATE
    IF (TG_OP = 'INSERT' OR TG_OP = 'UPDATE') AND NEW.faculty_id IS NOT NULL THEN
        SELECT code INTO v_parent_code FROM faculties WHERE id = NEW.faculty_id;
        IF v_parent_code IS NOT NULL THEN
            INSERT INTO authorization_tuples (object_type, object_id, relation, subject_type, subject_id)
            VALUES ('department', NEW.code, 'parent', 'faculty', v_parent_code)
            ON CONFLICT (object_type, object_id, relation, subject_type, subject_id) DO NOTHING;
        END IF;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Направление → Кафедра
CREATE OR REPLACE FUNCTION sync_program_hierarchy() RETURNS TRIGGER AS $$
DECLARE
    v_parent_code TEXT;
BEGIN
    IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
        DELETE FROM authorization_tuples
        WHERE object_type = 'program'
          AND object_id = OLD.code
          AND relation = 'parent';
    END IF;

    IF (TG_OP = 'INSERT' OR TG_OP = 'UPDATE') AND NEW.department_id IS NOT NULL THEN
        SELECT code INTO v_parent_code FROM departments WHERE id = NEW.department_id;
        IF v_parent_code IS NOT NULL THEN
            INSERT INTO authorization_tuples (object_type, object_id, relation, subject_type, subject_id)
            VALUES ('program', NEW.code, 'parent', 'department', v_parent_code)
            ON CONFLICT (object_type, object_id, relation, subject_type, subject_id) DO NOTHING;
        END IF;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Поток → Направление
CREATE OR REPLACE FUNCTION sync_stream_hierarchy() RETURNS TRIGGER AS $$
DECLARE
    v_parent_code TEXT;
BEGIN
    IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
        DELETE FROM authorization_tuples
        WHERE object_type = 'stream'
          AND object_id = OLD.code
          AND relation = 'parent';
    END IF;

    IF (TG_OP = 'INSERT' OR TG_OP = 'UPDATE') AND NEW.program_id IS NOT NULL THEN
        SELECT code INTO v_parent_code FROM programs WHERE id = NEW.program_id;
        IF v_parent_code IS NOT NULL THEN
            INSERT INTO authorization_tuples (object_type, object_id, relation, subject_type, subject_id)
            VALUES ('stream', NEW.code, 'parent', 'program', v_parent_code)
            ON CONFLICT (object_type, object_id, relation, subject_type, subject_id) DO NOTHING;
        END IF;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Группа → Поток
CREATE OR REPLACE FUNCTION sync_study_group_hierarchy() RETURNS TRIGGER AS $$
DECLARE
    v_parent_code TEXT;
BEGIN
    IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
        DELETE FROM authorization_tuples
        WHERE object_type = 'group'
          AND object_id = OLD.code
          AND relation = 'parent';
    END IF;

    IF (TG_OP = 'INSERT' OR TG_OP = 'UPDATE') AND NEW.stream_id IS NOT NULL THEN
        SELECT code INTO v_parent_code FROM streams WHERE id = NEW.stream_id;
        IF v_parent_code IS NOT NULL THEN
            INSERT INTO authorization_tuples (object_type, object_id, relation, subject_type, subject_id)
            VALUES ('group', NEW.code, 'parent', 'stream', v_parent_code)
            ON CONFLICT (object_type, object_id, relation, subject_type, subject_id) DO NOTHING;
        END IF;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Подгруппа → Группа
CREATE OR REPLACE FUNCTION sync_subgroup_hierarchy() RETURNS TRIGGER AS $$
DECLARE
    v_parent_code TEXT;
BEGIN
    IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
        DELETE FROM authorization_tuples
        WHERE object_type = 'subgroup'
          AND object_id = OLD.code
          AND relation = 'parent';
    END IF;

    IF (TG_OP = 'INSERT' OR TG_OP = 'UPDATE') AND NEW.study_group_id IS NOT NULL THEN
        SELECT code INTO v_parent_code FROM study_groups WHERE id = NEW.study_group_id;
        IF v_parent_code IS NOT NULL THEN
            INSERT INTO authorization_tuples (object_type, object_id, relation, subject_type, subject_id)
            VALUES ('subgroup', NEW.code, 'parent', 'group', v_parent_code)
            ON CONFLICT (object_type, object_id, relation, subject_type, subject_id) DO NOTHING;
        END IF;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- sync_student_tuples: синхронизация членства студентов в группах
-- При создании/изменении StudentPosition создаёт кортеж member
-- ============================================================

CREATE OR REPLACE FUNCTION sync_student_tuples() RETURNS TRIGGER AS $$
DECLARE
    v_person_ext_id TEXT;
    v_group_code    TEXT;
BEGIN
    -- Удаляем старые кортежи
    IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
        SELECT p.external_id INTO v_person_ext_id
        FROM persons p WHERE p.id = OLD.person_id;

        IF v_person_ext_id IS NOT NULL AND OLD.study_group_id IS NOT NULL THEN
            SELECT code INTO v_group_code FROM study_groups WHERE id = OLD.study_group_id;
            IF v_group_code IS NOT NULL THEN
                DELETE FROM authorization_tuples
                WHERE object_type = 'group'
                  AND object_id = v_group_code
                  AND relation = 'member'
                  AND subject_type = 'user'
                  AND subject_id = v_person_ext_id;
            END IF;
        END IF;
    END IF;

    -- Создаём новые кортежи (только для активных позиций)
    IF (TG_OP = 'INSERT' OR TG_OP = 'UPDATE') AND NEW.status = 'active' THEN
        SELECT p.external_id INTO v_person_ext_id
        FROM persons p WHERE p.id = NEW.person_id;

        IF v_person_ext_id IS NOT NULL AND NEW.study_group_id IS NOT NULL THEN
            SELECT code INTO v_group_code FROM study_groups WHERE id = NEW.study_group_id;
            IF v_group_code IS NOT NULL THEN
                INSERT INTO authorization_tuples (object_type, object_id, relation, subject_type, subject_id)
                VALUES ('group', v_group_code, 'member', 'user', v_person_ext_id)
                ON CONFLICT (object_type, object_id, relation, subject_type, subject_id) DO NOTHING;
            END IF;
        END IF;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- sync_student_subgroup_tuples: синхронизация членства в подгруппах
-- ============================================================

CREATE OR REPLACE FUNCTION sync_student_subgroup_tuples() RETURNS TRIGGER AS $$
DECLARE
    v_person_ext_id TEXT;
    v_subgroup_code TEXT;
BEGIN
    IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
        SELECT p.external_id INTO v_person_ext_id
        FROM persons p
        JOIN student_positions sp ON sp.person_id = p.id
        WHERE sp.id = OLD.student_position_id;

        SELECT code INTO v_subgroup_code FROM subgroups WHERE id = OLD.subgroup_id;

        IF v_person_ext_id IS NOT NULL AND v_subgroup_code IS NOT NULL THEN
            DELETE FROM authorization_tuples
            WHERE object_type = 'subgroup'
              AND object_id = v_subgroup_code
              AND relation = 'member'
              AND subject_type = 'user'
              AND subject_id = v_person_ext_id;
        END IF;
    END IF;

    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        SELECT p.external_id INTO v_person_ext_id
        FROM persons p
        JOIN student_positions sp ON sp.person_id = p.id
        WHERE sp.id = NEW.student_position_id;

        SELECT code INTO v_subgroup_code FROM subgroups WHERE id = NEW.subgroup_id;

        IF v_person_ext_id IS NOT NULL AND v_subgroup_code IS NOT NULL THEN
            INSERT INTO authorization_tuples (object_type, object_id, relation, subject_type, subject_id)
            VALUES ('subgroup', v_subgroup_code, 'member', 'user', v_person_ext_id)
            ON CONFLICT (object_type, object_id, relation, subject_type, subject_id) DO NOTHING;
        END IF;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- sync_teacher_tuples: синхронизация назначений преподавателей
-- Создаёт кортежи teacher / foreign_teacher в зависимости от student_scope
-- ============================================================

CREATE OR REPLACE FUNCTION sync_teacher_tuples() RETURNS TRIGGER AS $$
DECLARE
    v_person_ext_id TEXT;
    v_target_code   TEXT;
    v_target_type   TEXT;
    v_relation      TEXT;
BEGIN
    -- Удаляем старые кортежи
    IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
        SELECT p.external_id INTO v_person_ext_id
        FROM persons p
        JOIN teacher_positions tp ON tp.person_id = p.id
        WHERE tp.id = OLD.teacher_position_id;

        IF v_person_ext_id IS NOT NULL THEN
            -- Определяем тип и код целевого объекта
            IF OLD.stream_id IS NOT NULL THEN
                SELECT code INTO v_target_code FROM streams WHERE id = OLD.stream_id;
                v_target_type := 'stream';
            ELSIF OLD.study_group_id IS NOT NULL THEN
                SELECT code INTO v_target_code FROM study_groups WHERE id = OLD.study_group_id;
                v_target_type := 'group';
            END IF;

            IF v_target_code IS NOT NULL THEN
                DELETE FROM authorization_tuples
                WHERE object_type = v_target_type
                  AND object_id = v_target_code
                  AND relation IN ('teacher', 'foreign_teacher')
                  AND subject_type = 'user'
                  AND subject_id = v_person_ext_id;
            END IF;
        END IF;
    END IF;

    -- Создаём новые кортежи
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        SELECT p.external_id INTO v_person_ext_id
        FROM persons p
        JOIN teacher_positions tp ON tp.person_id = p.id
        WHERE tp.id = NEW.teacher_position_id;

        IF v_person_ext_id IS NOT NULL THEN
            IF NEW.stream_id IS NOT NULL THEN
                SELECT code INTO v_target_code FROM streams WHERE id = NEW.stream_id;
                v_target_type := 'stream';
            ELSIF NEW.study_group_id IS NOT NULL THEN
                SELECT code INTO v_target_code FROM study_groups WHERE id = NEW.study_group_id;
                v_target_type := 'group';
            END IF;

            -- Определяем relation по student_scope
            IF NEW.student_scope = 'foreign_only' THEN
                v_relation := 'foreign_teacher';
            ELSE
                v_relation := 'teacher';
            END IF;

            IF v_target_code IS NOT NULL THEN
                INSERT INTO authorization_tuples (object_type, object_id, relation, subject_type, subject_id)
                VALUES (v_target_type, v_target_code, v_relation, 'user', v_person_ext_id)
                ON CONFLICT (object_type, object_id, relation, subject_type, subject_id) DO NOTHING;
            END IF;
        END IF;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- sync_admin_tuples: синхронизация административных назначений
-- Маппинг: appointment_type → relation, scope_type → object_type
-- ============================================================

CREATE OR REPLACE FUNCTION sync_admin_tuples() RETURNS TRIGGER AS $$
DECLARE
    v_person_ext_id TEXT;
    v_relation      TEXT;
    v_object_type   TEXT;
    v_object_id     TEXT;
BEGIN
    -- Удаляем старые кортежи
    IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
        SELECT p.external_id INTO v_person_ext_id
        FROM persons p WHERE p.id = OLD.person_id;

        IF v_person_ext_id IS NOT NULL THEN
            -- Маппинг appointment_type → relation
            v_relation := CASE OLD.appointment_type
                WHEN 'dean'                    THEN 'dean'
                WHEN 'dept_head'               THEN 'head'
                WHEN 'program_director'        THEN 'director'
                WHEN 'stream_curator'          THEN 'curator'
                WHEN 'group_curator'           THEN 'curator'
                WHEN 'foreign_student_curator' THEN 'curator'
            END;

            -- Для куратора иностранцев — особый тип объекта
            IF OLD.appointment_type = 'foreign_student_curator' THEN
                v_object_type := 'nationality_category';
                v_object_id := 'foreign';
            ELSE
                v_object_type := CASE OLD.scope_type
                    WHEN 'university_wide' THEN 'university'
                    WHEN 'faculty'         THEN 'faculty'
                    WHEN 'department'      THEN 'department'
                    WHEN 'program'         THEN 'program'
                    WHEN 'stream'          THEN 'stream'
                    WHEN 'group'           THEN 'group'
                END;

                -- Получаем код сущности-скоупа
                IF OLD.scope_type = 'university_wide' THEN
                    v_object_id := 'main';
                ELSIF OLD.scope_type = 'faculty' THEN
                    SELECT code INTO v_object_id FROM faculties WHERE id = OLD.scope_id;
                ELSIF OLD.scope_type = 'department' THEN
                    SELECT code INTO v_object_id FROM departments WHERE id = OLD.scope_id;
                ELSIF OLD.scope_type = 'program' THEN
                    SELECT code INTO v_object_id FROM programs WHERE id = OLD.scope_id;
                ELSIF OLD.scope_type = 'stream' THEN
                    SELECT code INTO v_object_id FROM streams WHERE id = OLD.scope_id;
                ELSIF OLD.scope_type = 'group' THEN
                    SELECT code INTO v_object_id FROM study_groups WHERE id = OLD.scope_id;
                END IF;
            END IF;

            IF v_object_id IS NOT NULL THEN
                DELETE FROM authorization_tuples
                WHERE object_type = v_object_type
                  AND object_id = v_object_id
                  AND relation = v_relation
                  AND subject_type = 'user'
                  AND subject_id = v_person_ext_id;
            END IF;
        END IF;
    END IF;

    -- Создаём новые кортежи (только для активных назначений)
    IF (TG_OP = 'INSERT' OR TG_OP = 'UPDATE') AND NEW.status = 'active' THEN
        SELECT p.external_id INTO v_person_ext_id
        FROM persons p WHERE p.id = NEW.person_id;

        IF v_person_ext_id IS NOT NULL THEN
            v_relation := CASE NEW.appointment_type
                WHEN 'dean'                    THEN 'dean'
                WHEN 'dept_head'               THEN 'head'
                WHEN 'program_director'        THEN 'director'
                WHEN 'stream_curator'          THEN 'curator'
                WHEN 'group_curator'           THEN 'curator'
                WHEN 'foreign_student_curator' THEN 'curator'
            END;

            IF NEW.appointment_type = 'foreign_student_curator' THEN
                v_object_type := 'nationality_category';
                v_object_id := 'foreign';
            ELSE
                v_object_type := CASE NEW.scope_type
                    WHEN 'university_wide' THEN 'university'
                    WHEN 'faculty'         THEN 'faculty'
                    WHEN 'department'      THEN 'department'
                    WHEN 'program'         THEN 'program'
                    WHEN 'stream'          THEN 'stream'
                    WHEN 'group'           THEN 'group'
                END;

                IF NEW.scope_type = 'university_wide' THEN
                    v_object_id := 'main';
                ELSIF NEW.scope_type = 'faculty' THEN
                    SELECT code INTO v_object_id FROM faculties WHERE id = NEW.scope_id;
                ELSIF NEW.scope_type = 'department' THEN
                    SELECT code INTO v_object_id FROM departments WHERE id = NEW.scope_id;
                ELSIF NEW.scope_type = 'program' THEN
                    SELECT code INTO v_object_id FROM programs WHERE id = NEW.scope_id;
                ELSIF NEW.scope_type = 'stream' THEN
                    SELECT code INTO v_object_id FROM streams WHERE id = NEW.scope_id;
                ELSIF NEW.scope_type = 'group' THEN
                    SELECT code INTO v_object_id FROM study_groups WHERE id = NEW.scope_id;
                END IF;
            END IF;

            IF v_object_id IS NOT NULL THEN
                INSERT INTO authorization_tuples (object_type, object_id, relation, subject_type, subject_id)
                VALUES (v_object_type, v_object_id, v_relation, 'user', v_person_ext_id)
                ON CONFLICT (object_type, object_id, relation, subject_type, subject_id) DO NOTHING;
            END IF;
        END IF;
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- ============================================================
-- Привязка триггеров к бизнес-таблицам
-- ============================================================

-- Иерархия организаций
CREATE TRIGGER trg_departments_hierarchy
    AFTER INSERT OR UPDATE OR DELETE ON departments
    FOR EACH ROW EXECUTE FUNCTION sync_department_hierarchy();

CREATE TRIGGER trg_programs_hierarchy
    AFTER INSERT OR UPDATE OR DELETE ON programs
    FOR EACH ROW EXECUTE FUNCTION sync_program_hierarchy();

CREATE TRIGGER trg_streams_hierarchy
    AFTER INSERT OR UPDATE OR DELETE ON streams
    FOR EACH ROW EXECUTE FUNCTION sync_stream_hierarchy();

CREATE TRIGGER trg_study_groups_hierarchy
    AFTER INSERT OR UPDATE OR DELETE ON study_groups
    FOR EACH ROW EXECUTE FUNCTION sync_study_group_hierarchy();

CREATE TRIGGER trg_subgroups_hierarchy
    AFTER INSERT OR UPDATE OR DELETE ON subgroups
    FOR EACH ROW EXECUTE FUNCTION sync_subgroup_hierarchy();

-- Членство студентов
CREATE TRIGGER trg_student_positions_sync
    AFTER INSERT OR UPDATE OR DELETE ON student_positions
    FOR EACH ROW EXECUTE FUNCTION sync_student_tuples();

CREATE TRIGGER trg_student_subgroups_sync
    AFTER INSERT OR UPDATE OR DELETE ON student_subgroups
    FOR EACH ROW EXECUTE FUNCTION sync_student_subgroup_tuples();

-- Назначения преподавателей
CREATE TRIGGER trg_teaching_assignments_sync
    AFTER INSERT OR UPDATE OR DELETE ON teaching_assignments
    FOR EACH ROW EXECUTE FUNCTION sync_teacher_tuples();

-- Административные назначения
CREATE TRIGGER trg_admin_appointments_sync
    AFTER INSERT OR UPDATE OR DELETE ON administrative_appointments
    FOR EACH ROW EXECUTE FUNCTION sync_admin_tuples();

-- +goose Down
DROP TRIGGER IF EXISTS trg_admin_appointments_sync ON administrative_appointments;
DROP TRIGGER IF EXISTS trg_teaching_assignments_sync ON teaching_assignments;
DROP TRIGGER IF EXISTS trg_student_subgroups_sync ON student_subgroups;
DROP TRIGGER IF EXISTS trg_student_positions_sync ON student_positions;
DROP TRIGGER IF EXISTS trg_subgroups_hierarchy ON subgroups;
DROP TRIGGER IF EXISTS trg_study_groups_hierarchy ON study_groups;
DROP TRIGGER IF EXISTS trg_streams_hierarchy ON streams;
DROP TRIGGER IF EXISTS trg_programs_hierarchy ON programs;
DROP TRIGGER IF EXISTS trg_departments_hierarchy ON departments;

DROP FUNCTION IF EXISTS sync_admin_tuples;
DROP FUNCTION IF EXISTS sync_teacher_tuples;
DROP FUNCTION IF EXISTS sync_student_subgroup_tuples;
DROP FUNCTION IF EXISTS sync_student_tuples;
DROP FUNCTION IF EXISTS sync_subgroup_hierarchy;
DROP FUNCTION IF EXISTS sync_study_group_hierarchy;
DROP FUNCTION IF EXISTS sync_stream_hierarchy;
DROP FUNCTION IF EXISTS sync_program_hierarchy;
DROP FUNCTION IF EXISTS sync_department_hierarchy;
