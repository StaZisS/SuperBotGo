-- +goose Up

-- ============================================================
-- Seed-данные: пример организационной структуры университета
-- 1 факультет, 2 кафедры, 3 направления, потоки, группы, подгруппы
-- Пользователи: студент, преподаватель иностранцев, куратор, декан, аспирант-ассистент
-- ============================================================

-- ============================================================
-- Семестры
-- ============================================================
INSERT INTO semesters (year, semester_type) VALUES
    (2025, 'fall'),
    (2025, 'spring'),
    (2026, 'fall');

-- ============================================================
-- Организационная иерархия
-- ============================================================

-- Факультет инженерии
INSERT INTO faculties (code, name, short_name) VALUES
    ('engineering', 'Инженерный факультет', 'ИФ');

-- Кафедра информатики
INSERT INTO departments (faculty_id, code, name, short_name) VALUES
    ((SELECT id FROM faculties WHERE code = 'engineering'), 'cs', 'Кафедра информатики', 'КИ'),
    ((SELECT id FROM faculties WHERE code = 'engineering'), 'math', 'Кафедра математики', 'КМ');

-- Направления подготовки
INSERT INTO programs (department_id, code, name, degree_level) VALUES
    ((SELECT id FROM departments WHERE code = 'cs'), 'applied_cs', 'Прикладная информатика', 'bachelor'),
    ((SELECT id FROM departments WHERE code = 'cs'), 'cs_master', 'Информатика и вычислительная техника', 'master'),
    ((SELECT id FROM departments WHERE code = 'math'), 'applied_math', 'Прикладная математика', 'bachelor');

-- Потоки
INSERT INTO streams (program_id, code, name, year_started) VALUES
    ((SELECT id FROM programs WHERE code = 'applied_cs'), '9722', 'Поток 9722 (ПИ, бакалавриат, 2022)', 2022),
    ((SELECT id FROM programs WHERE code = 'cs_master'), '9821', 'Поток 9821 (ИВТ, магистратура, 2024)', 2024),
    ((SELECT id FROM programs WHERE code = 'applied_math'), '9622', 'Поток 9622 (ПМ, бакалавриат, 2022)', 2022);

-- Учебные группы
INSERT INTO study_groups (stream_id, code, name) VALUES
    ((SELECT id FROM streams WHERE code = '9722'), '972201', 'Группа 972201'),
    ((SELECT id FROM streams WHERE code = '9722'), '972202', 'Группа 972202'),
    ((SELECT id FROM streams WHERE code = '9722'), '972203', 'Группа 972203'),
    ((SELECT id FROM streams WHERE code = '9821'), '982101', 'Группа 982101'),
    ((SELECT id FROM streams WHERE code = '9622'), '962201', 'Группа 962201');

-- Подгруппы
INSERT INTO subgroups (study_group_id, code, name, subgroup_type) VALUES
    ((SELECT id FROM study_groups WHERE code = '972203'), '972203_eng1', 'Группа 972203, Английский 1', 'language'),
    ((SELECT id FROM study_groups WHERE code = '972203'), '972203_eng2', 'Группа 972203, Английский 2', 'language'),
    ((SELECT id FROM study_groups WHERE code = '972203'), '972203_pe1', 'Группа 972203, Физкультура 1', 'physical_education'),
    ((SELECT id FROM study_groups WHERE code = '972201'), '972201_lab1', 'Группа 972201, Лаборатория 1', 'lab');

-- ============================================================
-- Учебные курсы
-- ============================================================
INSERT INTO courses (code, name) VALUES
    ('CS101', 'Основы программирования'),
    ('CS201', 'Структуры данных и алгоритмы'),
    ('CS301', 'Базы данных'),
    ('MATH101', 'Математический анализ'),
    ('MATH201', 'Линейная алгебра'),
    ('ENG101', 'Английский язык');

-- ============================================================
-- Персоны (идентичности)
-- ============================================================
INSERT INTO persons (external_id, last_name, first_name, middle_name, email) VALUES
    -- Студенты
    ('ahmed_456',     'Ахмед',      'Мухаммад',   NULL,              'ahmed@university.ru'),
    ('ivanov_ms',     'Иванов',     'Михаил',     'Сергеевич',       'ivanov.ms@university.ru'),
    ('petrova_an',    'Петрова',    'Анна',       'Николаевна',      'petrova.an@university.ru'),
    ('kim_jh',        'Ким',        'Джонхён',    NULL,              'kim.jh@university.ru'),
    -- Преподаватели
    ('ivanov_aa',     'Иванов',     'Алексей',    'Александрович',   'ivanov.aa@university.ru'),
    ('petrov_bv',     'Петров',     'Борис',      'Васильевич',      'petrov.bv@university.ru'),
    ('sidorov_iv',    'Сидоров',    'Игорь',      'Валентинович',    'sidorov.iv@university.ru'),
    -- Администрация
    ('smirnov_dv',    'Смирнов',    'Дмитрий',    'Владимирович',    'smirnov.dv@university.ru'),
    ('kozlov_mn',     'Козлов',     'Максим',     'Николаевич',      'kozlov.mn@university.ru'),
    ('curator_ivanova', 'Иванова',  'Елена',      'Петровна',        'ivanova.ep@university.ru');

