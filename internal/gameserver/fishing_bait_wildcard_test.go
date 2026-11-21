package gameserver

import (
	"fmt"
	"testing"
	"time"
)

// 「各类饵」是全通配：规则里写它，等于接受所有 category 17/27 的饵。
// 狗鱼、鲑鱼这类规则一旦用它，土豆、煮饵这些和食性无关的饵也会被当成合法饵。
// 这里验证：只靠通配进来的饵，如果官方纪录里这条鱼几乎不吃（占比低于 tail），
// 就不参与判鱼；规则明确点名的饵不受影响。
func TestWildcardBaitNeedsEvidence(t *testing.T) {
	s, db := seedBaitRuleTest(t)
	if _, err := db.SQL.Exec(`INSERT INTO loc_wiki_fish(loc,fish_key,fish_zh) VALUES('rules-test','n.pike','常见狗鱼')`); err != nil {
		t.Fatal(err)
	}
	// 狗鱼 1,000 条官方纪录：999 条 rule_worm（野生蠕虫类）、1 条 rule_other。
	seq := 0
	ins := func(bait string, n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			seq++
			if _, err := db.SQL.Exec(`INSERT INTO rf4records_catch(fish_name_en,fish_key,bait_name,bait_id,weight_g,waterbody,region,categories,trophy_class,catch_date,fetched_at) VALUES('x','n.pike','n','` + bait + `',` + fmt.Sprintf("%d", seq) + `,'','','','','',1)`); err != nil {
				t.Fatal(err)
			}
		}
	}
	ins("rule_worm", 999)
	ins("rule_other", 1)
	// 显式开启分级（seedBaitRuleTest 默认把 bait_share_strong 置 0 关闭分级）。
	s.tunMu.Lock()
	delete(s.tunCache, "bait_share_strong")
	s.tunCache["bait_share_min_count"] = 1
	s.tunAt = time.Now()
	s.tunMu.Unlock()
	s.loadRf4Share()

	now := time.Unix(worldDayEpochUnix+1500, 0)
	keys := func(bait string) []string {
		k, _ := s.filterFishBaitSet("rules-test", []string{bait}, []string{"n.pike"}, []float64{1}, now)
		return k
	}
	// 0.1% 的长尾饵：靠「各类饵」通配进来，但没有证据，出池。
	if got := keys("rule_other"); len(got) != 0 {
		t.Fatalf("wildcard-only long tail bait kept: %v", got)
	}
	// 99.9% 的饵：同样只靠通配，但有强证据，保留。
	if got := keys("rule_worm"); len(got) != 1 {
		t.Fatalf("wildcard bait with evidence dropped: %v", got)
	}
}
