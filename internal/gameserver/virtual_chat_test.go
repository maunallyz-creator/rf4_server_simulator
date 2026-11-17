package gameserver

import (
	"strings"
	"testing"
	"time"

	"rf4server/internal/proto"
)

// TestVirtualChatPoolDiversity 校验假聊天语料：数量够、不重复、无空白条目。
// 刷屏时看起来"像真的"的前提是同一句话不要隔几轮就重复。
func TestVirtualChatPoolDiversity(t *testing.T) {
	t.Parallel()
	if len(virtualChatPool) < 200 {
		t.Fatalf("开场白只有 %d 条，至少 200 条才够刷屏不重样", len(virtualChatPool))
	}
	if len(virtualChatReplies) < 60 {
		t.Fatalf("接话池只有 %d 条，至少 60 条", len(virtualChatReplies))
	}
	if len(virtualChatFollowUps) < 20 {
		t.Fatalf("补话池只有 %d 条，至少 20 条", len(virtualChatFollowUps))
	}
	checkNoDup(t, "开场白", virtualChatPool)
	checkNoDup(t, "接话", virtualChatReplies)
	checkNoDup(t, "补话", virtualChatFollowUps)
}

func checkNoDup(t *testing.T, what string, pool []string) {
	t.Helper()
	seen := make(map[string]bool, len(pool))
	for _, line := range pool {
		if strings.TrimSpace(line) == "" {
			t.Fatalf("%s里有空白条目", what)
		}
		if seen[line] {
			t.Fatalf("%s重复: %q", what, line)
		}
		seen[line] = true
	}
}

// TestVirtualChatPoolCoversTones 确认语料确实覆盖了"各色人"：
// 短到 1 个字符的、带脏话的、纯英文/俄文/日文的、错别字的都必须有，
// 否则公屏看起来还是同一个人。
func TestVirtualChatPoolCoversTones(t *testing.T) {
	t.Parallel()
	hasShort, hasAscii, hasNonCJK, hasCrude := false, false, false, false
	crude := []string{"卧槽", "妈的", "草", "nmd", "tmd", "nmd", "傻逼", "狗屎", "我日", "我操"}
	for _, line := range virtualChatPool {
		r := []rune(line)
		if len(r) <= 2 {
			hasShort = true
		}
		allASCII := true
		for _, c := range r {
			if c > 127 {
				allASCII = false
				break
			}
		}
		if allASCII {
			hasAscii = true
		}
		for _, c := range r {
			if c >= 0x0400 && c <= 0x04FF { // 西里尔
				hasNonCJK = true
			}
			if c >= 0x3040 && c <= 0x30FF { // 日文假名
				hasNonCJK = true
			}
			if c >= 0xAC00 && c <= 0xD7AF { // 韩文
				hasNonCJK = true
			}
		}
		for _, c := range crude {
			if strings.Contains(line, c) {
				hasCrude = true
			}
		}
	}
	if !hasShort {
		t.Fatal("语料里没有 ≤2 字的短句，公屏会显得没人顺手敲字")
	}
	if !hasAscii {
		t.Fatal("语料里没有纯 ASCII 句（英文/拼音），不像多语种混服")
	}
	if !hasNonCJK {
		t.Fatal("语料里没有俄/日/韩文，不像多语种混服")
	}
	if !hasCrude {
		t.Fatal("语料里没有脏话/粗俗用语，不够像真实公屏")
	}
}

// TestBuildVirtualSayPushShape 校验闲聊推送的形状：
// 24/27 + [401{f0=0, f1=正文, f2=虚拟玩家uid, f4=null}, 空串, 人数]。
func TestBuildVirtualSayPushShape(t *testing.T) {
	t.Parallel()
	const uid = uint32(virtualPlayerUIDBase + 7)
	v := buildVirtualSayPush(uid, "今天鱼口怎么样", 42)
	if v == nil || v.Code == nil || *v.Code != "524" {
		t.Fatalf("外层 code 必须是 524（Args524）")
	}
	if len(v.Items) != 3 {
		t.Fatalf("Args524 应有 3 个参数，实际 %d", len(v.Items))
	}
	obj := v.Items[0]
	if obj.Class != "lfbdbaffeai" {
		t.Fatalf("f0 必须是 lfbdbaffeai，实际 %q", obj.Class)
	}
	if got := fieldStr(obj, 1); got != "今天鱼口怎么样" {
		t.Fatalf("f1 正文不符：%q", got)
	}
	if got := fieldU32(obj, 2); got != uid {
		t.Fatalf("f2 发送者 uid 不符：%d != %d", got, uid)
	}
	if f := obj.Field(0); f.U32() != 0 {
		t.Fatalf("f0 kind 必须是 0（普通发言）")
	}
	if f4 := fieldVal(obj, 4); f4 == nil || f4.Kind != proto.KindNull {
		t.Fatalf("f4 必须是 null（普通发言无模板参数）: %+v", f4)
	}
	if s := argStr(v, 1); s != "" {
		t.Fatalf("外层 locationId 必须是空串，实际 %q", s)
	}
	if n := argU32(v, 2); n != 42 {
		t.Fatalf("外层人数不符：%d", n)
	}
}

