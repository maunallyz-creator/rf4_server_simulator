package gameserver

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"rf4server/internal/config"
	"rf4server/internal/logging"
	"rf4server/internal/proto"
	"rf4server/internal/wiregen"
)

// newVirtualHandlerEnv 造一个带 DB + 一个"真实在线会话"的环境，
// 用来验证 24/3、24/4、24/19 三个出口把虚拟玩家叠加进去后的形状。
func newVirtualHandlerEnv(t *testing.T, uid int64) (*Server, *Session) {
	t.Helper()
	db := openMigratedTestDB(t)
	srv := NewServer(config.Game{Tunnel: config.Tunnel{PSK: "bot"}}, logging.Discard(), nil)
	srv.SetDB(db)
	sess := &Session{ID: 1, Userid: uid, Srv: srv, log: logging.Discard()}
	// 让 hSocialMembers 能把这个会话算作真实在线玩家。
	srv.sessions[sess.ID] = sess
	return srv, sess
}

// u32ArrOf 从 encodeSocialTree 的回包里解出 u32 数组。
// 24/4 的回包是 U32Arr（0x05 marker 或 typed 形状），这里用项目自带的
// proto 解码器还原，避免手工解析字节。
func u32ArrOf(t *testing.T, raw []byte, table *proto.OpsTable) []uint32 {
	t.Helper()
	v, _, err := proto.DecodeValue(raw, 0, table)
	if err != nil {
		t.Fatalf("解码 24/4 回包失败: %v (raw=%x)", err, raw)
	}
	out := make([]uint32, 0, v.ArgCount())
	for i := 0; i < v.ArgCount(); i++ {
		if it := v.ArgItem(i); it != nil {
			out = append(out, it.U32Val())
		}
	}
	return out
}

// TestSocialMembersIncludesVirtualPlayers 校验 24/4 在线成员：
// 前两位必须是两个系统联系人，随后是真实玩家，最后才是虚拟名册。
func TestSocialMembersIncludesVirtualPlayers(t *testing.T) {
	t.Parallel()
	srv, sess := newVirtualHandlerEnv(t, 1000001)
	ctx := &Ctx{Sess: sess, Main: 24, Sub: 4, DecodeOK: true,
		Tree: proto.Args524(proto.U32(0)), Log: logging.Discard()}

	raw, handled := hSocialMembers(ctx)
	if !handled {
		t.Fatal("24/4 未处理")
	}
	ids := u32ArrOf(t, raw, srv.table)
	want := 2 + 1 + len(srv.onlineVirtualPlayers()) // 两个系统联系人 + 真实会话 + 虚拟名册
	if len(ids) != want {
		t.Fatalf("成员数 %d, 期望 %d", len(ids), want)
	}
	// 契约：两个系统联系人固定占前两位（sort.Slice(ids[2:]) 依赖它）。
	if ids[0] != uint32(fishQueryContactUID) {
		t.Fatalf("ids[0]=%d, 期望查鱼助手 %d", ids[0], fishQueryContactUID)
	}
	if ids[1] != uint32(gearAssistantUID) {
		t.Fatalf("ids[1]=%d, 期望钓具助手 %d", ids[1], gearAssistantUID)
	}
	if ids[2] != 1000001 {
		t.Fatalf("ids[2]=%d, 期望真实玩家 1000001（虚拟玩家不应插到真人前面）", ids[2])
	}
	// 尾部必须是虚拟段，且整体去重。
	seen := make(map[uint32]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("成员列表有重复 uid %d", id)
		}
		seen[id] = true
	}
	for _, id := range ids[3:] {
		if int64(id) < virtualPlayerUIDBase {
			t.Fatalf("虚拟名册里混入了非虚拟 uid %d", id)
		}
	}

	// bot_online_count=0 时列表退回改动前行为。
	setTunableForTest(t, srv, "bot_online_count", "0")
	if ids := u32ArrOf(t, mustSocialMembers(t, ctx), srv.table); len(ids) != 3 {
		t.Fatalf("关闭虚拟玩家后成员数 %d, 期望 3", len(ids))
	}
}

