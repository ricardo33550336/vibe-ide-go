# 性能基准报告与优化建议

环境：Intel Xeon E5-2680 v3 @ 2.50GHz（24 线程），darwin/amd64，go1.27.3。
方法：`go test -run='^$' -bench=… -benchmem -benchtime=200ms`，另对 ScmIdleFrame / TypeFrame 抓 CPU + 内存 profile。
注意：基准为**平均值**（含冷帧）；项目自己的 `TestBudgets`（`VIBE_BUDGETS=1`）取 60 帧中位数，两者口径不同。

## 1. 基准结果

### 编辑器（约 25,000 行 Go 文件）

| 基准 | ns/op | B/op | allocs/op | 评价 |
|---|---:|---:|---:|---|
| IdleFrame | 0.35 ms | 12 KB | 257 | ✅ 远低于 2ms 预算 |
| IdleFrameTabs/30 | 1.00 ms | 24 KB | 475 | ✅ |
| ScrollFrame | 1.01 ms | 24 KB | 507 | ✅ |
| TypeFrame | **20.0 ms** | 507 KB | 10,642 | ⚠️ 超 4ms 预算（均值含冷帧） |
| WrapScrollFrame | 0.70 ms | 24 KB | 508 | ✅ |
| WrapTypeFrame | **20.8 ms** | 528 KB | 10,734 | ⚠️ |
| CJKScrollFrame | **4.78 ms** | 449 KB | 6,286 | ⚠️ |
| CJKTypeFrame | **30.3 ms** | 3.2 MB | 47,989 | 🔴 最差 typing 路径 |

### 源代码管理（2,000 个改动文件）

| 基准 | ns/op | B/op | allocs/op | 评价 |
|---|---:|---:|---:|---|
| ScmIdleFrame | **23.5 ms** | 1.6 MB | 29,501 | 🔴 空闲帧不应有成本 |
| ScmScrollFrame | **37.2 ms** | 1.9 MB | 31,232 | 🔴 |
| ScmStatusFrame | **27.7 ms** | 1.6 MB | 29,502 | 🔴 status 无变化仍全量重建 |
| ReviewScrollFrame | 13.9 ms | 353 KB | 4,786 | ⚠️ |
| ReviewLayout | 2.06 ms | 85 KB | 1,497 | ✅ |
| SplitScrollFrame | 18.1 ms | 458 KB | 5,647 | ⚠️ |

### Markdown / Live Preview

| 基准 | ns/op | B/op | allocs/op | 评价 |
|---|---:|---:|---:|---|
| ParseLive | **263 ms**（0.97 MB/s） | 97 MB | 726,545 | 🔴 |
| ParseLiveQuick | 47.6 ms | 29 MB | 122,264 | ⚠️ |
| LiveMapTo | 0.18 ms | **517 KB** | 130 | ⚠️ 一次坐标映射分配半 MB |
| MarkdownReparse/warm | 53 ms | 41 MB | 172,074 | ⚠️ |
| MarkdownReparse/cold | **2.14 s**（0.12 MB/s） | 445 MB | 4,387,489 | 🔴 公式/图表全量重渲染 |

### 选择器 / 其它

| 基准 | ns/op | B/op | allocs/op | 评价 |
|---|---:|---:|---:|---|
| BranchPickerTyping（2000 分支） | **13.7 ms/键** | 2.9 MB | 9,880 | ⚠️ |
| BranchPickerFirstKey | 22.1 ms | 2.4 MB | 13,077 | ⚠️ |
| RailFrame | 0.53 ms | 7 KB | 197 | ✅ |
| ProjectSwitch | 1.48 ms | 99 KB | 348 | ✅ |
| IdleFrameRepo | 0.40 ms | 8 KB | 173 | ✅ |
| RelexTyping（全量重词法） | 2.34 ms | 2.5 MB | 5,267 | ⚠️ 每次击键全文件 |
| RelexPatchTyping/MarkdownSparse | 2.88 ms | 1.5 MB | 17,208 | ⚠️ |
| Fuzzy100k | 2.8–4.8 ms | ~120 KB | ~115 | ✅ 10 万条很好 |
| DocumentURIMapping | 1.65 µs | 1.2 KB | 10 | ✅ |
| LinesLargeFewEdits | 0.77 ms | 458 KB | 36 | ✅ |

