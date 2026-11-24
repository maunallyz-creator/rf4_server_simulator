# 管理后台与控制中心外观重构

Status: done

## Problem Statement

管理后台（`/admin/`）沿用旧的一版深色设计：总览只有四格统计和常用功能卡，除商店页外没有二级页签与右栏信息块，表格行距偏松，下半页留白很多；控制中心（`:9500/`）也是同一套旧视觉，并且「诊断」页签与后台总览重复展示同一份进程状态。玩家在游戏里遇到问题时，既不容易找到「开始游戏 / 停止服务」的入口，也看不出后台现在改的是哪套参数。

## Solution

后台 12 个页面统一成一套纸白 + 航拍照片的外观，每页都是「顶部照片带（标题 + 关键数字 + 事实行）+ 二级页签 + 左内容右信息块」的固定骨架；总览改成「从这里开始 → 运行日志 → 当前生效的关键口径」，右栏给出服务器概况、最近修改与数据来源说明。控制中心改成两栏：左边状态、步骤、服务器工作台、后台信息与声明，右边使用说明、运行日志、版本与更新；进程状态与日志不再有独立的「诊断」页签。只读数据（总览、商店、参数、水域、鱼饵、审计、关于）不登录也能看，登录只用来改数据和管理玩家。

## User Stories

1. 作为本地开服的玩家，我希望打开 `:9500/` 就能看到服务器是否在跑、点一下能开始游戏或停止服务，以便不用去翻文件夹里的启动脚本。
2. 作为调参的服主，我希望总览直接告诉我「现在生效的是哪套口径、去哪改」，以便不用逐页翻参数。
3. 作为没登录的访客，我希望只读页面（总览、商店、参数、水域、鱼饵、审计、关于）能直接看，以便先判断这套服务端适不适合自己。
4. 边界：未登录时改任何东西应提示「这一步需要登录」，不跳页、不清掉已填内容；玩家管理页在未登录时显示登录引导而不是空白；读不到后端字段（如进程状态）时显示「未上报」而不是假数字。
5. 边界：控制中心在 iframe 里打开后台时，不能出现「控制中心顶栏 + 后台 hero」两条不对齐的横带。

## Implementation Decisions

- 前端样式：新增 `internal/admin/web/app/theme-aerial.css`，按已定稿的演示稿整份搬运，末尾追加一层桥接规则，把项目原有的 `.btn/.card/.input/.badge/.callout/.dialog/.savebar/.toast/.login/.md` 等组件改成同一套纸面语言；`index.html` 引入该表并预加载字体与首屏照片。
- 前端结构：`internal/admin/web/app/core.js` 负责 `pageHead/pageTabs/work/ib/pblock/tbl/kv/brief/logsBlock/startGrid/loginDialog` 等骨架函数；`main.js` 负责左栏与顶栏；`pages/*.js` 只拼内容。路由 hash 与演示稿对齐（`#shop/price`、`#tuning/gameplay`、`#release`），旧 hash 做兼容映射。
- 鉴权：`internal/admin/api.go` 把 22 个只读 GET 改为匿名可读（仪表盘、可调参数、目录、商店与货架、水域与鱼群、鱼钩档案、鱼饵偏好、潮汐、审计、关于、发布说明）；写接口与 `/api/players*`、`/api/me` 仍需令牌。前端只有在玩家管理页拦登录，其余页面未登录只禁用保存并给出提示。
- 控制中心：`cmd/launcher/panel.html` 与 `cmd/launcher/main.go`。删掉「诊断」页签，进程状态、运行日志、版本卡并入首页与后台总览；新增公开只读 `GET /api/overview`，返回后台地址、管理员名、主机、`ready` 与档案/物品/邮件/审计/可调参数计数，供首页「后台信息」与「服务器工作台」使用。
- 数据与迁移：不改表结构，不新增迁移。`/api/overview` 只做只读计数。
- 兼容：不改任何游戏协议与客户端行为，客户端版本基线不变；回滚方式为还原前端资源与 `cmd/launcher` 两处改动（样式与页面均为静态资源，删掉即可回到旧界面）。
- 资源与许可：字体与照片随前端静态资源内嵌（`//go:embed all:web`），许可文本与出处同目录存放，见下节「第三方资源与许可」。

## 第三方资源与许可

