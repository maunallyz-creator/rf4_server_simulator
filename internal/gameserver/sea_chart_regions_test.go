package gameserver

import (
	"math"
	"testing"
)

func TestSeaGuideAreasOnPrintedChart(t *testing.T) {
	// Pixels read from the 2048×2048 chart, including land and an unnumbered bank.
	for _, tc := range []struct {
		name       string
		u, v, spot int
	}{
		{"南部90区远离数字", 550, 1476, 90},
		{"港湾的另一处90区", 361, 1040, 90},
		{"北部左侧190区", 1290, 706, 190},
		{"北部右侧190区", 1775, 743, 190},
		{"在70区实际下饵", 1182, 850, 70},
		{"同一色块靠近92标注", 970, 722, 92},
		{"同一色块靠近83标注", 975, 723, 83},
		{"34区浅滩", 940, 1310, 34},
		{"41区浅滩", 700, 1550, 41},
		{"55区浅滩", 1500, 1200, 55},
		{"无标注的斜线浅滩", 1040, 930, 0},
		{"港口12区浅水", 165, 936, 12},
		{"北侧13区浅水", 198, 809, 13},
		{"西侧9区浅水", 152, 1089, 9},
		{"9区岸上的地形", 189, 1082, 0},
		{"陆地", 380, 380, 0},
		{"图例", 1650, 165, 0},
		{"图外", 2100, 1000, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x, z := (float64(tc.u)-78)/.38, (1970-float64(tc.v))/.38-1957
			if got := seaGuideSpotForPoint(x, z); got != tc.spot {
				t.Errorf("海图像素 (%d,%d) 标深=%d，期望 %d", tc.u, tc.v, got, tc.spot)
			}
		})
	}
	if seaGuideSpotForPoint(math.NaN(), 0) != 0 || seaGuideSpotForPoint(0, math.Inf(1)) != 0 {
		t.Fatal("无效的落点应视为未知")
	}
	for _, tc := range []struct {
		spot int
		want bool
	}{{75, true}, {120, true}, {133, false}, {144, false}, {0, false}} {
		if got := isSeaGuideSpot(tc.spot); got != tc.want {
			t.Errorf("标深 %d 有效性=%t，期望 %t", tc.spot, got, tc.want)
		}
	}
}
