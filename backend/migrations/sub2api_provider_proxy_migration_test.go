package migrations

import (
	"strings"
	"testing"
)

func TestSub2APIProviderProxyMigrationIsAdditive(t *testing.T) {
	content, err := FS.ReadFile("234_sub2api_provider_proxy.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(content))
	for _, required := range []string{
		"add column if not exists proxy_id bigint null",
		"foreign key (proxy_id) references proxies(id) on delete set null",
		"create index if not exists idx_sub2api_providers_proxy_id",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Contains(sql, "update sub2api_providers set proxy_id") {
		t.Fatal("migration must not change existing provider routing")
	}
}