| 资源 | 来源 | 许可 | 改动 |
| --- | --- | --- | --- |
| `app/fonts/kirakara-{min,bold-min}.woff2` | 耀圆体 **KirakaraMaru 1.2**（`feiyangjun-1/KirakaraMaru`） | SIL OFL 1.1（`app/fonts/OFL.txt`） | 按页面用字子集化；族名取自上游 `KirakaraMaru`，未改动字形轮廓 |
| `app/bg-hero.jpg`、`app/bg-band.jpg`、`app/bg-frost.jpg` | [Bontecou Lake aerial north shore.jpg](https://commons.wikimedia.org/wiki/File:Bontecou_Lake_aerial_north_shore.jpg)，作者 Juliancolton | CC BY-SA 4.0 | 缩放 / 横向裁剪 / 由 `bg-band.jpg` 模糊放大 |
| `app/bg-biwa.jpg` | [2011-11-17 13-15-36 Germany Baden-Württemberg Tuningen.jpg](https://commons.wikimedia.org/wiki/File:2011-11-17_13-15-36_Germany_Baden-W%C3%BCrttemberg_Tuningen.jpg)，作者 Hansueli Krapf（User:Simisa） | CC BY-SA 3.0 | 等比缩放 |

证据：字体文件的 `name` 表实测为 `KirakaraMaru` / `KirakaraMaru Bold`，`Version 1.2`（演示稿 `<style>` 顶部那句「得意黑 Smiley Sans」是早期迭代遗留注释，与二进制不符，已按实际字体更正）；照片出处由原图检索与图像相关性确认（`bg-frost.jpg` 与 `bg-band.jpg` 模糊后相关 0.997）。

义务：OFL 要求再分发时保留版权行与许可全文，并说明修改；CC BY-SA 要求署名并相同方式共享——图片的裁剪/缩放/模糊版本继续以 CC BY-SA 提供（集中署名在 `app/IMAGE-CREDITS.md`），ShareAlike 仅作用于图片，不改变代码许可。

## Testing Decisions

- 鉴权接缝（已有）：`internal/admin/api_test.go` 断言匿名读仪表盘返回 200、匿名写可调参数返回 401；`hook_profiles_test.go`、`release_notes_test.go` 断言对应只读接口匿名可读。回滚「匿名只读」后这三个断言会变红，正是需要的信号。
- 新接口接缝（新增）：`cmd/launcher/main_test.go` 的 `TestOverviewReportsAdminCounts` 用真实 SQLite（`store.Open` + `Migrate`）验证 `ready`、身份字段与四类计数；`TestOverviewWithoutAdminDB` 验证库未就绪时返回 `ready:false`。
- 前端：仓库没有前端测试设施。以实机回归为准（下面命令栏列出 Playwright 断言项与实测值），不把脚本提交进仓库。期望值来自与定稿演示稿在同一视口的逐项几何对照，不是「用同一套代码再算一遍」。
- 测试数据：`TestOverviewReportsAdminCounts` 的期望值来自「新库四类计数为 0、迁移自带的参数表非空」，属可手算的字面量。

## Out of Scope

- 不改游戏协议、结算、发鱼、饵料、经济链路。
- 不改数据库结构与迁移。
- 不做移动端专门适配（只保证窄屏不横向溢出、布局与演示稿同视口一致）。
- 不引入前端框架或打包工具，仍是原生 ES module + 手写 CSS。
- 不加入任何运营、支付、邀请码相关入口。

## Further Notes

- 外观基准为已定稿的演示稿，比对方式是同一视口下的计算样式与几何（`[事实]`：hero 高度、两栏宽度、表格字号与行高逐项对齐）。
- 字体 `KirakaraMaru` 与四张照片来自该演示稿；来源与许可已逐项核实并入库（`[事实]`：字体为 KirakaraMaru 1.2 / OFL 1.1，照片为 Wikimedia Commons 上的 CC BY-SA 4.0 与 3.0 两张原图及其派生图），许可文本见 `app/fonts/OFL.txt`、`app/fonts/README.md` 与 `app/IMAGE-CREDITS.md`。再分发时须保留这两份文件；若维护者希望免除署名/同享义务，需替换为自绘底纹或其他素材。
- 本地构建没有 `BuildTs`，控制中心的更新检测显示「本地构建 / 跳过检测」，属预期行为而非失败（`[事实]`：`update_status.error` 文案）。
