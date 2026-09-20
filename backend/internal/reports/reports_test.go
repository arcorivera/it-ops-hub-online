package reports

import "testing"

func TestReplaceWord_WholeWordOnly(t *testing.T) {
	cases := []struct {
		input, old, new, want string
	}{
		{"WHERE 1=1 AND created_at >= ?", "created_at", "t.created_at", "WHERE 1=1 AND t.created_at >= ?"},
		{"WHERE severity = ?", "severity", "t.severity", "WHERE t.severity = ?"},
		// Must NOT match "team_id" when replacing "id" or partial words.
		{"WHERE team_id = ?", "team_id", "t.team_id", "WHERE t.team_id = ?"},
		// Must not corrupt a column name that merely contains the target as a substring.
		{"WHERE project_id = ? AND severity = ?", "project_id", "t.project_id", "WHERE t.project_id = ? AND severity = ?"},
	}
	for _, c := range cases {
		got := replaceWord(c.input, c.old, c.new)
		if got != c.want {
			t.Errorf("replaceWord(%q, %q, %q) = %q, want %q", c.input, c.old, c.new, got, c.want)
		}
	}
}

func TestReplaceWord_DoesNotDoubleMatchSubstring(t *testing.T) {
	// "team_id" contains no other target words, but ensure replacing
	// "team_id" doesn't also corrupt "created_at" appearing later, and
	// vice versa — i.e. sequential prefixColumns replacements compose safely.
	input := "WHERE 1=1 AND created_at >= ? AND team_id = ? AND project_id = ? AND severity = ?"
	got := prefixColumns(input)
	want := "WHERE 1=1 AND t.created_at >= ? AND t.team_id = ? AND t.project_id = ? AND t.severity = ?"
	if got != want {
		t.Errorf("prefixColumns(%q) = %q, want %q", input, got, want)
	}
}

func TestReplaceWord_NoFalsePositiveOnPartialIdentifier(t *testing.T) {
	// A hypothetical column "my_severity_level" should NOT have "severity"
	// replaced inside it, since that would corrupt the identifier.
	input := "WHERE my_severity_level = ?"
	got := replaceWord(input, "severity", "t.severity")
	if got != input {
		t.Errorf("expected no replacement inside a longer identifier, got %q", got)
	}
}

func TestFilterWhereClause_BuildsExpectedSQL(t *testing.T) {
	f := Filter{StartDate: "2026-01-01", EndDate: "2026-01-31", TeamID: "team-1", Severity: "S1"}
	where, args := f.whereClause()

	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(args), args)
	}
	if args[0] != "2026-01-01" || args[3] != "S1" {
		t.Errorf("unexpected arg order: %v", args)
	}
	if where == "" {
		t.Error("expected a non-empty WHERE clause")
	}
}

func TestFilterWhereClause_EmptyFilterIsPermissive(t *testing.T) {
	f := Filter{}
	where, args := f.whereClause()
	if where != "WHERE 1=1" {
		t.Errorf("expected permissive WHERE 1=1 for empty filter, got %q", where)
	}
	if len(args) != 0 {
		t.Errorf("expected no args for empty filter, got %v", args)
	}
}
