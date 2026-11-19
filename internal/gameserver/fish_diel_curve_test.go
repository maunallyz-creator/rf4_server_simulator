package gameserver

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFishDielCurveWeightsParsing(t *testing.T) {
	if w := dielCurveWeights(""); w != nil {
		t.Fatalf("空曲线应返回 nil，得到 %v", w)
	}
	if w := dielCurveWeights("[1,0.5,0.2]"); w != nil {
		t.Fatalf("非四段曲线应返回 nil，得到 %v", w)
	}
	if w := dielCurveWeights("not json"); w != nil {
		t.Fatalf("非法 JSON 应返回 nil，得到 %v", w)
	}
	w := dielCurveWeights("[1,0.4,0.35,0.6]")
	if len(w) != 4 || w[0] != 1 || w[1] != 0.4 || w[3] != 0.6 {
		t.Fatalf("四段曲线解析错误: %v", w)
	}
}

func TestFishDielWeightUsesCurveAsWindow(t *testing.T) {
	// 夜行鱼：夜里满权重，白天明显衰减但不为零（窗口期语义）。
	p := fishEcologyProfile{DielCurve: "[1,0.25,0.12,0.6]", DielMask: 0b1111}
	cases := []struct {
		hour int64
		want float64
	}{
		{23, 1},   // 夜
		{3, 1},    // 夜
		{6, 0.25}, // 晨
		{12, 0.12},
		{18, 0.6},
	}
	for _, c := range cases {
		if got := fishDielWeight(p, c.hour); got != c.want {
			t.Fatalf("hour=%d 期望 %.2f 实得 %.2f", c.hour, c.want, got)
		}
	}
}

func TestFishDielWeightFallsBackToMask(t *testing.T) {
	// 无曲线时仍按位图：命中 1.0，未命中 0.35。
	day := fishEcologyProfile{DielMask: 1 << 2} // 只允许白天
	if got := fishDielWeight(day, 12); got != 1 {
		t.Fatalf("白天命中位图应得 1，实得 %.2f", got)
	}
	if got := fishDielWeight(day, 23); got != .35 {
		t.Fatalf("夜间未命中位图应得 0.35，实得 %.2f", got)
	}
}

func TestFishDielCurveIsMonotonicUnderPeak(t *testing.T) {
	// 曲线峰值必须是 1，否则「窗口期」会整体压低该鱼。
	var raw []fishEcologyProfile
	if err := json.Unmarshal(fishEcologyDefsJSON, &raw); err != nil {
		t.Fatalf("加载生态基线失败: %v", err)
	}
	seen := 0
	for _, p := range raw {
		key := p.FishKey
		w := dielCurveWeights(p.DielCurve)
		if w == nil {
			continue
		}
		seen++
		peak := 0.0
		for _, v := range w {
			if v > peak {
				peak = v
			}
			if v < 0 || v > 1 {
				t.Fatalf("%s 曲线越界: %v", key, w)
			}
		}
		if peak < 0.99 {
			t.Fatalf("%s 曲线峰值 %.2f 未归一到 1: %v", key, peak, w)
		}
	}
	if seen < 200 {
		t.Fatalf("带曲线的鱼种过少: %d", seen)
	}
}

func TestFishDielCurveCoversGlobalAndLocalHours(t *testing.T) {
	// 曲线按游戏小时取段，跨日边界必须一致（0 点与 24 点同段）。
	p := fishEcologyProfile{DielCurve: "[0.5,1,1,0.8]"}
	if a, b := fishDielWeight(p, 0), fishDielWeight(p, 24); a != b {
		t.Fatalf("0 点与 24 点应同权重，得到 %.2f vs %.2f", a, b)
	}
	if got := fishDielWeight(p, time.Now().Unix()%24); got <= 0 || got > 1 {
		t.Fatalf("任意小时权重越界: %.2f", got)
	}
}
