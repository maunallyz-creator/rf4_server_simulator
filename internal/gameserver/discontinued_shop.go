package gameserver

import (
	"context"
	"errors"
	"rf4server/internal/store"
	"strings"
	"time"
)

var errDiscontinuedShopClosed = errors.New("钓鱼站停产商品当前不在限时开放时段")

// discontinuedShopChangedSince reports whether any opening/closing boundary was
// crossed since the client's last shelf load, even across an entire game day.
func discontinuedShopChangedSince(since, now time.Time) bool {
	// 本地部署：停产商品改为全天 24 小时开放，不再有时段边界。
	_ = since
	_ = now
	return false
}

func isFishingShop(storeID string) bool {
	id := strings.ToLower(strings.TrimSpace(storeID))
	return strings.HasPrefix(id, "fishingstore_") || strings.HasPrefix(id, "fishing_store_")
}

func discontinuedShopOpenAt(now time.Time) bool {
	// 本地部署：全天 24 小时开放。
	_ = now
	return true
}

func (s *Server) shopItemsAt(ctx context.Context, storeID string, now time.Time) ([]*store.ShopPrice, error) {
	items, err := s.DB.ShopItems(ctx, storeID)
	if err != nil || !isFishingShop(storeID) || discontinuedShopOpenAt(now) {
		return items, err
	}
	out := make([]*store.ShopPrice, 0, len(items))
	for _, p := range items {
		if p != nil && !store.IsDiscontinuedShopItem(p.ItemID) {
			out = append(out, p)
		}
	}
	return out, nil
}
