# 超能力德扑 · Poker Superpower Simulator

**你亲自操作 A 或 B，对抗五个原版 PokerForBots `complex` 机器人。**

本地运行、离线可玩、虚拟筹码、无需账号。程序不替真人作决定，也不计算下注建议。A/B 是两种信息权限，玩家自己过牌、跟注、加注或弃牌。

[English](README.en.md) · [MIT License](LICENSE) · [上游及第三方声明](THIRD_PARTY_NOTICES.md)

![牌桌与独立统计界面](docs/screenshot.png)

## 能力与规则

| 模式 | 你能看见 | 仍然未知 |
|---|---|---|
| A · 底牌透视 | 所有玩家的底牌，包括已经弃牌的玩家 | 尚未发出的公共牌 |
| B · 预知公牌 | 提前固定的五张公牌，按 flop / turn / river 顺序显示 | 对手底牌，直到本手结束复盘 |

- 六人无限注德扑：一位真人、五个相同的原版 `complex` 机器人。
- 每手 100BB，盲注 0.5 / 1BB，无抽水、无前注，每手结束重置筹码。
- 发牌使用新随机种子；A、B 各自轮换庄位，切换模式不会固定占据某些位置。
- 每手开始前固定牌堆。B 的预知公牌就是实际会按顺序发出的牌。沿用上游引擎，不烧牌。
- 对手不知道真人的底牌或未来公共牌，不强制跟注，不根据能力模式改变策略。
- 进行中的一手不能切换能力、重新发牌或开始新会话。刷新、关闭、重启后继续该手。
- 真人弃牌后，机器人完成剩余动作并结算。只有完成的手牌计入统计。
- 结束后可以复盘所有底牌及预定公共牌；这些额外信息不会在 B 的进行中牌局 API 中出现。

机器人是固定的启发式策略，不代表真人或 GTO 水平。这是信息优势实验工具，没有真实下注、支付、充值或联网对战。

## 下载运行

