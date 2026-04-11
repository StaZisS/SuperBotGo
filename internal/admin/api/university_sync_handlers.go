package api

import (
	"context"
	"net/http"

	"SuperBotGo/internal/university"
)

// UniversitySyncHandler exposes SyncService methods as HTTP endpoints
// for receiving data from an external university system.
type UniversitySyncHandler struct {
	sync *university.SyncService
}

func NewUniversitySyncHandler(sync *university.SyncService) *UniversitySyncHandler {
	return &UniversitySyncHandler{sync: sync}
}

func (h *UniversitySyncHandler) RegisterRoutes(mux *http.ServeMux) {
	// Справочные сущности
	mux.HandleFunc("POST /api/admin/university/persons", h.handleSyncPersons)
	mux.HandleFunc("POST /api/admin/university/courses", h.handleSyncCourses)
	mux.HandleFunc("POST /api/admin/university/semesters", h.handleSyncSemesters)

	// Организационная иерархия
	mux.HandleFunc("POST /api/admin/university/faculties", h.handleSyncFaculties)
	mux.HandleFunc("POST /api/admin/university/departments", h.handleSyncDepartments)
	mux.HandleFunc("POST /api/admin/university/programs", h.handleSyncPrograms)
	mux.HandleFunc("POST /api/admin/university/streams", h.handleSyncStreams)
	mux.HandleFunc("POST /api/admin/university/groups", h.handleSyncGroups)
	mux.HandleFunc("POST /api/admin/university/subgroups", h.handleSyncSubgroups)

	// Позиции и назначения
	mux.HandleFunc("POST /api/admin/university/teacher-positions", h.handleSyncTeacherPositions)
	mux.HandleFunc("POST /api/admin/university/student-positions", h.handleSyncStudentPositions)
	mux.HandleFunc("POST /api/admin/university/student-subgroups", h.handleSyncStudentSubgroups)
	mux.HandleFunc("POST /api/admin/university/teaching-assignments", h.handleSyncTeachingAssignments)
	mux.HandleFunc("POST /api/admin/university/admin-appointments", h.handleSyncAdminAppointments)
}

type batchResult struct {
	Total   int      `json:"total"`
	Success int      `json:"success"`
	Errors  []string `json:"errors,omitempty"`
}

type batchSyncConfig[T any] struct {
	validate func(T) string
	sync     func(context.Context, T) error
	describe func(T) string
}

func runBatchSync[T any](ctx context.Context, items []T, cfg batchSyncConfig[T]) batchResult {
	res := batchResult{Total: len(items)}
	for _, item := range items {
		if msg := cfg.validate(item); msg != "" {
			res.Errors = append(res.Errors, msg)
			continue
		}
		if err := cfg.sync(ctx, item); err != nil {
			res.Errors = append(res.Errors, cfg.describe(item)+": "+err.Error())
			continue
		}
		res.Success++
	}
	return res
}

type syncPersonRequest struct {
	ExternalID string `json:"external_id"`
	LastName   string `json:"last_name"`
	FirstName  string `json:"first_name"`
	MiddleName string `json:"middle_name,omitempty"`
	Email      string `json:"email,omitempty"`
	Phone      string `json:"phone,omitempty"`
}

func (h *UniversitySyncHandler) handleSyncPersons(w http.ResponseWriter, r *http.Request) {
	var items []syncPersonRequest
	if !decodeJSONBody(w, r, &items) {
		return
	}

	res := runBatchSync(r.Context(), items, batchSyncConfig[syncPersonRequest]{
		validate: func(item syncPersonRequest) string {
			if item.ExternalID == "" || item.LastName == "" || item.FirstName == "" {
				return "person missing external_id, last_name or first_name"
			}
			return ""
		},
		sync: func(ctx context.Context, item syncPersonRequest) error {
			return h.sync.SyncPerson(ctx, university.PersonInput{
				ExternalID: item.ExternalID,
				LastName:   item.LastName,
				FirstName:  item.FirstName,
				MiddleName: item.MiddleName,
				Email:      item.Email,
				Phone:      item.Phone,
			})
		},
		describe: func(item syncPersonRequest) string {
			return "person " + item.ExternalID
		},
	})
	writeJSON(w, statusForBatch(res), res)
}

type syncCourseRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func (h *UniversitySyncHandler) handleSyncCourses(w http.ResponseWriter, r *http.Request) {
	var items []syncCourseRequest
	if !decodeJSONBody(w, r, &items) {
		return
	}

	res := runBatchSync(r.Context(), items, batchSyncConfig[syncCourseRequest]{
		validate: func(item syncCourseRequest) string {
			if item.Code == "" || item.Name == "" {
				return "course missing code or name"
			}
			return ""
		},
		sync: func(ctx context.Context, item syncCourseRequest) error {
			return h.sync.SyncCourse(ctx, university.CourseInput{
				Code: item.Code,
				Name: item.Name,
			})
		},
		describe: func(item syncCourseRequest) string {
			return "course " + item.Code
		},
	})
	writeJSON(w, statusForBatch(res), res)
}

type syncSemesterRequest struct {
	Year         int    `json:"year"`
	SemesterType string `json:"semester_type"`
}

func (h *UniversitySyncHandler) handleSyncSemesters(w http.ResponseWriter, r *http.Request) {
	var items []syncSemesterRequest
	if !decodeJSONBody(w, r, &items) {
		return
	}

	res := runBatchSync(r.Context(), items, batchSyncConfig[syncSemesterRequest]{
		validate: func(item syncSemesterRequest) string {
			if item.Year == 0 || item.SemesterType == "" {
				return "semester missing year or semester_type"
			}
			return ""
		},
		sync: func(ctx context.Context, item syncSemesterRequest) error {
			return h.sync.SyncSemester(ctx, university.SemesterInput{
				Year:         item.Year,
				SemesterType: item.SemesterType,
			})
		},
		describe: func(syncSemesterRequest) string {
			return "semester"
		},
	})
	writeJSON(w, statusForBatch(res), res)
}

type syncFacultyRequest struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	ShortName string `json:"short_name,omitempty"`
}

func (h *UniversitySyncHandler) handleSyncFaculties(w http.ResponseWriter, r *http.Request) {
	var items []syncFacultyRequest
	if !decodeJSONBody(w, r, &items) {
		return
	}

	res := runBatchSync(r.Context(), items, batchSyncConfig[syncFacultyRequest]{
		validate: func(item syncFacultyRequest) string {
			if item.Code == "" || item.Name == "" {
				return "faculty missing code or name"
			}
			return ""
		},
		sync: func(ctx context.Context, item syncFacultyRequest) error {
			return h.sync.SyncFaculty(ctx, university.FacultyInput{
				Code:      item.Code,
				Name:      item.Name,
				ShortName: item.ShortName,
			})
		},
		describe: func(item syncFacultyRequest) string {
			return "faculty " + item.Code
		},
	})
	writeJSON(w, statusForBatch(res), res)
}

type syncHierarchyNodeRequest struct {
	Code       string         `json:"code"`
	ParentCode string         `json:"parent_code"`
	Name       string         `json:"name"`
	Extra      map[string]any `json:"extra,omitempty"`
}

func (h *UniversitySyncHandler) handleHierarchyNodes(w http.ResponseWriter, r *http.Request, level university.HierarchyLevel) {
	var items []syncHierarchyNodeRequest
	if !decodeJSONBody(w, r, &items) {
		return
	}

	res := runBatchSync(r.Context(), items, batchSyncConfig[syncHierarchyNodeRequest]{
		validate: func(item syncHierarchyNodeRequest) string {
			if item.Code == "" || item.ParentCode == "" || item.Name == "" {
				return level.Table + " missing code, parent_code or name"
			}
			return ""
		},
		sync: func(ctx context.Context, item syncHierarchyNodeRequest) error {
			return h.sync.SyncHierarchyNode(ctx, level, university.HierarchyNodeInput{
				Code:       item.Code,
				ParentCode: item.ParentCode,
				Name:       item.Name,
				Extra:      item.Extra,
			})
		},
		describe: func(item syncHierarchyNodeRequest) string {
			return level.Table + " " + item.Code
		},
	})
	writeJSON(w, statusForBatch(res), res)
}

func (h *UniversitySyncHandler) handleSyncDepartments(w http.ResponseWriter, r *http.Request) {
	h.handleHierarchyNodes(w, r, university.LevelDepartment)
}

