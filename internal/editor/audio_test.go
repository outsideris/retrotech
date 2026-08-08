package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

// newAudioTestServer is newTestServer plus a fake uploader: it records the key
// and captures the spooled file's bytes instead of shelling out to wrangler.
func newAudioTestServer(t *testing.T) (*httptest.Server, *fakeUpload) {
	t.Helper()
	repo := t.TempDir()
	for _, d := range []string{"content/episodes", "public"} {
		if err := os.MkdirAll(repo+"/"+d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	ed, err := New(Config{RepoDir: repo})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fu := &fakeUpload{}
	ed.putObject = fu.put
	srv := httptest.NewServer(ed.Handler())
	t.Cleanup(srv.Close)
	return srv, fu
}

type fakeUpload struct {
	key  string
	body []byte
	err  error
}

func (f *fakeUpload) put(_ context.Context, key, file string) error {
	if f.err != nil {
		return f.err
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	f.key, f.body = key, b
	return nil
}

// postAudio submits a multipart upload of content under key.
func postAudio(t *testing.T, srv *httptest.Server, key, filename string, content []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if key != "" {
		if err := mw.WriteField("key", key); err != nil {
			t.Fatal(err)
		}
	}
	if filename != "" {
		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	mw.Close()
	resp, err := http.Post(srv.URL+"/_write/api/audio/upload", mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestAudioUpload(t *testing.T) {
	srv, fu := newAudioTestServer(t)
	content := []byte("ID3fake-mp3-bytes")

	resp := postAudio(t, srv, "2i.mp3", "episode.mp3", content)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	var out struct {
		URL  string `json:"url"`
		Key  string `json:"key"`
		Size int64  `json:"size"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.URL != "https://retrotech-episodes.outsider.dev/2i.mp3" {
		t.Errorf("url = %q", out.URL)
	}
	if out.Size != int64(len(content)) {
		t.Errorf("size = %d, want %d", out.Size, len(content))
	}
	if fu.key != "2i.mp3" || !bytes.Equal(fu.body, content) {
		t.Errorf("uploaded key=%q body=%q", fu.key, fu.body)
	}
}

func TestAudioUploadRejectsBadRequests(t *testing.T) {
	srv, fu := newAudioTestServer(t)
	cases := []struct {
		name, key, filename string
		content             []byte
	}{
		{"missing key", "", "a.mp3", []byte("x")},
		{"traversal key", "../evil.mp3", "a.mp3", []byte("x")},
		{"non-mp3 key", "2i.wav", "a.mp3", []byte("x")},
		{"space in key", "2 i.mp3", "a.mp3", []byte("x")},
		{"missing file", "2i.mp3", "", nil},
		{"empty file", "2i.mp3", "a.mp3", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postAudio(t, srv, tc.key, tc.filename, tc.content)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
	if fu.key != "" {
		t.Errorf("uploader was called with key %q", fu.key)
	}
}

func TestAudioUploadReportsUploaderFailure(t *testing.T) {
	srv, fu := newAudioTestServer(t)
	fu.err = errors.New("wrangler: no bucket")
	resp := postAudio(t, srv, "2i.mp3", "a.mp3", []byte("x"))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "no bucket") {
		t.Errorf("body %s should carry the uploader error", b)
	}
}

func TestWranglerPutArgs(t *testing.T) {
	got := strings.Join(wranglerPutArgs("retrotech", "2i.mp3", "/tmp/x.mp3"), " ")
	want := "r2 object put retrotech/2i.mp3 --file /tmp/x.mp3 --content-type audio/mpeg --remote"
	if got != want {
		t.Errorf("args = %q, want %q", got, want)
	}
}

// rangeHandler serves a fixed-size body honoring one-range requests, like R2.
func rangeHandler(size int64, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			http.Error(w, "nope", status)
			return
		}
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Range", "bytes 0-1/"+strconv.FormatInt(size, 10))
			w.WriteHeader(http.StatusPartialContent)
			w.Write([]byte("ab"))
			return
		}
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		w.Write(bytes.Repeat([]byte("a"), int(size)))
	}
}

func checkVia(t *testing.T, srv *httptest.Server, url string, size int64) audioCheckResult {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"url": url, "size": size})
	resp, err := http.Post(srv.URL+"/_write/api/audio/check", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	var out audioCheckResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAudioCheck(t *testing.T) {
	srv, _ := newAudioTestServer(t)

	t.Run("ok with matching size", func(t *testing.T) {
		target := httptest.NewServer(rangeHandler(12345, 0))
		defer target.Close()
		res := checkVia(t, srv, target.URL+"/2i.mp3", 12345)
		if !res.OK || res.Size != 12345 {
			t.Errorf("result = %+v", res)
		}
	})

	t.Run("ok without expected size", func(t *testing.T) {
		target := httptest.NewServer(rangeHandler(999, 0))
		defer target.Close()
		if res := checkVia(t, srv, target.URL+"/x.mp3", 0); !res.OK {
			t.Errorf("result = %+v", res)
		}
	})

	t.Run("size mismatch fails", func(t *testing.T) {
		target := httptest.NewServer(rangeHandler(100, 0))
		defer target.Close()
		res := checkVia(t, srv, target.URL+"/2i.mp3", 200)
		if res.OK || !strings.Contains(res.Error, "크기 불일치") {
			t.Errorf("result = %+v", res)
		}
	})

	t.Run("404 fails", func(t *testing.T) {
		target := httptest.NewServer(rangeHandler(0, http.StatusNotFound))
		defer target.Close()
		res := checkVia(t, srv, target.URL+"/missing.mp3", 0)
		if res.OK || res.Status != http.StatusNotFound {
			t.Errorf("result = %+v", res)
		}
	})

	t.Run("unreachable host fails", func(t *testing.T) {
		// A closed port: connection refused becomes a result, not a 500.
		target := httptest.NewServer(rangeHandler(1, 0))
		target.Close()
		res := checkVia(t, srv, target.URL+"/x.mp3", 0)
		if res.OK || res.Error == "" {
			t.Errorf("result = %+v", res)
		}
	})

	t.Run("non-http url is a 400", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{"url": "file:///etc/passwd"})
		resp, err := http.Post(srv.URL+"/_write/api/audio/check", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("server ignoring range still checks size", func(t *testing.T) {
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "4")
			w.Write([]byte("abcd")) // 200, no range support
		}))
		defer target.Close()
		res := checkVia(t, srv, target.URL+"/x.mp3", 4)
		if !res.OK || res.Size != 4 {
			t.Errorf("result = %+v", res)
		}
	})
}
