package gormsqlite

import (
	"strings"
	"testing"
)

func TestBuildDSNIncludesPerConnectionPragmas(t *testing.T) {
	reader := buildDSN("./db.sqlite", true)
	writer := buildDSN("./db.sqlite", false)

	checks := []string{
		"_pragma=journal_mode(WAL)",
		"_pragma=synchronous(NORMAL)",
		"_pragma=foreign_keys(1)",
		"_pragma=busy_timeout(5000)",
		"_pragma=trusted_schema(OFF)",
	}
	for _, c := range checks {
		if !strings.Contains(reader, c) {
			t.Fatalf("reader dsn missing %q: %s", c, reader)
		}
		if !strings.Contains(writer, c) {
			t.Fatalf("writer dsn missing %q: %s", c, writer)
		}
	}

	if !strings.Contains(reader, "_pragma=query_only(1)") {
		t.Fatalf("reader dsn missing query_only(1): %s", reader)
	}
	if !strings.Contains(writer, "_pragma=query_only(0)") {
		t.Fatalf("writer dsn missing query_only(0): %s", writer)
	}
}