// TestVirtualChatIntervalClamp 校验间隔 tunable 的默认值与钳位。
func TestVirtualChatIntervalClamp(t *testing.T) {
	t.Parallel()
	srv, db := newProgressSrv(t)
	_ = db
	if got := srv.virtualChatInterval(); got != 10*time.Second {
		t.Fatalf("默认间隔应为 10 秒，实际 %v", got)
	}
	setTunableForTest(t, srv, "bot_chat_interval_sec", "0")
	if got := srv.virtualChatInterval(); got != virtualCatchEventMinInterval {
		t.Fatalf("间隔应钳到下限 %v，实际 %v", virtualCatchEventMinInterval, got)
	}
	setTunableForTest(t, srv, "bot_chat_interval_sec", "999999")
	if got := srv.virtualChatInterval(); got != 24*time.Hour {
		t.Fatalf("间隔应钳到上限 24h，实际 %v", got)
	}
}

// TestVirtualChatCatchPctClamp 校验上鱼播报占比 tunable 的钳位。
// 默认值 80 是"公屏鱼刷屏、偶尔有人说话"的观感配比。
func TestVirtualChatCatchPctClamp(t *testing.T) {
	t.Parallel()
	srv, _ := newProgressSrv(t)
	if got := srv.virtualChatCatchPct(); got != virtualChatDefaultCatchPct {
		t.Fatalf("默认占比应为 %.0f，实际 %.0f", virtualChatDefaultCatchPct, got)
	}
	if virtualChatDefaultCatchPct != 80 {
		t.Fatalf("默认上鱼占比应为 80（鱼刷屏、聊天稀疏），实际 %.0f", virtualChatDefaultCatchPct)
	}
	setTunableForTest(t, srv, "bot_chat_catch_pct", "-50")
	if got := srv.virtualChatCatchPct(); got != 0 {
		t.Fatalf("负值应钳到 0，实际 %.0f", got)
	}
	setTunableForTest(t, srv, "bot_chat_catch_pct", "500")
	if got := srv.virtualChatCatchPct(); got != 100 {
		t.Fatalf("超限应钳到 100，实际 %.0f", got)
	}
}

// TestBroadcastVirtualChatDeliversSay 强制走闲聊分支，校验 24/27 上的
// 一条推送：命令号、kind=0、发送者 uid 落在虚拟段、正文非空。
func TestBroadcastVirtualChatDeliversSay(t *testing.T) {
	t.Parallel()
	srv, _ := newProgressSrv(t)
	sess, conn := newTunnelProbe(t, srv)
	sess.Userid = 1000001
	sess.ID = 43
	srv.sessions[sess.ID] = sess
	setTunableForTest(t, srv, "bot_chat_catch_pct", "0")    // 强制闲聊
	setTunableForTest(t, srv, "bot_chat_reply_pct", "0")    // 排除接话干扰
	setTunableForTest(t, srv, "bot_chat_followup_pct", "0") // 排除补话干扰

	if !srv.virtualChatEnabled() {
		t.Fatal("测试环境应默认开启假聊天")
	}
	srv.broadcastVirtualChat()

	env := awaitPushEnvelope(t, conn)
	if env == nil {
		t.Fatal("会话没收到闲聊推送")
	}
	if env.Main != 24 || env.Sub != 27 {
		t.Fatalf("推送命令 %d/%d, 期望 24/27", env.Main, env.Sub)
	}
	body, _, err := proto.DecodeValue(env.Body, 0, srv.table)
	if err != nil {
		t.Fatalf("解析 body: %v", err)
	}
	obj := body.Items[0]
	if f := obj.Field(0); f.U32() != 0 {
		t.Fatalf("闲聊 kind 必须是 0")
	}
	if uid := fieldU32(obj, 2); int64(uid) < virtualPlayerUIDBase {
		t.Fatalf("发送者 uid %d 不在虚拟段（应 ≥ %d）", uid, virtualPlayerUIDBase)
	}
	if text := fieldStr(obj, 1); text == "" {
		t.Fatal("闲聊正文不能为空")
	}
}

