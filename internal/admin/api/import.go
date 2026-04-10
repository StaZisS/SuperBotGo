// internal/admin/api/import.go
package api

import (
	"SuperBotGo/internal/university"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
)

type ImportHandler struct {
	pool    *pgxpool.Pool
	syncSvc *university.SyncService
}

func NewImportHandler(pool *pgxpool.Pool, syncSvc *university.SyncService) *ImportHandler {
	return &ImportHandler{pool: pool, syncSvc: syncSvc}
}

func (h *ImportHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/import/students", h.handleImportStudents)
	mux.HandleFunc("GET /api/admin/import/template", h.handleDownloadTemplate)
}

type ImportResult struct {
	Total   int           `json:"total"`
	Created int           `json:"created"`
	Updated int           `json:"updated"`
	Skipped int           `json:"skipped"`
	Errors  []ImportError `json:"errors,omitempty"`
}

type ImportError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// handleImportStudents обрабатывает POST с .xlsx файлом
func (h *ImportHandler) handleImportStudents(w http.ResponseWriter, r *http.Request) {
	maxSize := int64(10 << 20) // 10MB
	if err := r.ParseMultipartForm(maxSize); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form: "+err.Error())
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	// Parse Excel
	rows, err := parseStudentExcel(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result := ImportResult{Total: len(rows)}

	for i, row := range rows {
		// Validate row
		if verr := validateStudentRow(row); verr != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:     i + 2,
				Field:   verr.Field,
				Message: verr.Message,
			})
			result.Skipped++
			continue
		}

		// Import student
		created, err := h.importStudent(r.Context(), row)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:     i + 2,
				Message: err.Error(),
			})
			result.Skipped++
			continue
		}

		if created {
			result.Created++
		} else {
			result.Updated++
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// handleDownloadTemplate отдаёт шаблон Excel
func (h *ImportHandler) handleDownloadTemplate(w http.ResponseWriter, r *http.Request) {
	f := excelize.NewFile()
	defer f.Close()

	// Header row - только существующие поля
	headers := []string{
		"external_id", "last_name", "first_name", "middle_name",
		"email", "phone",
		"program_code", "stream_code", "group_code", "subgroup_codes",
		"status", "nationality_type", "funding_type", "education_form",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Sheet1", cell, h)
	}

	// Example row
	example := []string{
		"STU001", "Иванов", "Иван", "Иванович",
		"ivanov@university.ru", "+79001234567",
		"PROG01", "STRM01", "GRP01", "SG01,SG02",
		"active", "domestic", "budget", "full_time",
	}
	for i, v := range example {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue("Sheet1", cell, v)
	}

	// Style header
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	for i := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellStyle("Sheet1", cell, cell, style)
	}

	f.SetColWidth("Sheet1", "A", "N", 18)

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=students_template.xlsx")
	f.Write(w)
}

// studentRow представляет одну строку из Excel
type studentRow struct {
	// Person fields
	ExternalID string
	LastName   string
	FirstName  string
	MiddleName string
	Email      string
	Phone      string

	// StudentPosition fields
	ProgramCode   string
	StreamCode    string
	GroupCode     string
	SubgroupCodes []string

	// Enums
	Status          string
	NationalityType string
	FundingType     string
	EducationForm   string
}

// validationError описывает ошибку валидации строки
type validationError struct {
	Field   string
	Message string
}

// parseStudentExcel читает .xlsx и возвращает slice строк
func parseStudentExcel(r io.Reader) ([]studentRow, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel: %w", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Sheet1")
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("file must have header and at least one data row")
	}

	// Parse header to get column indices
	header := rows[0]
	colMap := make(map[string]int)
	for i, h := range header {
		colMap[strings.TrimSpace(strings.ToLower(h))] = i
	}

	var result []studentRow
	for i, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}

		getCol := func(name string) string {
			if idx, ok := colMap[name]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}

		sr := studentRow{
			// Person
			ExternalID: getCol("external_id"),
			LastName:   getCol("last_name"),
			FirstName:  getCol("first_name"),
			MiddleName: getCol("middle_name"),
			Email:      getCol("email"),
			Phone:      getCol("phone"),
			// StudentPosition
			ProgramCode:     getCol("program_code"),
			StreamCode:      getCol("stream_code"),
			GroupCode:       getCol("group_code"),
			Status:          getCol("status"),
			NationalityType: getCol("nationality_type"),
			FundingType:     getCol("funding_type"),
			EducationForm:   getCol("education_form"),
		}

		// Parse subgroup codes (comma-separated)
		sgStr := getCol("subgroup_codes")
		if sgStr != "" {
			parts := strings.Split(sgStr, ",")
			for _, p := range parts {
				if s := strings.TrimSpace(p); s != "" {
					sr.SubgroupCodes = append(sr.SubgroupCodes, s)
				}
			}
		}

		result = append(result, sr)
		slog.Debug("parsed row", "row", i+2, "external_id", sr.ExternalID)
	}

	return result, nil
}

