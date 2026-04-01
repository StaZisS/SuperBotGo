package university

import (
	"context"
	"fmt"
)

// StubDataSource is a placeholder implementation of DataSource.
// Replace the method bodies with actual calls to the external university system API.
//
// Example usage:
//
//	source := &university.StubDataSource{
//	    BaseURL: cfg.UniversitySync.BaseURL,
//	    Token:   cfg.UniversitySync.Token,
//	}
type StubDataSource struct {
	BaseURL string
	Token   string
}

var _ DataSource = (*StubDataSource)(nil)

func (s *StubDataSource) FetchPersons(ctx context.Context) ([]PersonInput, error) {
	// TODO: implement — GET {BaseURL}/api/persons
	return nil, fmt.Errorf("StubDataSource.FetchPersons: not implemented")
}

func (s *StubDataSource) FetchCourses(ctx context.Context) ([]CourseInput, error) {
	// TODO: implement — GET {BaseURL}/api/courses
	return nil, fmt.Errorf("StubDataSource.FetchCourses: not implemented")
}

func (s *StubDataSource) FetchSemesters(ctx context.Context) ([]SemesterInput, error) {
	// TODO: implement — GET {BaseURL}/api/semesters
	return nil, fmt.Errorf("StubDataSource.FetchSemesters: not implemented")
}

func (s *StubDataSource) FetchFaculties(ctx context.Context) ([]FacultyInput, error) {
	// TODO: implement — GET {BaseURL}/api/faculties
	return nil, fmt.Errorf("StubDataSource.FetchFaculties: not implemented")
}

func (s *StubDataSource) FetchDepartments(ctx context.Context) ([]HierarchyNodeInput, error) {
	// TODO: implement — GET {BaseURL}/api/departments
	return nil, fmt.Errorf("StubDataSource.FetchDepartments: not implemented")
}

func (s *StubDataSource) FetchPrograms(ctx context.Context) ([]HierarchyNodeInput, error) {
	// TODO: implement — GET {BaseURL}/api/programs
	return nil, fmt.Errorf("StubDataSource.FetchPrograms: not implemented")
}

func (s *StubDataSource) FetchStreams(ctx context.Context) ([]HierarchyNodeInput, error) {
	// TODO: implement — GET {BaseURL}/api/streams
	return nil, fmt.Errorf("StubDataSource.FetchStreams: not implemented")
}

func (s *StubDataSource) FetchGroups(ctx context.Context) ([]HierarchyNodeInput, error) {
	// TODO: implement — GET {BaseURL}/api/groups
	return nil, fmt.Errorf("StubDataSource.FetchGroups: not implemented")
}

func (s *StubDataSource) FetchSubgroups(ctx context.Context) ([]HierarchyNodeInput, error) {
	// TODO: implement — GET {BaseURL}/api/subgroups
	return nil, fmt.Errorf("StubDataSource.FetchSubgroups: not implemented")
}

func (s *StubDataSource) FetchTeacherPositions(ctx context.Context) ([]TeacherPositionInput, error) {
	// TODO: implement — GET {BaseURL}/api/teacher-positions
	return nil, fmt.Errorf("StubDataSource.FetchTeacherPositions: not implemented")
}

func (s *StubDataSource) FetchStudentPositions(ctx context.Context) ([]StudentPositionInput, error) {
	// TODO: implement — GET {BaseURL}/api/student-positions
	return nil, fmt.Errorf("StubDataSource.FetchStudentPositions: not implemented")
}

func (s *StubDataSource) FetchStudentSubgroups(ctx context.Context) ([]StudentSubgroupInput, error) {
	// TODO: implement — GET {BaseURL}/api/student-subgroups
	return nil, fmt.Errorf("StubDataSource.FetchStudentSubgroups: not implemented")
}

func (s *StubDataSource) FetchTeachingAssignments(ctx context.Context) ([]TeachingAssignmentInput, error) {
	// TODO: implement — GET {BaseURL}/api/teaching-assignments
	return nil, fmt.Errorf("StubDataSource.FetchTeachingAssignments: not implemented")
}

func (s *StubDataSource) FetchAdminAppointments(ctx context.Context) ([]AdminAppointmentInput, error) {
	// TODO: implement — GET {BaseURL}/api/admin-appointments
	return nil, fmt.Errorf("StubDataSource.FetchAdminAppointments: not implemented")
}
