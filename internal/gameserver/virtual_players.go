package gameserver

import (
	"math/rand"
	"sync"

	"rf4server/internal/proto"
	"rf4server/internal/wiregen"
)

// 虚拟玩家名册（bot 在线玩家）。
//
// 本地服务器只有玩家自己一个账号，社交面板、聊天频道、纪录页永远是空的。
// 这里维护一份**纯内存**的虚拟玩家名册：不建 profile 行、不写任何玩家数据表、
// 不参与存档与备份，因此和真实账号、成就、经济、统计完全隔离。
// UID 段 900000000+ 起，与真实账号（从 1 起）和两个系统虚拟联系人
// （2147483646 钓具助手 / 2147483647 查鱼助手）都不重叠。
//
// 名册只服务三个只读出口：
//   - 24/4  social/members  在线成员列表
//   - 24/19 资料卡          虚拟玩家的名字/等级/奖杯
//   - 24/3  频道人数        聊天频道显示的在线人数
const virtualPlayerUIDBase int64 = 900000001

// virtualPlayerDefaultOnline 是默认出现在在线列表里的虚拟玩家数量。
// 名册本身比它长，管理员可以用 tunable bot_online_count 上调或下调。
const virtualPlayerDefaultOnline = 200

type virtualPlayer struct {
	UID      int64
	Name     string
	Level    int64
	Region   string
	Trophies int64
}

