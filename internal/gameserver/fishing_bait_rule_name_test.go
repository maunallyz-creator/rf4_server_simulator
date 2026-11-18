package gameserver

import "testing"

// 客户端鱼名与规则表长期存在同物异写，差一个码位就整条规则失效：
// 「大西洋鲑」vs「大西洋鲑鱼」、「泥鳅鱼」vs「泥鳅」、
// 「钓鮟鱇」vs「钓𩽾𩾌」（异体字）。lookupFishBaitRule 必须把它们并起来。
func TestLookupFishBaitRuleToleratesNameVariants(t *testing.T) {
	pairs := [][2]string{
		{"大西洋鲑", "大西洋鲑鱼"},
		{"泥鳅鱼", "泥鳅"},
		{"钓鮟鱇", "钓𩽾𩾌"},
	}
	for _, p := range pairs {
		a, okA := lookupFishBaitRule(p[0])
		b, okB := lookupFishBaitRule(p[1])
		if !okA || !okB {
			t.Fatalf("%s/%s：命中最少一边失败 ok=%v/%v", p[0], p[1], okA, okB)
		}
		if len(a.allow) != len(b.allow) || len(a.allow) == 0 {
			t.Fatalf("%s/%s：允许列表不一致 %v vs %v", p[0], p[1], a.allow, b.allow)
		}
	}
	if _, ok := lookupFishBaitRule("完全不存在的鱼"); ok {
		t.Fatal("不存在的鱼不应命中规则")
	}
	if _, ok := lookupFishBaitRule(""); ok {
		t.Fatal("空名不应命中规则")
	}
}
