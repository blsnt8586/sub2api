package migrations

import (
	"strings"
	"testing"
)

func TestSub2APIOptimizeGroupMigrationIsAdditiveAndOptional(t *testing.T) {
	content, err := FS.ReadFile("239_accounts_add_sub2api_optimize_group_id.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{
		"add column if not exists sub2api_optimize_group_id bigint",
		"null 表示不限制",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
	if strings.Contains(sql, "not null") || strings.Contains(sql, "delete from") {
		t.Fatal("optional restriction migration must not rewrite or delete existing account data")
	}
}
