# 抛投标称改用官服实测锚点（UL 满技能 27m）

Status: in-progress

<!-- 状态取值：draft | ready | in-progress | done | cancelled -->

## Problem Statement

`castNominalLureM = 27.5`（来自 #464，2026-11-13）标定的来源是玩家一句
「其实只能 20~30 米」，取其**区间中值 25m**，再按当时实抓因子链 0.9076
反解出 27.5。它不是任何一次官服实测的数字，而是区间猜测值。

两个独立证据指向同一条结论：这个基准偏高。

1. **官服实测锚点**（用户提供，2026-11-17）：俄服 100% 路亚熟练 + 点满
   投距技能 + 未改装低轮廓水滴轮 + UL 竿 + 3.5g 载荷，实测约 **27m**；
   改装 CHYB 轴承与增距润滑油后 30m+。改装是额外距离，不并入技能乘数。
2. **上游注释自相矛盾**：`cast_distance.go` 顶部注释声称「满级投距技能的
   UL 配置满功率 ≈25m」，但同一条因子链实算为 **32.11m**（探针
   `TestZZProbeULAnchor`，配置取 2026-11-17 16:02:52 实抓日志
   `rod=3531d037 / reel=aa39fc83`）；注释与代码差 28%。

即：`27.5` 在满技能档给出 32.11m，比官服实测 27m 高 **18.9%**；而
`#464` 注释写的 25m 比它自己的代码低 28%。两个数字互相矛盾，
说明 27.5 的校准链漏算或多算了因子。

## Evidence

| 项 | 值 | 来源 | 证据等级 |
|---|---|---|---|
| 官服 UL 满技能未改装 | 27m | 用户官服实测（俄服，100% 路亚熟练、点满技能、水滴轮、3.5g） | 玩家实测（一手） |
| 官服 UL 改装后 | 30m+ | 同上（CHYB 轴承 + 增距润滑油） | 玩家实测（一手） |
| 上游代码满技能实算 | 32.11m | 探针复算，配置取实抓日志 | 可复现测试 |
| 上游注释声称 | ≈25m | `cast_distance.go:8` 注释 | 代码自述（与实算冲突） |
| 官方飞距离要素 | 竿长/调性/硬度、Test 适配、轮特性、线种、技能、体力 | 官方日文手册 `https://rf4game.jp/userguide/`「投げる距離と精度」 | 官方手册 |
| 官方技能加成 | 竿 7 级 +35%、轮 5 级 +25%（每级 +5%，加法叠加） | 客户端 `zh_CN.txt` `ability_501/514` 文案 | 官方文案 |
| 官方伸缩手竿公式 | 飞距离 = 竿长 × 2 | 同上手册 | 官方手册 |

官方手册同时确认：杆长/调性/硬度是距离要素之一，而当前标称
`castNominalDistanceM(rigType)` 完全不读竿参数（竿长只进 ±18% 窄带），
属于另一个独立缺口，见「后续待办」。

## Solution

只改乘法基准，不动修正器曲线（最小 diff）：

- `castNominalLureM = 27.5 → 23.13`。
  反解：`27.0 / 32.11 × 27.5 = 23.12`，取 23.13，使
  「UL 满熟练 + 满技能 + 未改装 + 3.5g」命中 27.0m。
- 同步更新受影响的断言（全部 ×0.8411）：
  `TestCastCalibrationBands`（6000 杯 14.6~15.1、2000 杯 17.0~17.5）、
  `TestCastHighEndGearCeilingNearThirtyMeters`（24.7~25.3）、
  `TestCastFreshwaterLureOfficialAnchorBand`（20.7~21.3）、
  `TestCastNominalDistanceByRigFamily`（23.13）。
- 新增 `TestCastOfficialULAnchorTwentySevenMeters` 锁定官服锚点本身，
  配置逐字段取自实抓日志，任何人可复跑。

## Non-Goals

- 不动 spool_size / line / rod 修正器曲线的形状（无实测依据支持改动）。
- 不动海钓路亚 45 与默认族 60（尚无官服锚点）。
- 不在 PR 正文记录俄服信息来源细节（用户要求：只记距离）。

## 后续待办（本次不做）

1. **手竿族固定 60m**：与官方「伸缩手竿 = 竿长 × 2」冲突（8m 竿应为 16m）。
   需要先确认客户端 rigType 与「伸缩手竿」的对应关系，再单独出 spec。
2. **装备自带抛投加成**（官方手册第 11 条「投げる際のボーナス」）当前未实现。
3. 竿长/调性/硬度的权重远小于技能（×1.6），与官方要素清单的排序不一致。

## Verification

- `go test ./internal/gameserver/ -run 'TestCast|TestRodTest|TestPlayerCondition'`
- `go test ./...` 全量
- `go vet ./...`、`go build ./...`
- 实机：本地服抛一杆 UL，`日志 distance_m` 应与 `TestCastOfficialULAnchorTwentySevenMeters` 同量级。