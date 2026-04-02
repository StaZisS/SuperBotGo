package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RefItem struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type UniversityRefHandler struct {
	pool *pgxpool.Pool
}

func NewUniversityRefHandler(pool *pgxpool.Pool) *UniversityRefHandler {
	return &UniversityRefHandler{pool: pool}
}

func (h *UniversityRefHandler) RegisterRoutes(mux *http.ServeMux) {
	// GET (read-only, used by cascading selects)
	mux.HandleFunc("GET /api/admin/university/faculties", h.handleListFaculties)
	mux.HandleFunc("GET /api/admin/university/departments", h.handleListDepartments)
	mux.HandleFunc("GET /api/admin/university/programs", h.handleListPrograms)
	mux.HandleFunc("GET /api/admin/university/streams", h.handleListStreams)
	mux.HandleFunc("GET /api/admin/university/groups", h.handleListGroups)
	mux.HandleFunc("GET /api/admin/university/subgroups", h.handleListSubgroups)
	mux.HandleFunc("GET /api/admin/university/courses", h.handleListCourses)
	mux.HandleFunc("GET /api/admin/university/semesters", h.handleListSemesters)

	// CRUD (manage prefix to avoid conflict with sync POST endpoints)
	p := "/api/admin/university/manage"

	mux.HandleFunc("POST "+p+"/faculties", h.handleCreateFaculty)
	mux.HandleFunc("PUT "+p+"/faculties/{id}", h.handleUpdateFaculty)
	mux.HandleFunc("DELETE "+p+"/faculties/{id}", h.handleDeleteFaculty)

	mux.HandleFunc("POST "+p+"/departments", h.handleCreateDepartment)
	mux.HandleFunc("PUT "+p+"/departments/{id}", h.handleUpdateDepartment)
	mux.HandleFunc("DELETE "+p+"/departments/{id}", h.handleDeleteDepartment)

	mux.HandleFunc("POST "+p+"/programs", h.handleCreateProgram)
	mux.HandleFunc("PUT "+p+"/programs/{id}", h.handleUpdateProgram)
	mux.HandleFunc("DELETE "+p+"/programs/{id}", h.handleDeleteProgram)

	mux.HandleFunc("POST "+p+"/streams", h.handleCreateStream)
	mux.HandleFunc("PUT "+p+"/streams/{id}", h.handleUpdateStream)
	mux.HandleFunc("DELETE "+p+"/streams/{id}", h.handleDeleteStream)

	mux.HandleFunc("POST "+p+"/groups", h.handleCreateGroup)
	mux.HandleFunc("PUT "+p+"/groups/{id}", h.handleUpdateGroup)
	mux.HandleFunc("DELETE "+p+"/groups/{id}", h.handleDeleteGroup)

	mux.HandleFunc("POST "+p+"/subgroups", h.handleCreateSubgroup)
	mux.HandleFunc("PUT "+p+"/subgroups/{id}", h.handleUpdateSubgroup)
	mux.HandleFunc("DELETE "+p+"/subgroups/{id}", h.handleDeleteSubgroup)

	mux.HandleFunc("POST "+p+"/courses", h.handleCreateCourse)
	mux.HandleFunc("PUT "+p+"/courses/{id}", h.handleUpdateCourse)
	mux.HandleFunc("DELETE "+p+"/courses/{id}", h.handleDeleteCourse)

	mux.HandleFunc("POST "+p+"/semesters", h.handleCreateSemester)
	mux.HandleFunc("PUT "+p+"/semesters/{id}", h.handleUpdateSemester)
	mux.HandleFunc("DELETE "+p+"/semesters/{id}", h.handleDeleteSemester)
}

func (h *UniversityRefHandler) pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func (h *UniversityRefHandler) queryInt64(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
}

