# 昼夜活跃度曲线（diel_curve）

Status: done

## Problem Statement

鱼的昼夜活跃度原先只有 `fish_ecology.diel_mask` 一个四位位图：命中时段权重 1.0、未命中 0.35。
位图无法表达窗口期——峰值时段最容易出、过了峰值逐渐变少但仍会出，而现在只有「开」和「惩罚 0.35」
两档，同一时段的所有鱼要么满权重、要么统一降到 0.35，无法体现鱼种之间的差异。

## Solution

1. 新增 `fish_ecology.diel_curve`（四段权重字符串，`[夜, 晨, 昼, 昏]`），`fishDielWeight` 优先按段取系数；
   曲线为空时回退旧位图，保持兼容。
2. 曲线数据来源分三级，逐级回退：
   - `rf4info`：社区公开抓取库 `ghoomt03-sketch/rf4info` 的 `spot_activity_events`，按 `game_time`
     统计四段占比，≥20 样本的鱼直接用自身占比（峰值归一为 1.0）；
   - `gamekee`：GameKee 鱼种页的「日间活跃度 / 夜间活跃度」表情图标（笑/平/愁 = 高/中/低），
     无实测样本时，取同组合（日档, 夜档）鱼在 rf4info 上的**平均四段占比**，只借实测形状；
   - `neutral`：两处都没有的鱼不写曲线，保持位图默认全天。
3. `diel_mask` 同步由曲线派生（≥峰值 50% 的段置位），保证只读位图的旧路径不倒退。
4. `profile_version` 升到 5，让线上旧档案按 `WHERE fish_ecology.profile_version < excluded.profile_version` 刷新。

## User Stories

1. 作为玩家，我在鳗鱼、江鳕这类夜行鱼的白天几乎钓不到，夜里稳定出货。
2. 作为玩家，我在欧鳊、梅花鲈这类鱼的夜间最容易爆护，白天偶尔也能碰到。
3. 作为玩家，我在葛氏鲈塘鳢这类日行鱼的白天最容易出货，夜里明显变少。

## Implementation Decisions

- 四段划分沿用 `fishActivityForHour`：夜 20–05、晨 05–09、昼 09–16、昏 16–20。
- 曲线是**乘性权重**，参与 `fishBiteEnvironmentWeight` 的 `EcologyInterest` 分组与窝料 `groundbaitGates`，
  不改变门限结构。
- 低档最低取 0.05 而不是 0：GameKee 的「愁」表示很少出，但把权重写成 0 会让该鱼在整段内彻底消失，
  与玩家的偶发记录冲突。
- `neutral` 组不写曲线，避免用无依据的数据压低任何鱼。
- `diel_curve` 与 `diel_mask` 同时下发，读位图的旧代码路径行为不变。

## Testing Decisions

- `TestFishDielCurveWeightsParsing`：空串、非四段、非法 JSON 均返回 nil。
- `TestFishDielWeightUsesCurveAsWindow`：曲线存在时按段取值，且各段单调对应观测占比。
- `TestFishDielWeightFallsBackToMask`：无曲线时仍按位图 1.0 / 0.35。
- `TestFishDielCurveIsMonotonicUnderPeak`：基线文件里每条曲线峰值必须为 1.0，且 ≥200 条带曲线。
- `TestFishDielCurveCoversGlobalAndLocalHours`：0 点与 24 点同段、任意小时权重落在 (0,1]。

## Out of Scope

- 不修改 `fish_distribution` 的硬编码族名单（独立路径，受 `fish_distribution_enabled` 控制）。
- 不调整 `fish_query_regions` 的分布系数口径。
- 不引入温度/气压与昼夜的交互项。

## Further Notes

数据来源与复核方式：

- rf4info：`https://github.com/ghoomt03-sketch/rf4info`，`public_data/runtime.sqlite3` 的
  `spot_activity_events` 表，字段 `fish_id` 与 `game_time`（游戏内时刻）。下载后统计：
  ```text
  SELECT fish_id, game_time FROM spot_activity_events
  WHERE game_time IS NOT NULL AND game_time <> ''
  ```
  按 `game_time` 的小时数归入夜/晨/昼/昏四段计数，再按 (count + 平滑) / 总数 归一。
- GameKee：`https://www.gamekee.com/rf4/` 鱼种页的「日间活跃度 / 夜间活跃度」图标，
  图标按 sha256 归并为高/中/低三档；只用于决定「这条鱼属于哪个组合」，具体形状来自 rf4info。
- 生成后的曲线写入 `internal/gameserver/assets/fish_ecology_defs.json` 的 `diel_curve` 字段，
  由 `ensureFishEcology` 灌入数据库。
