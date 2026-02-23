package sqlite

import "testing"

func TestDotPathToSQLiteJSONPathQuotesSegments(t *testing.T) {
	got := dotPathToSQLiteJSONPath("customer.first-name")
	want := `$."customer"."first-name"`
	if got != want {
		t.Fatalf("unexpected path: got %s want %s", got, want)
	}
}
