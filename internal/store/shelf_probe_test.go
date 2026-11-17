package store

import (
	"context"
	"testing"
)

func TestProbeVakulaShelfShape(t *testing.T) {
	d := openShopCatalogTestDB(t)
	shelf, err := d.effectiveShopShelf(context.Background(), "vakula_013")
	if err != nil {
		t.Fatal(err)
	}
	noGoldNoMeta, noGoldHasMeta, goldOK := 0, 0, 0
	var sampleNoGold, sampleNoMeta string
	for _, p := range shelf.Effective {
		if p.Silver > 0 && p.Gold <= 0 {
			if m, ok := d.shopSupplement.metas[p.ItemID]; ok {
				noGoldHasMeta++
				if sampleNoGold == "" {
					sampleNoGold = p.ItemID + " cat=" + itoa(m.Category)
				}
			} else {
				noGoldNoMeta++
				if sampleNoMeta == "" {
					sampleNoMeta = p.ItemID
				}
			}
		} else {
			goldOK++
		}
	}
	t.Logf("gold ok=%d, noGoldHasMeta=%d(%s), noGoldNoMeta=%d(%s)",
		goldOK, noGoldHasMeta, sampleNoGold, noGoldNoMeta, sampleNoMeta)
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	if neg {
		return "-" + digits
	}
	return digits
}