// virtualPlayerSpecs 是名册定义，一共 315 条，按**六类真实 ID 形态交错**排列：
//
//	default  平台默认/自动生成昵称   贴吧用户_5Xy8aB / bili_88776655 / 用户123456789
//	pinyin   拼音或英文 + 数字       wangwei1987 / jason_07 / qwq2333
//	en       纯英文裸名             PikeHunter / NoFishNoLife / CastAndPray
//	cn       中文 + 数字或符号       摸鱼小张233 / 阿伟_01 / 钓不到鱼的大王
//	meme     现代抽象短句           我不理解 / 建议重开 / 我这把c
//	chaos    火星文 / 符号 / 多语     ℡尐壞蛋 / ▓▓▓▓ / Клёв_есть / 낚시왕
//
// 分布依据：
//   - 纯中文、无数字无符号的"干净名字"只占 29%，其余都是带数字、下划线、符号或
//     纯英文的形态——真实公屏上干净的四字中文 ID 是少数，不是多数。
//   - 不用"XX出售""懂的都懂"这类论坛口头禅：那是**帖子里的**话，不是用户名。
//   - 长度不做限制（2~15 字），因为真实 ID 本来长短不一。
//
// 交错用"应出条数 - 已出条数"的最大缺口轮转，保证无论 bot_online_count 调成多少，
// 前缀截取出来都是混合风格，不会出现"前 50 个全英文"的一眼假观感。
// 等级 3~62 递减穿插，区域覆盖 CN/RU/US/EN/DE/PL/FR/JP/KR/GL。
var virtualPlayerSpecs = []struct {
	Name   string
	Level  int64
	Region string
}{

	{"摸鱼小张233", 62, "CN"},
	{"wangwei1987", 51, "RU"},
	{"我不理解", 48, "US"},
	{"℡尐壞蛋", 53, "EN"},
	{"PikeHunter", 45, "DE"},
	{"老张666", 41, "PL"},
	{"贴吧用户_5Xy8aB", 56, "FR"},
	{"zhangjie1990", 50, "JP"},
	{"啊这", 57, "KR"},
	{"阿伟_01", 44, "GL"},
	{"灬冷瞳", 39, "CN"},
	{"lijiang1995", 35, "RU"},
	{"CarpKiller", 42, "US"},
	{"小李不吃鱼", 61, "EN"},
	{"就这", 33, "DE"},
	{"chenyu2001", 29, "PL"},
	{"〆尐雨", 46, "FR"},
	{"胖虎爱吃鱼", 40, "JP"},
	{"贴吧用户_a3K9qZ", 52, "KR"},
	{"SilentAngler", 38, "GL"},
	{"急了", 36, "CN"},
	{"liuyang2026", 31, "RU"},
	{"钓不到鱼的大王", 55, "US"},
	{"ヽ(°▽°)ノ", 34, "EN"},
	{"huanglei_07", 27, "DE"},
	{"海边老陈", 49, "PL"},
	{"典", 43, "FR"},
	{"NightFisher", 30, "JP"},
	{"阿杰2026", 25, "KR"},
	{"zhaomin_01", 32, "GL"},
	{"▓▓▓▓", 37, "CN"},
	{"破防了", 47, "RU"},
	{"贴吧用户_0x7TbW", 28, "US"},
	{"张伟_0721", 26, "EN"},
	{"sunhao_92", 24, "DE"},
	{"BreamDream", 23, "PL"},
	{"建议重开", 22, "FR"},
	{"王师傅666", 21, "JP"},
	{"██████", 20, "KR"},
	{"zhouqi2333", 19, "GL"},
	{"老李头儿01", 18, "CN"},
	{"贴吧用户_Qm4Rtd", 17, "RU"},
	{"你说的都对", 16, "US"},
	{"wuxin6666", 15, "EN"},
	{"ZanderZone", 14, "DE"},
	{"░░░░░", 13, "PL"},
	{"二狗子2333", 12, "FR"},
	{"zhengyu0721", 11, "JP"},
	{"我的评价是不如", 10, "KR"},
	{"铁蛋儿2026", 9, "GL"},
	{"EelDeal", 8, "CN"},
	{"▰▱▰▱", 7, "RU"},
	{"fengbo_888", 6, "US"},
	{"小马哥666", 5, "EN"},
	{"这合理吗", 4, "DE"},
	{"贴吧用户_9pLk2S", 3, "PL"},
	{"xiaolong007", 62, "FR"},
	{"老王家的猫", 51, "JP"},
	{"♤♡◇♧", 48, "KR"},
	{"SnagCity", 53, "GL"},
	{"属于是", 45, "CN"},
	{"明天再钓鱼", 41, "RU"},
	{"gaoyuan1234", 56, "US"},
	{"▲▲▲▲▲", 50, "EN"},
	{"空军上尉07", 57, "DE"},
	{"我先润了", 44, "PL"},
	{"linfeng999", 39, "FR"},
	{"LureLoser", 35, "JP"},
	{"打龟青年233", 42, "KR"},
	{"贴吧用户_7vNc8Y", 61, "GL"},
	{"hejun520", 33, "CN"},
	{"已老实", 29, "RU"},
	{"●○●○●○", 46, "US"},
	{"晚风与鱼", 40, "EN"},
	{"CastAndPray", 52, "DE"},
	{"guobin0923", 38, "PL"},
	{"阿强的鱼竿", 36, "FR"},
	{"求放过", 31, "JP"},
	{"◢◣◢◣◢◣", 55, "KR"},
	{"machao0713", 34, "GL"},
	{"小陈钓鱼日记", 27, "CN"},
	{"贴吧用户_e5Hj3D", 49, "RU"},
	{"TrophyTravis", 43, "US"},
	{"松弛感", 30, "EN"},
	{"luoyu2020", 25, "DE"},
	{"老周不上鱼", 32, "PL"},
	{"¤¤¤¤", 37, "FR"},
	{"谁问你了", 47, "JP"},
	{"大刘的窝子", 28, "KR"},
	{"tangjie_77", 26, "GL"},
	{"GrandmaGoblin", 24, "CN"},
	{"####", 23, "RU"},
	{"阿龙888", 22, "US"},
	{"xiongwei314", 21, "EN"},
	{"版本答案", 20, "DE"},
	{"贴吧用户_Zr8WkP", 19, "PL"},
	{"黑坑杀手阿明", 18, "FR"},
	{"caiwen404", 17, "JP"},
	{"Rudderless", 16, "KR"},
	{"*****", 15, "GL"},
	{"路亚小王子", 14, "CN"},
	{"精神状态领先", 13, "RU"},
	{"pengfei1987", 12, "US"},
	{"浮漂一动不动", 11, "EN"},
	{"bili_88776655", 10, "DE"},
	{"电子榨菜", 9, "PL"},
	{"-----", 8, "FR"},
	{"DeepWater", 7, "JP"},
	{"hanbing1990", 6, "KR"},
	{"今天又空了", 5, "GL"},
	{"硬控三秒", 4, "CN"},
	{"jason6666", 3, "RU"},
	{"野钓老炮儿", 62, "US"},
	{"+++++", 51, "EN"},
	{"ColdFront", 48, "DE"},
	{"水库守夜人", 53, "PL"},
	{"kevin0721", 45, "FR"},
	{"牛马打工人", 41, "JP"},
	{"bili_12345678", 56, "KR"},
	{"=====", 50, "GL"},
	{"老孙头2026", 57, "CN"},
	{"mike_888", 44, "RU"},
	{"RiverRat", 39, "US"},
	{"抽象工作室", 35, "EN"},
	{"阿飞_Fei07", 42, "DE"},
	{"alex007", 61, "PL"},
	{"~~~~~", 33, "FR"},
	{"小刘_Liu233", 29, "JP"},
	{"不接受批评", 46, "KR"},
	{"BaitBandit", 40, "GL"},
	{"chris1234", 52, "CN"},
	{"老王_0721", 38, "RU"},
	{"bili_55667788", 36, "US"},
	{"/////", 31, "EN"},
	{"tommy999", 55, "DE"},
	{"这瓜保熟吗", 34, "PL"},
	{"阿杰_888", 27, "FR"},
	{"HookLine", 49, "JP"},
	{"ivano520", 43, "KR"},
	{"钓鱼佬小赵", 30, "GL"},
	{"差不多得了", 25, "CN"},
	{"Клёв_есть", 32, "RU"},
	{"用户123456789", 37, "US"},
	{"小胖钓鱼中", 47, "EN"},
	{"dmitry0923", 28, "DE"},
	{"SinkerSoul", 26, "PL"},
	{"我不道啊", 24, "FR"},
	{"Рыбак_Иван", 23, "JP"},
	{"老赵头儿", 22, "KR"},
	{"yuki0713", 21, "GL"},
	{"别急慢慢来", 20, "CN"},
	{"大表哥666", 19, "RU"},
	{"haruto2020", 18, "US"},
	{"MudPuppy", 17, "EN"},
	{"абвгдеёж", 16, "DE"},
	{"阿坤2026", 15, "PL"},
	{"用户987654321", 14, "FR"},
	{"minjun_77", 13, "JP"},
	{"有被冒犯到", 12, "KR"},
	{"夜钓的阿伟", 11, "GL"},
	{"ывафыва", 10, "CN"},
	{"SlackTide", 9, "RU"},
	{"seojun314", 8, "US"},
	{"钓鱼不空军", 7, "EN"},
	{"笑不活了", 6, "DE"},
	{"lukas404", 5, "PL"},
	{"阿光_01", 4, "FR"},
	{"Сом_Охотник", 3, "JP"},
	{"真别急", 62, "KR"},
	{"StillWater", 51, "GL"},
	{"用户20260713", 48, "CN"},
	{"老马识鱼", 53, "RU"},
	{"piotr1987", 45, "US"},
	{"낚시왕", 41, "EN"},
	{"一轮游", 56, "DE"},
	{"小李子2333", 50, "PL"},
	{"andre1990", 57, "FR"},
	{"FloatKing", 44, "JP"},
	{"钓鱼佬阿彪", 39, "KR"},
	{"nick1995", 35, "GL"},
	{"下水道玩家", 42, "CN"},
	{"물고기친구", 61, "RU"},
	{"steam_765611", 33, "US"},
	{"王二狗子66", 29, "EN"},
	{"tony2001", 46, "DE"},
	{"SpoolBoy", 40, "PL"},
	{"菜就多练练", 52, "FR"},
	{"阿珍不吃鱼", 38, "JP"},
	{"눈팅만합니다", 36, "KR"},
	{"erik2026", 31, "GL"},
	{"老陈头儿01", 55, "CN"},
	{"这把我c", 34, "RU"},
	{"DragQueen", 27, "US"},
	{"marco_07", 49, "EN"},
	{"二表哥888", 43, "DE"},
	{"개드립충", 30, "PL"},
	{"qq_123456789", 25, "FR"},
	{"我尽力了", 32, "JP"},
	{"ivan_01", 37, "KR"},
	{"钓鱼的猫", 47, "GL"},
	{"BraidBaron", 28, "CN"},
	{"ㅇㅇㅇㅇㅇ", 26, "RU"},
	{"阿亮_0721", 24, "US"},
	{"slava_92", 23, "EN"},
	{"手残党本党", 22, "DE"},
	{"微信用户8888", 21, "PL"},
	{"小镇青年阿杰", 20, "FR"},
	{"kenji2333", 19, "JP"},
	{"TightLines", 18, "KR"},
	{"キモヲタ釣り師", 17, "GL"},
	{"肝帝在此", 16, "CN"},
	{"老徐_2026", 15, "RU"},
	{"junho6666", 14, "US"},
	{"白嫖怪", 13, "EN"},
	{"阿豪钓鱼记", 12, "DE"},
	{"渓流の達人", 11, "PL"},
	{"NoFishNoLife", 10, "FR"},
	{"qwq0923", 9, "JP"},
	{"空军少校07", 8, "KR"},
	{"抖音用户2026", 7, "GL"},
	{"不氪不改命", 6, "CN"},
	{"kkkk0713", 5, "RU"},
	{"老潘头儿", 4, "US"},
	{"名無しさん", 3, "EN"},
	{"SundayAngler", 62, "DE"},
	{"hhhh2020", 51, "PL"},
	{"小鱼儿233", 48, "FR"},
	{"躺平不想动", 53, "JP"},
	{"通りすがり", 45, "KR"},
	{"阿文2026", 41, "GL"},
	{"zzz_77", 56, "CN"},
	{"摆烂式钓鱼", 50, "RU"},
	{"WetBoots", 57, "US"},
	{"小红书_0713", 44, "EN"},
	{"大熊钓鱼中", 39, "DE"},
	{"lolicon314", 35, "PL"},
	{"誰も知らない", 42, "FR"},
	{"主打一个随缘", 61, "JP"},
	{"老蒋666", 33, "KR"},
	{"wdnmd404", 29, "GL"},
	{"ShallowEnd", 46, "CN"},
	{"阿涛_01", 40, "RU"},
	{"ﾊﾞｽ釣り名人", 52, "US"},
	{"已放弃治疗", 38, "EN"},
	{"nmd1987", 36, "DE"},
	{"快手用户6666", 31, "PL"},
	{"海边阿明", 55, "FR"},
	{"LineStripper", 34, "JP"},
	{"abcde1990", 27, "KR"},
	{"爷青回", 49, "GL"},
	{"钓鱼佬老K", 43, "CN"},
	{"ＦＵＬＬＷＩＤＴＨ", 30, "RU"},
	{"asdf1995", 25, "US"},
	{"小杨_0721", 32, "EN"},
	{"过于真实", 37, "DE"},
	{"BacklashBob", 47, "PL"},
	{"ツ", 28, "FR"},
	{"阿勇2026", 26, "JP"},
	{"qwer2001", 24, "KR"},
	{"网易云用户233", 23, "GL"},
	{"好活当赏", 22, "CN"},
	{"老崔头儿", 21, "RU"},
	{"lmao2026", 20, "US"},
	{"匿名希望", 19, "EN"},
	{"RodBreaker", 18, "DE"},
	{"阿明_888", 17, "PL"},
	{"稳中向好", 16, "FR"},
	{"ggwp_07", 15, "JP"},
	{"知乎用户9f2c", 14, "KR"},
	{"赵四钓鱼", 13, "GL"},
	{"匿名网友某某", 12, "CN"},
	{"nofish_01", 11, "RU"},
	{"NetZero", 10, "US"},
	{"不出意外又要空", 9, "EN"},
	{"小胖_2333", 8, "DE"},
	{"fishman_92", 7, "PL"},
	{"null", 6, "FR"},
	{"阿健钓鱼中", 5, "JP"},
	{"又菜又爱玩", 4, "KR"},
	{"RainyDay", 3, "GL"},
	{"void2333", 62, "CN"},
	{"老胡头儿01", 51, "RU"},
	{"微博用户8823", 48, "US"},
	{"菜且自信", 53, "EN"},
	{"undefined", 45, "DE"},
	{"null6666", 41, "PL"},
	{"阿辉2026", 56, "FR"},
	{"TwoPoles", 50, "JP"},
	{"打不过就加入", 57, "KR"},
	{"钓鱼佬阿凯", 44, "GL"},
	{"sleepy0721", 39, "CN"},
	{"0x1F600", 35, "RU"},
	{"小曾_666", 42, "US"},
	{"lazy_888", 61, "EN"},
	{"顶不住了兄弟", 33, "DE"},
	{"OldBoot", 29, "PL"},
	{"tg_88888888", 46, "FR"},
	{"老聂头儿", 40, "JP"},
	{"-=[]\\;,./", 52, "KR"},
	{"busy007", 38, "GL"},
	{"我是自愿的", 36, "CN"},
	{"阿康_0721", 31, "RU"},
	{"PuddleJump", 55, "US"},
	{"tang1234", 34, "EN"},
	{"ºΩ≈ç√", 27, "DE"},
	{"钓鱼佬阿龙", 49, "PL"},
	{"求求你别钓了", 43, "FR"},
	{"moyu999", 30, "JP"},
	{"用户77889900", 25, "KR"},
	{"老温2026", 32, "GL"},
	{"CatchRelease", 37, "CN"},
	{"＿人人＿", 47, "RU"},
	{"谁钓鱼谁孙子", 28, "US"},
	{"bailan520", 26, "EN"},
	{"小谢_233", 24, "DE"},
}

