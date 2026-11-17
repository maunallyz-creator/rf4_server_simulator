package store

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
)

// 商店货架补齐：历史抓取的 shop_item 与内嵌 kind 模板都不完整，新商品
// （例如 2025 款限量产品）没有货架行，客户端就会显示"无库存"。
// 这里按“店内已有品类 +（品牌店）同品牌”把缺的商品补回货架，
// 价格缺失时用同类商品的中位价估价。管理机构按目录登记的 books 白名单
// 上架品类 80 的技能书（见 shopBookCategory）。

// partCategories 是装配零件品类，不单独在商店售卖。
// shopTicketCategory 船票（boatstation）只收银币，金币价保持 0。
const shopTicketCategory int64 = 39

var partCategories = map[int64]bool{
	4: true, 5: true, 7: true, 8: true, 9: true, 10: true,
	34: true, 36: true, 52: true, 63: true, 81: true,
}

// shopGearCategories 是“品牌店也不按品牌过滤”的核心渔具品类：鱼竿 3、
// 卷线器 6、拟饵 17。客户端把 22/1 响应里缺失的商品统一标成
// kDlg_Shop_Obsolete（停产），品牌店实录货架本身跨十几个品牌，若继续按
// def.Brands 过滤，剩下约 356 根竿 / 415 个轮 / 912 个饵会被整片判成停产。
var shopGearCategories = map[int64]bool{
	3: true, 6: true, 17: true,
}

// shopKindTackle 是 shop_kind_defs.json 里 “钓具店” 的 kind 值；只有这类店
// 才放开 shopGearCategories 的品牌过滤，咖啡馆/杂货铺保持原有品牌限制。
const shopKindTackle = 0

// 管理机构的书籍货架：shop_kind_defs.json 按地区登记具体书目，
// 技能书品类为 80，与店内地图（40）不同类，且静态 item_def 中品牌为空，
// 通用补齐规则（同品类 + 同品牌）永远补不到它们，必须按白名单显式上架。
// 所有技能书统一挂银币 998500 / 金币 500 两个有效价，避免各店静态价目漂移。
const (
	shopBookCategory    int64 = 80
	shopBookSilverPrice int64 = 998500
	shopBookGoldPrice   int64 = 500
)

func shopBookIDs(def shopStoreDefinition) map[string]bool {
	out := make(map[string]bool, len(def.Books))
	for _, id := range def.Books {
		if id = strings.TrimSpace(id); id != "" {
			out[id] = true
		}
	}
	return out
}

type shopItemMeta struct {
	Category int64
	Kind     string
	Brand    string
	Name     string
}

type shopSupplement struct {
	mu           sync.Mutex
	ready        bool
	err          error
	metas        map[string]shopItemMeta
	prices       map[string]*ShopPrice
	medianByKind map[string]int64
	medianByCat  map[int64]int64
	seriesMax    map[string]int64
}

