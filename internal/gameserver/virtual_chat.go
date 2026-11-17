package gameserver

import (
	"math/rand"
	"time"

	"rf4server/internal/proto"
)

// 虚拟玩家的「假聊天」。
//
// 目的：本地服务器只有玩家一个人，聊天频道永远冷清。这里让名册里的虚拟
// 玩家定时说几句话，把频道撑出"有人气"的观感。
//
// 只走 24/27 这一条已被实机验证的推送通道（与落地播报、跑鱼提示、升级
// 播报同一条）。普通发言用 kind=0、f4 传 null，与 buildChatEchoPush 的
// 公聊回显形状完全一致；唯一区别是 f2 填虚拟玩家 uid（900000001+）而不是
// 玩家自己。客户端按 uid 反查昵称，24/19 资料卡已能返回名册里的名字，
// 所以气泡上显示的是虚拟昵称。
//
// 语料是刻意"不像人写的文案"而是"像公屏"：大量短句、脏话、错别字、
// 无意义喊话、复读、阴阳怪气、方言味，参考钓鱼贴吧/游戏公屏的真实语言。
// 目标不是每句都通顺，而是让玩家扫一眼觉得"这服里真有人在钓"。
//
// 所有内容都是内存常量，不写任何玩家数据表。
//
// tunable（30 秒内热生效）：
//
//	bot_chat_enabled      0=完全关闭（默认 1）
//	bot_chat_interval_sec 每轮间隔秒数，最小 5（默认 10）
//	bot_chat_catch_pct    每轮做成"别人上鱼"播报的百分比（默认 80：鱼刷屏）
//	bot_chat_reply_pct    闲聊后紧跟一条别人接话的百分比（默认 35）
//	bot_chat_followup_pct 闲聊后同一个人再补一句的百分比（默认 20）
//
// 密度设计：单 ticker + catch_pct 分流。默认 10 秒一轮、80% 上鱼，则
// 鱼获约每 12 秒一条（刷屏感），聊天约每 50 秒一条（偶尔有人说话），
// 正好是"公屏鱼刷屏 + 偶尔几句话"的观感。