var (
	virtualPlayersOnce sync.Once
	virtualPlayerList  []virtualPlayer
	virtualPlayerByUID map[int64]*virtualPlayer
)

func buildVirtualPlayers() {
	virtualPlayerList = make([]virtualPlayer, 0, len(virtualPlayerSpecs))
	virtualPlayerByUID = make(map[int64]*virtualPlayer, len(virtualPlayerSpecs))
	for i, spec := range virtualPlayerSpecs {
		uid := virtualPlayerUIDBase + int64(i)
		// 奖杯数按等级线性缩放，高等级玩家看起来更有战绩。
		trophies := spec.Level*2 + int64(i%7)
		virtualPlayerList = append(virtualPlayerList, virtualPlayer{
			UID: uid, Name: spec.Name, Level: spec.Level,
			Region: spec.Region, Trophies: trophies,
		})
	}
	for i := range virtualPlayerList {
		virtualPlayerByUID[virtualPlayerList[i].UID] = &virtualPlayerList[i]
	}
}

// allVirtualPlayers 返回完整名册（只读，调用方不得修改）。
func allVirtualPlayers() []virtualPlayer {
	virtualPlayersOnce.Do(buildVirtualPlayers)
	return virtualPlayerList
}

// onlineVirtualPlayers 返回当前应该出现在在线列表里的虚拟玩家。
// 数量由 tunable bot_online_count 控制（默认 64，范围 0..名册长度）。
func (s *Server) onlineVirtualPlayers() []virtualPlayer {
	all := allVirtualPlayers()
	n := len(all)
	if s != nil {
		if v := int(s.tunableF("bot_online_count", virtualPlayerDefaultOnline)); v >= 0 {
			n = v
		}
	}
	if n > len(all) {
		n = len(all)
	}
	if n <= 0 {
		return nil
	}
	return all[:n]
}

