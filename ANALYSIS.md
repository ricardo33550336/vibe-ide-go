# 用 Mayoi 的技术复刻 vibe-ide：可行性分析

> 目标：保留 vibe-ide 的信息架构（终端第一、左侧工作区/多终端、右侧目录 + Git），
> 用 Mayoi 的技术栈（Go + MyGo 原生 UI + Ghostty 终端）重写。
> 明确不要：内置 Claude Code / pi / dsh 等 AI 后端。

---

## 0. 结论先行

1. **技术可行，而且终端底座比 vibe-ide 更强。** Mayoi 的终端是 Ghostty 的 emulator（`libghostty-vt`），
   原生 GPU 绘制、纯 Go、无 cgo、无 Electron；vibe-ide 是 xterm.js + node-pty + Electron。
2. **但不能靠"改配置/删组件"完成。** 两者的核心模型不同：
   - Mayoi：**一个项目 = 一个文件夹**（一个窗口，或同一窗口里的 project tab），中心是**编辑器列 + 标签页**，
     终端在**底部/侧边面板**里。
   - vibe-ide：**一个窗口 = 多个工作区（文件夹）**，左侧是**工作区 → 终端**的两级列表，
     中心默认只显示**当前终端**（文件/Diff 是另一种中心视图），右侧是工具面板。
3. **推荐路线：以 Mayoi 为基座 fork，把"项目模型"改造成"工作区模型"，新增左侧工作区/终端面板，
   中心加一个 terminal-first 模式，右栏只留 Explorer + SCM。** 编辑器、Diff、Git、文件树这些不要重写。
4. **主工作量不在终端，而在工作区模型与导航。** 粗估新增/改造 3–6k 行 Go，可裁剪 20–30k 行；
   保留编辑器的前提下，3–5 周可到"日常可用"，6–10 周达到可发布质量。
5. **必须先拍板：中间是否保留编辑器/文件编辑能力。** 保留 = 省 2–4 周（Mayoi 已有）；
   不保留 = 应用更纯粹、更小，但右栏"目录"点击文件后无处可去，Git Diff 也要另做展示。

---

## 1. 两个项目的现状

| 维度 | Mayoi | vibe-ide |
|---|---|---|
| 语言/框架 | Go 1.27 + MyGo（原生 UI，GPU 绘制） | Electron + React 18 + TS |
| 终端 | Ghostty emulator（`libghostty-vt`，运行时加载，纯 Go） | xterm.js（WebGL）+ node-pty / ConPTY |
| 编辑器 | 自研文本区（Chroma 高亮、Vim、多列分屏、Diff） | Monaco |
| Git | 自带完整 SCM（stage/commit/branch/stash/push/worktree/graph/blame/diff） | simple-git |
| 布局 | 左/右侧栏 + 编辑器列 + 底部/侧边终端面板；project tab 竖排 rail | 左会话栏 + 中终端/文件 + 右工具面板 |
| 工作区 | 一个项目 = 一个文件夹；多项目 = 多窗口或 project tabs | 一个窗口内多个 workspace（按 cwd 分组），每组多个终端 |
| AI | Sessions 视图、Work on Issue、AI commit（可全部去掉） | Claude CLI / pi Agent（**不要**） |
| 规模 | 非测试代码约 8.7 万行（`cmd/mayoi`）+ 2.1 万行（`internal`）+ 1.9 万行生成 i18n | 6.3 万行 TS/TSX，149 个文件 |
| 产物 | 约 18 MB 原生 app | 100 MB+ Electron 应用 |
| 平台 | macOS / Linux / Windows | 主要 Windows |

关键源码位置：

- Mayoi 布局：`_reference/Mayoi/cmd/mayoi/view.go`（`view` 42、`main` 221、`columns` 542、`sidePanel` 265）
- 终端：`_reference/Mayoi/cmd/mayoi/panel.go`（`newTerminalIn` 326、`panel` 545、`terminalView` 700）
- 侧栏：`_reference/Mayoi/cmd/mayoi/sidebar.go`（`sidebar` 22）
- 项目/工作区外壳：`_reference/Mayoi/cmd/mayoi/projects.go`（`frame`、project rail 547）、`main.go`（`desktop` 33）
- 终端字段：`_reference/Mayoi/cmd/mayoi/app.go:300`（`terms`、`panelTerm`）
- 编辑器列/标签：`_reference/Mayoi/cmd/mayoi/columns.go`（`column` 22、`colTab` 47）
- 持久化：`_reference/Mayoi/internal/settings/settings.go`（`Session` 358、`Layout` 404）
- vibe-ide 布局：`_reference/vibe-ide/src/renderer/src/App.tsx`（sessions 381、左 3780、中 3960、右 4074）
- vibe-ide 分组：`_reference/vibe-ide/src/renderer/src/components/SessionPanel.tsx:1085`
- vibe-ide 右栏：`_reference/vibe-ide/src/renderer/src/components/RightPanel.tsx:71`（git/terminal/file/game）
- vibe-ide 会话持久化：`_reference/vibe-ide/src/renderer/src/sessionRestore.ts`
- vibe-ide 终端后端：`_reference/vibe-ide/src/main/pty.ts`

