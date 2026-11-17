package gameserver

import (
	"testing"
	"time"

	"rf4server/internal/proto"
)

// setTunableForTest 直接写 tunables 表并让 30 秒缓存立刻失效。
func setTunableForTest(t *testing.T, srv *Server, key, value string) {
	t.Helper()
	if _, err := srv.DB.SQL.Exec(
		`INSERT INTO tunables(key,value) VALUES(?,?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value); err != nil {
		t.Fatalf("写入 tunable %s=%s: %v", key, value, err)
	}
	srv.tunMu.Lock()
	srv.tunCache = nil
	srv.tunText = nil
	srv.tunAt = time.Time{}
	srv.tunMu.Unlock()
}

// TestVirtualPlayerRosterDiversity 校验虚拟玩家名册的规模与多样性。
// 用户明确要求：名字要抽象、中文、英文、乱码多元混合，在线人数要多。
func TestVirtualPlayerRosterDiversity(t *testing.T) {
	t.Parallel()
	all := allVirtualPlayers()
	if len(all) < 60 {
		t.Fatalf("虚拟玩家名册只有 %d 条，至少要 60 条", len(all))
	}
	seenName := make(map[string]bool, len(all))
	seenUID := make(map[int64]bool, len(all))
	var hasPureASCII, hasCJK, hasNonASCII bool
	for _, p := range all {
		if p.Name == "" {
			t.Fatalf("UID %d 的名字为空", p.UID)
		}
		if seenName[p.Name] {
			t.Fatalf("名字重复: %q", p.Name)
		}
		seenName[p.Name] = true
		if seenUID[p.UID] {
			t.Fatalf("UID 重复: %d", p.UID)
		}
		seenUID[p.UID] = true
		if p.UID < virtualPlayerUIDBase {
			t.Fatalf("UID %d 低于虚拟段基址 %d", p.UID, virtualPlayerUIDBase)
		}
		// 不能撞上两个系统联系人 uid（它们固定占 24/4 的前两位）。
		if p.UID == fishQueryContactUID || p.UID == gearAssistantUID {
			t.Fatalf("UID %d 与系统虚拟联系人冲突", p.UID)
		}
		if p.Level <= 0 || p.Level > 80 {
			t.Fatalf("%q 等级 %d 越界", p.Name, p.Level)
		}
		pureASCII := true
		for _, r := range p.Name {
			if r >= 0x80 {
				pureASCII = false
				hasNonASCII = true
				if r >= 0x4e00 && r <= 0x9fff {
					hasCJK = true
				}
			}
		}
		if pureASCII {
			hasPureASCII = true
		}
	}
	if !hasPureASCII {
		t.Fatal("名册里没有纯 ASCII 风格的名字")
	}
	if !hasCJK {
		t.Fatal("名册里没有中文名字")
	}
	if !hasNonASCII {
		t.Fatal("名册里没有非 ASCII 名字")
	}
}

// TestOnlineVirtualPlayersRespectsTunable 校验 bot_online_count 的钳制行为：
// 0 表示完全关闭（回到改动前行为），超量按名册长度截断。
func TestOnlineVirtualPlayersRespectsTunable(t *testing.T) {
	t.Parallel()
	srv, _ := newProgressSrv(t)
	all := allVirtualPlayers()

	if got := len(srv.onlineVirtualPlayers()); got == 0 {
		t.Fatal("默认配置下在线虚拟玩家不应为 0")
	}
	setTunableForTest(t, srv, "bot_online_count", "0")
	if got := len(srv.onlineVirtualPlayers()); got != 0 {
		t.Fatalf("bot_online_count=0 时仍有 %d 个虚拟在线玩家", got)
	}
	setTunableForTest(t, srv, "bot_online_count", "10")
	if got := len(srv.onlineVirtualPlayers()); got != 10 {
		t.Fatalf("bot_online_count=10 实际 %d", got)
	}
	setTunableForTest(t, srv, "bot_online_count", "100000")
	if got := len(srv.onlineVirtualPlayers()); got != len(all) {
		t.Fatalf("超量时应截断到名册长度 %d，实际 %d", len(all), got)
	}
}

// TestVirtualPlayerCardShape 校验 24/19 资料卡形状：与真实玩家同一 wire
// class，f6 必须是 0（普通玩家），不能是虚拟联系人的 true。
func TestVirtualPlayerCardShape(t *testing.T) {
	t.Parallel()
	p := &virtualPlayer{UID: virtualPlayerUIDBase + 7, Name: "钓神老王", Level: 33, Region: "CN", Trophies: 66}
	v := virtualPlayerCard(p)
	if v == nil {
		t.Fatal("资料卡为空")
	}
	if v.Class != "dgbmlekfpbg" {
		t.Fatalf("wire class = %q, 期望 dgbmlekfpbg", v.Class)
	}
	if got := fieldU32(v, 0); got != uint32(p.UID) {
		t.Fatalf("f0 uid = %d, 期望 %d", got, p.UID)
	}
	if got := fieldStr(v, 1); got != p.Name {
		t.Fatalf("f1 name = %q, 期望 %q", got, p.Name)
	}
	if got := fieldU32(v, 4); got != uint32(p.Level) {
		t.Fatalf("f4 level = %d, 期望 %d", got, p.Level)
	}
	// f6 是“普通玩家(0) vs 系统虚拟联系人(true)”的分水岭，客户端据此决定
	// 是否把这个人当真人展示（能否私聊、能否看资料）。
	if got := fieldU32(v, 6); got != 0 {
		t.Fatalf("f6 = %d, 普通玩家必须是 0", got)
	}
}

// TestBuildVirtualAnglerPushShape 校验世界播报推送符合 24/27 的
// [401, str, u32] 合同，并且 401.f2 是虚拟玩家 uid 而不是自己。
func TestBuildVirtualAnglerPushShape(t *testing.T) {
	t.Parallel()
	srv, _ := newProgressSrv(t)
	kv := fishTVKv("丁鱵", 7777)
	body := buildVirtualAnglerPush(26, 900000012, kv, 64)
	if body == nil {
		t.Fatal("推送 body 为空")
	}
	if reason := checkPushArgsStrict(24, 27, body); reason != "" {
		t.Fatalf("推送形状不合合同: %s", reason)
	}
	if got := body.ArgCount(); got != 3 {
		t.Fatalf("外层 args 长度 %d, 期望 3", got)
	}
	msg := body.ArgItem(0)
	if msg == nil || msg.Class != "lfbdbaffeai" {
		t.Fatalf("args[0] 不是 lfbdbaffeai: %+v", msg)
	}
	if got := fieldU32(msg, 0); got != 26 {
		t.Fatalf("401.f0 kind = %d, 期望 26", got)
	}
	if got := fieldU32(msg, 2); got != 900000012 {
		t.Fatalf("401.f2 uid = %d, 期望虚拟玩家 uid", got)
	}
	if f4 := fieldVal(msg, 4); f4 == nil || f4.Kind != proto.KindRaw {
		t.Fatalf("401.f4 模板参数缺失或形状不对: %+v", f4)
	}
	if got := argStr(body, 1); got != "" {
		t.Fatalf("外层 locationId = %q, 必须是空串", got)
	}
	if got := argU32(body, 2); got != 64 {
		t.Fatalf("外层人数 = %d, 期望 64", got)
	}
	// 形状还要能真正编码出来（Push 会在编码失败时静默拒发）。
	raw, err := proto.EncodeValue(body, srv.table)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	if len(raw) == 0 || raw[0] != 0x03 {
		t.Fatalf("顶层不是 args(0x03): %x", raw)
	}
}

// TestVirtualCatchKindsFallback 校验 bot_chat_kinds 解析：非法值回落默认。
func TestVirtualCatchKindsFallback(t *testing.T) {
	t.Parallel()
	srv, _ := newProgressSrv(t)
	setTunableForTest(t, srv, "bot_chat_kinds", "2,3")
	if got := srv.virtualCatchKinds(); len(got) != 2 {
		t.Fatalf("2,3 → %v", got)
	}
	setTunableForTest(t, srv, "bot_chat_kinds", "3")
	if got := srv.virtualCatchKinds(); len(got) != 1 || got[0] != 3 {
		t.Fatalf("3 → %v", got)
	}
	// 未达标档 1 不参与播报；全非法时回落 {2,3}。
	setTunableForTest(t, srv, "bot_chat_kinds", "1,x,zzz")
	got := srv.virtualCatchKinds()
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("非法值应回落 {2,3}，实际 %v", got)
	}
}

// TestRollVirtualCatchEventWeightInBand 校验抽出来的鱼获重量确实落在
// 该鱼种对应档位的区间里——这是“看起来真”的核心：不能出现 1 克的鲤鱼。
func TestRollVirtualCatchEventWeightInBand(t *testing.T) {
	t.Parallel()
	srv, db := newProgressSrv(t)
	for _, row := range []struct {
		key                 string
		val, tro, sup, maxG int64
	}{
		{"crucian", 300, 1900, 3200, 4932},
		{"perch", 280, 1600, 2700, 2400},
		{"carp", 1000, 5000, 9000, 20000},
		{"c.roach", 220, 1200, 2000, 1800},
	} {
		if _, err := db.SQL.Exec(`
			INSERT INTO species_weight(fish_key, min_g, max_g, trophy_g, trophy_line_g,
				super_trophy_line_g)
			VALUES(?,?,?,?,?,?)`,
			row.key, 30, row.maxG, row.tro, row.tro, row.sup); err != nil {
			t.Fatalf("灌 species_weight %s: %v", row.key, err)
		}
		// 档位线单独放 fish_grade_lines（真实库就是这么存的，
		// species_weight 只有 trophy_line_g / super_trophy_line_g 两列）。
		// 迁移 00027 已经灌过一批真实档位线，这里用 REPLACE 覆盖成本用例
		// 自洽的数值，保证区间断言可复现。
		if _, err := db.SQL.Exec(`
			INSERT OR REPLACE INTO fish_grade_lines(fish_key, qualified_g, trophy_g, blue_trophy_g)
			VALUES(?,?,?,?)`, row.key, row.val, row.tro, row.sup); err != nil {
			t.Fatalf("灌 fish_grade_lines %s: %v", row.key, err)
		}
	}
	for _, tc := range []struct {
		kinds string
		want  int64
	}{{"2", 2}, {"3", 3}} {
		setTunableForTest(t, srv, "bot_chat_kinds", tc.kinds)
		for i := 0; i < 200; i++ {
			ev, ok := srv.rollVirtualCatchEvent()
			if !ok {
				t.Fatalf("kinds=%s 第 %d 次抽样失败", tc.kinds, i)
			}
			if ev.Grade != tc.want {
				t.Fatalf("kinds=%s 抽出 %s %dg grade=%d, 期望 grade=%d",
					tc.kinds, ev.FishKey, ev.WeightG, ev.Grade, tc.want)
			}
			if ev.WeightG < 100 {
				t.Fatalf("kinds=%s 抽出过轻鱼获: %s %dg", tc.kinds, ev.FishKey, ev.WeightG)
			}
			if ev.FishZh == "" || ev.Angler == "" || ev.AnglerUID == 0 {
				t.Fatalf("鱼名/钓手为空: %+v", ev)
			}
		}
	}
}

// TestBroadcastVirtualCatchSkipsWithoutSessions 校验没有在线会话时不生成
// 播报（本地服绝大多数时间离线，不该往日志灌数据）。
func TestBroadcastVirtualCatchSkipsWithoutSessions(t *testing.T) {
	t.Parallel()
	srv, _ := newProgressSrv(t)
	if got := srv.liveSessions(); len(got) != 0 {
		t.Fatalf("新服务器不应有在线会话，实际 %d", len(got))
	}
	// 不应 panic，也不应留下任何副作用。
	srv.broadcastVirtualCatch()
}

// 小工具：proto.Field 的 U32/StrOr 是指针方法，链式调用取到的是值副本。
func fieldU32(v *proto.Value, i int) uint32 {
	f := v.Field(i)
	return f.U32()
}

func fieldStr(v *proto.Value, i int) string {
	f := v.Field(i)
	return f.StrOr()
}

func fieldVal(v *proto.Value, i int) *proto.Value {
	f := v.Field(i)
	return f.Val
}

func argStr(v *proto.Value, i int) string {
	it := v.ArgItem(i)
	if it == nil {
		return ""
	}
	return it.StrOr()
}

func argU32(v *proto.Value, i int) uint32 {
	it := v.ArgItem(i)
	if it == nil {
		return 0
	}
	return it.U32Val()
}
