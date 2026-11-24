# 官方实测数据表入库：曲线池直配 + f5 每克均值表 + f6 量化表

Status: in-progress

## Problem Statement

P1（fish-pull-k-table）修好了拉力主链，但三类搏鱼参数仍是旧口径，与官服
14/14 A 级样本（31 条，2026-09-18 真值表 + 2026-11-10/11 新抓）对拍有系统性偏差：

1. **曲线池错配**：模拟器只有 8 张池，其余鱼按 archetype 借池——官方抓包证实
   merlang 借了 crucian 池（官方 8 行 vs 借到 28 行）、piksha 借 perch 池等
   6+ 处行为结构错配。本次新抓包已拿到 9 种官方池的完整行数据。
2. **f5 体力池**：常数 `verifiedStaminaPerGram=2.388/g` × archetype 猜测倍率，
   官方每克均值从 crucian 2.29 到 merlang **10.50**——海钓体力池系统性偏低
   62~78%，搏鱼时长严重偏短。
3. **f6 抗疲劳系数**：全鱼种发常数 0.5628；官方为 0.05 步进逐鱼种量化值
   （perch 0.70、n.pike 0.80、s.bream 0.30……），种内还有随体重 +0.05 台阶。

## Solution

### 1. 官方池入库（fight_curves_sets.json）

用一次性导出器（tmp_analysis/poolexport，不入库）从抓包 14/14 解出 19 列行数据
（f0/f15–f18 按整数，其余按 float32），新增 8 个鱼种键池并替换 c.roach 池
（官方 680g 样本与旧池有 2 行微差，以官方为准）：

| 池键 | 行数 | 供体样本 |
|---|---|---|
| taran | 13 | float_14_14_16（611g） |
| b_gill | 13 | lure_14_14_157（113g） |
| n.pike | 12 | lure_14_14_37（4305g） |
| merlang | 8 | marine_14_14_56（252g） |
| piksha | 9 | marine_14_14_87（2975g） |
| din_l_carp | 24 | push_1414（1715g） |
| g_carp | 13 | dd2_1414_818（5716g） |
| carp | 16 | dd2_1414_44（1063g） |
| c.roach（替换） | 18 | float_14_14_44（680g） |

池副本带供体鱼的 uniform scale，`buildFightCurves` 按 `pull/池基准` 再归一，
对任意鱼误差 ≤3%（= 官方 row100/f8 比率 0.968~1.048 的抖动）。既有
crucian/perch 池已被官方多重量级逐行验证，保持不动。选择逻辑不变：
`s.curves[鱼种键]` 命中即直配，未入库种维持 archetype 借池（不比现状差）。

### 2. f5 每克均值表（fishStaminaRatePerGram，14 种）

`f5 = rate(species) × 体重g`；官方种内散布 ±13%（perch）~±57%（g_carp），
均值表把个体误差压回散布带（grade 检验证实散布与档位无关，是官方随机因子）。
未实测种回退旧公式（2.388 × archetype StaminaScale）。

均值（每克）：merlang 10.50 / piksha 6.39 / s.bream 5.87 / b_gill 4.07 /
taran 3.55 / din_l_carp 2.77 / n.pike 2.76 / perch 2.75 / c.carp 2.52 /
c.roach 2.51 / g_carp 2.44 / crucian 2.29 / carp 2.10 / b_carp 2.09。

### 3. f6 量化表（fishF6Table + fishF6Step）

实测 14 种基值 + 两个已钉死的体重台阶（perch ≥500g→0.80、crucian ≥450g→0.60）。
未实测种回退 0.5628（`fish_initial_stamina_ratio` tunable 语义不变，仅对未实测
鱼种生效——管理端「游戏参数」已收录该条目并注明此边界，避免运维对实测种误调）。

## Non-Goals

- f5 散布的模型形式（长度/随机因子）——需更多样本，P3；
- 未实测种的 f5/f6 覆盖（P3）；amp 列独立缩放因子（样本不足）。

