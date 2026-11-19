# 昼夜活跃度曲线（diel_curve）

Status: in-progress

## Problem Statement

鱼的昼夜活跃度原先只有 `fish_ecology.diel_mask` 一个四位位图：命中时段权重 1.0、未命中 0.35。
位图无法表达「窗口期」——玩家要的是峰时段最容易、过了逐渐变少但仍会出，而现在只有「开」
和「惩罚 0.35」两档，导致 `catches` 里大量鱼在其实际高峰时段被判为低谷。

更糟的是这 252 条 `diel_mask` 全部来自 `local-sim:deterministic-v3` 合成，没有任何外部数据支撑；
与玩家实测（`catch_log` 1,532 条）对照时，58 条可校验鱼里 33 条（57%）在「实测占比 ≥20% 的时段」
被判低谷，`perch` 白天、`eel` 白天都有实钓记录。

## Solution

1. 新增 `fish_ecology.diel_curve`（四段权重字符串，`[夜,晨,昼,昏]`），`fishDielWeight` 优先用它按段取系数；
   曲线为空时回退旧位图，保持兼容。
2. 曲线数据来源分级：
   - `rf4info`（`ghoomt03-sketch/rf4info` 的 `spot_activity_events`，7,125 条带 `game_time` 的真实垂钓事件，
     ≥20 样本的 81 条鱼）→ 按四段占比归一，峰值恒为 1.0；
   - `gamekee`（GameKee 鱼种页的日/夜活跃度表情，251 条）→ 只在无实测样本时使用，笑/平/愁 = 高/中/低；
   - `neutral`（无任何数据，31 条）→ 不写曲线，保持位图默认全天。
3. `diel_mask` 同步由曲线派生（≥峰值 50% 的段置位），保证仅读位图的旧路径不倒退。
4. `profile_version` 升到 5，让线上旧档案按 `WHERE profile_version < excluded.profile_version` 刷新。

## User Stories

1. 作为玩家，我在鳗鱼、江鳕这类夜行鱼的白天几乎钓不到，夜里稳定出货。
2. 作为玩家，我在欧鳊、梅花鲈这类鱼的夜间最容易爆护，白天偶尔也能碰到。
3. 作为玩家，我在葛氏鲈塘鳢这类日行鱼的白天最容易出货，夜里明显变少。

## Implementation Decisions

- 四段划分沿用 `fishActivityForHour`：夜 20–05、晨 05–09、昼 09–16、昏 16–20。
- 曲线是**乘性权重**，参与 `fishBiteEnvironmentWeight` 的 `EcologyInterest` 分组与窝料 `groundbaitGates`，不改变门限结构。
- 未采信 GameKee「愁 = 完全不出」作为 0：因为 `perch`/`n.pike` 夜间实测 30%+，写 0 会误杀；低档取 0.05 仍接近不出。
- `neutral` 的 31 条不写曲线，避免用无依据的数据压低任何鱼。

## Testing Decisions

- `TestFishDielCurveWeightsParsing`：空串、非四段、非法 JSON 均返回 nil。
- `TestFishDielWeightUsesCurveAsWindow`：曲线存在时按段取值，且各段单调对应观测占比。
- `TestFishDielWeightFallsBackToMask`：无曲线时仍按位图 1.0 / 0.35。
- `TestFishDielCurveIsMonotonicUnderPeak`：基线文件里每条曲线峰值必须为 1.0，且 ≥200 条带曲线。
- `TestFishDielCurveCoversGlobalAndLocalHours`：0 点与 24 点同段、任意小时权重落在 (0,1]。

## Out of Scope

- 不修改 `fish_distribution` 的硬编码族名单（那是另一条独立路径，开关当前关闭）。
- 不调整 `fish_query_regions` 的分布系数口径。
- 不引入温度/气压与昼夜的交互项。

## Further Notes

数据可复核：
- rf4info：`https://github.com/ghoomt03-sketch/rf4info`，`public_data/runtime.sqlite3` 的 `spot_activity_events.game_time`。
- GameKee：`https://www.gamekee.com/rf4/` 鱼种页「日间活跃度 / 夜间活跃度」表情图，按图标 sha256 归并为高/中/低三档。
- 本地实测：`catch_log` 1,532 条按四段聚合。
