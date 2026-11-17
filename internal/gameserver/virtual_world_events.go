package gameserver

import (
	"context"
	"math"
	"math/rand"
	"strings"
	"time"

	"rf4server/internal/proto"
	"rf4server/internal/wiregen"
)

// 虚拟玩家的“别人的鱼获”世界播报。
//
// 只走 24/27 聊天推送通道——这是本项目里唯一被实机验证过、且与核心逻辑
// 零耦合的展示出口（pushChatProbe 已在正式功能里使用：落地播报、跑鱼提示、
// 升级播报）。因此这里不新增任何协议形状，也不写任何玩家数据表。
//
// 播报内容取自真实静态数据（species_weight 的达标/奖杯/蓝冠线），所以重量
// 一定落在该鱼种的真实档位区间里，不会出现“1 克的鲤鱼”这种一眼假的东西。
//
// 全部行为受三个 tunable 控制（30 秒内热生效）：
//
//	bot_chat_enabled        0=完全关闭（默认 1 开启）
//	bot_chat_interval_sec   播报间隔秒数，最小 5（默认 90）
//	bot_chat_kinds          允许播报的档位，逗号分隔，默认 "2,3"
//	                        2=达标、3=奖杯、4=蓝冠
type virtualCatchEvent struct {
	AnglerUID int64
	Angler    string
	FishKey   string
	FishZh    string
	WeightG   int64
	Grade     int64
}

// virtualCatchEventMinInterval 是播报间隔下限，防止间隔被调成 0 后刷屏。
const virtualCatchEventMinInterval = 5 * time.Second

// runVirtualWorldEvents 是挂在 Serve() 上的后台 ticker，随服务端生命周期退出。
// 与 runDrinkUseReceiptCleanup 同一套 stop/done 通道模式。
func (s *Server) runVirtualWorldEvents(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	if s == nil {
		return
	}
	// 启动后先等一个间隔再播，避免玩家刚登录就刷一条。
	timer := time.NewTimer(s.virtualChatInterval())
	defer timer.Stop()
	for {
		select {
		case <-stop:
			return
		case <-timer.C:
			s.broadcastVirtualChat()
			timer.Reset(s.virtualChatInterval())
		}
	}
}

// liveSessions 快照当前所有未关闭会话；与 hSocialPublic 收集 peers 的方式一致。
func (s *Server) liveSessions() []*Session {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	peers := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		if sess != nil && !sess.Closed() {
			peers = append(peers, sess)
		}
	}
	s.mu.Unlock()
	return peers
}

// broadcastVirtualCatch 生成一条虚拟玩家鱼获播报并推给所有在线会话。
//
// 没有在线会话时直接返回：本地服务器绝大多数时间只有一个玩家，
// 离线期间播报没有意义，也不该往日志里灌。
func (s *Server) broadcastVirtualCatch() {
	if s == nil || s.DB == nil {
		return
	}
	if s.tunableF("bot_chat_enabled", 1) == 0 {
		return
	}
	peers := s.liveSessions()
	if len(peers) == 0 {
		return
	}
	ev, ok := s.rollVirtualCatchEvent()
	if !ok {
		return
	}
	kind := int(fishAnnouncementKind(ev.Grade))
	if kind == 0 {
		kind = 3
	}
	kv := fishTVKv(ev.FishZh, ev.WeightG)
	uid := uint32(ev.AnglerUID)
	for _, sess := range peers {
		pushVirtualAnglerChat(sess, kind, uid, kv, uint32(len(peers)))
	}
}

// pushVirtualAnglerChat 与 pushChatProbe 的默认分支形状完全一致（24/27 +
// [401, 空 locationId, 人数]），只有 401.f2（发送者 uid）换成虚拟玩家 uid，
// 这样客户端“谁说的”一栏显示的是虚拟昵称而不是自己。
//
// 外层 locationId 传空串：客户端 peilhchjenn 对频道名做严格相等校验，
// 空串无条件放行（与公聊回显、跑鱼提示同一结论）。
func pushVirtualAnglerChat(sess *Session, kind int, uid uint32, kv *proto.Value, users uint32) {
	if sess == nil || sess.closed.Load() {
		return
	}
	sess.Push(24, 27, buildVirtualAnglerPush(kind, uid, kv, users))
}

// buildVirtualAnglerPush 抽出成纯函数，便于单测校验推送形状合同
// （checkPushArgsStrict 与 24/27 的 [401, str, u32] 契约）。
func buildVirtualAnglerPush(kind int, uid uint32, kv *proto.Value, users uint32) *proto.Value {
	return buildVirtualAnglerPushText(kind, uid, "", kv, users)
}

// buildVirtualAnglerPushText 是带正文的版本：闲聊用 kind=0 + 文本，
// 上鱼播报用 kind=2/3/4 + 模板 KV（此时 text 为空）。
func buildVirtualAnglerPushText(kind int, uid uint32, text string, kv *proto.Value, users uint32) *proto.Value {
	if users < 1 {
		users = 1
	}
	f4 := proto.Null()
	if kv != nil {
		f4 = kv
	}
	msgObj := wiregen.MustMakeObj("lfbdbaffeai", map[int]*proto.Value{
		0: proto.U32(uint32(kind)),
		1: proto.StrVal(text),
		2: proto.U32(uid),
		3: proto.U64(uint64(proto.WindowsFiletimeNow())),
		4: f4,
	})
	return proto.Args524(msgObj, proto.StrVal(""), proto.U32(users))
}

