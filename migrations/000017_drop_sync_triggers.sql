-- +goose Up

-- ============================================================
-- Удаляем триггеры автосинхронизации authorization_tuples.
-- Тьюплы теперь формируются в Go-коде (internal/authz/tuples)
-- и записываются в той же транзакции, что и бизнес-данные.
-- ============================================================

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

-- +goose Down

-- Восстанавливаем триггеры из 000013 (полный текст — см. 000013_create_sync_triggers.sql).
-- При откате миграции триггеры нужно будет создать заново из 000013.
-- goose автоматически пересоздаст их при re-apply 000013.