## 2. Profile 定位

**ScmIdleFrame（23.5ms 空闲帧）**
- 内存分配约 40% 在 mygo 引擎的 `spanCache.update → encodeSpans → EncodeSpans → AppendUvarint`：`paintDiffLine`（cmd/vibe/diffpaint.go:1741）每帧为每条可见行重新构造 `ui.Span` 并触发重新编码。
- `expandTabs` / `clipLine` 每行每帧重新分配字符串。
- `layoutRows` 已有 `b == v.built` 缓存（好），成本集中在**绘制路径没有缓存**。
- 大量分配 → GC（`memclrNoHeapPointers` 占 CPU ~18%）。
- profile 中的 `syscall.Open/WriteFile/git` 大头是 benchScm 的 setup（profiler 不受 ResetTimer 影响），不是每帧成本。

**TypeFrame（20ms）**
- CPU 大头是 GC（madvise/memclr ~17%）与引擎调度等待；`app.view` 每帧重建整棵 UI 树（含 sidebar/fileTree）。
- 击键触发全文件 relex（2.3ms、2.5MB），CJK 路径分配放大到 48k allocs/帧（疑似逐 rune 宽度测量未缓存）。

**MarkdownReparse/cold（2.14s）**
- cache 命中时 53ms，未命中 2.14s：成本几乎全在公式（mathjax/moejs）与图表的重新渲染，而非 goldmark 解析本身。

## 3. 优化建议（按收益排序）

1. **SCM 空闲帧绘制缓存**（预期 23.5ms → <2ms）
   为 `paintDiffLine` 增加按 `(file, side, n, cols, wrap, 主题代)` 键控的已编码 span 缓存，编辑/换主题/改宽度时才失效；空闲帧直接复用，让 mygo 的 spanCache 命中。这是单点最大收益。

2. **setStatus 相等短路**（27.7ms → ~0.4ms）
   `ScmStatusFrame` 模拟"几秒后 status 原样回来"。在 `setStatus` 入口对 entries 做轻量相等比较（长度+路径+字母+hash），相同则只更新必要的时间戳，不触发重建。

3. **增量 relex**（typing 20ms 的主要可削部分）
   击键只重词法化受影响行及其后 lexer 状态变化传播到的区域，而非 25k 行全量；`internal/highlight` 已有 patch 基准（RelexPatchTyping 0.85–2.9ms），说明补丁路径存在但未用于主 typing 路径或未覆盖 CJK。

4. **CJK 宽度缓存**（CJKType 30ms/48k allocs）
   按 rune 缓存 `f.width` 结果（rune→width 的小 map 或分段表），消除逐 rune 测量分配；`expandTabs` 结果按行缓存（内容不变则复用）。

5. **Markdown 冷解析懒渲染**（2.14s → 首屏 <100ms）
   公式/mermaid 只同步渲染视口内的块，视口外异步/滚动到时再渲染；`markdownCache` 已有按块缓存，补上"可见性优先"的调度即可。`LiveMapTo` 的 517KB/次改为复用缓冲。

6. **分支选择器预索引**（13.7ms/键 → <3ms）
   打开时一次性 lowercase + 构建模糊匹配索引（参考 Fuzzy100k 的低分配实现），击键只做过滤；2000 分支 2.9MB/键说明每次都在重建候选集。

7. **paintDiffLine 微优化**
   `sc` scratch 已复用 breaks/runs/spans（好），但 `f.span(...)` 每行新建字符串片段、`runeAt` 每行重建：把 runeAt 按行缓存，spans 底层数组跨行复用。

