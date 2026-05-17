package cmd

import "testing"

func TestAssertReadOnlyStmt(t *testing.T) {
	cases := []struct {
		name    string
		stmt    string
		wantErr bool
	}{
		{"select", "SELECT 1", false},
		{"with", "WITH x AS (SELECT 1) SELECT * FROM x", false},
		{"explain", "EXPLAIN SELECT * FROM transactions", false},
		{"pragma", "PRAGMA user_version", false},
		{"lowercase select", "select 1", false},
		{"leading comment + select", "-- comment\nSELECT 1", false},
		{"trailing semicolon", "SELECT 1;", false},
		{"insert", "INSERT INTO items VALUES (1)", true},
		{"update", "UPDATE items SET id = '2'", true},
		{"delete", "DELETE FROM items", true},
		{"drop", "DROP TABLE items", true},
		{"multi-statement", "SELECT 1; DELETE FROM items", true},
		{"empty after comments", "-- nothing\n/* still nothing */", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := assertReadOnlyStmt(tc.stmt)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