// virtualChatPool 是聊天池。按题材分块，方便增删；运行时会整体拼接。
// 风格刻意混杂：文明与粗俗、通顺与语病、中文与外文、正常人与纯抽象。
var virtualChatPool = []string{
	// --- 打招呼 / 试探有人在不在 ---
	"有人吗",
	"有人吗有人吗",
	"在线等 急",
	"来个活人",
	"这服还有人吗",
	"怎么没人说话",
	"都钓鱼呢不说话是吧",
	"说句话啊",
	"1",
	"111",
	"扣1",
	"在的扣1",
	"报个数",
	"几点了还有人",
	"半夜还有这么多人啊",
	"这个点数还有人 服了",
	"都几点了还不睡",
	"睡了睡了 明天还要上班",

	// --- 日常：天气 / 现实生活打断 ---
	"下雨了",
	"外面打雷了 网会不会断",
	"风好大 漂都看不清楚",
	"这天气钓个屁",
	"太冷了 手都僵了",
	"冻死了兄弟们",
	"热死了 一点风都没有",
	"起雾了 什么都看不见",
	"我这停电了 等下再上",
	"刚才网卡了一下 差点跑鱼",
	"我妈叫我吃饭 先挂会",
	"刚下夜班 来钓两竿",
	"上班摸鱼中 偷偷玩",
	"老板在旁边 不敢说话",
	"明天还要早起 今天到几点",
	"晚饭还没吃 饿死了",
	"先去泡碗面",
	"抽根烟回来继续",
	"上厕所去了 别偷我鱼",

	// --- 空军 / 打龟 / 自嘲（贴吧核心梗） ---
	"空军第八天了",
	"空军司令在此",
	"我已经不会钓鱼了",
	"今天又是打龟",
	"龟了龟了",
	"我连鱼影子都没见着",
	"三个小时 一条没有",
	"鱼护是干的",
	"我钓的是水",
	"我怀疑这湖里没鱼",
	"手残党报道",
	"菜就多练 我知道",
	"我不玩了 真的",
	"再钓我就是狗",
	"汪汪汪",
	"说好的爆护呢",
	"谁说这图好钓的 出来",
	"被坑了 这图根本不出鱼",
	"我这战绩 建议删号",
	"我这种是不是不适合钓鱼",
	"新手保护期是不是过了",
	"我今天手气差到离谱",
	"运气全用在上班了",
	"今天不出鱼 明天再来",
	"空手而归第N次",

	// --- 粗口 / 骂街 / 破防（游戏公屏真实态） ---
	"卧槽",
	"卧槽无情",
	"我日",
	"我靠",
	"我操",
	"妈的",
	"他妈的",
	"草",
	"草草草",
	"淦",
	"韩币",
	"nmd",
	"tmd",
	"nm",
	"wdnmd",
	"cnm 又跑了",
	"妈的又断线",
	"草 刚那条绝对是大货",
	"我日 跑了跑了",
	"操 差一点",
	"傻逼鱼",
	"这破鱼",
	"狗屎运气",
	"狗游戏",
	"破服务器",
	"卡成幻灯片了",
	"这延迟能玩?",
	"什么破网",
	"气死我了",
	"破防了",
	"我裂开了",
	"心态崩了",
	"真的会谢",
	"服了",
	"我服了",
	"无语",
	"绝了",
	"离大谱",
	"离谱他妈给离谱开门",
	"这不科学",
	"见鬼了",
	"邪门",

	// --- 抽象 / 无意义喊话 / 复读 ---
	"？",
	"？？？",
	"？？？？？？",
	"。",
	"。。。",
	"......",
	"啊？",
	"啥",
	"嗯",
	"哦",
	"嗷",
	"哈",
	"哈哈",
	"哈哈哈",
	"笑死",
	"绷不住了",
	"乐",
	"难绷",
	"离谱",
	"典",
	"急了",
	"急急急",
	"6",
	"66",
	"666",
	"666666",
	"牛逼",
	"牛",
	"强",
	"太强了",
	"yyds",
	"xswl",
	"awsl",
	"emmm",
	"emm",
	"qwq",
	"2333",
	"哈哈哈哈哈哈",
	"AAAAAAAA",
	"我测",
	"我超",
	"啊这",
	"这这这",
	"来了来了",
	"好家伙",
	"好家伙 直接好家伙",
	"妙啊",
	"可以的",
	"行吧",
	"随便吧",
	"无所谓了",
	"算了",
	"不钓了",
	"退出游戏",
	"卸载了卸载了",
	"退了退了",
	"挂机了",
	"人麻了",

	// --- 地域味 / 方言 / 口语 ---
	"老铁们 这啥情况",
	"整不会了",
	"这咋整",
	"咋回事啊",
	"搞么子",
	"弄啥嘞",
	"锤子 又是空",
	"龟儿子鱼",
	"宝器",
	"俺也一样",
	"咱们这钓法不一样",
	"我这边叫打龟 你们叫啥",
	"南方人说空军 北方人说打龟 一个意思",
	"哥几个哪的人",
	"我东北的 冷得要死",
	"广东这边还穿短袖",
	"四川的 这边水库多",
	"这鱼在我们那边叫白条",
	"这鱼我们叫餐条",
	"各地叫法都不一样 哈哈",

	// --- 装备 / 炫 / 求推荐 ---
	"刚出了个蓝冠 截图了",
	"这条是奖杯 入护了",
	"我这个轮子新买的 试试手感",
	"竿子有点软 提不起来",
	"线太粗了 鱼不咬",
	"换了0.3的线 口明显好了",
	"这个饵真的好用 推荐",
	"亮片yyds",
	"软虫也可以",
	"雷蛙你们用过吗",
	"我这套装备花了不少钱",
	"便宜的轮子能用吗",
	"求推荐个300以内的轮",
	"这钩子老是脱",
	"钩子要换大的",
	"我的线杯老是炸线",
	"炸线炸到想砸键盘",
	"这个轮泄力好滑 不爽",
	"谁教我怎么调泄力",
	"装备党路过",
	"我全身上下就竿子是好的",
	"穷钓也是一种钓法",

	// --- 求助 / 问答 ---
	"怎么调漂啊",
	"调几目钓几目",
	"用什么饵",
	"这图用什么饵好",
	"水深多少",
	"这地方晚上能钓吗",
	"求个坐标",
	"标点在哪",
	"哪个位置出鱼",
	"大佬带带我",
	"萌新求教",
	"第一次来 不懂",
	"这鱼怎么拉",
	"提竿时机怎么把握",
	"为什么我老是脱钩",
	"为什么我老是空竿",
	"这鱼要多大的钩",
	"要不要打窝",
	"窝料用什么",
	"要不要换位置",
	"多久换一次饵",
	"是不是该换图了",

	// --- 上鱼 / 跑鱼 / 现场直播 ---
	"上鱼了上鱼了",
	"有口了",
	"咬了一口 没中",
	"刚跑了 气死",
	"这条不小",
	"有点分量",
	"拉了半天",
	"差点断竿",
	"抄网没带 尴尬了",
	"手滑了 鱼掉水里了",
	"刚拍完照就跑了",
	"这条放了 太小",
	"这条进鱼护了",
	"连续三条 起飞",
	"连竿了兄弟们",
	"今天口不错",
	"口很轻 要仔细看",
	"都是小鱼闹",
	"白条闹窝",
	"一直被啃 不上钩",
	"刚那条绝对破纪录",
	"这条得有十斤吧",
	"我看错了 是水草",

	// --- 阴阳怪气 / 嘴炮 / 抬杠 ---
	"厉害厉害",
	"真牛啊",
	"大佬6666",
	"就这?",
	"这也叫鱼?",
	"我上我也行",
	"说得跟真的一样",
	"你确定?",
	"骗人",
	"吹吧你就",
	"又在装",
	"我信你个鬼",
	"你鱼护能给我看看吗",
	"别吹了 上一个也是这么说的",
	"这服有托吧",
	"是不是GM在这",
	"这鱼获是真的假的",
	"我不信",
	"呵呵",
	"笑而不语",
	"你说得对",
	"行行行 你对",
	"我不听我不听",
	"让子弹飞一会",

	// --- 中英混杂 / 外语 / 拼音 ---
	"anyone here?",
	"this spot dead",
	"no fish at all",
	"nice one",
	"gg",
	"good luck",
	"fishing at night is better",
	"what bait?",
	"any tip?",
	"ok got it",
	"lol",
	"wtf",
	"omg",
	"hi all",
	"hello",
	"привет",
	"как дела",
	"рыба есть?",
	"こんにちは",
	"釣れますか",
	"안녕하세요",
	"물고기 없어",
	"ni zai gan ma",
	"wo ye kong jun le",
	"you ren ma",
	"shang yu le",
	"kuai lai kan",

	// --- 错别字 / 语病 / 打字错误（真实公屏一定有） ---
	"我家网又断偶了",
	"这鱼好大只阿",
	"真的是绝绝子",
	"我好南啊",
	"太难了把",
	"有没有人告诉我在那",
	"我以经空军很久了",
	"这轮子好想买",
	"谁能教下我怎么玩坝",
	"鱼在那啊",
	"在线等 挺急的",
	"等会我也去试是",
	"打龟打习惯了把",
	"这把稳了把",
	"我晕",
	"我裂",
	"救命",
	"救救孩子",
	"emmmmm",
	"哦哦哦哦哦",
	"好好好好好",
	"是是是对对对",
	"嗯嗯嗯",
	"行行行行行",
}

