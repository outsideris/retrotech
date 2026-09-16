package research

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

//go:embed assets/*
var assets embed.FS
var articleRE = regexp.MustCompile(`(?s)(<article class="prose">)(.*?)(</article>)`)
var paragraphRE = regexp.MustCompile(`<p(?:\s[^>]*)?>`)
var tagsRE = regexp.MustCompile(`<[^>]*>`)
var countRE = regexp.MustCompile(`\d+개 출처 링크`)
var ErrConflict = errors.New("보고서 파일이 다른 곳에서 변경되었습니다. 덮어쓰지 않았습니다. 변경본을 보존한 뒤 서버를 다시 확인해 주세요")

type Store struct {
	mu                    sync.Mutex
	Dir, SkillDir, Origin string
	baseHTML, baseMD      []byte
	Anchors               []Anchor
	State                 State
}

func hash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func newID() string        { b := make([]byte, 8); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func atomicWrite(path string, b []byte, mode os.FileMode) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".research-write-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(mode); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, path)
}
func NewStore(dir, skillDir, origin string) (*Store, error) {
	abs, e := filepath.Abs(dir)
	if e != nil {
		return nil, e
	}
	abs, e = filepath.EvalSymlinks(abs)
	if e != nil {
		return nil, e
	}
	// Never accept the manuscript source tree as a report destination.
	for _, p := range strings.Split(filepath.ToSlash(abs), "/") {
		if p == "episodes" {
			return nil, errors.New("episodes 원본 디렉터리는 조사 출력에 사용할 수 없습니다")
		}
	}
	s := &Store{Dir: abs, SkillDir: skillDir, Origin: origin}
	meta := filepath.Join(abs, ".research")
	if e = os.MkdirAll(meta, 0700); e != nil {
		return nil, e
	}
	currentHTML, e := os.ReadFile(filepath.Join(abs, "index.html"))
	if e != nil {
		return nil, e
	}
	currentMD, e := os.ReadFile(filepath.Join(abs, "research.md"))
	if e != nil {
		return nil, e
	}
	stateBytes, se := os.ReadFile(filepath.Join(meta, "state.json"))
	if se == nil {
		if e = json.Unmarshal(stateBytes, &s.State); e != nil {
			return nil, e
		}
		if s.State.Version != 1 {
			return nil, errors.New("지원하지 않는 조사 상태 버전입니다")
		}
		if hash(currentHTML) != s.State.HTMLHash || hash(currentMD) != s.State.MDHash {
			return nil, ErrConflict
		}
		s.baseHTML, e = os.ReadFile(filepath.Join(meta, "base.html"))
		if e != nil {
			return nil, e
		}
		s.baseMD, e = os.ReadFile(filepath.Join(meta, "base.md"))
		if e != nil {
			return nil, e
		}
	} else if os.IsNotExist(se) {
		if bytes.Contains(currentHTML, []byte("retrotech-research-ui")) {
			return nil, errors.New("추가 조사 상태 파일이 없습니다. .research 폴더를 복원해 주세요")
		}
		s.baseHTML, s.baseMD = currentHTML, currentMD
		s.State = State{Version: 1, HTMLHash: hash(currentHTML), MDHash: hash(currentMD), Jobs: []*Job{}}
		for name, b := range map[string][]byte{"base.html": currentHTML, "base.md": currentMD} {
			path := filepath.Join(meta, name)
			if old, er := os.ReadFile(path); er == nil && !bytes.Equal(old, b) {
				return nil, ErrConflict
			}
			if e = atomicWrite(path, b, 0600); e != nil {
				return nil, e
			}
		}
	} else {
		return nil, se
	}
	if e = s.parseAnchors(); e != nil {
		return nil, e
	}
	for _, j := range s.State.Jobs {
		if j.Status == "running" || j.Status == "queued" {
			j.Status = "failed"
			j.Error = "서버가 종료되어 조사가 중단되었습니다. 질문을 다시 실행할 수 있습니다."
			j.UpdatedAt = now()
		}
	}
	if e = s.saveLocked(true); e != nil {
		return nil, e
	}
	return s, nil
}
func (s *Store) parseAnchors() error {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := md.Parser().Parse(text.NewReader(s.baseMD))
	chapter, title, n := "orientation", "조사 개요", 0
	_ = ast.Walk(doc, func(node ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}
		if h, ok := node.(*ast.Heading); ok && h.Level == 2 {
			n++
			chapter = fmt.Sprintf("chapter-%02d", n)
			title = string(h.Text(s.baseMD))
		}
		if _, ok := node.(*ast.ThematicBreak); ok {
			chapter = "closing"
			title = "이 에피소드가 남기는 질문"
		}
		if p, ok := node.(*ast.Paragraph); ok && p.Lines().Len() > 0 {
			first, last := p.Lines().At(0), p.Lines().At(p.Lines().Len()-1)
			s.Anchors = append(s.Anchors, Anchor{ID: fmt.Sprintf("research-p-%03d", len(s.Anchors)+1), ChapterID: chapter, ChapterTitle: title, Text: string(p.Text(s.baseMD)), Start: first.Start, End: last.Stop})
		}
		return ast.WalkContinue, nil
	})
	m := articleRE.FindSubmatch(s.baseHTML)
	if len(m) != 4 || len(paragraphRE.FindAll(m[2], -1)) != len(s.Anchors) {
		return errors.New("HTML과 Markdown의 문단 구조가 맞지 않습니다. 두 원본을 먼저 맞춰 주세요")
	}
	return nil
}
func (s *Store) anchor(id string) (Anchor, bool) {
	for _, a := range s.Anchors {
		if a.ID == id {
			return a, true
		}
	}
	return Anchor{}, false
}
func (s *Store) job(id string) *Job {
	for _, j := range s.State.Jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}