func (d *DB) loadShopSupplement(ctx context.Context) error {
	d.shopSupplement.mu.Lock()
	defer d.shopSupplement.mu.Unlock()
	if d.shopSupplement.ready {
		return nil
	}
	d.shopSupplement.err = nil
	func() {
		d.shopSupplement.metas = make(map[string]shopItemMeta, 4096)
		d.shopSupplement.prices = make(map[string]*ShopPrice, 4096)

		rows, err := d.SQL.QueryContext(ctx, `
			SELECT d.item_id, d.category, COALESCE(d.brand,''), COALESCE(c.kind,''), COALESCE(c.name,'')
			FROM item_def d
			LEFT JOIN item_catalog c ON c.item_id = d.item_id`)
		if err != nil {
			d.shopSupplement.err = err
			return
		}
		for rows.Next() {
			var id, brand, kind, name string
			var cat int64
			if err := rows.Scan(&id, &cat, &brand, &kind, &name); err != nil {
				rows.Close()
				d.shopSupplement.err = err
				return
			}
			d.shopSupplement.metas[id] = shopItemMeta{Category: cat, Kind: kind, Brand: brand, Name: name}
		}
		rowErr := rows.Err()
		rows.Close()
		if rowErr != nil {
			d.shopSupplement.err = rowErr
			return
		}

		prows, perr := d.SQL.QueryContext(ctx,
			`SELECT item_id, silver, gold, stock FROM item_price`)
		if perr != nil {
			d.shopSupplement.err = perr
			return
		}
		if perr == nil {
			for prows.Next() {
				p := &ShopPrice{}
				if err := prows.Scan(&p.ItemID, &p.Silver, &p.Gold, &p.Stock); err != nil {
					prows.Close()
					d.shopSupplement.err = err
					return
				}
				d.shopSupplement.prices[p.ItemID] = p
			}
			priceErr := prows.Err()
			prows.Close()
			if priceErr != nil {
				d.shopSupplement.err = priceErr
				return
			}
		}

		// shop_item 是真实货架价，覆盖 item_price 里没有的整件商品（竿/轮等）。
		srows, serr := d.SQL.QueryContext(ctx,
			`SELECT item_id, silver, gold, stock FROM shop_item`)
		if serr != nil {
			d.shopSupplement.err = serr
			return
		}
		if serr == nil {
			for srows.Next() {
				p := &ShopPrice{}
				if err := srows.Scan(&p.ItemID, &p.Silver, &p.Gold, &p.Stock); err != nil {
					srows.Close()
					d.shopSupplement.err = err
					return
				}
				cur, ok := d.shopSupplement.prices[p.ItemID]
				if !ok || (cur.Silver <= 0 && p.Silver > 0) {
					d.shopSupplement.prices[p.ItemID] = p
				}
			}
			shelfErr := srows.Err()
			srows.Close()
			if shelfErr != nil {
				d.shopSupplement.err = shelfErr
				return
			}
		}

		// 中位价：先按 item_catalog.kind，再按 item_def.category 兜底。
		byKind := map[string][]int64{}
		byCat := map[int64][]int64{}
		for id, p := range d.shopSupplement.prices {
			if p.Silver <= 0 {
				continue
			}
			m, ok := d.shopSupplement.metas[id]
			if !ok {
				continue
			}
			if m.Kind != "" {
				byKind[m.Kind] = append(byKind[m.Kind], p.Silver)
			}
			byCat[m.Category] = append(byCat[m.Category], p.Silver)
		}
		d.shopSupplement.medianByKind = medianMap(byKind)
		d.shopSupplement.medianByCat = medianMapInt64(byCat)

		// 系列最高价：同 kind + 同系列首词 + 同尺寸数字。
		// 估价时取“同类同级里最贵的那个”，保证只高不低。
		d.shopSupplement.seriesMax = make(map[string]int64, len(d.shopSupplement.prices))
		for id, p := range d.shopSupplement.prices {
			eff := p.Silver
			if eff <= 0 && p.Gold > 0 {
				eff = p.Gold * 100
			}
			if eff <= 0 {
				continue
			}
			m, ok := d.shopSupplement.metas[id]
			if !ok {
				continue
			}
			key := shopSeriesKey(m.Kind, m.Name)
			if key == "" {
				continue
			}
			if eff > d.shopSupplement.seriesMax[key] {
				d.shopSupplement.seriesMax[key] = eff
			}
		}
	}()
	d.shopSupplement.ready = d.shopSupplement.err == nil
	return d.shopSupplement.err
}

func medianMap(in map[string][]int64) map[string]int64 {
	out := make(map[string]int64, len(in))
	for k, v := range in {
		if len(v) == 0 {
			continue
		}
		sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
		out[k] = v[len(v)/2]
	}
	return out
}

func medianMapInt64(in map[int64][]int64) map[int64]int64 {
	out := make(map[int64]int64, len(in))
	for k, v := range in {
		if len(v) == 0 {
			continue
		}
		sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
		out[k] = v[len(v)/2]
	}
	return out
}

func brandMatches(brand string, brands []string) bool {
	b := strings.ToLower(strings.TrimSpace(brand))
	if b == "" {
		return false
	}
	for _, want := range brands {
		w := strings.ToLower(strings.TrimSpace(want))
		if w != "" && strings.HasPrefix(b, w) {
			return true
		}
	}
	return false
}