-- ============================================================
-- Позиции студентов
-- ============================================================

-- Ahmed — иностранный студент группы 972203
INSERT INTO student_positions (person_id, program_id, stream_id, study_group_id, status, nationality_type, funding_type, education_form, enrolled_at) VALUES
    ((SELECT id FROM persons WHERE external_id = 'ahmed_456'),
     (SELECT id FROM programs WHERE code = 'applied_cs'),
     (SELECT id FROM streams WHERE code = '9722'),
     (SELECT id FROM study_groups WHERE code = '972203'),
     'active', 'foreign', 'contract', 'full_time', '2022-09-01');

-- Ким — иностранный студент группы 972203
INSERT INTO student_positions (person_id, program_id, stream_id, study_group_id, status, nationality_type, funding_type, education_form, enrolled_at) VALUES
    ((SELECT id FROM persons WHERE external_id = 'kim_jh'),
     (SELECT id FROM programs WHERE code = 'applied_cs'),
     (SELECT id FROM streams WHERE code = '9722'),
     (SELECT id FROM study_groups WHERE code = '972203'),
     'active', 'foreign', 'budget', 'full_time', '2022-09-01');

-- Петрова — российская студентка группы 972203
INSERT INTO student_positions (person_id, program_id, stream_id, study_group_id, status, nationality_type, funding_type, education_form, enrolled_at) VALUES
    ((SELECT id FROM persons WHERE external_id = 'petrova_an'),
     (SELECT id FROM programs WHERE code = 'applied_cs'),
     (SELECT id FROM streams WHERE code = '9722'),
     (SELECT id FROM study_groups WHERE code = '972203'),
     'active', 'domestic', 'budget', 'full_time', '2022-09-01');

-- Иванов М.С. — аспирант-ассистент: студент магистратуры группы 982101 И ассистент на кафедре CS
INSERT INTO student_positions (person_id, program_id, stream_id, study_group_id, status, nationality_type, funding_type, education_form, enrolled_at) VALUES
    ((SELECT id FROM persons WHERE external_id = 'ivanov_ms'),
     (SELECT id FROM programs WHERE code = 'cs_master'),
     (SELECT id FROM streams WHERE code = '9821'),
     (SELECT id FROM study_groups WHERE code = '982101'),
     'active', 'domestic', 'budget', 'full_time', '2024-09-01');

-- ============================================================
-- Привязка студентов к подгруппам
-- ============================================================

-- Ahmed — подгруппа английского 1
INSERT INTO student_subgroups (student_position_id, subgroup_id) VALUES
    ((SELECT sp.id FROM student_positions sp JOIN persons p ON p.id = sp.person_id WHERE p.external_id = 'ahmed_456'),
     (SELECT id FROM subgroups WHERE code = '972203_eng1'));

-- Петрова — подгруппа английского 2
INSERT INTO student_subgroups (student_position_id, subgroup_id) VALUES
    ((SELECT sp.id FROM student_positions sp JOIN persons p ON p.id = sp.person_id WHERE p.external_id = 'petrova_an'),
     (SELECT id FROM subgroups WHERE code = '972203_eng2'));

-- ============================================================
-- Позиции преподавателей
-- ============================================================

-- Иванов А.А. — доцент кафедры информатики
INSERT INTO teacher_positions (person_id, department_id, position_title, employment_type) VALUES
    ((SELECT id FROM persons WHERE external_id = 'ivanov_aa'),
     (SELECT id FROM departments WHERE code = 'cs'),
     'доцент', 'full_time');

-- Петров Б.В. — профессор кафедры информатики + руководитель направления
INSERT INTO teacher_positions (person_id, department_id, position_title, employment_type) VALUES
    ((SELECT id FROM persons WHERE external_id = 'petrov_bv'),
     (SELECT id FROM departments WHERE code = 'cs'),
     'профессор', 'full_time');

-- Сидоров И.В. — старший преподаватель кафедры информатики
INSERT INTO teacher_positions (person_id, department_id, position_title, employment_type) VALUES
    ((SELECT id FROM persons WHERE external_id = 'sidorov_iv'),
     (SELECT id FROM departments WHERE code = 'cs'),
     'старший преподаватель', 'full_time');

-- Иванов М.С. — ассистент (вторая позиция помимо студенческой)
INSERT INTO teacher_positions (person_id, department_id, position_title, employment_type) VALUES
    ((SELECT id FROM persons WHERE external_id = 'ivanov_ms'),
     (SELECT id FROM departments WHERE code = 'cs'),
     'ассистент', 'part_time');

