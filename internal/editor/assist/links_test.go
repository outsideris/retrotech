package assist

import (
	"reflect"
	"testing"
)

func TestExtractLinks(t *testing.T) {
	script := `# 2i. 제목

[John Resig](https://johnresig.com/)이 만든 [jQuery](https://jquery.com/)는
[Wired](https://en.wikipedia.org/wiki/Wired_(magazine)) 기사에도 나옵니다.
자세한 내용은 https://example.com/post 를 보세요 (https://en.wikipedia.org/wiki/JSONP).
![스크린샷](https://example.com/shot.png) 이미지는 제외.
같은 링크 반복: [jQuery 다시](https://jquery.com/)
`
	want := []Link{
		{Text: "John Resig", URL: "https://johnresig.com/"},
		{Text: "jQuery", URL: "https://jquery.com/"},
		{Text: "Wired", URL: "https://en.wikipedia.org/wiki/Wired_(magazine)"},
		{Text: "", URL: "https://example.com/post"},
		{Text: "", URL: "https://en.wikipedia.org/wiki/JSONP"},
	}
	got := ExtractLinks(script)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractLinks:\n got %#v\nwant %#v", got, want)
	}
}

func TestExtractLinksOrderMixesBareAndMarkdown(t *testing.T) {
	script := "먼저 https://first.example.com 그리고 [둘째](https://second.example.com) 순서."
	got := ExtractLinks(script)
	if len(got) != 2 || got[0].URL != "https://first.example.com" || got[1].URL != "https://second.example.com" {
		t.Errorf("document order not kept: %#v", got)
	}
}

func TestExtractLinksTrimsTrailingPunctuation(t *testing.T) {
	got := ExtractLinks("링크는 https://example.com/a. 그리고 https://example.com/b, 입니다.")
	want := []Link{
		{Text: "", URL: "https://example.com/a"},
		{Text: "", URL: "https://example.com/b"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestExtractLinksEmpty(t *testing.T) {
	if got := ExtractLinks("링크가 하나도 없는 대본입니다."); len(got) != 0 {
		t.Errorf("want no links, got %#v", got)
	}
}
