# 抛投标称按竿 Test 分档（修 #464 的"一族一个常量"）

Status: in-progress

<!-- 状态取值：draft | ready | in-progress | done | cancelled
     完成时原地改这里，不要归档、不要搬家、不要另开第二份。 -->

## Problem Statement

标称距离只有一个族常量，**完全不读竿的 Test 上限**，导致同一族里所有竿
共享同一个基准，两端同时错：

1. **轻装偏高**。`castNominalLureM=27.5` 是 #464 按一根 1.8m、Test 1-8g 的
   UL 配置实抓的。同一常量套到满技能、满熟练的 2m/Test 0.5-4g UL 上，
   实测 32.11m——比官服实抛（100% 路亚熟练 + 技能点满 + 水滴轮 + 未改装，
   约 27m）高 **18.9%**。`cast_distance.go` 第 8 行注释自己写的是
   "满级投距技能的 UL 配置满功率 ≈25m"，与代码实跑值自相矛盾。
2. **中重装偏低**。同一常量套到 2.7m/Test 15-49g + 63g 载荷 + 6000 线杯上
   只有约 20-25m，玩家口径是"封顶 50m 才对"。竿越硬、饵越重，被压得越狠。

根因在官方手册里写得很直白——飞距离要素第 3 条是「釣り竿の特性：**長さ、
構造、強さ**」，第 1 条是「仕掛けの重さの釣り竿に対する**適応度**」，第 2 条
是「リールの特性」。上游把竿特性收敛成一个族常量，等于把第 1、3 条要素
丢掉了；#464 又把一根 UL 的实抓值当成整个淡水路亚族的基准，是"用 UL 的
问题推广到全族"。

## 证据来源

| 证据 | 内容 | 来源 |
| --- | --- | --- |
| 官方公式 | 伸缩手竿飞距离 = **竿长 × 2** | `https://rf4game.jp/userguide/`「投げる距離と精度」 |
| 官方要素表 | 11 条飞距离要素，含竿长/结构/硬度、钓组重量适配度、轮特性、线种、经验值、竿轮技能、导环与部件状态、剩余体力、抛投加成 | 同上（第 215-226 行） |
| 官方定性 | 软/中调长竿最远、硬竿抛不远；轻饵配小线杯更远；线种影响距离 | 同上（228-233 行） |
| 技能数值 | 竿类 7 级 +35%、轮类 5 级 +25%，**加法叠加 +60%** | 客户端 `zh_CN.txt` `ability_501_1..7` / `ability_514_1..5` |
| 玩家实测（锚点 1） | 俄罗斯官服：100% 路亚熟练 + 技能点满 + 水滴轮 + 未改装 UL ≈ **27m**；改装 CHYB 轴承与距离润滑油后 30+m | 用户 m02092 实抛（本项目维护者） |
| 玩家实测（锚点 2） | Test 49g 竿 + 63g 载荷 + 6000 线杯封顶 **50m** | 用户 m01182 实抛 |
| 玩家实测（旁证） | 鲤鱼竿 4m + 140g 铅坠 135-146m；Atlas 比 Dominator 远 10-15m | `rf4game.de` topic 2219（2018 无风实测） |
| 已合并 PR 对照 | #464 引入 27.5 时锚点换算含满级投距技能 ×1.6，但只覆盖 UL 一档；#491 补了 Test 缺失档的 0.85 保守折减；#492 补了载荷未知档的 0.80 回归测试；#462 接入了投距技能 | `git log --oneline -- internal/gameserver/cast_distance.go` |

> 官服具体米数不写进 PR 正文（维护者要求），只作为本地锚点与测试断言依据。

## Solution

标称从"族常量"改成"族基准 × 竿 Test 上限的分档曲线"，两个锚点反解：

```
nominal(rigType, testMaxG) =
    marshal(50-54)              → 45.0            // 待补锚点，不动
    非路亚族                     → 60.0            // 手竿/浮钓/底钓，不动
    testMaxG < 4                → 23.06           // 锚点档，低于 4g 一律取锚点
    否则                         → 23.06 × (testMaxG / 4)^0.41
```