8. **基准设施本身**
   - `BenchmarkMarkdownParse/IdleFrame/ScrollFrame` 用相对路径 `../../CHANGELOG.md`，从包外运行二进制即失败；改用 `runtime.Caller` 定位。
   - `BenchmarkWorktreePickerTyping` setup 真实创建 200 个 git worktree（数分钟），建议 `-short` 跳过或减到 20 个。
   - 官方口径以 `VIBE_BUDGETS=1 go test -run TestBudgets ./cmd/vibe` 的中位数为准；本报告的均值用于定位热点，不宜直接当延迟结论。

## 4. 健康项（无需动）

IdleFrame、ScrollFrame、Fuzzy100k、ProjectSwitch、RailFrame、DocumentURIMapping、LinesLargeFewEdits 都在预算内且分配克制；`layoutRows` 的 build-key 缓存、`diffLineScratch`、`file.words` 词级 diff 缓存都是正确的设计，建议把同样的模式推广到绘制路径。

## 5. 已落地优化：mygo 富文本 span 缓存（已验证）

### 根因（profile 确认）
引擎每帧全量重建视图（immediate-mode，`rt.view` 每帧调用）。SCM 空闲帧约 **64% 的分配**来自文本 span 的重复编码（`EncodeSpans`/`AppendUvarint`/`encodeSpans`/`spanCache.update`），并由此引发 GC（`memclrNoHeapPointers` 占 CPU ~21%）。两个缺陷都在 mygo 的 `RichText` 路径：

1. `newSpanPaint` 每次 `RichText` 调用都新建 `spanCache{}`，`key==""` 导致**永远重新编码**。
2. `richParams` 的环形缓存只有 `[8]`，一帧 ~80 行 diff 全部 miss，`encodeSpans` 重跑。

### 改动（fork `ricardo33550336/mygo`，分支 `perf/richtext-span-cache`，go.mod replace 指向它）
基于 sorrycc fork 的精确 pin 提交 `8859ace8bfa6`（保留 vibe 依赖的 30 个独有提交），补丁提交 `6e08d39555e2`：
- `_mygo/ui/richtext.go`（fork 内 `ui/richtext.go`）：把 `spanPaint` 并入 `richMeasured` 的缓存条目，一次查找同时拿到布局参数与绘制样式，命中即不再编码；新增 `spansHash` 指纹 + `measuredIdx` 映射，使查找为 **O(1)**（命中仍用 `slices.Equal` 校验，碰撞只触发重建，不会出错）。`spanPaint` 读缓存保留的 spans 副本而非调用方_scratch，避免下一帧覆写。
- `_mygo/ui/runtime.go`（fork 内 `ui/runtime.go`）：环形缓存 `[8]→[256]`，覆盖一帧的不同文本；新增 `measuredIdx map[uint64]int`。
- 绘制路径对 `spanPaint` 只读（已核对 `textForeground`/`textBackgrounds`），跨帧共享安全。

### 实测（同机 1s 稳态，原始 vs 补丁）
| 基准 | 原始 | 补丁后 | 时间 | 分配 |
|---|---|---|---|---|
| ScmIdleFrame | 23.5 ms / 29.5k | **1.51 ms / 511** | **-94%** | -98% |
| ScmStatusFrame | 27.7 ms / 29.5k | **1.65 ms / 491** | **-94%** | -98% |
| ScmScrollFrame | 46.2 ms / 40966 | **34.1 ms / 8117** | **-26%** | -80% |
| TypeFrame | 5.54 ms / 3189 | **5.10 ms / 2487** | -8% | -22% |
| CJKScrollFrame | ~4.8 ms | **3.50 ms** | -27% | — |

注：先用 `[256]` 线性扫描时滚动反而变慢（miss 时 O(256) 扫描），改 O(1) 哈希索引后滚动也改善。`setStatus` 本就有 `Status.Equal` 短路，ScmStatusFrame 的成本其实来自空闲帧本身，随本修复一并降到 1.65ms。

### 验证
- `_mygo` 完整 `ui` 套件（`-short`）通过；`RichText/Span/Paint/Measure/Cache` 子集通过。
- vibe `internal/highlight` 全量、`cmd/vibe -short -run 'Diff|Markdown|Wrap|CJK|Highlight|Paint'` 通过。

