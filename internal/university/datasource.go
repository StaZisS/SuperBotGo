package university

import "context"

// DataSource describes an external system that provides university data.
// Implement this interface to connect a specific upstream (REST API, SOAP, DB, file, etc.).
//
// Each method returns the full current list of entities.
// The Puller calls them in dependency order and upserts via SyncService.
//
// Return (nil, nil) for entity types the source does not provide.
type DataSource interface {
	FetchPersons(ctx context.Context) ([]PersonInput, error)
	FetchCourses(ctx context.Context) ([]CourseInput, error)
	FetchSemesters(ctx context.Context) ([]SemesterInput, error)

	FetchFaculties(ctx context.Context) ([]FacultyInput, error)
	FetchDepartments(ctx context.Context) ([]HierarchyNodeInput, error)
	FetchPrograms(ctx context.Context) ([]HierarchyNodeInput, error)
	FetchStreams(ctx context.Context) ([]HierarchyNodeInput, error)
	FetchGroups(ctx context.Context) ([]HierarchyNodeInput, error)
	FetchSubgroups(ctx context.Context) ([]HierarchyNodeInput, error)

	FetchTeacherPositions(ctx context.Context) ([]TeacherPositionInput, error)
	FetchStudentPositions(ctx context.Context) ([]StudentPositionInput, error)
	FetchStudentSubgroups(ctx context.Context) ([]StudentSubgroupInput, error)
	FetchTeachingAssignments(ctx context.Context) ([]TeachingAssignmentInput, error)
	FetchAdminAppointments(ctx context.Context) ([]AdminAppointmentInput, error)
}