func (h *UniversityRefHandler) listRefItems(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	defer rows.Close()
	items := make([]RefItem, 0)
	for rows.Next() {
		var item RefItem
		if err := rows.Scan(&item.ID, &item.Code, &item.Name); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *UniversityRefHandler) deleteEntity(w http.ResponseWriter, r *http.Request, table string) {
	id, err := h.pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	_, err = h.pool.Exec(r.Context(), fmt.Sprintf("DELETE FROM %s WHERE id = $1", table), id)
	if err != nil {
		writeError(w, http.StatusConflict, "cannot delete: entity may be referenced")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *UniversityRefHandler) handleListFaculties(w http.ResponseWriter, r *http.Request) {
	h.listRefItems(w, r, `SELECT id, code, name FROM faculties ORDER BY name`)
}

func (h *UniversityRefHandler) handleCreateFaculty(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	var id int64
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO faculties (code, name, short_name) VALUES ($1, $2, NULLIF($3,'')) RETURNING id`,
		req.Code, req.Name, req.ShortName).Scan(&id)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to create: code may already exist")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (h *UniversityRefHandler) handleUpdateFaculty(w http.ResponseWriter, r *http.Request) {
	id, err := h.pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	_, err = h.pool.Exec(r.Context(),
		`UPDATE faculties SET code=$2, name=$3, short_name=NULLIF($4,''), updated_at=now() WHERE id=$1`,
		id, req.Code, req.Name, req.ShortName)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to update")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *UniversityRefHandler) handleDeleteFaculty(w http.ResponseWriter, r *http.Request) {
	h.deleteEntity(w, r, "faculties")
}

func (h *UniversityRefHandler) handleListDepartments(w http.ResponseWriter, r *http.Request) {
	facultyID, err := h.queryInt64(r, "faculty_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "faculty_id is required")
		return
	}
	h.listRefItems(w, r, `SELECT id, code, name FROM departments WHERE faculty_id = $1 ORDER BY name`, facultyID)
}

func (h *UniversityRefHandler) handleCreateDepartment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FacultyID int64  `json:"faculty_id"`
		Code      string `json:"code"`
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	var id int64
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO departments (faculty_id, code, name, short_name) VALUES ($1, $2, $3, NULLIF($4,'')) RETURNING id`,
		req.FacultyID, req.Code, req.Name, req.ShortName).Scan(&id)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to create: code may already exist")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (h *UniversityRefHandler) handleUpdateDepartment(w http.ResponseWriter, r *http.Request) {
	id, err := h.pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		FacultyID int64  `json:"faculty_id"`
		Code      string `json:"code"`
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	_, err = h.pool.Exec(r.Context(),
		`UPDATE departments SET faculty_id=$2, code=$3, name=$4, short_name=NULLIF($5,''), updated_at=now() WHERE id=$1`,
		id, req.FacultyID, req.Code, req.Name, req.ShortName)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to update")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *UniversityRefHandler) handleDeleteDepartment(w http.ResponseWriter, r *http.Request) {
	h.deleteEntity(w, r, "departments")
}

func (h *UniversityRefHandler) handleListPrograms(w http.ResponseWriter, r *http.Request) {
	departmentID, err := h.queryInt64(r, "department_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "department_id is required")
		return
	}
	h.listRefItems(w, r, `SELECT id, code, name FROM programs WHERE department_id = $1 ORDER BY name`, departmentID)
}

func (h *UniversityRefHandler) handleCreateProgram(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DepartmentID int64  `json:"department_id"`
		Code         string `json:"code"`
		Name         string `json:"name"`
		DegreeLevel  string `json:"degree_level"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	var id int64
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO programs (department_id, code, name, degree_level) VALUES ($1, $2, $3, $4) RETURNING id`,
		req.DepartmentID, req.Code, req.Name, req.DegreeLevel).Scan(&id)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to create")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (h *UniversityRefHandler) handleUpdateProgram(w http.ResponseWriter, r *http.Request) {
	id, err := h.pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		DepartmentID int64  `json:"department_id"`
		Code         string `json:"code"`
		Name         string `json:"name"`
		DegreeLevel  string `json:"degree_level"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	_, err = h.pool.Exec(r.Context(),
		`UPDATE programs SET department_id=$2, code=$3, name=$4, degree_level=$5, updated_at=now() WHERE id=$1`,
		id, req.DepartmentID, req.Code, req.Name, req.DegreeLevel)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to update")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *UniversityRefHandler) handleDeleteProgram(w http.ResponseWriter, r *http.Request) {
	h.deleteEntity(w, r, "programs")
}

func (h *UniversityRefHandler) handleListStreams(w http.ResponseWriter, r *http.Request) {
	programID, err := h.queryInt64(r, "program_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "program_id is required")
		return
	}
	h.listRefItems(w, r, `SELECT id, code, COALESCE(name, code) FROM streams WHERE program_id = $1 ORDER BY COALESCE(name, code)`, programID)
}

func (h *UniversityRefHandler) handleCreateStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProgramID   int64  `json:"program_id"`
		Code        string `json:"code"`
		Name        string `json:"name"`
		YearStarted *int   `json:"year_started"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	var id int64
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO streams (program_id, code, name, year_started) VALUES ($1, $2, NULLIF($3,''), $4) RETURNING id`,
		req.ProgramID, req.Code, req.Name, req.YearStarted).Scan(&id)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to create")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (h *UniversityRefHandler) handleUpdateStream(w http.ResponseWriter, r *http.Request) {
	id, err := h.pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		ProgramID   int64  `json:"program_id"`
		Code        string `json:"code"`
		Name        string `json:"name"`
		YearStarted *int   `json:"year_started"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	_, err = h.pool.Exec(r.Context(),
		`UPDATE streams SET program_id=$2, code=$3, name=NULLIF($4,''), year_started=$5, updated_at=now() WHERE id=$1`,
		id, req.ProgramID, req.Code, req.Name, req.YearStarted)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to update")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *UniversityRefHandler) handleDeleteStream(w http.ResponseWriter, r *http.Request) {
	h.deleteEntity(w, r, "streams")
}

func (h *UniversityRefHandler) handleListGroups(w http.ResponseWriter, r *http.Request) {
	streamID, err := h.queryInt64(r, "stream_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "stream_id is required")
		return
	}
	h.listRefItems(w, r, `SELECT id, code, COALESCE(name, code) FROM study_groups WHERE stream_id = $1 ORDER BY COALESCE(name, code)`, streamID)
}

