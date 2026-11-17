package store

import "testing"

// 渔具三类的估价不得走 kind 链路：item_catalog.kind 把大量轮标成
// misc/rod_spinning（如 bclp_* 水滴轮），按 kind 中位会把轮估到便宜配件
// 价（几十银），还会丢失金币价。修复后：竿 3 / 轮 6 / 饵 17 直接用
// 品类中位价，金币按 silver/110 换算。
func TestShopAugmentGearUsesCategoryMedianAndGold(t *testing.T) {
	d := openShopCatalogTestDB(t)
	ctx := t.Context()
	// 灌真实价目：三个同 kind（misc）但不同品类的商品，轮价远高于 misc 其它件。
	seed := []struct {
		id        string
		cat       int64
		kind      string
		silver    int64
		withPrice bool
	}{
		{"reel_a", 6, "misc", 400000, true},
		{"reel_b", 6, "misc", 500000, true},
		{"misc_cheap_1", 40, "misc", 100, true},
		{"misc_cheap_2", 40, "misc", 200, true},
		{"misc_cheap_3", 40, "misc", 300, true},
		// 待估价的轮：同 kind=misc、无任何价目
		{"reel_estimate_me", 6, "misc", 0, false},
	}
	for _, s := range seed {
		if _, err := d.SQL.ExecContext(ctx,
			`INSERT INTO item_def (item_id, category) VALUES (?, ?)`, s.id, s.cat); err != nil {
			t.Fatalf("seed item_def %s: %v", s.id, err)
		}
		if _, err := d.SQL.ExecContext(ctx,
			`INSERT OR REPLACE INTO item_catalog (item_id, kind, name) VALUES (?, ?, ?)`, s.id, s.kind, s.id); err != nil {
			t.Fatalf("seed item_catalog %s: %v", s.id, err)
		}
		if s.withPrice {
			if _, err := d.SQL.ExecContext(ctx,
				`INSERT INTO item_price (item_id, silver, gold, stock) VALUES (?, ?, 0, 100)`, s.id, s.silver); err != nil {
				t.Fatalf("seed item_price %s: %v", s.id, err)
			}
		}
	}
	def := shopStoreDefinition{Kind: 0}
	shelf := []*ShopPrice{{ItemID: "reel_a", Silver: 400000, Stock: 1}}
	aug, _, err := d.augmentShopShelf(ctx, def, shelf)
	if err != nil {
		t.Fatalf("augmentShopShelf: %v", err)
	}
	p := findShopPrice(t, aug, "reel_estimate_me")
	if p.Silver < 100000 {
		t.Fatalf("轮估价 %d 银，仍落在 misc 中位量级，期望 ≥ 同品类中位（40万+）", p.Silver)
	}
	if p.Gold <= 0 {
		t.Fatal("轮估价缺金币价")
	}
	want := int64(float64(p.Silver)/110 + 0.5)
	if p.Gold != want {
		t.Fatalf("金币价 %d，应为银价 1/110 = %d", p.Gold, want)
	}
}

// 原货架上的商店实录/静态价目保持官方语义：shop_item 里 gold=0 的商品是
// 「纯银购买」，augmentShopShelf 不得改写（上游 TestStoreCurrencyAndWalletAtomic /
// TestTungusShopShelfUsesPricedLocalGoods 同口径）。只有本次补齐新增的商品才
// 按 1 金 ≈ 110 银生成金币价。
func TestShopShelfKeepsRecordedGoldPrices(t *testing.T) {
	d := openShopCatalogTestDB(t)
	ctx := t.Context()
	// 直接调用 augmentShopShelf：需要 metas 里能查到类别。
	if err := d.loadShopSupplement(ctx); err != nil {
		t.Fatalf("loadShopSupplement: %v", err)
	}
	shelf := []*ShopPrice{
		{ItemID: "ticket_boat_x_24", Silver: 350, Gold: 0, Stock: 9}, // category 39 → 不补
		{ItemID: "mono_line_01", Silver: 2000, Gold: 0, Stock: 9},    // 线 → 补 18
		{ItemID: "already_gold", Silver: 500, Gold: 7, Stock: 9},     // 已有金币价 → 不动
		{ItemID: "unknown_item", Silver: 1100, Gold: 0, Stock: 9},    // metas 查不到 → 保守不补
	}
	d.shopSupplement.metas["ticket_boat_x_24"] = shopItemMeta{Category: 39}
	d.shopSupplement.metas["mono_line_01"] = shopItemMeta{Category: 12}
	def := shopStoreDefinition{Kind: 0}
	out, _, err := d.augmentShopShelf(ctx, def, shelf)
	if err != nil {
		t.Fatalf("augmentShopShelf: %v", err)
	}
	ticket := findShopPrice(t, out, "ticket_boat_x_24")
	if ticket.Gold != 0 {
		t.Fatalf("船票金币价被补成 %d，应保持 0", ticket.Gold)
	}
	line := findShopPrice(t, out, "mono_line_01")
	if line.Gold != 0 {
		t.Fatalf("原货架商品金币价被补成 %d，应保持官方价目 0", line.Gold)
	}
	if g := findShopPrice(t, out, "already_gold").Gold; g != 7 {
		t.Fatalf("已有金币价被覆盖成 %d", g)
	}
	if g := findShopPrice(t, out, "unknown_item").Gold; g != 0 {
		t.Fatalf("metas 外的商品金币价被补成 %d，应保守不补", g)
	}
}