- 锚点档 4g：`23.06 × 0.9132(载荷) × … × 1.6(技能) = 27.00m`，命中官服 UL。
- Test 49g 档：`23.06 × (49/4)^0.41 = 64.4`，乘同一因子链得 **50.04m**，命中
  玩家"封顶 50m"口径。
- 派生：Test 8g ≈ 32.6m、Test 15g ≈ 38.7m、Test 30g ≈ 51m、Test 70g ≈ 72m
  （3m 竿、满技能口径），随 Test 单调递增，覆盖"UL 27m → 中重装 50m+ → 重装 70m+"。

## User Stories

1. 作为超轻玩家，我希望满技能、未改装的 UL 抛 27m 上下，与官服一致。
2. 作为中重装玩家，我希望 Test 49g 竿配 63g 载荷能抛到 50m，而不是被压到 20m。
3. 作为重装玩家，我希望越硬的竿越远，而不是所有竿共享一个基准。
4. 边界：不填 Test 上限（`testMaxG<=0`）时取 4g 锚点档 23.06，不 panic、
   不归零。
5. 边界：Test 上限低于 4g（如 0.5-3g 的微物竿）时同样取锚点档，不被
   `(testMax/4)^0.41` 反向缩小。
6. 边界：海钓路亚（50-54）与手竿/浮钓/底钓族标称完全不动，仍是 45 / 60。
7. 边界：载荷未知（payload<=0）时仍走 #492 的 0.80 折减，不因本次改动回归。

## Implementation Decisions

1. `castNominalDistanceForRod(rigType int64, testMaxG float64) float64` 为新
   唯一入口；`castNominalDistanceM(rigType)` 保留为 `testMaxG=0` 的兼容包装。
2. `castNominalLureM` / `castNominalTestExp` 从 const 改成 **var**，便于测试按
   锚点重定标后回填；生产代码只读。
3. `calculateCastDistance` 首行由 `castNominalDistanceM(c.RigType)` 改为
   `castNominalDistanceForRod(c.RigType, c.RodTestMaxG)`。
4. `p.BaseM` 同步改为按竿分档，保证日志与结算界面展示的基准与实算一致。
5. 最小 diff：只动 `internal/gameserver/cast_distance.go`（+30/-6 量级）与
   对应测试；不碰修正器曲线、不碰 `handlers_game.go` 的红区收口、不碰
   `player_condition` / `fishing.go`。

## Testing Decisions

层次：单元测试（包内 `calculateCastDistance` 直调，不需要数据库/网关）。

| 测试 | 断言 |
| --- | --- |
| `TestCastOfficialULAnchor` | 2m/Test 0.5-4g + 3.53g + 水滴轮 + 满熟练满技能 = **27.0m**（±0.1） |
| `TestCastOfficialMHAnchor` | 2.7m/Test 15-49g + 63g + 6000 杯 + 满熟练满技能 = **50m**（±0.4） |
| `TestCastNominalDistanceByRigFamily` | 锚点档 = 23.06；海钓 45；默认族 60 不变 |
| `TestCastCalibrationBands` | 8/10g 载荷 6000 杯 33.4-34.4m、2000 杯 39.0-40.0m；小杯比大杯远 ≥5m |
| `TestCastHighEndGearCeilingAfterRodTestScaling` | 4.2m/Test30/8g/1 号杯 56.4-57.2m |
| `TestCastFreshwaterLureOfficialAnchorBand` | 1.8m/Test8 旧实抓配置 27.6-28.1m |

回滚：本次只改标称与测试断言，`git revert` 单个 commit 即可回到 #464 口径；
数据层无迁移、无 DB 变更。

## 风险与未覆盖

- 海钓路亚（50-54）仍是 45，缺官服锚点，本次不动。
- 手竿/伸缩竿仍走 60，与官方「竿长 × 2」不符（8m 竿官方 16m vs 本地 60m），
  这是另一个独立问题，另立 spec，不在本次范围。
- 曲线指数 0.41 只由两个锚点反解，Test 70g 以上无实测校验。
- `castRodLengthFactor`（0.85-1.18 窄带）未动；官方说竿长是主要要素，若后续
  拿到同 Test、不同长度的官服对照，再单独校准。
