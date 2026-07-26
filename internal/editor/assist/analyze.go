package assist

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ScriptMeta is the metadata an LLM extracts from a podcast script: the title
// (the markdown heading), an episode id pulled from that title, and a summary
// for the episode description.
type ScriptMeta struct {
	Title       string `json:"title"`
	ID          string `json:"id"`
	Description string `json:"description"`
}

// AnalyzeScript asks the provider to read a podcast script (markdown) and return
// title/id/description as JSON, then parses that JSON. The Meta is the call's
// telemetry, so a script import shows up in the usage trace like any other run.
func AnalyzeScript(ctx context.Context, p Provider, opts Options, script string) (ScriptMeta, Meta, error) {
	out, meta, err := p.Run(ctx, analyzePrompt(script), opts)
	if err != nil {
		return ScriptMeta{}, meta, err
	}
	sm, perr := parseScriptMeta(out)
	if perr != nil {
		return ScriptMeta{}, meta, perr
	}
	return sm, meta, nil
}

// analyzePrompt builds the extraction prompt. It asks for a bare JSON object so
// parsing stays simple; parseScriptMeta still tolerates fences/prose.
func analyzePrompt(script string) string {
	return `다음은 팟캐스트 대본(마크다운)입니다. 내용을 읽고 아래 JSON 객체 하나만 출력하세요. ` +
		`코드펜스나 다른 설명 없이 순수 JSON 만 출력합니다.

{"title": "대본의 제목. 보통 첫 번째 마크다운 제목(# 또는 ##) 줄의 텍스트.",
 "id": "제목 맨 앞의 회차 식별자(슬러그). 예: '2h. VCS: ...' → '2h'. 식별자가 없으면 빈 문자열.",
 "description": "대본 내용을 한국어 2~4문장으로 요약. 청취자에게 이 회차에서 무엇을 다루는지 설명."}

대본:
---
` + script
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
