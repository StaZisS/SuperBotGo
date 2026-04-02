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
	return nil, fmt.Errorf("StubDataSource.FetchPersons: not implemented")
}

func (s *StubDataSource) FetchCourses(ctx context.Context) ([]CourseInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchCourses: not implemented")
}

func (s *StubDataSource) FetchSemesters(ctx context.Context) ([]SemesterInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchSemesters: not implemented")
}

func (s *StubDataSource) FetchFaculties(ctx context.Context) ([]FacultyInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchFaculties: not implemented")
}

func (s *StubDataSource) FetchDepartments(ctx context.Context) ([]HierarchyNodeInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchDepartments: not implemented")
}

func (s *StubDataSource) FetchPrograms(ctx context.Context) ([]HierarchyNodeInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchPrograms: not implemented")
}

func (s *StubDataSource) FetchStreams(ctx context.Context) ([]HierarchyNodeInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchStreams: not implemented")
}

func (s *StubDataSource) FetchGroups(ctx context.Context) ([]HierarchyNodeInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchGroups: not implemented")
}

func (s *StubDataSource) FetchSubgroups(ctx context.Context) ([]HierarchyNodeInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchSubgroups: not implemented")
}

func (s *StubDataSource) FetchTeacherPositions(ctx context.Context) ([]TeacherPositionInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchTeacherPositions: not implemented")
}

func (s *StubDataSource) FetchStudentPositions(ctx context.Context) ([]StudentPositionInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchStudentPositions: not implemented")
}

func (s *StubDataSource) FetchStudentSubgroups(ctx context.Context) ([]StudentSubgroupInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchStudentSubgroups: not implemented")
}

func (s *StubDataSource) FetchTeachingAssignments(ctx context.Context) ([]TeachingAssignmentInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchTeachingAssignments: not implemented")
}

func (s *StubDataSource) FetchAdminAppointments(ctx context.Context) ([]AdminAppointmentInput, error) {
	return nil, fmt.Errorf("StubDataSource.FetchAdminAppointments: not implemented")
}