// virtualChatReplies 是接话池：配合开场白，营造"有人在对话"的感觉。
// 同样刻意不礼貌、不同质，短到像真的在公屏上顺手敲的。
var virtualChatReplies = []string{
	"同问",
	"我也一样",
	"我也空军",
	"我这边也空",
	"一样一样",
	"+1",
	"+1+1",
	"加一",
	"确实",
	"有道理",
	"说得对",
	"我也想知道",
	"蹲一个答案",
	"等大佬",
	"同求",
	"哈哈",
	"哈哈哈",
	"笑死",
	"绷",
	"6",
	"66",
	"666",
	"牛",
	"厉害",
	"强啊",
	"太强了",
	"羡慕",
	"我羡慕哭了",
	"换饵",
	"换个饵试试",
	"亮片试试",
	"软虫啊",
	"往深水扔",
	"靠岸边试试",
	"晚上口好",
	"再等等",
	"别急",
	"慢慢来",
	"稳住",
	"加油",
	"祝爆护",
	"祝你好运",
	"good luck",
	"nice",
	"same here",
	"yeah",
	"lol",
	"草",
	"卧槽",
	"我日",
	"真的假的",
	"不会吧",
	"这么猛",
	"骗人",
	"我不信",
	"就这",
	"呵呵",
	"你在逗我",
	"别吹",
	"截图看看",
	"发图",
	"上图上图",
	"没图说个屁",
	"哪个位置",
	"在哪",
	"坐标给我",
	"我过去",
	"等我",
	"马上到",
	"一起啊",
	"带我一个",
	"加个好友",
	"我也来",
	"这边没鱼",
	"我这条河也没口",
	"这图就这样",
	"换个图吧",
	"换个点",
	"去下游试试",
	"上游好点",
	"我一般在这钓",
	"我空军一天了",
	"我今天也龟了",
	"习惯了",
	"正常",
	"这服就这样",
	"别抱希望",
	"我打算退了",
	"再钓十分钟",
	"最后一竿",
	"我也说最后一竿 说了三小时",
	"哈哈哈真实",
	"太真实了",
	"破防",
	"难绷",
	"我也裂了",
	"共勉",
	"兄弟稳住",
	"老哥加油",
	"冲",
	"冲了",
	"干就完了",
	"不亏",
	"血赚",
	"回本了",
}

