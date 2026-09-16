// Package research adds anchored follow-up investigations to a local report.
package research

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Source struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Author   string `json:"author"`
	Date     string `json:"date"`
	URL      string `json:"url"`
	Locator  string `json:"locator"`
	Supports string `json:"supports"`
	Kind     string `json:"kind"`
}
type Paragraph struct {
	Text      string   `json:"text"`
	SourceIDs []string `json:"sourceIds"`
}
type Learning struct {
	Gap  string `json:"gap"`
	Rule string `json:"rule"`
}
type Result struct {
	Title       string      `json:"title"`
	Answer      string      `json:"answer"`
	Status      string      `json:"status"`
	Correction  bool        `json:"correction"`
	Paragraphs  []Paragraph `json:"paragraphs"`
	Sources     []Source    `json:"sources"`
	Uncertainty string      `json:"uncertainty"`
	Learning    Learning    `json:"learning"`
}
type Request struct {
	AnchorID string `json:"anchorId"`
	Question string `json:"question"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Effort   string `json:"effort"`
	ParentID string `json:"parentId"`
}
type Job struct {
	ID          string  `json:"id"`
	Request     Request `json:"request"`
	ChapterID   string  `json:"chapterId"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	Status      string  `json:"status"`
	Progress    string  `json:"progress"`
	Error       string  `json:"error,omitempty"`
	Result      *Result `json:"result,omitempty"`
	Hidden      bool    `json:"hidden"`
	AppliedRule string  `json:"appliedRule,omitempty"`
	HTML        string  `json:"html,omitempty"`
}
type Anchor struct {
	ID           string `json:"id"`
	ChapterID    string `json:"chapterId"`
	ChapterTitle string `json:"chapterTitle"`
	Text         string `json:"text"`
	Start        int    `json:"-"`
	End          int    `json:"-"`
}
type State struct {
	Version  int    `json:"version"`
	Revision int    `json:"revision"`
	HTMLHash string `json:"htmlHash"`
	MDHash   string `json:"mdHash"`
	Jobs     []*Job `json:"jobs"`
}
type Runner func(context.Context, string, Request, string, func(string)) (*Result, error)

var Schema = `{"type":"object","additionalProperties":false,"required":["title","answer","status","correction","paragraphs","sources","uncertainty","learning"],"properties":{"title":{"type":"string"},"answer":{"type":"string"},"status":{"type":"string","enum":["verified","partial","unresolved"]},"correction":{"type":"boolean"},"paragraphs":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["text","sourceIds"],"properties":{"text":{"type":"string"},"sourceIds":{"type":"array","items":{"type":"string"}}}}},"sources":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["id","title","author","date","url","locator","supports","kind"],"properties":{"id":{"type":"string"},"title":{"type":"string"},"author":{"type":"string"},"date":{"type":"string"},"url":{"type":"string"},"locator":{"type":"string"},"supports":{"type":"string"},"kind":{"type":"string"}}}},"uncertainty":{"type":"string"},"learning":{"type":"object","additionalProperties":false,"required":["gap","rule"],"properties":{"gap":{"type":"string"},"rule":{"type":"string"}}}}}`

func DecodeResult(raw string) (*Result, error) {
	var r Result
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		return nil, fmt.Errorf("조사 결과 형식 오류: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, errors.New("조사 결과는 하나의 JSON이어야 합니다")
	}
	if err := validateResult(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

var sourceIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,50}$`)

func validateResult(r *Result) error {
	if r == nil {
		return errors.New("조사 결과가 없습니다")
	}
	if r.Status != "verified" && r.Status != "partial" && r.Status != "unresolved" {
		return errors.New("알 수 없는 근거 상태")
	}
	if strings.TrimSpace(r.Title) == "" || strings.TrimSpace(r.Answer) == "" || len(r.Title) > 600 || len(r.Answer) > 15000 {
		return errors.New("조사 제목이나 요약이 올바르지 않습니다")
	}
	if len(r.Paragraphs) == 0 || len(r.Paragraphs) > 15 || len(r.Sources) > 25 {
		return errors.New("조사 문단 또는 출처 범위가 올바르지 않습니다")
	}
	ids := map[string]bool{}
	for _, s := range r.Sources {
		u, e := url.Parse(s.URL)
		if e != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return errors.New("출처 URL은 공개 HTTP/HTTPS 주소여야 합니다")
		}
		if ip := net.ParseIP(u.Hostname()); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified()) {
			return errors.New("로컬 주소는 출처로 사용할 수 없습니다")
		}
		if strings.EqualFold(u.Hostname(), "localhost") || !sourceIDRE.MatchString(s.ID) || ids[s.ID] || strings.TrimSpace(s.Title) == "" || strings.TrimSpace(s.Locator) == "" || strings.TrimSpace(s.Supports) == "" {
			return errors.New("출처 식별자·제목·근거 위치를 확인해 주세요")
		}
		ids[s.ID] = true
	}
	if r.Status == "verified" && len(ids) == 0 {
		return errors.New("확인된 조사에는 열람한 출처가 필요합니다")
	}
	for _, p := range r.Paragraphs {
		if strings.TrimSpace(p.Text) == "" || len(p.Text) > 20000 {
			return errors.New("빈 문단 또는 지나치게 긴 문단입니다")
		}
		if r.Status == "verified" && len(p.SourceIDs) == 0 {
			return errors.New("확인된 각 문단에는 근거 출처가 필요합니다")
		}
		for _, id := range p.SourceIDs {
			if !ids[id] {
				return fmt.Errorf("없는 출처 참조: %s", id)
			}
		}
	}
	if (r.Status == "partial" || r.Status == "unresolved") && strings.TrimSpace(r.Uncertainty) == "" {
		return errors.New("미확인 범위를 설명해야 합니다")
	}
	if len(r.Learning.Rule) > 6000 || len(r.Learning.Gap) > 6000 {
		return errors.New("스킬 규칙 초안이 너무 깁니다")
	}
	return nil
}
func now() string { return time.Now().Format(time.RFC3339) }
