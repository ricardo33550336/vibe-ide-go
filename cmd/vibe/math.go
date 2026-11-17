package main

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"errors"
	"html"
	"io"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/MoeForge/moejs"
	"github.com/egoist/mygo/ui"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"vibe/locales"
)

// mathJS is MathJax, which typesets TeX and draws it as SVG, its glyphs
// paths, bundled by scripts/mathjax.sh into one script that sets the
// global VibeMath.
//
//go:embed mathjax.js.gz
var mathJS []byte

// mathFormula is a formula drawn, in ems of the text it is in: as wide
// as w, reaching ascent above the baseline and depth below it.
type mathFormula struct {
	svg              *ui.SVG
	w, ascent, depth float32
	display          bool
	// err is why the formula could not be drawn, "" when it was.
	err string
}

// draw draws the formula in r, in color.
func (f *mathFormula) draw(pt *ui.Painter, r ui.Rect, color ui.Color) {
	if f.svg != nil && r.W > 0 {
		pt.Icon(f.svg, r, color)
	}
}

// mathKey is a formula's TeX and whether it is in display style.
type mathKey struct {
	tex     string
	display bool
}

// mathTimeout is how long MathJax may take with a formula.
const mathTimeout = 5 * time.Second

// mathEngine is one MathJax runtime that draws formulas, its own so
// several can draw at once.
type mathEngine struct {
	js     *jsEngine
	render moejs.Value
}

// mathPool draws formulas through several engines at once rather than
// one after another: a cold parse of a note of many unique formulas
// typesets them in parallel. Engines start as needed, up to a cap, and
// stay for the app to reuse.
var mathPool struct {
	once sync.Once
	sem  chan struct{}
	mu   sync.Mutex
	idle []*mathEngine
	err  error
}

// mathPoolSize is the most formulas drawn at once.
func mathPoolSize() int { return max(1, min(runtime.GOMAXPROCS(0), 8)) }

func mathStartEngine() (*mathEngine, error) {
	zr, err := gzip.NewReader(bytes.NewReader(mathJS))
	if err != nil {
		return nil, err
	}
	script, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	js, err := newJSEngine("mathjax.js", string(script), nil)
	if err != nil {
		return nil, err
	}
	fn, err := js.eval("mathjax-render.js", "VibeMath.render")
	if err != nil {
		return nil, err
	}
	if !fn.IsObject() || !fn.AsObject().IsCallable() {
		return nil, errors.New("mathjax: no render")
	}
	return &mathEngine{js: js, render: fn}, nil
}

// mathPoolGet borrows an idle engine, or starts one if none is free.
// The semaphore holds the pool to its size, so an engine is never shared
// by two draws at once.
func mathPoolGet() (*mathEngine, error) {
	mathPool.mu.Lock()
	if mathPool.err != nil {
		err := mathPool.err
		mathPool.mu.Unlock()
		return nil, err
	}
	if n := len(mathPool.idle); n > 0 {
		e := mathPool.idle[n-1]
		mathPool.idle[n-1] = nil
		mathPool.idle = mathPool.idle[:n-1]
		mathPool.mu.Unlock()
		return e, nil
	}
	mathPool.mu.Unlock()
	e, err := mathStartEngine()
	if err != nil {
		mathPool.mu.Lock()
		mathPool.err = err
		mathPool.mu.Unlock()
		return nil, err
	}
	return e, nil
}

func mathPoolPut(e *mathEngine) {
	mathPool.mu.Lock()
	mathPool.idle = append(mathPool.idle, e)
	mathPool.mu.Unlock()
}

// mathSVG draws tex as SVG, in display style or within text.
func mathSVG(tex string, display bool) (string, error) {
	mathPool.once.Do(func() { mathPool.sem = make(chan struct{}, mathPoolSize()) })
	mathPool.sem <- struct{}{}
	defer func() { <-mathPool.sem }()
	e, err := mathPoolGet()
	if err != nil {
		return "", err
	}
	defer mathPoolPut(e)
	out, err := e.js.callString(e.render, mathTimeout, tex, display)
	if err != nil {
		if timedOut(err) {
			return "", errors.New(locales.ErrMathTimeout())
		}
		msg, _ := e.js.thrown(err)
		return "", errors.New(msg)
	}
	return out, nil
}

// mathRender is a formula a parse left to draw: its key and the
// placeholder its blocks hold, filled once it is drawn.
type mathRender struct {
	key mathKey
	f   *mathFormula
}

// renderMathPending draws the formulas a parse collected, several at
// once through the pool, filling the placeholders its blocks hold. Each
// placeholder is its own, so the draws do not share memory; current,
// when set, skips the drawing if the parse has gone stale.
func renderMathPending(pending []mathRender, current func() bool) {
	if len(pending) == 0 || current != nil && !current() {
		return
	}
	ch := make(chan mathRender)
	var wg sync.WaitGroup
	for range min(len(pending), mathPoolSize()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range ch {
				*p.f = *renderMath(p.key.tex, p.key.display)
			}
		}()
	}
	for _, p := range pending {
		ch <- p
	}
	close(ch)
	wg.Wait()
}

var (
	mathViewBox = regexp.MustCompile(`viewBox="([^"]*)"`)
	mathError   = regexp.MustCompile(`data-mjx-error="([^"]*)"`)
	mathSize    = regexp.MustCompile(` (width|height)="[^"]*"`)
)

