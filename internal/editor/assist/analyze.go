package assist

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ScriptRef is one reference the model titled for a link extracted from the
// script: the linked page's title (per the naming rules in analyzePrompt) plus
// its URL.
type ScriptRef struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// ScriptMeta is the metadata an LLM extracts from a podcast script: the title
// (the markdown heading), an episode id pulled from that title, a summary the
// editor uses as the episode intro (the frontmatter description is derived
// from the intro at save time), and the script's links titled as references.
type ScriptMeta struct {
	Title       string      `json:"title"`
	ID          string      `json:"id"`
	Description string      `json:"description"`
	References  []ScriptRef `json:"references"`
}

// AnalyzeScript asks the provider to read a podcast script (markdown) and
// return title/id/description plus a titled reference entry for every link in
// the script, then parses that JSON. The links themselves are extracted in Go
// (ExtractLinks) and reconciled afterwards, so the reference list always covers
// exactly the script's links in document order — the model only supplies
// titles. The Meta is the call's telemetry, so a script import shows up in the
// usage trace like any other run.
func AnalyzeScript(ctx context.Context, p Provider, opts Options, script string) (ScriptMeta, Meta, error) {
	links := ExtractLinks(script)
	out, meta, err := p.Run(ctx, analyzePrompt(script, links), opts)
	if err != nil {
		return ScriptMeta{}, meta, err
	}
	sm, perr := parseScriptMeta(out)
	if perr != nil {
		return ScriptMeta{}, meta, perr
	}
	sm.References = reconcileRefs(links, sm.References)
	return sm, meta, nil
}

// refTitleRules is the house style for reference titles, derived from the
// published episodes' reference lists (2026-08, episodes 2g/2h being the
// current convention). It is embedded in the analyze prompt so every script
// import follows it; docs/DESIGN.md documents the same rules for humans.
const refTitleRules = `- title 은 대본의 앵커 텍스트가 아니라 **링크가 가리키는 페이지/사이트의 실제 제목**이다. 앵커 텍스트는 힌트로만 쓴다.
- 제목 표기 규칙(기존 회차의 관례):
  1. 글·논문·영상·발표자료는 그 콘텐츠의 원래 제목 그대로, 원문 언어를 유지한다(번역 금지).
  2. Wikipedia 문서: "<문서 제목>: Wikipedia" — 예: "Brian Behlendorf: Wikipedia".
  3. 인물·회사의 프로필/홈: "<이름>: <플랫폼>" — 예: "Tim O'Reilly: LinkedIn", "Brian Behlendorf: X", 개인 홈은 "<이름>: Homepage".
  4. web.archive.org 스냅샷: 특정 시점의 사이트/홈페이지면 "<연도>년의 <사이트 이름>" — 예: "1997년의 HotWired". 아카이브된 글/문서면 그 글의 원래 제목을 쓴다.
  5. 사이트/프로젝트 홈은 이름만 — 예: "Wired", "Bugzilla".
  6. 제목만으로 무엇인지 모호하거나 같은 제목이 반복되면 "<저자>가 쓴 <제목>" 또는 "<저자>의 <제목>"으로 구분한다.
  7. 페이지 제목을 확실히 알 수 없으면 앵커 텍스트를 다듬어 쓰고, 그마저 없으면 URL 로 무엇인지 서술한다. 제목을 지어내지 않는다.`

// analyzePrompt builds the extraction prompt. It asks for a bare JSON object so
// parsing stays simple; parseScriptMeta still tolerates fences/prose. The
// pre-extracted link list is included verbatim so the model titles exactly
// those URLs instead of re-finding (and possibly missing) them.
func analyzePrompt(script string, links []Link) string {
	var b strings.Builder
	b.WriteString(`다음은 팟캐스트 대본(마크다운)입니다. 내용을 읽고 아래 JSON 객체 하나만 출력하세요. ` +
		`코드펜스나 다른 설명 없이 순수 JSON 만 출력합니다.

{"title": "대본의 제목. 보통 첫 번째 마크다운 제목(# 또는 ##) 줄의 텍스트.",
 "id": "제목 맨 앞의 회차 식별자(슬러그). 예: '2h. VCS: ...' → '2h'. 식별자가 없으면 빈 문자열.",
 "description": "대본 내용을 한국어 2~4문장으로 요약. 청취자에게 이 회차에서 무엇을 다루는지 설명."`)
	if len(links) > 0 {
		b.WriteString(`,
 "references": [{"title": "링크 대상의 제목", "url": "링크 URL"}, ...]}

references 규칙:
- 아래 "링크 목록"의 모든 URL 을 순서 그대로, 하나도 빠짐없이 포함한다. 목록에 없는 URL 을 추가하지 않는다.
`)
		b.WriteString(refTitleRules)
		b.WriteString("\n\n링크 목록 (앵커 텍스트 | URL):\n")
		for i, l := range links {
			text := l.Text
			if text == "" {
				text = "(없음)"
			}
			fmt.Fprintf(&b, "%d. %s | %s\n", i+1, text, l.URL)
		}
	} else {
		b.WriteString("}\n")
	}
	b.WriteString("\n대본:\n---\n")
	b.WriteString(script)
	return b.String()
}

// reconcileRefs maps the model's titles back onto the extracted links: the
// result covers every extracted link exactly once, in document order, no matter
// what the model returned. A link the model skipped falls back to its anchor
// text, then to the URL itself; URLs the model invented are dropped.
func reconcileRefs(links []Link, refs []ScriptRef) []ScriptRef {
	titles := make(map[string]string, len(refs))
	for _, r := range refs {
		u, t := strings.TrimSpace(r.URL), strings.TrimSpace(r.Title)
		if u == "" || t == "" {
			continue
		}
		if _, ok := titles[u]; !ok {
			titles[u] = t
		}
	}
	out := make([]ScriptRef, 0, len(links))
	for _, l := range links {
		title := titles[l.URL]
		if title == "" {
			title = l.Text
		}
		if title == "" {
			title = l.URL
		}
		out = append(out, ScriptRef{Title: title, URL: l.URL})
	}
	return out
}

// parseScriptMeta decodes the JSON object out of the model's response, tolerating
// a model that wrapped it in a ```json fence or added surrounding prose.
func parseScriptMeta(out string) (ScriptMeta, error) {
	raw := extractJSONObject(out)
	var sm ScriptMeta
	if err := json.Unmarshal([]byte(raw), &sm); err != nil {
		return ScriptMeta{}, fmt.Errorf("analyze: response was not the expected JSON: %w", err)
	}
	sm.Title = strings.TrimSpace(sm.Title)
	sm.ID = strings.TrimSpace(sm.ID)
	sm.Description = strings.TrimSpace(sm.Description)
	if sm.Title == "" && sm.Description == "" {
		return ScriptMeta{}, fmt.Errorf("analyze: model returned no title or description")
	}
	return sm, nil
}

// extractJSONObject returns the substring from the first "{" to the last "}",
// which strips a leading ```json fence / trailing prose around the object.
func extractJSONObject(s string) string {
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}