从 [GitHub Releases](https://github.com/wykhao/Poker_Superpower_Simulator/releases/latest) 下载对应系统 ZIP；每个 ZIP 都是独立程序，不需要 Go、Node、Python 或原实验项目。

[Windows x64 下载](https://github.com/wykhao/Poker_Superpower_Simulator/releases/download/v0.1.0/poker-human-simulator-0.1.0-windows-amd64.zip) · [全部平台和源码](https://github.com/wykhao/Poker_Superpower_Simulator/releases/tag/v0.1.0)

### Windows

1. 下载并解压 `poker-human-simulator-版本-windows-amd64.zip`；Windows ARM 设备可选择 `windows-arm64`。
2. 双击 `poker-simulator.exe`，浏览器会打开本地牌桌。
3. 页面右上角“退出应用”会关闭本地服务，已经提交的动作和历史仍保存。

如果使用本地源码目录中的现成构建，也可以双击根目录 `Start-Poker.vbs`，它会启动 `bin/poker-simulator.exe`。

### macOS / Linux

解压后在目录中运行：

```sh
chmod +x poker-simulator
./poker-simulator
```

提供 macOS Intel / Apple Silicon、Linux x64 / ARM64 构建。退出网页右上角应用按钮，或在前台终端按 Ctrl+C。

当前构建未做商业代码签名或 macOS 公证，系统可能要求确认来源；也可以按下面步骤从源码构建。默认仅监听 `127.0.0.1:8787`，不能供局域网其他电脑访问。

## 从源码运行

需要 **Go 1.25.1 或以上**。前端是内嵌的 HTML/CSS/JavaScript，没有前端编译步骤。依赖已经放入 `vendor/`，构建不依赖旁边的实验目录。

```sh
go run ./cmd/poker-simulator
```

或者编译单个文件：

```sh
go build -trimpath -buildvcs=false -o poker-simulator ./cmd/poker-simulator
```

Windows PowerShell：

```powershell
.\scripts\build.ps1
.\bin\poker-simulator.exe
```

`build.ps1 -Go <go.exe路径>` 可指定 Go；Windows 发布构建不弹出控制台。

常用参数：

```sh
poker-simulator --port 0                 # 自动选择可用端口
poker-simulator --no-browser             # 只输出地址，不自动打开浏览器
poker-simulator --data-dir ./my-history  # 单独的数据目录
poker-simulator --version
```

同一数据目录只允许一个应用进程。应用运行时再次双击启动，会重新打开已有牌桌。若默认端口被其他软件占用，可以使用 `--port 0`。

## 操作与成绩

- **加注至**：输入的是本条街的累计下注额，以 BB 显示、0.1BB 为最小单位。程序会检查最低加注和剩余筹码。
- 快捷尺寸：⅓ 池、½ 池、1 倍池、2 倍池，均按“先补足跟注后，再按底池比例加注”计算。
- 快捷键：`F` 弃牌，`C` 过牌／跟注，`R` 聚焦金额输入。输入文字时快捷键不生效。
- A/B 分开统计：完成手数、净盈利、BB/100、盈利手率、VPIP、PFR、最大单手盈亏和累计走势。
- 统计范围：全部历史、本次会话、各模式最近 20 / 100 / 500 手。累计走势图从所选范围起点的零开始。
- **BB/100 = 净盈利 BB ÷ 已完成手数 × 100**。例如 10 万手净赢 20 万 BB，就是 200 BB/100。
- “盈利手率”表示净盈利大于零的手牌比例。VPIP / PFR 只统计翻前主动投入／加注，强制盲注不算 VPIP。
- 历史按模式分页查看，每页 30 手；每手可以打开底牌、公牌和行动过程复盘。
- CSV 导出当前所选模式和统计范围；JSON 导出全部已完成手牌，不导出正在进行中的隐藏状态。
- 新会话只建立新的统计分组，保留全部旧记录。

## 本地数据

默认保存 `history.json`：

| 系统 | 默认目录 |
|---|---|
| Windows | `%APPDATA%\poker-human-simulator` |
| macOS | `~/Library/Application Support/poker-human-simulator` |
| Linux | `$XDG_CONFIG_HOME/poker-human-simulator`，未设置时为 `~/.config/poker-human-simulator` |

存档包括独立 A/B 记录、当前会话、未结束手牌的种子及真人动作，以便精确恢复。每次动作先原子保存成功，再更新界面；写盘失败不会静默推进牌局。双击或多个浏览器标签页重复提交同一动作会被拒绝。

备份整个数据目录即可迁移，或者导出完成手牌 JSON。应用不会上传数据。发布脚本只使用文件白名单，不会把存档、测试成绩或旧实验结果装进发布包。

## 测试与发布

```sh
go test ./...
go vet ./...
go test -race ./internal/app ./internal/table ./internal/web
```

Race detector 需要对应 C 工具链；GitHub CI 在 Linux 中运行它。测试包含能力隔离、人工动作边界、350 个种子的逐步恢复与结算、A/B 分组、分页、失败写盘回滚、重复动作竞争、本地 HTTP 请求边界与进程锁。

可选真实浏览器测试（开发时安装 Playwright，不影响用户运行）：

```sh
npm install --no-save playwright@1.62.1
npx playwright install chromium
node tests/browser.cjs
```

测试前先构建 `bin/poker-simulator`（Windows 为 `.exe`）。可以通过 `BROWSER_CHANNEL=msedge` 或 `chrome` 使用已安装浏览器；`PLAYWRIGHT_MODULE` 和 `POKER_BINARY` 可显式指定路径。测试使用临时存档目录，不接触玩家正常历史。

Python 3.9+ 可生成六个平台及源码 ZIP：

```sh
python scripts/release.py --version 0.1.0
```

输出在 `dist/`，包含 SHA256 校验和及构建清单。提交 GitHub 后，CI 会在 Windows、macOS 和 Linux 跑测试。推送 `v0.1.0` 一类版本标签会构建并生成 **Draft Release**，维护者审阅后手动发布。工作流使用官方 [checkout](https://github.com/actions/checkout) 和 [setup-go](https://github.com/actions/setup-go)。

## 项目结构

```text
cmd/poker-simulator/   本地程序入口
internal/table/        真人座位、五个机器人、合法动作和结算
internal/app/          存档、会话、统计与历史
internal/storelock/    操作系统级单实例数据锁
internal/web/          HTTP API 和内嵌页面
internal/game/        移植的规则引擎
poker/ protocol/ sdk/  牌型评估与原版机器人依赖
tests/                可选浏览器检查
scripts/              构建与发布包生成
vendor/               离线构建依赖及许可证
```

没有导入自动控制 A/B 的实验策略、范围推断或下注搜索模块。规则引擎保留已验证的零碎筹码分配及 A23456 应取六高顺等修正，详见第三方声明。
