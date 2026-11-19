package store

import (
	"context"
	"path/filepath"
	"testing"
)

func openFishWeightTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenWithStatic(filepath.Join(t.TempDir(), "fish-weight.db"), filepath.Join("..", "..", "server_data", "static.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	return db
}

// 每个水域的权重行必须与该水域名册一一对应：既不能少（少一条鱼就少一种可能），
// 也不能多（多出来的鱼会被判鱼逻辑追加进候选池，等于该水域凭空出现本地没有的鱼）。
func TestFishWeightOverrideMatchesRoster(t *testing.T) {
	db := openFishWeightTestDB(t)
	ctx := context.Background()
	var locs, overridden int
	if err := db.SQL.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT loc) FROM loc_wiki_fish`).Scan(&locs); err != nil {
		t.Fatal(err)
	}
	if err := db.SQL.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT loc) FROM fish_weight_override WHERE updated_by='rf4info'`).Scan(&overridden); err != nil {
		t.Fatal(err)
	}
	if overridden != locs {
		t.Fatalf("覆盖权重的水域=%d，名册水域=%d", overridden, locs)
	}
	var missing, extra int
	if err := db.SQL.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM loc_wiki_fish w
		LEFT JOIN fish_weight_override o ON o.loc=w.loc AND o.fish_key=w.fish_key
		WHERE o.fish_key IS NULL`).Scan(&missing); err != nil {
		t.Fatal(err)
	}
	if missing != 0 {
		t.Fatalf("有 %d 条名册鱼没有权重", missing)
	}
	if err := db.SQL.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM fish_weight_override o
		LEFT JOIN loc_wiki_fish w ON w.loc=o.loc AND w.fish_key=o.fish_key
		WHERE w.fish_key IS NULL`).Scan(&extra); err != nil {
		t.Fatal(err)
	}
	if extra != 0 {
		t.Fatalf("有 %d 条权重不属于该水域名册", extra)
	}
}

// 权重是图内相对值：正数、且每图最大值归一为 100。
func TestFishWeightOverrideNormalizedPerWater(t *testing.T) {
	db := openFishWeightTestDB(t)
	ctx := context.Background()
	var invalid int
	if err := db.SQL.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM fish_weight_override WHERE weight <= 0`).Scan(&invalid); err != nil {
		t.Fatal(err)
	}
	if invalid != 0 {
		t.Fatalf("有 %d 条权重非正数", invalid)
	}
	rows, err := db.SQL.QueryContext(ctx,
		`SELECT loc, MAX(weight), MIN(weight) FROM fish_weight_override GROUP BY loc`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var loc string
		var maxW, minW float64
		if err := rows.Scan(&loc, &maxW, &minW); err != nil {
			t.Fatal(err)
		}
		if maxW != 100 {
			t.Errorf("%s 的最大权重=%v，期望 100", loc, maxW)
		}
		if minW < 1 {
			t.Errorf("%s 的最小权重=%v，期望不小于 1", loc, minW)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

// 迁移必须可重复执行：行数与数值不变。
func TestFishWeightOverrideMigrationIsRepeatable(t *testing.T) {
	db := openFishWeightTestDB(t)
	ctx := context.Background()
	var before int
	if err := db.SQL.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM fish_weight_override`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	body, err := migrationsFS.ReadFile("migrations/00071_fish_weight_official.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL.ExecContext(ctx, string(body)); err != nil {
		t.Fatal(err)
	}
	var after int
	if err := db.SQL.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM fish_weight_override`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("重复执行后行数 %d → %d", before, after)
	}
}