func (h *UniversityRefHandler) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StreamID int64  `json:"stream_id"`
		Code     string `json:"code"`
		Name     string `json:"name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	var id int64
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO study_groups (stream_id, code, name) VALUES ($1, $2, NULLIF($3,'')) RETURNING id`,
		req.StreamID, req.Code, req.Name).Scan(&id)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to create")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (h *UniversityRefHandler) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	id, err := h.pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		StreamID int64  `json:"stream_id"`
		Code     string `json:"code"`
		Name     string `json:"name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	_, err = h.pool.Exec(r.Context(),
		`UPDATE study_groups SET stream_id=$2, code=$3, name=NULLIF($4,''), updated_at=now() WHERE id=$1`,
		id, req.StreamID, req.Code, req.Name)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to update")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *UniversityRefHandler) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	h.deleteEntity(w, r, "study_groups")
}

func (h *UniversityRefHandler) handleListSubgroups(w http.ResponseWriter, r *http.Request) {
	groupID, err := h.queryInt64(r, "study_group_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "study_group_id is required")
		return
	}
	h.listRefItems(w, r, `SELECT id, code, COALESCE(name, code) FROM subgroups WHERE study_group_id = $1 ORDER BY COALESCE(name, code)`, groupID)
}

func (h *UniversityRefHandler) handleCreateSubgroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StudyGroupID int64  `json:"study_group_id"`
		Code         string `json:"code"`
		Name         string `json:"name"`
		SubgroupType string `json:"subgroup_type"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	var id int64
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO subgroups (study_group_id, code, name, subgroup_type) VALUES ($1, $2, NULLIF($3,''), $4) RETURNING id`,
		req.StudyGroupID, req.Code, req.Name, req.SubgroupType).Scan(&id)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to create")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (h *UniversityRefHandler) handleUpdateSubgroup(w http.ResponseWriter, r *http.Request) {
	id, err := h.pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		StudyGroupID int64  `json:"study_group_id"`
		Code         string `json:"code"`
		Name         string `json:"name"`
		SubgroupType string `json:"subgroup_type"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	_, err = h.pool.Exec(r.Context(),
		`UPDATE subgroups SET study_group_id=$2, code=$3, name=NULLIF($4,''), subgroup_type=$5, updated_at=now() WHERE id=$1`,
		id, req.StudyGroupID, req.Code, req.Name, req.SubgroupType)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to update")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *UniversityRefHandler) handleDeleteSubgroup(w http.ResponseWriter, r *http.Request) {
	h.deleteEntity(w, r, "subgroups")
}

func (h *UniversityRefHandler) handleListCourses(w http.ResponseWriter, r *http.Request) {
	h.listRefItems(w, r, `SELECT id, code, name FROM courses ORDER BY name`)
}

func (h *UniversityRefHandler) handleCreateCourse(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	var id int64
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO courses (code, name) VALUES ($1, $2) RETURNING id`,
		req.Code, req.Name).Scan(&id)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to create: code may already exist")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (h *UniversityRefHandler) handleUpdateCourse(w http.ResponseWriter, r *http.Request) {
	id, err := h.pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	_, err = h.pool.Exec(r.Context(),
		`UPDATE courses SET code=$2, name=$3, updated_at=now() WHERE id=$1`,
		id, req.Code, req.Name)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to update")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *UniversityRefHandler) handleDeleteCourse(w http.ResponseWriter, r *http.Request) {
	h.deleteEntity(w, r, "courses")
}

func (h *UniversityRefHandler) handleListSemesters(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `SELECT id, year, semester_type FROM semesters ORDER BY year DESC, semester_type`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	defer rows.Close()
	type semItem struct {
		ID           int64  `json:"id"`
		Year         int    `json:"year"`
		SemesterType string `json:"semester_type"`
	}
	items := make([]semItem, 0)
	for rows.Next() {
		var s semItem
		if err := rows.Scan(&s.ID, &s.Year, &s.SemesterType); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		items = append(items, s)
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *UniversityRefHandler) handleCreateSemester(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Year         int    `json:"year"`
		SemesterType string `json:"semester_type"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	var id int64
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO semesters (year, semester_type) VALUES ($1, $2) RETURNING id`,
		req.Year, req.SemesterType).Scan(&id)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to create: semester may already exist")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (h *UniversityRefHandler) handleUpdateSemester(w http.ResponseWriter, r *http.Request) {
	id, err := h.pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Year         int    `json:"year"`
		SemesterType string `json:"semester_type"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	_, err = h.pool.Exec(r.Context(),
		`UPDATE semesters SET year=$2, semester_type=$3, updated_at=now() WHERE id=$1`,
		id, req.Year, req.SemesterType)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to update")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *UniversityRefHandler) handleDeleteSemester(w http.ResponseWriter, r *http.Request) {
	h.deleteEntity(w, r, "semesters")
}