func (h *UniversitySyncHandler) handleSyncPrograms(w http.ResponseWriter, r *http.Request) {
	h.handleHierarchyNodes(w, r, university.LevelProgram)
}

func (h *UniversitySyncHandler) handleSyncStreams(w http.ResponseWriter, r *http.Request) {
	h.handleHierarchyNodes(w, r, university.LevelStream)
}

func (h *UniversitySyncHandler) handleSyncGroups(w http.ResponseWriter, r *http.Request) {
	h.handleHierarchyNodes(w, r, university.LevelGroup)
}

func (h *UniversitySyncHandler) handleSyncSubgroups(w http.ResponseWriter, r *http.Request) {
	h.handleHierarchyNodes(w, r, university.LevelSubgroup)
}

type syncTeacherPositionRequest struct {
	PersonExternalID string `json:"person_external_id"`
	DepartmentCode   string `json:"department_code"`
	PositionTitle    string `json:"position_title"`
	EmploymentType   string `json:"employment_type"`
}

func (h *UniversitySyncHandler) handleSyncTeacherPositions(w http.ResponseWriter, r *http.Request) {
	var items []syncTeacherPositionRequest
	if !decodeJSONBody(w, r, &items) {
		return
	}

	res := runBatchSync(r.Context(), items, batchSyncConfig[syncTeacherPositionRequest]{
		validate: func(item syncTeacherPositionRequest) string {
			if item.PersonExternalID == "" || item.DepartmentCode == "" || item.PositionTitle == "" {
				return "teacher_position missing required fields"
			}
			return ""
		},
		sync: func(ctx context.Context, item syncTeacherPositionRequest) error {
			return h.sync.SyncTeacherPosition(ctx, university.TeacherPositionInput{
				PersonExternalID: item.PersonExternalID,
				DepartmentCode:   item.DepartmentCode,
				PositionTitle:    item.PositionTitle,
				EmploymentType:   item.EmploymentType,
			})
		},
		describe: func(item syncTeacherPositionRequest) string {
			return "teacher_position " + item.PersonExternalID
		},
	})
	writeJSON(w, statusForBatch(res), res)
}

type syncStudentPositionRequest struct {
	PersonExternalID string `json:"person_external_id"`
	ProgramCode      string `json:"program_code"`
	StreamCode       string `json:"stream_code"`
	GroupCode        string `json:"group_code"`
	Status           string `json:"status"`
	NationalityType  string `json:"nationality_type"`
	FundingType      string `json:"funding_type"`
	EducationForm    string `json:"education_form"`
}

func (h *UniversitySyncHandler) handleSyncStudentPositions(w http.ResponseWriter, r *http.Request) {
	var items []syncStudentPositionRequest
	if !decodeJSONBody(w, r, &items) {
		return
	}

	res := runBatchSync(r.Context(), items, batchSyncConfig[syncStudentPositionRequest]{
		validate: func(item syncStudentPositionRequest) string {
			if item.PersonExternalID == "" {
				return "student_position missing person_external_id"
			}
			return ""
		},
		sync: func(ctx context.Context, item syncStudentPositionRequest) error {
			return h.sync.SyncStudentPosition(ctx, university.StudentPositionInput{
				PersonExternalID: item.PersonExternalID,
				ProgramCode:      item.ProgramCode,
				StreamCode:       item.StreamCode,
				GroupCode:        item.GroupCode,
				Status:           item.Status,
				NationalityType:  item.NationalityType,
				FundingType:      item.FundingType,
				EducationForm:    item.EducationForm,
			})
		},
		describe: func(item syncStudentPositionRequest) string {
			return "student_position " + item.PersonExternalID
		},
	})
	writeJSON(w, statusForBatch(res), res)
}

type syncStudentSubgroupRequest struct {
	PersonExternalID string `json:"person_external_id"`
	SubgroupCode     string `json:"subgroup_code"`
}