func mustSocialMembers(t *testing.T, ctx *Ctx) []byte {
	t.Helper()
	raw, ok := hSocialMembers(ctx)
	if !ok {
		t.Fatal("24/4 未处理")
	}
	return raw
}

// TestSocialCurrentAddsVirtualUsers 校验 24/3 频道对象的人数把虚拟玩家算进去。
func TestSocialCurrentAddsVirtualUsers(t *testing.T) {
	t.Parallel()
	srv, sess := newVirtualHandlerEnv(t, 1000001)

	read := func() int64 {
		raw, ok := hSocialCurrent(&Ctx{Sess: sess, Main: 24, Sub: 3, DecodeOK: true,
			Tree: proto.Args524(proto.U32(0)), Log: logging.Discard()})
		if !ok {
			t.Fatal("24/3 未处理")
		}
		v, _, err := proto.DecodeValue(raw, 0, srv.table)
		if err != nil {
			t.Fatalf("解码 24/3 回包失败: %v", err)
		}
		// encodeSocialTree 的载荷可能是 bytes 包裹的树，也可能是直接对象；
		// 用 findGodhi 递归找 godhiicbgng 再读 f3。
		if got := findChannelUsers(v); got >= 0 {
			return int64(got)
		}
		t.Fatalf("24/3 回包里找不到 godhiicbgng.f3: %x", raw)
		return 0
	}

	virtual := int64(len(srv.onlineVirtualPlayers()))
	if got := read(); got != 1+virtual {
		t.Fatalf("频道人数 %d, 期望 1 真实 + %d 虚拟", got, virtual)
	}
	setTunableForTest(t, srv, "bot_online_count", "0")
	if got := read(); got != 1 {
		t.Fatalf("关闭虚拟玩家后频道人数 %d, 期望 1", got)
	}
}

// findChannelUsers 在解码树里递归找 godhiicbgng 的 f3；找不到返回 -1。
func findChannelUsers(v *proto.Value) int {
	if v == nil {
		return -1
	}
	if v.Class == "godhiicbgng" {
		f := v.Field(3)
		return int(f.U32())
	}
	for i := 0; i < v.ArgCount(); i++ {
		if n := findChannelUsers(v.ArgItem(i)); n >= 0 {
			return n
		}
	}
	for i := 0; i < v.FieldCount(); i++ {
		if n := findChannelUsers(v.Field(i).Val); n >= 0 {
			return n
		}
	}
	return -1
}

// TestSocialUserCardVirtualPlayer 校验 24/19 对名册 UID 返回虚拟昵称，
// 而不是回落到 profile 查不到时的空名字。
func TestSocialUserCardVirtualPlayer(t *testing.T) {
	t.Parallel()
	srv, sess := newVirtualHandlerEnv(t, 1000001)
	vp := srv.onlineVirtualPlayers()[0]

	ctx := &Ctx{Sess: sess, Main: 24, Sub: 19, DecodeOK: true,
		Tree: proto.Args524(proto.U32(uint32(vp.UID)), proto.U32(0)), Log: logging.Discard()}
	raw, ok := hSocialUserCard(ctx)
	if !ok {
		t.Fatal("24/19 未处理")
	}
	// 回包必须是 bytes 包裹的 dgbmlekfpbg（虚拟联系人那条路径同形状）。
	if len(raw) == 0 {
		t.Fatal("24/19 回包为空")
	}
	inner := unwrapSocialBytes(t, raw)
	v, _, err := proto.DecodeValue(inner, 0, srv.table)
	if err != nil {
		t.Fatalf("解码资料卡失败: %v (raw=%x)", err, inner)
	}
	if v.Class != "dgbmlekfpbg" {
		t.Fatalf("资料卡 class=%q, 期望 dgbmlekfpbg", v.Class)
	}
	if got := fieldU32(v, 0); got != uint32(vp.UID) {
		t.Fatalf("f0 uid=%d, 期望 %d", got, vp.UID)
	}
	if got := fieldStr(v, 1); got != vp.Name {
		t.Fatalf("f1 name=%q, 期望 %q", got, vp.Name)
	}
	if got := fieldU32(v, 6); got != 0 {
		t.Fatalf("f6=%d, 普通玩家必须是 0（true 会被当成系统虚拟联系人）", got)
	}
}