---

## 2. 目标信息架构

```
┌──────────────────────────────────────────────────────────────────────┐
│ 原生标题栏（MyGo TitleBarHidden / 红绿灯）                            │
├──────────────────┬─────────────────────────────────┬─────────────────┤
│ 工作区 A         │                                 │  Dir | Git      │
│   ● term 1       │                                 │                 │
│   ○ term 2       │      终端（当前 session）        │  文件树 /        │
│ 工作区 B         │      Ghostty vt 全尺寸           │  SCM 变更列表    │
│   ● term 3       │                                 │                 │
│   + 新终端       │   （可选：文件/Diff 作为中心视图）│                 │
│ + 打开工作区     │                                 │                 │
├──────────────────┴─────────────────────────────────┴─────────────────┤
│ 状态栏：Git 分支 / 程序状态 / 字号 …                                  │
└──────────────────────────────────────────────────────────────────────┘
```

### 2.1 数据模型（新增）

```go
type workspace struct {
    ID        string      // 稳定 ID，进 state.json
    Path      string      // 文件夹绝对路径
    Name      string      // 默认 filepath.Base(Path)，可重命名
    Order     int
    Collapsed bool
    Terminals []*termSession
    ActiveTerm int
}

type termSession struct {
    ID        string
    Title     string      // 程序 OSC 标题 / 用户重命名
    Cwd       string
    Shell     string      // 可选
    Icon      string      // emoji 或图标（vibe-ide 有）
    Order     int
    // 运行期
    term      *terminal.Terminal
    view      ui.Handle
    started   bool        // 懒启动
    status    terminal.ProgramStatus
}
```

要点：

- **工作区 = Mayoi 的 `desktop`（一个 `app`）**。它已经封装了 root、文件树、Git 状态、文档、终端。
  `frame` 已经支持一窗多 desktop（project tabs），因此多工作区不必从零造。
- **终端不再放在列标签里，而是 desktop 级的会话列表**；`session.tab == nil` 的形态已经存在
  （Mayoi 的 panel 终端）。中心区显示 `workspace.ActiveTerm`。
- **懒启动**：恢复会话时只恢复元数据，选中时才 `terminal.New`，避免启动即拉起 N 个 shell。
  Mayoi 目前 `addTerminal` 是急启动，需要拆成 `ensureStarted()`。
- **持久化**：在 `internal/settings` 增加 `Workspaces []WorkspaceState`，写入 `state.json`；
  参考 vibe-ide `sessionRestore.ts` 的字段（cwd/name/icon/order/active）。
- **只恢复拓扑，不恢复进程**：退出后重新 spawn 新 shell，工作目录保持不变。
  若要"退出后进程还活着"，需要接 Mayoi 的 herdr-core（`hc`），属于额外依赖，建议不做。

### 2.2 中心区：terminal-first 模式

给 `app` 增加一个中心模式：

```go
// centerMode: "terminal"（默认） | "editor"
```

