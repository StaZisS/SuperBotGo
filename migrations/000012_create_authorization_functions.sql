-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- PL/pgSQL-функции авторизации
-- Рекурсивный обход графа связей через authorization_tuples
-- ============================================================

-- ============================================================
-- check_permission: проверка доступа пользователя к объекту
-- Обходит граф вверх по parent-связям: если пользователь — dean факультета,
-- он получает доступ ко всем кафедрам, направлениям, потокам и группам факультета.
--
-- Параметры:
--   p_user_id        — external_id пользователя (subject_id в кортежах)
--   p_relation        — тип связи для проверки (dean, teacher, member, ...)
--   p_object_type     — тип объекта (faculty, department, stream, group, ...)
--   p_object_id       — id объекта (код сущности)
--
-- Возвращает: TRUE если доступ есть, FALSE если нет
-- ============================================================
CREATE OR REPLACE FUNCTION check_permission(
    p_user_id     VARCHAR,
    p_relation    VARCHAR,
    p_object_type VARCHAR,
    p_object_id   VARCHAR
) RETURNS BOOLEAN
LANGUAGE plpgsql STABLE
AS $$
BEGIN
    RETURN EXISTS (
        WITH RECURSIVE ancestors AS (
            -- Стартуем с целевого объекта
            SELECT
                p_object_type AS obj_type,
                p_object_id   AS obj_id,
                0             AS depth
            UNION ALL
            -- Поднимаемся по parent-связям
            SELECT
                at.subject_type,
                at.subject_id,
                a.depth + 1
            FROM ancestors a
            JOIN authorization_tuples at
                ON at.object_type = a.obj_type
                AND at.object_id = a.obj_id
                AND at.relation = 'parent'
            WHERE a.depth < 10  -- защита от бесконечной рекурсии
        )
        SELECT 1
        FROM ancestors a
        JOIN authorization_tuples at
            ON at.object_type = a.obj_type
            AND at.object_id = a.obj_id
            AND at.relation = p_relation
            AND at.subject_type = 'user'
            AND at.subject_id = p_user_id
        LIMIT 1
    );
END;
$$;

COMMENT ON FUNCTION check_permission IS 'Проверяет, имеет ли пользователь указанную связь с объектом (с обходом иерархии вверх)';

-- ============================================================
-- get_accessible_resources: все ресурсы, доступные пользователю
-- Находит объекты, к которым у пользователя есть прямой доступ,
-- затем обходит граф вниз по parent-связям для нахождения потомков.
--
-- Параметры:
--   p_user_id      — external_id пользователя
--   p_relation     — тип связи (dean, teacher, member, ...)
--   p_object_type  — фильтр по типу объекта (NULL = все типы)
--
-- Возвращает: набор (resource_type, resource_id)
-- ============================================================
CREATE OR REPLACE FUNCTION get_accessible_resources(
    p_user_id     VARCHAR,
    p_relation    VARCHAR,
    p_object_type VARCHAR DEFAULT NULL
) RETURNS TABLE(resource_type VARCHAR, resource_id VARCHAR)
LANGUAGE plpgsql STABLE
AS $$
BEGIN
    RETURN QUERY
    WITH RECURSIVE
    -- Шаг 1: прямые связи пользователя
    direct_access AS (
        SELECT
            at.object_type AS obj_type,
            at.object_id   AS obj_id
        FROM authorization_tuples at
        WHERE at.relation = p_relation
          AND at.subject_type = 'user'
          AND at.subject_id = p_user_id
    ),
    -- Шаг 2: спускаемся вниз по parent-связям (находим потомков)
    descendants AS (
        SELECT da.obj_type, da.obj_id, 0 AS depth
        FROM direct_access da
        UNION ALL
        SELECT
            at.object_type,
            at.object_id,
            d.depth + 1
        FROM descendants d
        JOIN authorization_tuples at
            ON at.relation = 'parent'
            AND at.subject_type = d.obj_type
            AND at.subject_id = d.obj_id
        WHERE d.depth < 10
    )
    SELECT DISTINCT
        d.obj_type::VARCHAR,
        d.obj_id::VARCHAR
    FROM descendants d
    WHERE p_object_type IS NULL OR d.obj_type = p_object_type;
END;
$$;

COMMENT ON FUNCTION get_accessible_resources IS 'Возвращает все ресурсы, доступные пользователю через указанную связь (с обходом иерархии вниз)';