// virtualChatFollowUps 是"同一个人补一句"的池子：真实聊天里，人们常把一句话
// 拆成两三条发（想到什么补什么）。跟在上一条后面的补充永远是短的、碎的。
var virtualChatFollowUps = []string{
	"草",
	"？",
	"算了",
	"没事",
	"真的",
	"骗你的",
	"啊不对",
	"打错了",
	"发错了",
	"笑死",
	"哈哈哈哈",
	"我服了",
	"nmd",
	"没了",
	"又跑了",
	"就这",
	"唉",
	"哎",
	"烦",
	"无语",
	"尴尬",
	"撤了",
	"睡觉去了",
	"明天见",
	"溜了溜了",
	"挂机",
	"AFK一会",
}

// 默认值：
//   - 间隔 10 秒（单 ticker）
//   - catch_pct 80 → 鱼获约每 12 秒一条（公屏鱼刷屏），聊天约每 50 秒一条
//   - reply_pct 35 → 偶尔有人接话
//   - followup_pct 20 → 更偶尔有人把一句话拆成两条
const virtualChatDefaultIntervalSec = 10.0
const virtualChatDefaultCatchPct = 80.0
const virtualChatDefaultReplyPct = 35.0
const virtualChatDefaultFollowUpPct = 20.0

// broadcastVirtualChat 是一轮播报：按 bot_chat_catch_pct 决定这一轮做成
// "别人上鱼"还是"普通闲聊"，并让其随机接话/补话。
func (s *Server) broadcastVirtualChat() {
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
	if rand.Float64()*100 < s.virtualChatCatchPct() {
		s.broadcastVirtualCatch()
		return
	}
	angler := s.randomVirtualAngler()
	if angler == nil {
		return
	}
	line := virtualChatPool[rand.Intn(len(virtualChatPool))]
	s.pushVirtualChatLine(angler, line, peers)

	// 同一个人补一句：把一句话拆成两条发，比一次性说完更像真人。
	if pct := s.tunableF("bot_chat_followup_pct", virtualChatDefaultFollowUpPct); pct > 0 &&
		rand.Float64()*100 < pct {
		s.scheduleVirtualChatFollowUp(angler)
	}

	// 另一个人接话：形成"对话"而不是独白。
	if pct := s.tunableF("bot_chat_reply_pct", virtualChatDefaultReplyPct); pct > 0 &&
		rand.Float64()*100 < pct {
		s.scheduleVirtualChatReply()
	}
}

