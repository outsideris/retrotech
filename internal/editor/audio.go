// Audio hosting support: uploading an episode mp3 to the Cloudflare R2 bucket
// that serves retrotech-episodes.outsider.dev, and verifying before publish
// that the enclosure URL actually downloads (so a feed never ships a dead or
// half-uploaded mp3).
//
// The upload shells out to the wrangler CLI (directly, or through npx when only
// npx is installed), which authenticates itself via `wrangler login` — the app
// owns no Cloudflare credentials, mirroring how the assist CLIs work.
package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// audioBucket is the R2 bucket behind retrotech-episodes.outsider.dev; its
	// object keys are the public paths (key "2i.mp3" → /2i.mp3).
	audioBucket     = "retrotech"
	episodesBaseURL = "https://retrotech-episodes.outsider.dev/"

	// maxAudioUpload bounds the request body; episodes run tens of MB, so 1 GiB
	// is comfortably above any real mp3 while still refusing runaway requests.
	maxAudioUpload = 1 << 30
	// audioUploadTimeout covers pushing tens of MB through a home uplink plus
	// wrangler's own startup (npx may fetch it on first run).
	audioUploadTimeout = 15 * time.Minute
	audioCheckTimeout  = 30 * time.Second
)

// audioKeyRe accepts the object keys the editor derives from an episode id
// (<id>.mp3) and rejects anything that could escape the bucket root or need
// quoting (slashes, spaces, "..").
var audioKeyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.mp3$`)

// wranglerPutArgs builds the wrangler subcommand that writes one object.
// --remote targets the real bucket (wrangler defaults to its local simulator),
// and the content type makes R2 serve the file as audio rather than a download.
func wranglerPutArgs(bucket, key, file string) []string {
	return []string{"r2", "object", "put", bucket + "/" + key,
		"--file", file, "--content-type", "audio/mpeg", "--remote"}
}

// wranglerCommand resolves how to run wrangler: the binary itself when
// installed, else through npx (which fetches/caches it). ok is false when
// neither exists.
func wranglerCommand() (bin string, prefix []string, ok bool) {
	if p, err := exec.LookPath("wrangler"); err == nil {
		return p, nil, true
	}
	if h, err := os.UserHomeDir(); err == nil {
		f := filepath.Join(h, ".local", "bin", "wrangler")
		if info, err := os.Stat(f); err == nil && !info.IsDir() {
			return f, nil, true
		}
	}
	if p, err := exec.LookPath("npx"); err == nil {
		return p, []string{"-y", "wrangler"}, true
	}
	return "", nil, false
}

// wranglerPut uploads file to the audio bucket under key via the wrangler CLI.
func wranglerPut(ctx context.Context, key, file string) error {
	bin, prefix, ok := wranglerCommand()
	if !ok {
		return errors.New("wrangler CLI 를 찾을 수 없습니다 — wrangler 또는 npx(Node.js) 설치가 필요합니다")
	}
	args := append(append([]string{}, prefix...), wranglerPutArgs(audioBucket, key, file)...)
	cmd := exec.CommandContext(ctx, bin, args...)
	// wrangler drops a .wrangler/ cache dir in its cwd — keep that out of the
	// repo the sidecar usually runs from.
	cmd.Dir = os.TempDir()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if _, err := cmd.Output(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("R2 업로드가 제한 시간(%.0f분)을 넘어 중단되었습니다", audioUploadTimeout.Minutes())
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("wrangler: %s", lastLines(msg, 5))
		}
		return fmt.Errorf("wrangler: %v", err)
	}
	return nil
}

// lastLines keeps the tail of a multi-line CLI error (wrangler prints banners
// before the actual failure).
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// handleAudioUpload receives an mp3 (multipart fields: key, file) and uploads
// it to the R2 bucket, responding with the public URL. The key comes from the
// episode id (<id>.mp3) so the object lands where the derived enclosure URL
// already points.
func (e *Editor) handleAudioUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAudioUpload)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "업로드 요청을 읽지 못했습니다: "+err.Error())
		return
	}
	key := strings.TrimSpace(r.FormValue("key"))
	if !audioKeyRe.MatchString(key) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("잘못된 파일 키 %q — <ID>.mp3 형식이어야 합니다", key))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "업로드할 파일이 없습니다")
		return
	}
	defer file.Close()

	// Spool to a temp file wrangler can read (its input is a path, not a stream).
	tmp, err := os.CreateTemp("", "retrotech-upload-*.mp3")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.Remove(tmp.Name())
	size, err := io.Copy(tmp, file)
	closeErr := tmp.Close()
	if err != nil || closeErr != nil {
		writeError(w, http.StatusInternalServerError, "임시 파일 저장에 실패했습니다")
		return
	}
	if size == 0 {
		writeError(w, http.StatusBadRequest, "빈 파일은 업로드할 수 없습니다")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), audioUploadTimeout)
	defer cancel()
	if err := e.putObject(ctx, key, tmp.Name()); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":  episodesBaseURL + key,
		"key":  key,
		"size": size,
	})
}

// audioCheckResult is the outcome of verifying an enclosure URL. OK is false
// both when the download fails and when the size on the server disagrees with
// the form (a truncated or stale upload); Error carries the reason to show.
type audioCheckResult struct {
	OK     bool   `json:"ok"`
	Status int    `json:"status,omitempty"`
	Size   int64  `json:"size,omitempty"`
	Error  string `json:"error,omitempty"`
}

// checkEnclosure fetches the first bytes of url (a ranged GET, so a huge mp3
// costs almost nothing) and compares the server-reported size to wantSize
// (0 = don't compare). Any failure is a result, not a Go error, so the handler
// always answers 200 and the UI branches on ok.
func checkEnclosure(ctx context.Context, client *http.Client, url string, wantSize int64) audioCheckResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return audioCheckResult{Error: "잘못된 URL: " + err.Error()}
	}
	req.Header.Set("Range", "bytes=0-1")
	resp, err := client.Do(req)
	if err != nil {
		return audioCheckResult{Error: "다운로드 요청 실패: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return audioCheckResult{Status: resp.StatusCode, Error: fmt.Sprintf("HTTP %d 응답 — 파일이 아직 업로드되지 않았을 수 있습니다", resp.StatusCode)}
	}
	// Actually read the body: a 200/206 whose body errors out is still a bad
	// download.
	if _, err := io.ReadAll(io.LimitReader(resp.Body, 2)); err != nil {
		return audioCheckResult{Status: resp.StatusCode, Error: "본문을 읽지 못했습니다: " + err.Error()}
	}
	size := enclosureSize(resp)
	if wantSize > 0 && size > 0 && size != wantSize {
		return audioCheckResult{
			Status: resp.StatusCode,
			Size:   size,
			Error:  fmt.Sprintf("파일 크기 불일치 — 서버 %d bytes, 폼 %d bytes (업로드가 누락/미완료일 수 있습니다)", size, wantSize),
		}
	}
	return audioCheckResult{OK: true, Status: resp.StatusCode, Size: size}
}

// enclosureSize extracts the full file size from a ranged (Content-Range:
// bytes 0-1/N) or plain (Content-Length) response; 0 when unknown.
func enclosureSize(resp *http.Response) int64 {
	if resp.StatusCode == http.StatusPartialContent {
		cr := resp.Header.Get("Content-Range")
		if i := strings.LastIndex(cr, "/"); i >= 0 {
			if n, err := strconv.ParseInt(strings.TrimSpace(cr[i+1:]), 10, 64); err == nil {
				return n
			}
		}
		return 0
	}
	if resp.ContentLength > 0 {
		return resp.ContentLength
	}
	return 0
}

// handleAudioCheck verifies that an enclosure URL downloads (and optionally
// that its size matches), for the pre-publish warning. The response is always
// 200 with an audioCheckResult; only a malformed request is an HTTP error.
func (e *Editor) handleAudioCheck(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL  string `json:"url"`
		Size int64  `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	url := strings.TrimSpace(req.URL)
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		writeError(w, http.StatusBadRequest, "확인할 URL 이 올바르지 않습니다: "+url)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), audioCheckTimeout)
	defer cancel()
	writeJSON(w, http.StatusOK, checkEnclosure(ctx, e.audioClient, url, req.Size))
}