// virtualPlayerByUIDLookup 按 UID 查虚拟玩家；不是虚拟玩家时返回 nil。
func virtualPlayerByUIDLookup(uid int64) *virtualPlayer {
	virtualPlayersOnce.Do(buildVirtualPlayers)
	return virtualPlayerByUID[uid]
}

// virtualPlayerCard 构造 24/19 资料卡：与真实玩家同一 wire class
// dgbmlekfpbg，只有 f6 用 0（普通玩家）而不是虚拟联系人的 true。
func virtualPlayerCard(p *virtualPlayer) *proto.Value {
	return wiregen.MustMakeObj("dgbmlekfpbg", map[int]*proto.Value{
		0: proto.U32(uint32(p.UID)),
		1: proto.StrVal(p.Name),
		2: proto.Str(nil, 0x15),
		3: proto.U32(0),
		4: proto.U32(uint32(p.Level)),
		5: proto.U32(uint32(p.Trophies)),
		6: proto.U32(0),
		7: proto.U32(uint32(socialRegionCode(p.Region))),
		8: proto.U32(0),
	})
}

// randomVirtualAngler 供世界播报/纪录页随机挑一个虚拟钓手。
func (s *Server) randomVirtualAngler() *virtualPlayer {
	list := s.onlineVirtualPlayers()
	if len(list) == 0 {
		list = allVirtualPlayers()
	}
	if len(list) == 0 {
		return nil
	}
	idx := rand.Intn(len(list))
	return &list[idx]
}