- `centerMode == "terminal"`：`main` 里不画 `columns()`，画当前工作区 active terminal 的 `terminalView`。
- 打开文件 / 点击 Git 变更：`centerMode = "editor"`，仍走 Mayoi 现有的列/预览标签逻辑。
- 左侧点击终端：切回 `centerMode = "terminal"`。
- 快捷键（`Ctrl+`` 或 `Cmd+J`）在两种模式间切换；关闭最后一个编辑器标签自动回终端。

这样既满足"终端第一公民"，又不用删掉 Mayoi 的编辑器/Diff。

### 2.3 右栏

- 直接复用 Mayoi 的 `sidebar`，设置 `SidebarRight = true`。
- 只保留两个视图：`""`（Explorer）与 `"git"`（SCM）。
- 隐藏 `search` / `sessions` / `github` / plugin 视图的入口（先隐藏，后删除）。
- 点击文件：`centerMode=editor` + `previewFile`（Mayoi 已有 transient preview tab）。
- 点击变更：`previewCommit` / diff tab（Mayoi 已有）。

---

## 3. 能力对照与复用映射

| vibe-ide 能力 | Mayoi 对应 | 结论 |
|---|---|---|
| 终端（多会话、链接跳转、右键粘贴、字号、搜索） | `plugins/terminal` + `panel.go`、`termfind.go`、`terminallink.go` | **直接复用**，且 Ghostty vt 更强 |
| 终端在中心显示 | 目前终端在 panel / 列标签 | **新增中心模式**（小改） |
| 左侧工作区分组 + 多终端 | 无；有 project rail 与面板 chips | **新写 UI**（主要工作量） |
| 右侧目录树（Git 忽略变灰、重命名、新建、拖放） | `sidebar.go` + `explorer.go` + `workspace/tree.go` | **直接复用**，改到右侧 |
| 右侧 Git（stage/commit/branch/stash/push/worktree/graph/blame） | `scm.go`、`internal/git`、`git.go`、`graph.go` 等 | **直接复用** |
| Diff（并排、行内、hunk 操作） | `diffview.go`、`diffpaint.go`、`diffhunks.go` | **直接复用** |
| 文件编辑（Monaco） | 自研 editor（Chroma、Vim、多列） | 保留则复用；不保留则删除 |
| 会话恢复 | `internal/settings` + 新增 workspaces | **新增** |
| 快捷键 / 命令面板 / 设置 / 主题 | `keymap`、`commandpalette.go`、`prefswindow.go`、`theme.go` | **直接复用** |
| 中英文 i18n | ICU 目录 + 生成代码（含 zh-Hans） | **直接复用** |
| AI（Claude / pi / dsh） | `internal/agents`、`sessions.go`、`aicommit.go`、GitHub Work on Issue | **删除/隐藏** |
| 搜索面板 | `search.go` | 不在保留清单，建议隐藏 |
| Markdown / Mermaid / 数学 / 图片预览 | `markdown*.go`、`mermaid.go`、`viewer.go` | 不在清单，建议隐藏/后删 |
| Remote SSH | `internal/remote` + `vfs` | 不在清单，v1 不做（架构上保留可能） |
| Plugins / LSP | `internal/plugin`、`internal/lsp` | v1 不做 |
| herdr-core（终端常驻） | `hc*.go` | 不做（vibe-ide 也没有） |
| 内嵌浏览器 / 宠物 / 小游戏 | 无 / 无 / 无 | 不做 |

结论：vibe-ide 的四项保留需求，Mayoi 已覆盖 2.5 项（终端、目录、Git、Diff），
缺的是**左侧工作区/终端导航**和**中心终端模式**。

---

## 4. 三条实现路线

### 路线 A（推荐）：fork Mayoi，改造 IA

**做法**：把 Mayoi 源码作为新仓库基线（保留 git 历史/MIT 许可），改 module 名与品牌，
先隐藏无关入口让构建保持绿色，再做工作区模型与左侧面板。

- 优点：终端、编辑器、Git、Diff、Explorer、设置、主题、i18n 全部即取即用；测试一起带走；最快出可用版本。
- 缺点：代码量大（约 11 万行非测试），裁剪是"手术"；与上游 Mayoi 后续分叉。
- 适合：目标是尽快得到一个自己每天能用的 vibe-ide 替代品。

### 路线 B：新建小工程，只搬内部包

**做法**：新 module 只依赖 MyGo；复用 `internal/{git,gitmarks,linediff,workspace,vfs,highlight,settings,keymap,bg}`，
把 `cmd/mayoi` 中需要的 UI 片段（panel、sidebar、scm、diff、editor）拷进新 `main`。

- 优点：干净、可理解的模型，代码量可控制在 25–40k 行；终端-only 时尤其合适。
- 缺点：`cmd/mayoi` 是单包，剪切粘贴后要重接大量状态与命令；编辑器/Diff 的回归测试要重建。
- 适合：明确只要终端 + 目录 + Git，不要编辑器。

### 路线 C：MyGo 的 Web 前端模式

**做法**：保留 vibe-ide 的 React UI，用 MyGo 的 webview + Go 后端替换 Electron。

- 优点：前端改动最小，能复用 vibe-ide 组件。
- 缺点：终端只能继续用 xterm.js（MyGo 的 Ghostty 终端只支持原生 UI），拿不到 Mayoi 的核心优势；
  仍然是 webview 架构，和"用 Mayoi 的技术"目标不一致。**不推荐**。

### 推荐

**路线 A + 保留编辑器能力**。把"terminal-only"做成中心模式，而不是删代码；
等产品稳定后再按需裁剪 Markdown/GitHub/AI/Remote/Plugins 等包。

---

## 5. 改造清单（路线 A）

### 5.1 直接保留

- 终端：`panel.go`、`termfind.go`、`terminallink.go`、`termstatus.go`
- 编辑器：`document.go`、`editor.go`、`columns.go`、`tabs.go`、`history.go`、`editing.go`、
  `autoclose.go`、`find.go`、`decorations.go`、`selection.go`、`wrap.go`、`vim*.go`
- 文件树/索引：`sidebar.go`、`explorer.go`、`quickopen.go`、`internal/workspace`
- Git：`git*.go`、`scm*.go`、`diff*.go`、`commit*.go`、`merge*.go`、`branchpicker.go`、
  `stash.go`、`worktree*.go`、`blame.go`、`graph.go`、`internal/git`、`internal/gitmarks`、`internal/linediff`
- 基础设施：`main.go`、`app.go`、`view.go`、`projects.go`、`commands.go`、`commandpalette.go`、
  `keymap.go`、`internal/keymap`、`internal/settings`、`theme.go`、`icons.go`、`internal/bg`、
  `internal/cli`、`ipc.go`、`mygo.json`

### 5.2 新增/重写

1. **工作区注册表**：`frame` 从"project tabs"扩展为"workspace tabs"；默认一个窗口容纳全部工作区；
   打开文件夹 = 追加 desktop，而不是新窗口。
2. **左侧工作区面板**（约 1.2–2k 行 UI）：
   - 工作区组头：名称、路径 tooltip、活跃标记、`+ 新终端`、`⋯`（重命名/关闭/在文件管理器中显示）
   - 终端行：图标/emoji、标题、运行状态、关闭、拖拽排序
   - 空工作区（没有终端）、组排序、折叠
   - 跨工作区拖拽终端？v1 可不做
   - 底部 `+ 打开工作区`
3. **中心终端模式**：`app.centerMode`、`activeTerminal()`、模式切换、焦点管理。
4. **会话持久化**：`WorkspaceState` / `TermState` 进 `state.json`；启动懒恢复；退出时保存顺序与活跃项。
5. **懒启动**：`startTerminal` → `ensureTerminalStarted`；退出/关闭时 `term.Close()`。
6. **右侧栏收敛**：默认 `SidebarRight=true`；只显示 Explorer/Git；去掉其他切换按钮。
7. **快捷键/命令**：`view.newWorkspace`、`view.newTerminalHere`、`view.nextTerminal`、
   `view.previousTerminal`、`view.centerTerminal`、`view.centerEditor`、关闭工作区/终端。

### 5.3 隐藏 / 删除（分批）

第一批（只隐藏入口，保构建）：
- AI：Sessions 视图、Work on Issue、AI commit、agent 状态、`clauding` 主题装饰
- GitHub 视图 / PR / issue
- 搜索视图、Markdown 预览、Mermaid、图片查看器、插件视图、LSP 入口
- herdr-core / Attach 菜单
- Remote SSH / 端口转发
- 小游戏、宠物、账户、云同步、用量统计（`internal/usage`、`internal/mon`）

第二批（确认不需要后删包）：
- `internal/agents`、`internal/plugin`、`internal/lsp`、`internal/remote`、`internal/cloudsync`、
  `internal/usage`、`internal/mon`、`aicommit.go`、`github*.go`、`pr*.go`、`markdown*.go`、
  `mermaid.go`、`math.go`、`viewer.go`、`hc*.go`

> 注意：`sidebar.go` 同时引用 explorer/scm/search/sessions/github/plugin；
> `commands.go` 引用几乎所有功能。先加开关/把入口置灰，再删实现，避免长时间编译不过。

---

## 6. 分阶段计划

| 阶段 | 内容 | 估时 |
|---|---|---|
| P0 基线 | fork Mayoi 到目标仓库、改 module/品牌、`go run` 跑通、CI 跑测试 | 0.5–1 天 |
| P1 技术验证 | 左侧面板原型：两个工作区、切换、终端懒启动、右栏 Explorer/Git、中心终端 | 2–3 天 |
| P2 工作区模型 | frame/desktop 改造、工作区注册表、打开/关闭/重命名/排序、持久化 | 5–8 天 |
| P3 终端第一 | 中心终端模式、焦点/快捷键、新建/关闭/切换、退出处理、状态显示 | 3–5 天 |
| P4 右栏 | 移到右侧、只留 Dir/Git、点文件开编辑器、点变更开 Diff、宽度记忆 | 2–3 天 |
| P5 裁剪 | 隐藏 AI/GitHub/Search/Plugins/Remote 等入口，清理菜单与命令 | 2–4 天 |
| P6 打磨 | 主题、字体、i18n、命令面板、设置页、拖拽、空态、无工作区欢迎页 | 3–5 天 |
| P7 打包 | `mygo.json`、图标、macOS 签名/公证、Windows 安装包、Linux 包、自动更新 | 2–4 天 |
| **合计** | 到日常可用 | **3–5 周** |
| 可选 | 编辑器/Diff 全量保留打磨、Remote SSH、插件、搜索面板 | +3–6 周 |

P1 的成功标准（spike）：

- 一个窗口里两个工作区，各自有 2 个终端；
- 点击左侧终端，中心即时切换，键盘输入正确进入该 PTY；
- 未显示工作区的终端继续输出，切回不丢屏；
- 右侧目录树与 Git 状态跟随当前工作区；
- 退出重启后，工作区/终端拓扑和 cwd 恢复（新 shell）。

---

## 7. 主要风险

| 风险 | 说明 | 缓解 |
|---|---|---|
| **IA 不匹配（最大风险）** | Mayoi 是"项目 = 文件夹 + 编辑器列"，不是"工作区树 + 终端"。左侧面板与新模型是新增，不是开关 | 先用 2–3 天 spike 验证 frame/desktop 能否承载；不行再退回"每个工作区一个 desktop + 全局面板"的轻量方案 |
| MyGo 是 fork | `go.mod` replace 到 `sorrycc/mygo` 的伪版本，非上游 `egoist/mygo` | vendor 或 fork 并锁 commit；关注上游合并情况；预留升级预算 |
| 工具链 | Go 1.27.3、`go tool mygo`、发布时需 Zig 构建/下载 `libghostty-vt` | CI 固定 Go 版本；`mygo build` 负责打包 native lib；本机先验证三平台 |
| 终端跨平台 | Windows 走 ConPTY（需 Win10 1809+），macOS/Linux 走 PTY | 按 vibe-ide 的 Windows 优先场景做实测：pwsh、中文/emoji、IME、CJK 宽字符 |
| 裁剪牵连广 | `package main` 相互引用深，删一个视图可能连带命令/菜单/测试 | 两阶段：先隐藏入口，后删实现；每步保持 `go test ./...` 绿 |
| 会话恢复语义 | 不做 herdr-core 就没有"进程常驻" | 明确只恢复拓扑；UI 上写清楚"重启后重新启动 shell" |
| 编辑器取舍 | 不保留编辑器会让 Dir/Git 的点击失去落点；保留则应用更大 | 建议保留，默认中心是终端 |
| 许可证/归属 | Mayoi MIT、MyGo MIT、Ghostty MIT；需保留版权 | 保留 LICENSE 与 NOTICE，派生作品注明来源 |
| 与 vibe-ide 功能落差 | vibe-ide 有搜索、浏览器、Monaco 多语言、SSH 等 | 明确 v1 范围；不要承诺"全功能复刻" |

---

## 8. 需要拍板的问题

1. **中心区是否保留编辑器/文件编辑？**（推荐：保留，默认终端，文件/Diff 为次级视图）
2. **多工作区形态**：单窗口容纳全部工作区（vibe-ide 式，推荐）还是 Mayoi 的 project rail + 多窗口？
3. **多终端"排列"的含义**：左侧列表里排序（vibe-ide 现状，推荐 v1），还是中心需要 2/4 分屏平铺？
4. **会话恢复**：只恢复拓扑（推荐）还是要求进程退出后仍存活（需 herdr-core）？
5. **Remote SSH 是否要**：v1 砍掉（推荐），后续再加？
6. **目标平台优先级**：macOS + Windows 优先、Linux 其次？（Mayoi 三平台都支持，但终端行为要分别测）
7. **是否要中英文 i18n**：直接复用 Mayoi 的 zh-Hans 目录（推荐），还是先英文？
8. **仓库形态**：把 Mayoi 整棵树复制到本仓库根目录改造（推荐，最快），还是新建干净 module 只搬内部包？

---

## 9. 建议的下一步

1. 先回答第 1、2、3、8 题（决定工作量和目录结构）。
2. 做 P1 spike：不改产品结构，只在 Mayoi 里硬编码两个工作区和左侧面板原型，验证：
   - `frame` 的多 desktop 切换 + 隐藏 desktop 终端持续运行；
   - 中心终端视图的焦点/键盘/尺寸；
   - 右侧栏 Explorer/Git 跟随当前 desktop；
   - `state.json` 新增工作区拓扑后的恢复。
3. spike 通过后，再决定是否正式 fork 与进入 P2。
