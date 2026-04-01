package university

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Puller periodically fetches data from an external DataSource
// and syncs it into PostgreSQL (+ SpiceDB via outbox).
type Puller struct {
	source   DataSource
	sync     *SyncService
	logger   *slog.Logger
	interval time.Duration
}

func NewPuller(source DataSource, sync *SyncService, logger *slog.Logger, interval time.Duration) *Puller {
	return &Puller{
		source:   source,
		sync:     sync,
		logger:   logger,
		interval: interval,
	}
}

// Run starts the pull loop. Blocks until ctx is cancelled.
// Runs an immediate sync on startup, then repeats every interval.
func (p *Puller) Run(ctx context.Context) error {
	p.logger.Info("university puller started", slog.Duration("interval", p.interval))

	p.pull(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("university puller stopped")
			return ctx.Err()
		case <-ticker.C:
			p.pull(ctx)
		}
	}
}

// PullOnce runs a single sync cycle. Useful for manual/test triggers.
func (p *Puller) PullOnce(ctx context.Context) error {
	return p.pull(ctx)
}

func (p *Puller) pull(ctx context.Context) error {
	start := time.Now()
	p.logger.Info("university sync started")

	var firstErr error
	record := func(step string, err error) {
		if err != nil {
			p.logger.Error("university sync step failed", slog.String("step", step), slog.Any("error", err))
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", step, err)
			}
		}
	}

	// Phase 1: reference data (no dependencies)
	record("persons", syncItems(ctx, p, p.source.FetchPersons, p.sync.SyncPerson, "persons"))
	record("courses", syncItems(ctx, p, p.source.FetchCourses, p.sync.SyncCourse, "courses"))
	record("semesters", syncItems(ctx, p, p.source.FetchSemesters, p.sync.SyncSemester, "semesters"))

	// Phase 2: hierarchy (top-down order)
	record("faculties", syncItems(ctx, p, p.source.FetchFaculties, p.sync.SyncFaculty, "faculties"))
	record("departments", p.syncHierarchy(ctx, p.source.FetchDepartments, LevelDepartment, "departments"))
	record("programs", p.syncHierarchy(ctx, p.source.FetchPrograms, LevelProgram, "programs"))
	record("streams", p.syncHierarchy(ctx, p.source.FetchStreams, LevelStream, "streams"))
	record("groups", p.syncHierarchy(ctx, p.source.FetchGroups, LevelGroup, "groups"))
	record("subgroups", p.syncHierarchy(ctx, p.source.FetchSubgroups, LevelSubgroup, "subgroups"))

	// Phase 3: positions (depend on persons + hierarchy)
	record("teacher_positions", syncItems(ctx, p, p.source.FetchTeacherPositions, p.sync.SyncTeacherPosition, "teacher_positions"))
	record("student_positions", syncItems(ctx, p, p.source.FetchStudentPositions, p.sync.SyncStudentPosition, "student_positions"))
	record("student_subgroups", syncItems(ctx, p, p.source.FetchStudentSubgroups, p.sync.SyncStudentSubgroup, "student_subgroups"))

	// Phase 4: assignments (depend on positions + courses + semesters)
	record("teaching_assignments", syncItems(ctx, p, p.source.FetchTeachingAssignments, p.sync.SyncTeachingAssignment, "teaching_assignments"))
	record("admin_appointments", syncItems(ctx, p, p.source.FetchAdminAppointments, p.sync.SyncAdminAppointment, "admin_appointments"))

	elapsed := time.Since(start)
	if firstErr != nil {
		p.logger.Warn("university sync completed with errors", slog.Duration("elapsed", elapsed))
	} else {
		p.logger.Info("university sync completed", slog.Duration("elapsed", elapsed))
	}
	return firstErr
}

// syncItems fetches items from the source and syncs each one.
func syncItems[T any](ctx context.Context, p *Puller, fetch func(context.Context) ([]T, error), syncOne func(context.Context, T) error, label string) error {
	items, err := fetch(ctx)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	if items == nil {
		return nil // source does not provide this entity
	}

	var errs int
	for _, item := range items {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := syncOne(ctx, item); err != nil {
			p.logger.Warn("sync item failed", slog.String("entity", label), slog.Any("error", err))
			errs++
		}
	}

	p.logger.Info("synced entity", slog.String("entity", label), slog.Int("total", len(items)), slog.Int("errors", errs))
	if errs > 0 {
		return fmt.Errorf("%d of %d items failed", errs, len(items))
	}
	return nil
}

func (p *Puller) syncHierarchy(ctx context.Context, fetch func(context.Context) ([]HierarchyNodeInput, error), level HierarchyLevel, label string) error {
	items, err := fetch(ctx)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	if items == nil {
		return nil
	}

	var errs int
	for _, item := range items {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := p.sync.SyncHierarchyNode(ctx, level, item); err != nil {
			p.logger.Warn("sync hierarchy node failed", slog.String("entity", label), slog.String("code", item.Code), slog.Any("error", err))
			errs++
		}
	}

	p.logger.Info("synced entity", slog.String("entity", label), slog.Int("total", len(items)), slog.Int("errors", errs))
	if errs > 0 {
		return fmt.Errorf("%d of %d items failed", errs, len(items))
	}
	return nil
}
