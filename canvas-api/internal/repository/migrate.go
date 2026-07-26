package repository

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"time"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate 顺序执行 migrations/ 下的所有 .sql 文件（按文件名排序）。
// 每条迁移都写成幂等（IF NOT EXISTS），可安全重复运行。
// canvas-api 只创建/维护 canvas_* 表，绝不触碰 sub2api 的表。
func (d *DB) Migrate(ctx context.Context) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, name := range names {
		sqlBytes, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := d.sql.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("exec migration %s: %w", name, err)
		}
	}
	return nil
}