func (s *Store) Snapshot() ([]Anchor, State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(s.State)
	var st State
	_ = json.Unmarshal(b, &st)
	for _, j := range st.Jobs {
		if j.Status == "completed" && j.Result != nil && !j.Hidden {
			j.HTML = renderAddition(j)
		}
	}
	return append([]Anchor(nil), s.Anchors...), st
}
func (s *Store) checkFiles() error {
	for name, want := range map[string]string{"index.html": s.State.HTMLHash, "research.md": s.State.MDHash} {
		b, e := os.ReadFile(filepath.Join(s.Dir, name))
		if e != nil {
			return e
		}
		if hash(b) != want {
			return ErrConflict
		}
	}
	return nil
}
func (s *Store) saveLocked(render bool) error {
	if e := s.checkFiles(); e != nil {
		return e
	}
	oldHTML, oldMD, oldRevision := s.State.HTMLHash, s.State.MDHash, s.State.Revision
	var previousHTML, previousMD []byte
	rollback := func() {
		s.State.HTMLHash, s.State.MDHash, s.State.Revision = oldHTML, oldMD, oldRevision
		if render {
			_ = atomicWrite(filepath.Join(s.Dir, "index.html"), previousHTML, 0644)
			_ = atomicWrite(filepath.Join(s.Dir, "research.md"), previousMD, 0644)
		}
	}
	if render {
		h, m, e := s.render()
		if e != nil {
			return e
		}
		previousHTML, e = os.ReadFile(filepath.Join(s.Dir, "index.html"))
		if e != nil {
			return e
		}
		previousMD, e = os.ReadFile(filepath.Join(s.Dir, "research.md"))
		if e != nil {
			return e
		}
		if e = atomicWrite(filepath.Join(s.Dir, "index.html"), h, 0644); e != nil {
			return e
		}
		if e = atomicWrite(filepath.Join(s.Dir, "research.md"), m, 0644); e != nil {
			rollback()
			return e
		}
		s.State.HTMLHash, s.State.MDHash = hash(h), hash(m)
	}
	s.State.Revision++
	b, e := json.MarshalIndent(s.State, "", "  ")
	if e != nil {
		rollback()
		return e
	}
	if e = atomicWrite(filepath.Join(s.Dir, ".research/state.json"), b, 0600); e != nil {
		rollback()
		return e
	}
	return nil
}
func (s *Store) render() ([]byte, []byte, error) {
	i := 0
	m := articleRE.FindStringSubmatch(string(s.baseHTML))
	body := paragraphRE.ReplaceAllStringFunc(m[2], func(tag string) string {
		a := s.Anchors[i]
		i++
		return strings.TrimSuffix(tag, ">") + ` id="` + a.ID + `" data-research-anchor="` + a.ID + `" tabindex="0">`
	})
	pEnd := regexp.MustCompile(`(?s)<p[^>]*data-research-anchor="([^"]+)"[^>]*>.*?</p>`)
	grouped := map[string]string{}
	addedLinks := 0
	for _, j := range s.State.Jobs {
		if j.Status == "completed" && j.Result != nil && !j.Hidden {
			grouped[j.Request.AnchorID] += renderAddition(j)
			addedLinks += len(j.Result.Sources)
		}
	}
	body = pEnd.ReplaceAllStringFunc(body, func(p string) string { id := pEnd.FindStringSubmatch(p)[1]; return p + grouped[id] })
	h := strings.Replace(string(s.baseHTML), m[0], m[1]+body+m[3], 1)
	if addedLinks > 0 {
		h = countRE.ReplaceAllStringFunc(h, func(s string) string {
			n, _ := strconv.Atoi(strings.Split(s, "개")[0])
			return strconv.Itoa(n+addedLinks) + "개 출처 링크"
		})
	}
	css, _ := assets.ReadFile("assets/workbench.css")
	js, _ := assets.ReadFile("assets/workbench.js")
	panel, _ := assets.ReadFile("assets/panel.html")
	config, _ := json.Marshal(map[string]string{"origin": s.Origin, "reportId": hash([]byte(s.Dir))[:16]})
	injection := "<!-- retrotech-research-ui -->\n<style>" + string(css) + "</style>" + string(panel) + "<script>window.RESEARCH_CONFIG=" + string(config) + ";</script><script>" + string(js) + "</script>"
	h = strings.Replace(h, "</body>", injection+"</body>", 1)
	md := append([]byte(nil), s.baseMD...)
	sorted := append([]Anchor(nil), s.Anchors...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].End > sorted[j].End })
	for _, a := range sorted {
		add := ""
		for _, j := range s.State.Jobs {
			if j.Request.AnchorID == a.ID && j.Status == "completed" && j.Result != nil && !j.Hidden {
				add += renderAdditionMD(j)
			}
		}
		if add != "" {
			md = append(append(append([]byte(nil), md[:a.End]...), []byte("\n\n"+add)...), md[a.End:]...)
		}
	}
	return []byte(h), md, nil
}
func esc(s string) string { return html.EscapeString(s) }
func statusLabel(s string) string {
	switch s {
	case "verified":
		return "출처 열람 확인"
	case "partial":
		return "일부 미확인"
	default:
		return "미확인"
	}
}
func renderAddition(j *Job) string {
	r := j.Result
	var b strings.Builder
	badge := "추가 조사"
	if r.Correction {
		badge = "추가 조사 · 기존 내용 정정"
	}
	fmt.Fprintf(&b, `<aside class="research-addition" id="research-add-%s" data-job="%s" aria-label="%s"><div class="addition-meta"><strong>%s</strong><time datetime="%s">%s</time><span>%s · %s</span></div><h3>%s</h3><div class="addition-question">질문 · %s</div>`, esc(j.ID), esc(j.ID), badge, badge, esc(j.CreatedAt), esc(strings.ReplaceAll(j.CreatedAt[:min(16, len(j.CreatedAt))], "T", " ")), esc(j.Request.Model), esc(j.Request.Effort), esc(r.Title), esc(j.Request.Question))
	for _, p := range r.Paragraphs {
		fmt.Fprintf(&b, "<p>%s", strings.ReplaceAll(esc(p.Text), "\n", "<br>"))
		for _, id := range p.SourceIDs {
			fmt.Fprintf(&b, ` <a class="source-ref" href="#research-source-%s-%s">[%s]</a>`, esc(j.ID), esc(id), esc(id))
		}
		b.WriteString("</p>")
	}
	if r.Uncertainty != "" {
		fmt.Fprintf(&b, `<p class="addition-limit"><strong>%s:</strong> %s</p>`, statusLabel(r.Status), esc(r.Uncertainty))
	}
	if len(r.Sources) > 0 {
		b.WriteString(`<ol class="addition-sources">`)
		for _, src := range r.Sources {
			fmt.Fprintf(&b, `<li id="research-source-%s-%s"><a href="%s" target="_blank" rel="noopener noreferrer">[%s] %s</a><span>%s · %s · %s</span><div>찾을 곳: %s</div><div>근거: %s</div></li>`, esc(j.ID), esc(src.ID), esc(src.URL), esc(src.ID), esc(src.Title), esc(src.Author), esc(src.Date), esc(src.Kind), esc(src.Locator), esc(src.Supports))
		}
		b.WriteString(`</ol>`)
	}
	fmt.Fprintf(&b, `<button type="button" class="addition-chat" data-open-job="%s">이 조사 대화 보기 ↗</button></aside>`, esc(j.ID))
	return b.String()
}
func mdEscape(s string) string {
	return strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "*", "\\*", "_", "\\_", "<", "&lt;", ">", "&gt;", "`", "\\`").Replace(s)
}
func renderAdditionMD(j *Job) string {
	r := j.Result
	var b strings.Builder
	label := "추가 조사"
	if r.Correction {
		label += " · 기존 내용 정정"
	}
	fmt.Fprintf(&b, "> **%s — %s**\n>\n> %s · %s / %s\n>\n> 질문: %s\n", label, mdEscape(r.Title), j.CreatedAt, j.Request.Model, j.Request.Effort, strings.ReplaceAll(mdEscape(j.Request.Question), "\n", " "))
	for _, p := range r.Paragraphs {
		fmt.Fprintf(&b, ">\n> %s\n", strings.ReplaceAll(mdEscape(p.Text), "\n", "\n> ")+" ["+strings.Join(p.SourceIDs, ", ")+"]")
	}
	if r.Uncertainty != "" {
		fmt.Fprintf(&b, ">\n> **%s:** %s\n", statusLabel(r.Status), strings.ReplaceAll(mdEscape(r.Uncertainty), "\n", "\n> "))
	}
	for _, src := range r.Sources {
		u := strings.NewReplacer("(", "%28", ")", "%29").Replace(src.URL)
		fmt.Fprintf(&b, ">\n> [%s. %s](%s) — %s · %s · %s. 찾을 곳: %s. 근거: %s\n", mdEscape(src.ID), mdEscape(src.Title), u, mdEscape(src.Author), mdEscape(src.Date), mdEscape(src.Kind), mdEscape(src.Locator), mdEscape(src.Supports))
	}
	return b.String() + "\n"
}