// virtualCatchKinds 解析 tunable bot_chat_kinds（如 "2,3"），返回允许的档位集合。
// 解析失败或结果为空时回落到 {2, 3}（达标 + 奖杯）。
func (s *Server) virtualCatchKinds() []int64 {
	raw := s.tunableS("bot_chat_kinds", "2,3")
	out := make([]int64, 0, 3)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		switch part {
		case "1":
			// 未达标鱼不播报（与 fishAnnouncementKind 的正式口径一致）
		case "2":
			out = append(out, 2)
		case "3":
			out = append(out, 3)
		case "4":
			out = append(out, 4)
		}
	}
	if len(out) == 0 {
		out = []int64{2, 3}
	}
	return out
}

// rollVirtualCatchEvent 从真实鱼种表里抽一条“别人上的鱼”。
//
// 做法：随机挑一个在 species_weight 里有完整档位线（达标/奖杯）的鱼种，
// 再按选定档位在该档区间内做对数均匀采样——与玩家自己上鱼的采样方式
// 同源，所以重量分布看起来自然。
func (s *Server) rollVirtualCatchEvent() (virtualCatchEvent, bool) {
	var ev virtualCatchEvent
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	angler := s.randomVirtualAngler()
	if angler == nil {
		return ev, false
	}

	kinds := s.virtualCatchKinds()
	target := kinds[rand.Intn(len(kinds))]

	// 每条播报现查一次（间隔 ≥5 秒，代价可忽略，且能实时跟随数据修正）。
	// 档位线取法与 store.GradeLines 完全一致：fish_grade_lines 覆盖优先，
	// 缺失时回落到 species_weight 的 trophy_g / trophy_line_g /
	// super_trophy_line_g。species_weight 本身没有 qualified_g /
	// blue_trophy_g 两列（见 migrations/00002_static.sql）。
	var key string
	var val, tro, sup, maxG float64
	err := s.DB.SQL.QueryRowContext(ctx, `
		SELECT w.fish_key,
		       COALESCE(g.qualified_g, w.trophy_g, 0),
		       COALESCE(g.trophy_g, w.trophy_line_g, 0),
		       COALESCE(g.blue_trophy_g, w.super_trophy_line_g, 0),
		       COALESCE(w.max_g, 0)
		FROM species_weight w
		LEFT JOIN fish_grade_lines g ON g.fish_key = w.fish_key
		WHERE COALESCE(g.trophy_g, w.trophy_line_g, 0) > 0
		ORDER BY RANDOM() LIMIT 1`).Scan(&key, &val, &tro, &sup, &maxG)
	if err != nil || key == "" {
		return ev, false
	}

	// 档位区间：达标 [val, tro)、奖杯 [tro, sup)、蓝冠 [sup, max]。
	lo, hi := 0.0, 0.0
	switch target {
	case 4:
		lo, hi = sup, maxG
	case 3:
		lo, hi = tro, sup
	default:
		lo, hi = val, tro
	}
	if lo <= 0 || hi <= lo {
		// 该鱼种没有这一档，退到达标档；再不行就放弃这条（下一轮重抽）。
		lo, hi = val, tro
		if lo <= 0 || hi <= lo {
			return ev, false
		}
		target = 2
	}
	// 对数均匀采样，再夹回区间内，保证 grade 判定与所选档位一致。
	w := math.Exp(math.Log(lo) + rand.Float64()*(math.Log(hi)-math.Log(lo)))
	weightG := int64(math.Round(w))
	if weightG < int64(math.Ceil(lo)) {
		weightG = int64(math.Ceil(lo))
	}
	// hi 是开区间上界（next 档的线），落到 hi 上会被判成下一档，减 1 克避开。
	if weightG >= int64(math.Ceil(hi)) {
		weightG = int64(math.Ceil(hi)) - 1
	}
	if weightG < 1 {
		return ev, false
	}
	grade := gradeFromLines(nonZeroI64Ptr(int64(math.Round(val))), nonZeroI64Ptr(int64(math.Round(tro))), nonZeroI64Ptr(int64(math.Round(sup))), weightG)

	name := s.speciesZhName(ctx, key)
	if name == "" {
		name = key
	}
	ev = virtualCatchEvent{
		AnglerUID: angler.UID,
		Angler:    angler.Name,
		FishKey:   key,
		FishZh:    name,
		WeightG:   weightG,
		Grade:     grade,
	}
	return ev, true
}

// nonZeroI64Ptr 是 gradeFromLines 需要的 *int64 小工具；0 视作缺失。
// 注意不要叫 ptrI64：progress_test.go 已有一个同名测试辅助函数。
func nonZeroI64Ptr(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}
