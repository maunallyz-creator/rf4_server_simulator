# 图片素材来源与许可

管理后台用的 4 张背景照片都来自 **Wikimedia Commons**，两张原图都是自由许可（CC BY-SA），允许商用与修改，需要署名并以相同方式共享。文件都在 `internal/admin/web/app/` 下。

| 文件 | 原始文件 | 作者 | 许可 | 本项目做的加工 |
| --- | --- | --- | --- | --- |
| `bg-hero.jpg`（1920×1078） | [Bontecou Lake aerial north shore.jpg](https://commons.wikimedia.org/wiki/File:Bontecou_Lake_aerial_north_shore.jpg)（3967×2228） | [Juliancolton](https://commons.wikimedia.org/wiki/User:Juliancolton) | [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/) | 等比缩放到 1920×1078 |
| `bg-band.jpg`（1920×560） | 同上 | Juliancolton | CC BY-SA 4.0 | 从原图横向裁出 1920×560 横幅 |
| `bg-frost.jpg`（3428×1000） | 同上（经由 `bg-band.jpg`） | Juliancolton | CC BY-SA 4.0 | 由 `bg-band.jpg` 高斯模糊后放大到 3428×1000，用作页面底纹 |
| `bg-biwa.jpg`（1920×1282） | [2011-11-17 13-15-36 Germany Baden-Württemberg Tuningen.jpg](https://commons.wikimedia.org/wiki/File:2011-11-17_13-15-36_Germany_Baden-W%C3%BCrttemberg_Tuningen.jpg)（4179×2775） | [Hansueli Krapf (User:Simisa)](https://commons.wikimedia.org/wiki/User:Simisa) | [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/) | 等比缩放到 1920×1282 |

## 许可义务

- **署名**：界面上以照片作为氛围背景、不再单独列署名，因此把署名集中放在本文件；再分发这份静态资源时请保留本文件与下面两行出处。
- **相同方式共享（ShareAlike）**：上表的裁剪、缩放、模糊版本属于改编作品，按原许可的 ShareAlike 条款，这些**图片**继续以 CC BY-SA 4.0 / 3.0 提供。代码本身的许可不受影响，两者互不传染。
- **无额外限制**：未对原图添加 DRM、未声明额外授权限制。

## 复核方法

1. 原图页面（Commons）里有 `Licensing` 段落、作者与许可版本，可直接核对上表。
2. 本仓库内的派生关系可用图像相关性验证：`bg-frost.jpg` 与 `bg-band.jpg` 在重度模糊后归一化相关 0.997（即 frost 就是 band 的模糊放大版），`bg-band.jpg` 与 `bg-hero.jpg` 来自同一张航拍原图。
3. 到 Commons 检索标题 `Bontecou Lake aerial north shore` 与 `Tuningen` 即可拿到 3967×2228 / 4179×2775 的原始文件。