# 拟饵通配按官方渔获占比分档

Status: done

## Problem Statement

规则表里的“拟饵”是 `item_def.category = 17` 的全通配，草鱼、欧鳊、短头梭鲃等偶尔咬到拟饵的非路亚鱼也会成为日常路亚目标。河蚌、淡水贻贝、河螯虾还有少量微铅挂底误捕纪录，若只按鱼种占比放行，仍可能被亮片整天刷出。

## Solution

`fish_bait_rules_enabled=1` 且 `bait_share_*` 三项阈值均为正时，拟饵候选按鱼种官方拟饵占比分档：低于 1% 出池，1%~5% 满档压到四分之一，5% 以上不额外干预。双壳类与河螯虾即使有具体饵的强证据也硬排除。关闭任一占比阈值时整道拟饵门关闭，恢复旧行为。

## User Stories

1. 作为路亚玩家，我希望挂拟饵时不再整天刷出草鱼、河蚌和河螯虾，以便路亚鱼种组成符合实际。
2. 边界：官方数据表缺失、缓存过期或查询失败时按未知处理，不把整池鱼封死。
3. 边界：管理员把 `bait_share_strong/medium/tail` 任一设为 0 时，可立即回到旧判鱼行为。

## Implementation Decisions

- `internal/gameserver/fishing_bait_rules.go`：拟饵门仅在食性分级开启时生效；硬排除先于具体饵强证据。
- `internal/gameserver/server.go`：rf4 缓存同时保存鱼种拟饵占比，与饵占比共用锁和 TTL。
- 缓存查询失败时清空两张旧快照并记录为已加载，避免继续使用旧数据或在每次判鱼时重试全表聚合；30 分钟后重试。
- 不改表结构、协议字段和经济链路。回滚方式为将任一 `bait_share_*` 阈值设为 0。

## Testing Decisions

- `TestLureGateByFishShare`：覆盖 ≥5%、1%~5%、<1%、未知数据和天然饵。
- `TestLureGateHardExclusionBeatsStrongBaitShare`：三种硬排除鱼即使具体饵占比与鱼种占比均为高值也不接受拟饵。
- `TestLureGateDisabledWithShareThresholdsRestoresLegacy`：三项阈值归零后恢复旧行为。
- `TestRf4LureShareTTLExpiresToUnknown`：过期快照返回未知。
- `TestRf4ShareQueryErrorClearsStaleSnapshot`：SQL 查询失败后清空旧快照。
- 回滚拟饵门后，前四条新增行为测试会变红。

## Out of Scope

- 不调整 `rf4records_catch` 抓取频率、饵映射覆盖率或鱼种权重。
- 不改变 strict 精确饵模式的 allow/deny 语义。
- 不为拟饵门新增管理页 UI。

## Further Notes

- `[事实]` `item_def.category = 17` 包含拟饵及其同槽吸引成分；现有判鱼链一直用该分类识别拟饵。
- `[事实]` `rf4records_catch` 的占比缓存由现有异步预热路径维护，TTL 为 30 分钟。
- `[未验证]` 本地环境没有可用生产 `rf4records_catch` 数据，分档边界使用合成缓存测试。