// pushVirtualChatLine 把一句话以指定虚拟玩家的身份推给所有在线会话。
func (s *Server) pushVirtualChatLine(angler *virtualPlayer, text string, peers []*Session) {
	if angler == nil || text == "" || len(peers) == 0 {
		return
	}
	users := uint32(len(peers) + len(s.onlineVirtualPlayers()))
	if users < 1 {
		users = 1
	}
	body := buildVirtualSayPush(uint32(angler.UID), text, users)
	for _, sess := range peers {
		if sess == nil || sess.Closed() {
			continue
		}
		sess.Push(24, 27, body)
	}
}

// buildVirtualSayPush 构造一条普通发言的 24/27 推送，形状与
// buildChatEchoPush 完全一致，只有 f2（发送者 uid）换成虚拟玩家。
// 直接复用播报侧的 buildVirtualAnglerPush，保证两条出口只有一个形状来源。
func buildVirtualSayPush(uid uint32, text string, users uint32) *proto.Value {
	return buildVirtualAnglerPushText(0, uid, text, nil, users)
}

// scheduleVirtualChatReply 在 2~5 秒后让另一个虚拟玩家接一句。
// 独立 goroutine，不阻塞主 ticker。
func (s *Server) scheduleVirtualChatReply() {
	delay := time.Duration(2000+rand.Intn(3000)) * time.Millisecond
	go func() {
		time.Sleep(delay)
		peers := s.liveSessions()
		if len(peers) == 0 {
			return
		}
		angler := s.randomVirtualAngler()
		if angler == nil {
			return
		}
		line := virtualChatReplies[rand.Intn(len(virtualChatReplies))]
		s.pushVirtualChatLine(angler, line, peers)
	}()
}

// scheduleVirtualChatFollowUp 在 1~3 秒后让**同一个人**再补一句碎话，
// 模拟"话没说完又补一条"的真实打字习惯。
func (s *Server) scheduleVirtualChatFollowUp(angler *virtualPlayer) {
	if angler == nil {
		return
	}
	delay := time.Duration(800+rand.Intn(2200)) * time.Millisecond
	go func() {
		time.Sleep(delay)
		peers := s.liveSessions()
		if len(peers) == 0 {
			return
		}
		line := virtualChatFollowUps[rand.Intn(len(virtualChatFollowUps))]
		s.pushVirtualChatLine(angler, line, peers)
	}()
}

// virtualChatInterval 读闲聊/播报共用的轮询间隔。
func (s *Server) virtualChatInterval() time.Duration {
	sec := s.tunableF("bot_chat_interval_sec", virtualChatDefaultIntervalSec)
	d := time.Duration(sec * float64(time.Second))
	if d < virtualCatchEventMinInterval {
		d = virtualCatchEventMinInterval
	}
	if d > 24*time.Hour {
		d = 24 * time.Hour
	}
	return d
}

// virtualChatCatchPct 返回"这一轮做成上鱼播报"的概率百分比。
func (s *Server) virtualChatCatchPct() float64 {
	pct := s.tunableF("bot_chat_catch_pct", virtualChatDefaultCatchPct)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct
}

// virtualChatEnabled 报告假聊天总开关是否打开（bot_chat_enabled，默认 1）。
func (s *Server) virtualChatEnabled() bool {
	if s == nil {
		return false
	}
	return s.tunableF("bot_chat_enabled", 1) != 0
}