// TestBroadcastVirtualChatForcedCatchHasKv 强制走上鱼播报分支，
// 校验 f0 是档位、f4 带模板 KV。
func TestBroadcastVirtualChatForcedCatchHasKv(t *testing.T) {
	t.Parallel()
	srv, db := newProgressSrv(t)
	if _, err := db.SQL.Exec(`
		INSERT INTO species_weight(fish_key, min_g, max_g, trophy_g, trophy_line_g, super_trophy_line_g)
		VALUES('crucian', 30, 4932, 1900, 1900, 3200)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL.Exec(`
		INSERT OR REPLACE INTO fish_grade_lines(fish_key, qualified_g, trophy_g, blue_trophy_g)
		VALUES('crucian', 300, 1900, 3200)`); err != nil {
		t.Fatal(err)
	}
	sess, conn := newTunnelProbe(t, srv)
	sess.Userid = 1000001
	sess.ID = 44
	srv.sessions[sess.ID] = sess
	setTunableForTest(t, srv, "bot_chat_catch_pct", "100") // 强制上鱼播报

	srv.broadcastVirtualChat()

	env := awaitPushEnvelope(t, conn)
	if env == nil {
		t.Fatal("会话没收到播报")
	}
	if env.Main != 24 || env.Sub != 27 {
		t.Fatalf("推送命令 %d/%d, 期望 24/27", env.Main, env.Sub)
	}
	body, _, err := proto.DecodeValue(env.Body, 0, srv.table)
	if err != nil {
		t.Fatalf("解析 body: %v", err)
	}
	obj := body.Items[0]
	if f := obj.Field(0); f.U32() == 0 {
		t.Fatal("上鱼播报的 kind 不能是 0")
	}
	if f4 := fieldVal(obj, 4); f4 == nil || f4.Kind == proto.KindNull {
		t.Fatalf("上鱼播报必须带 f4 模板 KV: %+v", f4)
	}
	if uid := fieldU32(obj, 2); int64(uid) < virtualPlayerUIDBase {
		t.Fatalf("发送者 uid %d 不在虚拟段", uid)
	}
}

// TestBroadcastVirtualChatRespectsDisabled 校验总开关：关掉后不再产生推送。
func TestBroadcastVirtualChatRespectsDisabled(t *testing.T) {
	t.Parallel()
	srv, _ := newProgressSrv(t)
	setTunableForTest(t, srv, "bot_chat_enabled", "0")
	if srv.virtualChatEnabled() {
		t.Fatal("bot_chat_enabled=0 时 virtualChatEnabled 应为 false")
	}
	// 无会话时也必须是安全的空操作。
	srv.broadcastVirtualChat()
}

// TestVirtualChatLineUsesAnglerIdentity 校验 pushVirtualChatLine 会把
// 指定虚拟玩家的 uid 作为 f2，而不是玩家自己的 uid。
func TestVirtualChatLineUsesAnglerIdentity(t *testing.T) {
	t.Parallel()
	srv, _ := newProgressSrv(t)
	sess, conn := newTunnelProbe(t, srv)
	sess.Userid = 1000001
	sess.ID = 45
	srv.sessions[sess.ID] = sess

	all := allVirtualPlayers()
	if len(all) == 0 {
		t.Fatal("名册为空")
	}
	target := &all[3]
	srv.pushVirtualChatLine(target, "测试发言", srv.liveSessions())

	env := awaitPushEnvelope(t, conn)
	if env == nil {
		t.Fatal("没收到推送")
	}
	body, _, err := proto.DecodeValue(env.Body, 0, srv.table)
	if err != nil {
		t.Fatalf("解析 body: %v", err)
	}
	obj := body.Items[0]
	if uid := fieldU32(obj, 2); int64(uid) != target.UID {
		t.Fatalf("f2 应是虚拟玩家 %d，实际 %d", target.UID, uid)
	}
	if text := fieldStr(obj, 1); text != "测试发言" {
		t.Fatalf("f1 正文不符：%q", text)
	}
}

// TestVirtualChatFollowUpUsesSameAngler 校验补话用的是**同一个人**的 uid，
// 这样客户端上看起来就是"他连着发了两条"，而不是两个人各一句。
func TestVirtualChatFollowUpUsesSameAngler(t *testing.T) {
	t.Parallel()
	srv, _ := newProgressSrv(t)
	sess, conn := newTunnelProbe(t, srv)
	sess.Userid = 1000001
	sess.ID = 46
	srv.sessions[sess.ID] = sess

	all := allVirtualPlayers()
	if len(all) == 0 {
		t.Fatal("名册为空")
	}
	target := &all[5]
	srv.scheduleVirtualChatFollowUp(target)

	env := awaitPushEnvelope(t, conn)
	if env == nil {
		t.Fatal("补话没推到会话")
	}
	if env.Main != 24 || env.Sub != 27 {
		t.Fatalf("补话命令 %d/%d, 期望 24/27", env.Main, env.Sub)
	}
	body, _, err := proto.DecodeValue(env.Body, 0, srv.table)
	if err != nil {
		t.Fatalf("解析 body: %v", err)
	}
	obj := body.Items[0]
	if uid := fieldU32(obj, 2); int64(uid) != target.UID {
		t.Fatalf("补话的 uid 应是 %d（同一个人），实际 %d", target.UID, uid)
	}
	if f := obj.Field(0); f.U32() != 0 {
		t.Fatal("补话的 kind 必须是 0")
	}
	if text := fieldStr(obj, 1); text == "" {
		t.Fatal("补话正文不能为空")
	}
}

// TestVirtualChatFollowUpNilAnglerIsSafe 校验传 nil 名册项时是安全空操作。
func TestVirtualChatFollowUpNilAnglerIsSafe(t *testing.T) {
	t.Parallel()
	srv, _ := newProgressSrv(t)
	srv.scheduleVirtualChatFollowUp(nil) // 不应 panic
}
