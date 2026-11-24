# 字体来源与许可

| 文件 | 字重 | 说明 |
| --- | --- | --- |
| `kirakara-min.woff2` | 400 / Regular | 页面用字子集 |
| `kirakara-bold-min.woff2` | 700 / Bold | 页面用字子集（粗体标题用） |

## 上游

- 字体：**耀圆体 KirakaraMaru 1.2**
- 作者/维护：feiyangjun-1 — <https://github.com/feiyangjun-1/KirakaraMaru>
- 许可：**SIL Open Font License 1.1**，全文见同目录 `OFL.txt`（与上游 `LICENSE` 逐字节一致，4302 B）

## 我们做了什么（OFL 要求声明修改）

1. **子集化**：只保留管理后台实际用到的字符，字形数从上游 12086 降到 1022，生成两个 woff2；
2. **格式转换**：TrueType 轮廓原样保留（`unitsPerEm` 仍为 1000），只是裁字并打包成 WOFF2；
3. **未改名**：上游字体内部名就是 `KirakaraMaru`，我们不占用、也不修改它；上游 `LICENSE` 未声明保留字体名（Reserved Font Name），因此没有改名义务。

## 核对方式

把上游 `KirakaraMaru_1.2-Regular.ttf` 与 `kirakara-min.woff2` 放进同一进程比较字形坐标：抽样「总览商店钓场鱼饵参数玩家管理控制中心」18 个字，轮廓坐标 **18/18 完全一致**，`unitsPerEm` 均为 1000；`name` 表 `Version 1.2` 与上游一致。

## 分发义务

再分发这些文件时必须同时带上 `OFL.txt` 与本说明；字体不得单独售卖，不得用上游名称为修改版背书。