# Third-party notices

Vibe is distributed under the MIT License (see [LICENSE](LICENSE)). It is a
fork of Mayoi and includes work from the projects below. The full license
texts are in [`licenses/`](licenses/) and are copied into the app at
`Contents/Resources/licenses/`.

## The projects Vibe is built from

| Project | Copyright | License |
|---|---|---|
| [Mayoi](https://github.com/sorrycc/Mayoi) | Copyright (c) 2026 chencheng | MIT |
| [vibe-ide](https://github.com/luguan1998/vibe-ide) | Copyright (c) 2026 luguan | MIT |
| [MyGo](https://github.com/egoist/mygo) | Copyright (c) 2026 MyGo contributors | MIT |
| [Ghostty](https://github.com/ghostty-org/ghostty) / `libghostty-vt` | Copyright (c) 2024 Mitchell Hashimoto, Ghostty contributors | MIT |

Vibe's information architecture was inspired by vibe-ide. The current code
is a Go rewrite on top of Mayoi; no vibe-ide source is compiled into the
app. Its MIT notice is included anyway.

## Go modules compiled into Vibe

| Module | License |
|---|---|
| `github.com/MoeForge/moejs` | Apache-2.0 (with Unicode/ICU/V8 notices) |
| `github.com/MoeForge/moego` | MIT |
| `github.com/ebitengine/purego` | Apache-2.0 |
| `github.com/alecthomas/chroma/v2` | MIT |
| `github.com/yuin/goldmark` | MIT |
| `github.com/creack/pty` | MIT |
| `github.com/fsnotify/fsnotify` | BSD-3-Clause |
| `github.com/go-text/typesetting` | Unlicense or BSD-3-Clause |
| `github.com/BurntSushi/toml` | MIT |
| `github.com/dlclark/regexp2/v2` | MIT |
| `golang.org/x/image`, `x/sys`, `x/text` | BSD-3-Clause |
| Go standard library | BSD-3-Clause |

`moejs` is Apache-2.0 and its `THIRD_PARTY_NOTICES` covers Unicode CLDR and
character data, ICU, and V8 code that are compiled into the JavaScript
engine. Those notices are included in full in
[`licenses/moejs-THIRD_PARTY_NOTICES`](licenses/moejs-THIRD_PARTY_NOTICES).

## JavaScript bundled for previews

| Component | Version | License | Where |
|---|---|---|---|
| [MathJax](https://github.com/mathjax/MathJax-src) | 3.2.2 | Apache-2.0 | `cmd/vibe/mathjax.js.gz` |
| [Pele](https://github.com/obsidianmd/pele) | 1.9.0 | MIT | `cmd/vibe/pele.js.gz` |
| [Mermaid](https://github.com/mermaid-js/mermaid) (code carried by Pele) | — | MIT | inside `pele.js.gz` |

The MathJax and Pele bundles keep their upstream license text at the top of
the script. The full Apache-2.0 and MIT texts are also in `licenses/`.

## Source availability

The source of every component is available at the link above and at the
version recorded in `go.mod`, `scripts/mathjax.sh`, or `scripts/pele.sh`.
The Apache-2.0 and MIT licenses permit redistribution and modification under
their terms; they do not require Vibe to be released under the same license.

Vibe is not affiliated with Mayoi, vibe-ide, Ghostty, Obsidian, or the
MathJax project.