### 回退 / 上游
go.mod 中已注释保留未补丁的 sorrycc replace 行，还原它即可回到原 fork（无需本地副本，依赖经 GitHub fork 拉取，CI/协作者可复现）。已开上游 issue **egoist/mygo#1976**；建议把修复 PR 到 sorrycc/mygo（vibe 实际构建源）或 egoist/mygo，合入后 bump 版本、删除自有 fork。

## 6. 后续目标（本次未动）

1. **CJKTypeFrame ~28ms**：span 编码已不是瓶颈，剩下的是逐 rune 宽度测量与全文件 relex。按 rune 缓存 `f.width`、把 `internal/highlight` 的 patch 增量路径接入主 typing 路径。
2. **MarkdownReparse/cold**：✅ 已完成（见第 8 节，MathJax 引擎池，2.14s→434ms）。初判“视口懒渲染”未采用——根因是全局锁串行渲染，池化并行更直接。
3. **BranchPickerTyping**：✅ 已完成（见第 7 节）。初判“预建模糊索引”有误——Fuzzy 本身已很快，真凶是结果列表不虚拟化 + 背后 SCM 面板的 refBadges 零缓存。
4. **基准设施**：`BenchmarkMarkdown*` 改用 `runtime.Caller` 定位 `CHANGELOG.md`；`WorktreePickerTyping` 的 200 个真实 worktree setup 在 `-short` 下跳过。

## 7. 已落地优化：分支选择器虚拟化 + refBadges 构建期缓存（已验证）

### 根因（profile 确认，颠覆初判）
BranchPickerTyping 的 CPU 大头**不是** Fuzzy 匹配（它已无分配打分、2000 名字 <1ms），而是**光栅渲染**：`renderer.transformed` 占 73%、`transformedShadowAt` 39%、`contCorner.dist` 22%。结果列表用 `ui.Scroll` 包裹，**不虚拟化**，每帧构建并光栅化全部 ~200 行 `Radius(6)` 圆角 + `hoverFade` 阴影按钮。
内存 profile 另揭示：分支选择器**背后 SCM 历史面板**的 `refBadges` 占整帧分配 70%——它每次调用都新建 map + `slices.Clone(refs)` + 重建 badge 切片，零缓存；基准里 2000 分支全指向 HEAD，该提交每帧重算 2000 条 refs。

### 改动
- `cmd/vibe/branchpicker.go`：结果列表 `ui.Scroll`→`ui.List`（虚拟列表，只构建可见行）。`choices` 本就不含组标题、`b.sel` 直接索引 choices，故 flat list = choices，组标题作为组首行的一部分渲染（ui.List 支持变高行），**选择/上下导航/Enter/选中态全保留**；`ListState.Key` 按分支名稳定标识，键盘移动时用 `ListState.ScrollIntoView(b.sel)` 滚动。列表不设 `Selected`（branchRow 已用 pickMark 自绘选中态），避免与输入框焦点竞争。
- `cmd/vibe/scm.go`：`scmRow` 增 `badges []refBadge`，在构建提交行时（scmRows 已按 historyGen/status 缓存）算好 `refBadges(h[i].Refs)`；`scmCommit` 改读 `r.badges`，不再每帧重算。diffpaint/coauthors 的调用点不变。

### 实测（同机 1s 稳态）
| 基准 | 原始 | 虚拟化后 | +refBadges 缓存 |
|---|---|---|---|
| BranchPickerTyping | 10.84 ms / 2.8 MB / 7216 | 4.14 ms / 2.26 MB / 1360 | **3.04 ms / 378 KB / 1234** |
| BranchPickerFirstKey（1x） | 22.1 ms / 13077 | — | **5.83 ms / 2552** |
| ScmIdleFrame | 1.65 ms / 517 | — | 1.65 ms / 517（无回归） |

BranchPickerTyping 累计 **时间 -72%、分配 -87%、allocs -83%**，达到 <3ms 目标。