func (h *UniversitySyncHandler) handleSyncStudentSubgroups(w http.ResponseWriter, r *http.Request) {
	var items []syncStudentSubgroupRequest
	if !decodeJSONBody(w, r, &items) {
		return
	}

	res := runBatchSync(r.Context(), items, batchSyncConfig[syncStudentSubgroupRequest]{
		validate: func(item syncStudentSubgroupRequest) string {
			if item.PersonExternalID == "" || item.SubgroupCode == "" {
				return "student_subgroup missing required fields"
			}
			return ""
		},
		sync: func(ctx context.Context, item syncStudentSubgroupRequest) error {
			return h.sync.SyncStudentSubgroup(ctx, university.StudentSubgroupInput{
				PersonExternalID: item.PersonExternalID,
				SubgroupCode:     item.SubgroupCode,
			})
		},
		describe: func(item syncStudentSubgroupRequest) string {
			return "student_subgroup " + item.PersonExternalID
		},
	})
	writeJSON(w, statusForBatch(res), res)
}

type syncTeachingAssignmentRequest struct {
	PersonExternalID string `json:"person_external_id"`
	CourseCode       string `json:"course_code"`
	SemesterYear     int    `json:"semester_year"`
	SemesterType     string `json:"semester_type"`
	StreamCode       string `json:"stream_code,omitempty"`
	GroupCode        string `json:"group_code,omitempty"`
	AssignmentType   string `json:"assignment_type"`
	StudentScope     string `json:"student_scope"`
}

func (h *UniversitySyncHandler) handleSyncTeachingAssignments(w http.ResponseWriter, r *http.Request) {
	var items []syncTeachingAssignmentRequest
	if !decodeJSONBody(w, r, &items) {
		return
	}

	res := runBatchSync(r.Context(), items, batchSyncConfig[syncTeachingAssignmentRequest]{
		validate: func(item syncTeachingAssignmentRequest) string {
			if item.PersonExternalID == "" || item.CourseCode == "" || item.AssignmentType == "" {
				return "teaching_assignment missing required fields"
			}
			return ""
		},
		sync: func(ctx context.Context, item syncTeachingAssignmentRequest) error {
			return h.sync.SyncTeachingAssignment(ctx, university.TeachingAssignmentInput{
				PersonExternalID: item.PersonExternalID,
				CourseCode:       item.CourseCode,
				SemesterYear:     item.SemesterYear,
				SemesterType:     item.SemesterType,
				StreamCode:       item.StreamCode,
				GroupCode:        item.GroupCode,
				AssignmentType:   item.AssignmentType,
				StudentScope:     item.StudentScope,
			})
		},
		describe: func(item syncTeachingAssignmentRequest) string {
			return "teaching_assignment " + item.PersonExternalID
		},
	})
	writeJSON(w, statusForBatch(res), res)
}

type syncAdminAppointmentRequest struct {
	PersonExternalID string `json:"person_external_id"`
	AppointmentType  string `json:"appointment_type"`
	ScopeType        string `json:"scope_type"`
	ScopeCode        string `json:"scope_code,omitempty"`
}

func (h *UniversitySyncHandler) handleSyncAdminAppointments(w http.ResponseWriter, r *http.Request) {
	var items []syncAdminAppointmentRequest
	if !decodeJSONBody(w, r, &items) {
		return
	}

	res := runBatchSync(r.Context(), items, batchSyncConfig[syncAdminAppointmentRequest]{
		validate: func(item syncAdminAppointmentRequest) string {
			if item.PersonExternalID == "" || item.AppointmentType == "" || item.ScopeType == "" {
				return "admin_appointment missing required fields"
			}
			return ""
		},
		sync: func(ctx context.Context, item syncAdminAppointmentRequest) error {
			return h.sync.SyncAdminAppointment(ctx, university.AdminAppointmentInput{
				PersonExternalID: item.PersonExternalID,
				AppointmentType:  item.AppointmentType,
				ScopeType:        item.ScopeType,
				ScopeCode:        item.ScopeCode,
			})
		},
		describe: func(item syncAdminAppointmentRequest) string {
			return "admin_appointment " + item.PersonExternalID
		},
	})
	writeJSON(w, statusForBatch(res), res)
}

func statusForBatch(res batchResult) int {
	if res.Success == 0 && len(res.Errors) > 0 {
		return http.StatusUnprocessableEntity
	}
	if len(res.Errors) > 0 {
		return http.StatusPartialContent
	}
	return http.StatusOK
}