-- ============================================================
-- get_authorized_users: кто имеет доступ к объекту
-- Обходит граф вверх по parent-связям и собирает всех пользователей
-- с указанной связью на каждом уровне иерархии.
--
-- Параметры:
--   p_object_type — тип объекта
--   p_object_id   — id объекта
--   p_relation     — тип связи
--
-- Возвращает: набор user_id (external_id пользователей)
-- ============================================================
CREATE OR REPLACE FUNCTION get_authorized_users(
    p_object_type VARCHAR,
    p_object_id   VARCHAR,
    p_relation    VARCHAR
) RETURNS TABLE(user_id VARCHAR)
LANGUAGE plpgsql STABLE
AS $$
BEGIN
    RETURN QUERY
    WITH RECURSIVE ancestors AS (
        -- Стартуем с целевого объекта
        SELECT
            p_object_type AS obj_type,
            p_object_id   AS obj_id,
            0             AS depth
        UNION ALL
        -- Поднимаемся по parent-связям
        SELECT
            at.subject_type,
            at.subject_id,
            a.depth + 1
        FROM ancestors a
        JOIN authorization_tuples at
            ON at.object_type = a.obj_type
            AND at.object_id = a.obj_id
            AND at.relation = 'parent'
        WHERE a.depth < 10
    )
    SELECT DISTINCT at.subject_id::VARCHAR
    FROM ancestors a
    JOIN authorization_tuples at
        ON at.object_type = a.obj_type
        AND at.object_id = a.obj_id
        AND at.relation = p_relation
        AND at.subject_type = 'user';
END;
$$;

COMMENT ON FUNCTION get_authorized_users IS 'Возвращает всех пользователей, имеющих указанную связь с объектом (с учётом иерархии)';

-- ============================================================
-- get_students_for_teacher: получение списка студентов для преподавателя
-- Учитывает student_scope (all / foreign_only) через JOIN с student_positions.
-- Используется для use case 2 (преподаватель иностранных студентов).
--
-- Параметры:
--   p_teacher_ext_id — external_id преподавателя
--
-- Возвращает: набор (person_external_id, student_position_id, nationality_type)
-- ============================================================
CREATE OR REPLACE FUNCTION get_students_for_teacher(
    p_teacher_ext_id VARCHAR
) RETURNS TABLE(
    person_external_id VARCHAR,
    student_position_id BIGINT,
    nationality_type VARCHAR
)
LANGUAGE plpgsql STABLE
AS $$
BEGIN
    RETURN QUERY
    -- Найти все назначения преподавателя: teacher и foreign_teacher
    WITH RECURSIVE teacher_access AS (
        SELECT
            at.object_type AS target_type,
            at.object_id   AS target_id,
            at.relation     AS access_relation
        FROM authorization_tuples at
        WHERE at.subject_type = 'user'
          AND at.subject_id = p_teacher_ext_id
          AND at.relation IN ('teacher', 'foreign_teacher')
    ),
    -- Раскрыть группы через иерархию (если назначение на поток — найти группы)
    expanded AS (
        SELECT ta.target_type, ta.target_id, ta.access_relation, 0 AS depth
        FROM teacher_access ta
        UNION ALL
        SELECT at.object_type, at.object_id, e.access_relation, e.depth + 1
        FROM expanded e
        JOIN authorization_tuples at
            ON at.relation = 'parent'
            AND at.subject_type = e.target_type
            AND at.subject_id = e.target_id
        WHERE e.depth < 5
    ),
    -- Собрать всех студентов-членов доступных групп
    student_members AS (
        SELECT DISTINCT
            at.subject_id AS student_ext_id,
            e.access_relation
        FROM expanded e
        JOIN authorization_tuples at
            ON at.object_type = e.target_type
            AND at.object_id = e.target_id
            AND at.relation = 'member'
            AND at.subject_type = 'user'
    )
    SELECT
        p.external_id::VARCHAR,
        sp.id,
        sp.nationality_type::VARCHAR
    FROM student_members sm
    JOIN persons p ON p.external_id = sm.student_ext_id
    JOIN student_positions sp ON sp.person_id = p.id AND sp.status = 'active'
    WHERE
        -- Если преподаватель = foreign_teacher, показываем только иностранцев
        CASE
            WHEN sm.access_relation = 'foreign_teacher' THEN sp.nationality_type = 'foreign'
            ELSE TRUE
        END;
END;
$$;

COMMENT ON FUNCTION get_students_for_teacher IS 'Возвращает студентов, доступных преподавателю (с учётом foreign_teacher фильтрации)';

-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS get_students_for_teacher;
DROP FUNCTION IF EXISTS get_authorized_users;
DROP FUNCTION IF EXISTS get_accessible_resources;
DROP FUNCTION IF EXISTS check_permission;
