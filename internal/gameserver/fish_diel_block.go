package gameserver

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

// fish_diel_block.json 记录「某个半天的活跃度是愁（基本不出）」的鱼种，
// 由 GameKee 鱼种图鉴的日间/夜间活跃度图标转录而来（2026-11-19 抓取）。
// 值只有两种："day"（白天不出）、"night"（夜里不出），两者都写表示整天都不活跃。
//
//go:embed assets/fish_diel_block.json
var fishDielBlockJSON []byte

var (
	fishDielBlockOnce sync.Once
	fishDielBlock     map[string]map[string]bool
)

func fishDielBlocked() map[string]map[string]bool {
	fishDielBlockOnce.Do(func() {
		fishDielBlock = decodeFishDielBlock(fishDielBlockJSON)
	})
	return fishDielBlock
}

func decodeFishDielBlock(raw []byte) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	var src map[string][]string
	if err := json.Unmarshal(raw, &src); err != nil {
		return out
	}
	for key, halves := range src {
		set := map[string]bool{}
		for _, h := range halves {
			set[strings.ToLower(strings.TrimSpace(h))] = true
		}
		out[key] = set
	}
	return out
}

// fishDielBlockedFor 判断该鱼的某个半天是否被标为「基本不出」。
// half 取 "day" 或 "night"。
func fishDielBlockedFor(fishKey, half string) bool {
	blocked, ok := fishDielBlocked()[fishKey]
	return ok && blocked[half]
}