-- ============================================================
-- Назначения преподавателей
-- ============================================================

-- Иванов А.А. — лектор CS101 для потока 9722 (все студенты)
INSERT INTO teaching_assignments (teacher_position_id, course_id, semester_id, stream_id, assignment_type, student_scope) VALUES
    ((SELECT tp.id FROM teacher_positions tp JOIN persons p ON p.id = tp.person_id WHERE p.external_id = 'ivanov_aa' LIMIT 1),
     (SELECT id FROM courses WHERE code = 'CS101'),
     (SELECT id FROM semesters WHERE year = 2025 AND semester_type = 'fall'),
     (SELECT id FROM streams WHERE code = '9722'),
     'lecturer', 'all');

-- Петров Б.В. — foreign_teacher потока 9722 (только иностранные студенты)
INSERT INTO teaching_assignments (teacher_position_id, course_id, semester_id, stream_id, assignment_type, student_scope) VALUES
    ((SELECT tp.id FROM teacher_positions tp JOIN persons p ON p.id = tp.person_id WHERE p.external_id = 'petrov_bv' LIMIT 1),
     (SELECT id FROM courses WHERE code = 'CS201'),
     (SELECT id FROM semesters WHERE year = 2025 AND semester_type = 'fall'),
     (SELECT id FROM streams WHERE code = '9722'),
     'lecturer', 'foreign_only');

-- Сидоров И.В. — практик группы 972203 (все студенты)
INSERT INTO teaching_assignments (teacher_position_id, course_id, semester_id, study_group_id, assignment_type, student_scope) VALUES
    ((SELECT tp.id FROM teacher_positions tp JOIN persons p ON p.id = tp.person_id WHERE p.external_id = 'sidorov_iv' LIMIT 1),
     (SELECT id FROM courses WHERE code = 'CS301'),
     (SELECT id FROM semesters WHERE year = 2025 AND semester_type = 'fall'),
     (SELECT id FROM study_groups WHERE code = '972203'),
     'practice', 'all');

-- Иванов М.С. (ассистент) — практик потока 9722 бакалавров
INSERT INTO teaching_assignments (teacher_position_id, course_id, semester_id, stream_id, assignment_type, student_scope) VALUES
    ((SELECT tp.id FROM teacher_positions tp JOIN persons p ON p.id = tp.person_id
      WHERE p.external_id = 'ivanov_ms' AND tp.position_title = 'ассистент' LIMIT 1),
     (SELECT id FROM courses WHERE code = 'CS101'),
     (SELECT id FROM semesters WHERE year = 2025 AND semester_type = 'fall'),
     (SELECT id FROM streams WHERE code = '9722'),
     'practice', 'all');

-- ============================================================
-- Административные назначения
-- ============================================================

-- Смирнов — декан инженерного факультета
INSERT INTO administrative_appointments (person_id, appointment_type, scope_type, scope_id) VALUES
    ((SELECT id FROM persons WHERE external_id = 'smirnov_dv'),
     'dean', 'faculty',
     (SELECT id FROM faculties WHERE code = 'engineering'));

-- Козлов — заведующий кафедрой информатики
INSERT INTO administrative_appointments (person_id, appointment_type, scope_type, scope_id) VALUES
    ((SELECT id FROM persons WHERE external_id = 'kozlov_mn'),
     'dept_head', 'department',
     (SELECT id FROM departments WHERE code = 'cs'));

-- Петров Б.В. — руководитель направления «Прикладная информатика»
INSERT INTO administrative_appointments (person_id, appointment_type, scope_type, scope_id) VALUES
    ((SELECT id FROM persons WHERE external_id = 'petrov_bv'),
     'program_director', 'program',
     (SELECT id FROM programs WHERE code = 'applied_cs'));

-- Иванова — куратор ВСЕХ иностранных студентов (сквозная роль)
INSERT INTO administrative_appointments (person_id, appointment_type, scope_type, scope_id, student_filter) VALUES
    ((SELECT id FROM persons WHERE external_id = 'curator_ivanova'),
     'foreign_student_curator', 'university_wide', NULL,
     '{"nationality_type": "foreign"}'::jsonb);

-- +goose Down

-- Удаляем seed-данные в обратном порядке
DELETE FROM administrative_appointments;
DELETE FROM teaching_assignments;
DELETE FROM teacher_positions;
DELETE FROM student_subgroups;
DELETE FROM student_positions;
DELETE FROM persons;
DELETE FROM courses;
DELETE FROM subgroups;
DELETE FROM study_groups;
DELETE FROM streams;
DELETE FROM programs;
DELETE FROM departments;
DELETE FROM faculties;
DELETE FROM semesters;