// augmentShopShelf 补齐货架，返回补齐后的货架与新增条数。
func (d *DB) augmentShopShelf(ctx context.Context, def shopStoreDefinition, shelf []*ShopPrice) ([]*ShopPrice, int, error) {
	if len(shelf) == 0 {
		return shelf, 0, nil
	}
	if err := d.loadShopSupplement(ctx); err != nil {
		return shelf, 0, err
	}
	have := make(map[string]bool, len(shelf))
	cats := make(map[int64]bool)
	bookIDs := shopBookIDs(def)
	for _, p := range shelf {
		if p == nil {
			continue
		}
		have[p.ItemID] = true
		if m, ok := d.shopSupplement.metas[p.ItemID]; ok && !partCategories[m.Category] {
			cats[m.Category] = true
		}
	}
	if len(bookIDs) > 0 {
		cats[shopBookCategory] = true
	}
	if def.Kind == shopKindTackle && len(def.Brands) > 0 {
		// 钓具店的“停产”来源不只是品牌过滤：只要该店 22/1 里没有某个渔具品类，
		// 客户端对应的整个分类页都会被灰显成停产（例如 vakula_013 只进竿、不进轮，
		// 纺车式页 720 个轮全部显示停产）。因此钓具店一律把竿/轮/饵三类纳入补齐。
		for c := range shopGearCategories {
			cats[c] = true
		}
	}
	if len(cats) == 0 {
		return shelf, 0, nil
	}
	add := make([]*ShopPrice, 0, 1024)
	for id, m := range d.shopSupplement.metas {
		if have[id] || !cats[m.Category] || partCategories[m.Category] {
			continue
		}
		isBook := m.Category == shopBookCategory
		if isBook {
			// 技能书只按地区白名单上架，未登记的书一律不补。
			if !bookIDs[id] {
				continue
			}
		} else if len(def.Brands) > 0 && !brandMatches(m.Brand, def.Brands) && !(def.Kind == shopKindTackle && shopGearCategories[m.Category]) {
			// 品牌店（beluga/zeiman/ny_bazaar/vakula…）的非渔具货架仍按品牌过滤。
			// 但渔具店（kind 0）的竿 3 / 轮 6 / 饵 17 不能过滤：客户端拿到 22/1 后，
			// 凡店内货架没有的商品一律灰显并打 kDlg_Shop_Obsolete「停产」标签，而
			// 品牌店的实录基础货架本身就跨十几个品牌（见 vakula_013 / ny_bazaar），
			// 硬按品牌过滤会把剩下 356 竿 / 415 轮 / 912 饵全判成停产。
			// 只放开 kind 0：咖啡馆(1)、农场杂货(7) 等虽然也有带品牌的零散渔具，
			// 但它们是按品牌进的自家货，不该把全服渔具都摆上柜台。
			continue
		}
		if p, ok := d.shopSupplement.prices[id]; ok && (p.Silver > 0 || p.Gold > 0) {
			cp := *p
			if isBook {
				cp.Silver = shopBookSilverPrice
				cp.Gold = shopBookGoldPrice
			}
			if cp.Stock <= 0 {
				cp.Stock = 1000
			}
			add = append(add, &cp)
			continue
		}
		if isBook {
			// 书籍只按静态价目上架，缺价时不估价，避免生成错误的价格。
			continue
		}
		// 优先用“同类同级最贵”的价格，其次同类型中位价。
		// 渔具三大类（竿 3 / 轮 6 / 饵 17）的 item_catalog.kind 数据不可靠
		// （轮被大量标成 misc/rod_spinning，如 bclp_* 水滴轮），seriesMax 与
		// medianByKind 会把轮对到便宜配件的中位价上。这三类直接用同品类
		// （item_def.category）中位价，不再按 kind 估价。
		var silver int64
		if m.Category == 3 || m.Category == 6 || m.Category == 17 {
			silver = d.shopSupplement.medianByCat[m.Category]
		} else {
			silver = d.shopSupplement.seriesMax[shopSeriesKey(m.Kind, m.Name)]
			if silver <= 0 {
				silver = d.shopSupplement.medianByKind[m.Kind]
			}
			if silver <= 0 {
				silver = d.shopSupplement.medianByCat[m.Category]
			}
		}
		if silver <= 0 {
			silver = 1000
		}
		// 价格随机上浮 0~10%（按 item_id 稳定，不会每次刷新都变）
		silver = silver + silver*int64(stableHash(id)%11)/100
		// 金币价与银价同源换算（1 金 ≈ 110 银，与工坊/零件价目同一口径），
		// 不再留 0——客户端金币页没有价就会显示异常。
		gold := int64(math.Round(float64(silver) / 110))
		add = append(add, &ShopPrice{ItemID: id, Silver: silver, Gold: gold, Stock: 1000})
	}
	// 原货架（商店实录 / 静态价目）保持官方价目原样：shop_item 里 gold=0 的商品
	// 在官服是「纯银购买」语义（上游 TestStoreCurrencyAndWalletAtomic /
	// TestTungusShopShelfUsesPricedLocalGoods 都按此断言）。只有本次补齐新增的
	// 商品才按 1 金 ≈ 110 银生成金币价，避免把纯银商品改写成可金币购买。
	sort.Slice(add, func(i, j int) bool { return add[i].ItemID < add[j].ItemID })
	out := make([]*ShopPrice, 0, len(shelf)+len(add))
	out = append(out, shelf...)
	out = append(out, add...)
	return out, len(add), nil
}

// shopSeriesKey 生成"同类型+同系列首词+同尺寸"的键，例如
// rod_spinning|venga|10000。取不到尺寸时返回空串。
func shopSeriesKey(kind, name string) string {
	tokens := strings.Fields(strings.ToLower(name))
	if len(tokens) == 0 || kind == "" {
		return ""
	}
	size := ""
	for i := len(tokens) - 1; i >= 0; i-- {
		if isAllDigits(tokens[i]) {
			size = tokens[i]
			break
		}
	}
	if size == "" {
		return ""
	}
	return kind + "|" + tokens[0] + "|" + size
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// stableHash 是 item_id 的稳定散列，用于价格微浮动。
func stableHash(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}