// unwrapSocialBytes 取出 24/18/24/19 回包里 BytesOf 的多余一层。
// 回包形状是 [marker][u32 len][payload]；若不是这个形状则原样返回。
func unwrapSocialBytes(t *testing.T, raw []byte) []byte {
	t.Helper()
	if len(raw) < 5 {
		return raw
	}
	n := binary.LittleEndian.Uint32(raw[1:5])
	if int(n) == len(raw)-5 {
		return raw[5:]
	}
	return raw
}

// TestVirtualWorldBroadcastReachesSessions 端到端校验：有在线会话时，
// 一次播报会把 24/27 推到会话上，且 401.f2 是虚拟玩家 uid。
func TestVirtualWorldBroadcastReachesSessions(t *testing.T) {
	t.Parallel()
	srv, db := newProgressSrv(t)
	// 灌一个鱼种的真实档位，保证抽样必成功。
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
	sess.ID = 42
	srv.sessions[sess.ID] = sess

	srv.broadcastVirtualCatch()

	// 从隧道另一端读一条记录出来核形状。
	env := awaitPushEnvelope(t, conn)
	if env == nil {
		t.Fatal("会话没收到播报")
	}
	if env.Main != 24 || env.Sub != 27 {
		t.Fatalf("播报命令 %d/%d, 期望 24/27", env.Main, env.Sub)
	}
	body, _, err := proto.DecodeValue(env.Body, 0, srv.table)
	if err != nil {
		t.Fatalf("解码播报失败: %v", err)
	}
	if reason := checkPushArgsStrict(24, 27, body); reason != "" {
		t.Fatalf("播报形状不合合同: %s", reason)
	}
	msg := body.ArgItem(0)
	if msg == nil || msg.Class != "lfbdbaffeai" {
		t.Fatalf("播报 401 对象不对: %+v", msg)
	}
	uid := fieldU32(msg, 2)
	if int64(uid) < virtualPlayerUIDBase {
		t.Fatalf("401.f2=%d, 必须是虚拟玩家 uid（不能是自己）", uid)
	}
}

// TestVirtualCatchEventJSONShape 记录当前 record 页策略：24/21 保持空表。
// 这条用例的意义是防止有人“顺手”往 24/21 里塞猜测字段——官方 24/21 的
// record JSON key 名从未取证成功（GameAssembly.dll 字符串字面量已被剥离），
// 猜错会让客户端纪录页崩溃或列全空。
func TestSocialAbsoluteRecordsStaysEmpty(t *testing.T) {
	t.Parallel()
	_, sess := newVirtualHandlerEnv(t, 1000001)
	raw, ok := hSocialAbsoluteRecords(&Ctx{Sess: sess, Main: 24, Sub: 21, DecodeOK: true,
		Tree: proto.Args524(proto.StrVal("absolute"), proto.StrVal("DEF"), proto.StrVal("zh-CN")),
		Log:  logging.Discard()})
	if !ok {
		t.Fatal("24/21 未处理")
	}
	inner := unwrapSocialBytes(t, raw)
	// 回包里 bytes 是 JSON。
	if i := indexOfJSON(inner); i >= 0 {
		inner = inner[i:]
	}
	var payload struct {
		Success bool              `json:"success"`
		Type    string            `json:"type"`
		Records []json.RawMessage `json:"records"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		t.Fatalf("24/21 回包不是合法 JSON: %v (%q)", err, inner)
	}
	if !payload.Success {
		t.Fatal("24/21 success 必须为 true")
	}
	if payload.Type != "DEF" {
		t.Fatalf("24/21 type=%q, 期望 DEF（请求原样回显）", payload.Type)
	}
	if len(payload.Records) != 0 {
		t.Fatalf("24/21 records 必须为空（字段结构未取证，不许猜）: %s", payload.Records)
	}
	if wiregen.Table() == nil {
		t.Fatal("wiregen 表未初始化")
	}
}

// indexOfJSON 找第一个 '{'，用于跳过 bytes 包裹前缀。
func indexOfJSON(b []byte) int {
	for i, c := range b {
		if c == '{' {
			return i
		}
	}
	return -1
}
