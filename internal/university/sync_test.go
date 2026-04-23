package university

import (
	"strings"
	"testing"
)

func TestBuildHierarchyNodeUpsert_SortsAllowedExtraColumns(t *testing.T) {
	t.Parallel()

	level := HierarchyLevel{
		Table:       "departments",
		ParentFK:    "faculty_id",
		ParentTable: "faculties",
		ExtraCols:   []string{"short_name", "campus"},
	}

	query, args, err := buildHierarchyNodeUpsert(level, HierarchyNodeInput{
		Code:       "CS",
		ParentCode: "FIT",
		Name:       "Computer Science",
		Extra: map[string]any{
			"short_name": "CS",
			"campus":     "north",
		},
	})
	if err != nil {
		t.Fatalf("buildHierarchyNodeUpsert() error = %v", err)
	}

	wantColumns := "faculty_id, code, name, campus, short_name"
	if !strings.Contains(query, wantColumns) {
		t.Fatalf("query = %q, want columns %q", query, wantColumns)
	}

	if got, want := len(args), 5; got != want {
		t.Fatalf("len(args) = %d, want %d", got, want)
	}

	if got, want := args[0], any("FIT"); got != want {
		t.Fatalf("args[0] = %#v, want %#v", got, want)
	}
	if got, want := args[1], any("CS"); got != want {
		t.Fatalf("args[1] = %#v, want %#v", got, want)
	}
	if got, want := args[2], any("Computer Science"); got != want {
		t.Fatalf("args[2] = %#v, want %#v", got, want)
	}
	if got, want := args[3], any("north"); got != want {
		t.Fatalf("args[3] = %#v, want %#v", got, want)
	}
	if got, want := args[4], any("CS"); got != want {
		t.Fatalf("args[4] = %#v, want %#v", got, want)
	}
}

func TestBuildHierarchyNodeUpsert_RejectsUnknownExtraColumn(t *testing.T) {
	t.Parallel()

	_, _, err := buildHierarchyNodeUpsert(LevelDepartment, HierarchyNodeInput{
		Code:       "CS",
		ParentCode: "FIT",
		Name:       "Computer Science",
		Extra: map[string]any{
			"unsafe": "value",
		},
	})
	if err == nil {
		t.Fatal("buildHierarchyNodeUpsert() error = nil, want rejection for unknown column")
	}
	if !strings.Contains(err.Error(), `does not allow extra column "unsafe"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