// validateStudentRow проверяет корректность данных строки
func validateStudentRow(row studentRow) *validationError {
	if row.ExternalID == "" {
		return &validationError{Field: "external_id", Message: "required"}
	}
	if row.LastName == "" {
		return &validationError{Field: "last_name", Message: "required"}
	}
	if row.FirstName == "" {
		return &validationError{Field: "first_name", Message: "required"}
	}
	if row.GroupCode == "" {
		return &validationError{Field: "group_code", Message: "required"}
	}

	// Validate enums
	validStatus := map[string]bool{"active": true, "graduated": true, "expelled": true, "on_leave": true}
	if row.Status != "" && !validStatus[row.Status] {
		return &validationError{Field: "status", Message: "must be: active, graduated, expelled, on_leave"}
	}

	validNat := map[string]bool{"domestic": true, "foreign": true}
	if row.NationalityType != "" && !validNat[row.NationalityType] {
		return &validationError{Field: "nationality_type", Message: "must be: domestic, foreign"}
	}

	validFund := map[string]bool{"budget": true, "contract": true}
	if row.FundingType != "" && !validFund[row.FundingType] {
		return &validationError{Field: "funding_type", Message: "must be: budget, contract"}
	}

	validEdu := map[string]bool{"full_time": true, "part_time": true, "remote": true}
	if row.EducationForm != "" && !validEdu[row.EducationForm] {
		return &validationError{Field: "education_form", Message: "must be: full_time, part_time, remote"}
	}

	return nil
}

// importStudent импортирует одного студента, возвращает (created, error)
func (h *ImportHandler) importStudent(ctx context.Context, row studentRow) (bool, error) {
	// 1. Sync Person
	if err := h.syncSvc.SyncPerson(ctx, university.PersonInput{
		ExternalID: row.ExternalID,
		LastName:   row.LastName,
		FirstName:  row.FirstName,
		MiddleName: row.MiddleName,
		Email:      row.Email,
		Phone:      row.Phone,
	}); err != nil {
		return false, fmt.Errorf("sync person: %w", err)
	}

	// Check if position already exists
	var exists bool
	err := h.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM student_positions 
          WHERE person_id = (SELECT id FROM persons WHERE external_id = $1)
            AND study_group_id = (SELECT id FROM study_groups WHERE code = $2))`,
		row.ExternalID, row.GroupCode,
	).Scan(&exists)
	if err != nil {
		slog.Warn("failed to check existing position", "error", err)
	}

	// 2. Sync StudentPosition
	status := row.Status
	if status == "" {
		status = "active"
	}
	nat := row.NationalityType
	if nat == "" {
		nat = "domestic"
	}
	fund := row.FundingType
	if fund == "" {
		fund = "budget"
	}
	edu := row.EducationForm
	if edu == "" {
		edu = "full_time"
	}

	if err := h.syncSvc.SyncStudentPosition(ctx, university.StudentPositionInput{
		PersonExternalID: row.ExternalID,
		ProgramCode:      row.ProgramCode,
		StreamCode:       row.StreamCode,
		GroupCode:        row.GroupCode,
		Status:           status,
		NationalityType:  nat,
		FundingType:      fund,
		EducationForm:    edu,
	}); err != nil {
		return false, fmt.Errorf("sync student position: %w", err)
	}

	// 3. Sync subgroups
	for _, sgCode := range row.SubgroupCodes {
		if err := h.syncSvc.SyncStudentSubgroup(ctx, university.StudentSubgroupInput{
			PersonExternalID: row.ExternalID,
			SubgroupCode:     sgCode,
		}); err != nil {
			slog.Warn("failed to sync subgroup",
				"external_id", row.ExternalID,
				"subgroup", sgCode,
				"error", err)
		}
	}

	return !exists, nil
}