// renderMath draws the formula tex, in display style or within text.
func renderMath(tex string, display bool) *mathFormula {
	svg, err := mathSVG(tex, display)
	if err != nil {
		return &mathFormula{display: display, err: err.Error()}
	}
	if m := mathError.FindStringSubmatch(svg); m != nil {
		return &mathFormula{display: display, err: html.UnescapeString(m[1])}
	}
	// MyGo's SVGs have no text: MathJax writes in it what its fonts lack.
	if strings.Contains(svg, "<text") {
		return &mathFormula{display: display, err: locales.ErrMathText()}
	}
	m := mathViewBox.FindStringSubmatch(svg)
	if m == nil {
		return &mathFormula{display: display, err: locales.ErrSVGNoSize()}
	}
	box := strings.Fields(m[1])
	var v [4]float64
	for i := range min(len(box), 4) {
		v[i], _ = strconv.ParseFloat(box[i], 64)
	}
	if len(box) != 4 || v[2] <= 0 || v[3] <= 0 {
		return &mathFormula{display: display, err: locales.ErrSVGNoSize()}
	}
	// The root is as large as its view box, in thousandths of an em,
	// rather than in ex, which MyGo does not measure.
	end := strings.IndexByte(svg, '>')
	root := mathSize.ReplaceAllString(svg[:end], "")
	root += ` width="` + box[2] + `" height="` + box[3] + `"`
	s, err := ui.ParseSVG([]byte(root + svg[end:]))
	if err != nil {
		return &mathFormula{display: display, err: err.Error()}
	}
	// The view box starts its height above the baseline.
	return &mathFormula{svg: s, display: display, w: float32(v[2] / 1000), ascent: float32(-v[1] / 1000), depth: float32((v[1] + v[3]) / 1000)}
}

// mathSpan is the span of a formula within text of size: a box it is
// drawn in, as wide as it and as high as it reaches above the baseline.
func mathSpan(f *mathFormula, size float32) ui.Span {
	return ui.Span{Text: "￼", ObjectWidth: max(f.w*size, 1), ObjectHeight: max(f.ascent*size, 0)}
}

// mathPlaces finds where the formulas of a text drawn as a RichText of
// spans in font, width wide, its lines lineHeight high, stand in it: the
// boxes of the formulas, in their order, below their tops by their
// depths.
//
// The text's own layout is out of reach, so spans are laid out again,
// in the font's own line spacing, and their lines spaced as the text's:
// as high as lineHeight or what is on them, centered in that height.
func mathPlaces(spans []ui.Span, font ui.Font, width, lineHeight float32) []ui.Rect {
	tl := ui.ShapeRichText(spans, font, width)
	objects := tl.Objects()
	if len(objects) == 0 {
		return nil
	}
	n := 0
	for _, s := range spans {
		for _, r := range s.Text {
			n += utf16.RuneLen(r)
		}
	}
	type line struct{ y, h, top float32 }
	var lines []line
	top := float32(0)
	for i := 0; i <= n; i++ {
		c := tl.Caret(i)
		if len(lines) > 0 && lines[len(lines)-1].y == c.Y {
			continue
		}
		lines = append(lines, line{c.Y, c.H, top})
		top += max(lineHeight, c.H)
	}
	out := make([]ui.Rect, len(objects))
	for i, o := range objects {
		base := o.Rect.Y + o.Rect.H
		l := lines[0]
		for _, k := range lines {
			if base >= k.y {
				l = k
			}
		}
		out[i] = ui.Rect{X: o.Rect.X, Y: l.top + o.Rect.Y - l.y + (max(lineHeight, l.h)-l.h)/2, W: o.Rect.W, H: o.Rect.H}
	}
	return out
}

// mathNode is a formula of TeX within text, between $ and $, or $$ and
// $$ for one in display style.
type mathNode struct {
	ast.BaseInline
	tex     string
	display bool
	// start and stop are where the formula and its $s are in the source,
	// which Live Preview hides them by.
	start, stop int
}

var kindMath = ast.NewNodeKind("Math")

func (n *mathNode) Kind() ast.NodeKind { return kindMath }

func (n *mathNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"TeX": n.tex}, nil)
}

// mathParser parses the formulas of text as GitHub does: $$ to $$, or $
// to a $ that is not after a space nor before a digit, the first not
// before a space, the TeX between them left as it is.
type mathParser struct{}

func (mathParser) Trigger() []byte { return []byte{'$'} }

func (mathParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	l, pos := block.Position()
	line, _ := block.PeekLine()
	start := pos.Start
	fence := 1
	if len(line) > 1 && line[1] == '$' {
		fence = 2
	}
	if len(line) <= fence || fence == 1 && (line[1] == ' ' || line[1] == '\t' || line[1] == '\n') {
		return nil
	}
	block.Advance(fence)
	// The formula may go on over the lines of its paragraph.
	var tex []byte
	for {
		line, _ := block.PeekLine()
		if line == nil {
			// No end: the $ is a dollar.
			block.SetPosition(l, pos)
			return nil
		}
		for i := 0; i < len(line); i++ {
			switch {
			case line[i] == '\\':
				i++
				continue
			case line[i] != '$' || fence == 2 && (i+1 >= len(line) || line[i+1] != '$'):
				continue
			}
			tex = append(tex, line[:i]...)
			ok := len(bytes.TrimSpace(tex)) > 0
			if fence == 1 {
				last := tex[len(tex)-1]
				ok = ok && last != ' ' && last != '\t' && last != '\n' && (i+1 >= len(line) || line[i+1] < '0' || line[i+1] > '9')
			}
			if !ok {
				block.SetPosition(l, pos)
				return nil
			}
			block.Advance(i + fence)
			_, end := block.Position()
			return &mathNode{tex: string(bytes.TrimSpace(tex)), display: fence == 2, start: start, stop: end.Start}
		}
		tex = append(tex, line...)
		block.AdvanceLine()
	}
}
