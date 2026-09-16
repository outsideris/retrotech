package research

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// This opt-in browser check uses a real report copy and a deterministic runner.
// The ordinary Go suite covers the HTTP and persistence contracts without Chrome.
func TestBrowserWorkflow(t *testing.T) {
	node := os.Getenv("RETROTECH_BROWSER_QA_NODE")
	report := os.Getenv("RETROTECH_BROWSER_QA_REPORT")
	if node == "" || report == "" {
		t.Skip("set RETROTECH_BROWSER_QA_NODE and RETROTECH_BROWSER_QA_REPORT for Chrome integration")
	}
	s := fixture(t)
	for _, name := range []string{"index.html", "research.md"} {
		b, e := os.ReadFile(filepath.Join(report, name))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(s.Dir, name), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	os.RemoveAll(filepath.Join(s.Dir, ".research"))
	app := NewServer(nil, func(ctx context.Context, p string, r Request, d string, progress func(string)) (*Result, error) {
		select {
		case <-time.After(700 * time.Millisecond):
			res := result()
			res.Title = "브라우저 검증용 추가 조사"
			res.Correction = true
			return res, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	httpServer := httptest.NewServer(app.Handler())
	defer httpServer.Close()
	var e error
	app.Store, e = NewStore(s.Dir, s.SkillDir, httpServer.URL)
	if e != nil {
		t.Fatal(e)
	}
	defer app.Close()
	command := exec.Command(node, "../../scripts/test-research-browser.mjs", httpServer.URL, s.Dir)
	output, e := command.CombinedOutput()
	t.Log(string(output))
	if e != nil {
		t.Fatal(e)
	}
}