### 验证
`cmd/vibe -short -run 'Branch|Picker'`、`'Scm|History|Commit|Graph'`、`'Diff|Review|Split'` 全过；含 `TestBranchPickerGroupsAndAuthors`（组标题）、`TestBranchPickerNoRowsArrowsAndEnter`（导航+Enter）、`TestBranchPicker`（综合）、Create 系列。

### 剩余
BranchPickerTyping 余下 ~3ms 主要是引擎光栅（对话框大阴影 + 可见行圆角，`transformedShadowAt`），属 mygo 引擎级成本，非 vibe 侧快赢。

## 8. 并行分析：CJK typing 与 Markdown cold（含 MathJax 池最小验证）

用 codemode + `Promise.all` 并行采集（无 subagent），profile 实证纠正两处误判。

### CJK typing（~28–35ms）——第三方 bound，降级
- alloc 大头：`regexp2.newMatchText` 45.7% + `chroma.matchRules` cum 63.5% → **Chroma Markdown 词法正则**，非 f.width shaping。
- **纠正**：增量 `relexPatch` 已接入主 typing 路径（`highlight.go run()` 的 `prevFirst==r.first` 分支）；第 6 节“patch 未接入”有误。
- CJK doc ~8000 行 > WholeFile=6000，每键读 ~650 行（Context=400+Below=200）；patch 已只重词法变化区，但 Chroma 对 CJK 的正则本身重。CPU 分散在 GC（memclr 19%）/CoreText shaping（cgocall 6%）/字形光栅（Glyph 6%）。
- 结论：vibe 侧无干净快赢；f.width 缓存仅小幅 CPU 改善（纯中文散文无 mark）；缩 relex 范围/换 lexer 有正确性风险。**降级**。

### Markdown cold（2.14s）——MathJax 引擎池，已落地并验证
- **根因**：`mathSVG` 持全局 `mathjax.Mutex`，注释明言 “one formula at a time”，所有公式串行渲染。warm=53ms 因公式缓存命中。
- **杠杆**：`jsEngine` 每实例自带独立 `moejs.Runtime`，可池化并行。
- **最小验证**（300 个复杂公式、8 引擎，一次性测试已删）：
  - `-race` 无数据竞争；8 runtime 并发输出与单引擎**逐字节一致**。
  - 非 race：串行 3.40s（11.3ms/公式）→ 并行 0.55s（1.83ms/公式），**6.18x**。
  - 引擎启动 **148ms/个**（关键权衡）。
- **已落地**（`cmd/vibe/math.go` + `markdown.go`）：
  1. 全局单引擎 → 引擎池：`mathPool` 用信号量限并发 N=min(GOMAXPROCS,8)，`mathPoolGet/Put` 借还引擎；引擎**按需懒启动 + 常驻复用**（一公式笔记只起 1 个；启动只付一次）。每引擎独占使用，无需各自锁。
  2. `parseMarkdown` 的 `formula()` 改为返回**占位指针**并收集 `pending`；walk 后 `renderMathPending` 用 N 个 worker 并行 `renderMath` 填充占位（各占位独立，无共享写）。安全性已核对：公式宽度仅在渲染期（`markdownColumns`@1836）读取，parse 期不用，故占位不影响布局；`current()` 取消时跳过填充。
  3. `ParseSVG` 线程安全（`lastSVG` atomic、`svgSeed` 只读）。
- **实测**：MarkdownReparse/cold **2.14s → 434ms（~4.9x）**，warm 不变（51.8ms）；allocs 不变（渲染量同，仅并行）。
- **验证**：`-race` cold 基准（2000 公式并行）无竞争；新增 `TestMarkdownMathParallelFill`（60 唯一公式断言全渲染 + 与单次渲染逐值一致）连同 `TestRenderMath/TestMarkdownMath/TestMathPlaces` 均 `-race` 通过；`Math|Markdown|Live|Preview|Wiki|Embed|Issue|Viewer|Note|Callout|Mermaid` 子集全过。
- **LiveMapTo 纠正**：`mapTo` 已高效（共享 spans/blocks，仅移动 callouts）；517KB 来自 `snapDiff` 每次重算 diff，次要（0.18ms 已快）。