## Tests

- `TestFishStaminaMatchesOfficialAnchors`：14 种逐样本锚点，容差 = 该种种内
  散布带（merlang ±3% ~ g_carp ±60%）。
- `TestFishF6MatchesOfficialTable`：量化表 + 台阶 + fallback。
- `TestFightCurvesSpeciesPoolsLoaded`：新池行数直配（taran 13 / merlang 8 /
  g_carp 13 / din_l_carp 24…），借池兜底仍可用。
- `TestFightCurvesEveryPoolHasPhase100Base`：每个池恰 1 条 f0=100 基准行且
  loader 钉到首位——uniform 缩放锚（selected[0][8]）不许静默错行。
- 既有 `TestFish27UsesVerifiedFightFields` 的 c.roach f6 期望改为官方量化值 0.50。

## Risks

- 海钓搏鱼时长显著变长（merlang ≈ ×3.4：f5 ×3.9 与 f6 0.50 疲劳加速的联立），
  属官方口径修复；实机 A/B 对照海钓搏鱼时长。
- 借池种的曲线结构不变（fallback 路径原样），行为风险集中在 9 个直配种。

## v2 增补：旧包样本补入 f5/f6 表、f6 台阶按全样本重定（2026-11-24）

数据源同 fish-pull-k-table spec v2.2。

### f5 每克均值表新增 3 种

| species | 样本 | 每克均值 |
|---|---|---|
| a_char | 415g ×1 | 6.12 |
| gc_smelt | 772g ×1 | 5.49 |
| m_carp | 2312~5370g ×4 | 2.24（种内 2.09~2.44） |

m_carp 此前不在表内、运行时走鲤科组值 2.50/g，现按 4 样本实测入表。perch 加入
374g 样本后 10 样本的每克均值是 2.68，与表值 2.69 只差末位，表值不动。

### f6 量化表新增 3 种

a_char 0.50、gc_smelt 0.50（原走鲑科组 0.60）；m_carp 0.65（原走鲤科组 0.55）。

### f6 台阶：按全部样本重定

把 76 条样本按"鱼种×档位"分组后，f6 在每组内完全恒定（39 组零例外），上台阶
只发生在档位之间。体重阈值按两侧最近样本的中点重定，再按档位取整：

| species | 低档侧样本 → f6 | 高档侧样本 → f6 | 旧阈值 | 新阈值 |
|---|---|---|---|---|
| perch | 93~342g → 0.70 | 374~1070g → 0.80 | 500g | 360g |
| crucian | 193~341g → 0.55 | 527~743g → 0.60 | 450g | 450g（不变） |
| carp | 1063~2434g → 0.65 | 4137~5710g → 0.75 | 无 | 3300g |
| m_carp | 2312~3494g → 0.65 | 4756~5370g → 0.75 | 无 | 4100g |

- 阈值取整：perch 358→360g、carp 3285.5→3300g、m_carp 4125→4100g；
  crucian 保留既有 450g。
- perch：374g（2 档）实测 0.80，旧阈值 500g 会把它判成 0.70。
- carp / m_carp：新增 0.10 的台阶。此前 carp 全档 0.65。
- 未观察到台阶的实测种（coalfish、m_scorpius、rudd 各有两档样本）维持基值。
- 3 档及以上没有样本，沿用高档侧的值。
- 仍按体重阈值判定，不改成按档位字段：模拟器的档位线本身有误差（perch 达标线
  是 340g，而官服 342g 仍是 1 档），两侧样本的体重中点更贴近官服样本。

### Tests

- `officialStaminaAnchors` 新增 6 条（a_char / gc_smelt ±3%，m_carp ±15%）。
- `TestFishF6MatchesOfficialTable` 新增 15 个用例：perch 374g、carp ×2、
  m_carp ×2、a_char、gc_smelt，以及 perch/carp/m_carp/crucian 的台阶边界。
- `TestFishFightCrossLayerConsistency` 新增 a_char / gc_smelt 的全链路用例。
