package research

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// A report capability lets its file:// page call the loopback API without
// granting arbitrary websites (or other opaque origins) access to local CLIs.
// It is stable across server restarts and never sent in URLs or logs.
func loadBrowserToken(meta string) (string, error) {
	path := filepath.Join(meta, "browser-token")
	if b, e := os.ReadFile(path); e == nil {
		token := strings.TrimSpace(string(b))
		decoded, e := hex.DecodeString(token)
		if e != nil || len(decoded) != 32 {
			return "", errors.New("보고서 연결 정보가 손상되었습니다")
		}
		return token, nil
	} else if !os.IsNotExist(e) {
		return "", e
	}
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	token := hex.EncodeToString(b)
	if e := atomicWrite(path, []byte(token), 0600); e != nil {
		return "", e
	}
	return token, nil
}

func (s *Server) authorizeBrowser(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	fileAPI := origin == "null" && strings.HasPrefix(r.URL.Path, "/api/")
	if fileAPI {
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Origin", "null")
		if r.Method == http.MethodOptions {
			method := r.Header.Get("Access-Control-Request-Method")
			if method != "GET" && method != "POST" {
				fail(w, 403, errors.New("허용되지 않은 연결 요청입니다"))
				return false
			}
			for _, h := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
				switch strings.ToLower(strings.TrimSpace(h)) {
				case "content-type", "x-retrotech-request", "x-retrotech-token":
				default:
					fail(w, 403, errors.New("허용되지 않은 연결 헤더입니다"))
					return false
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-RetroTech-Request, X-RetroTech-Token")
			if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
				w.Header().Set("Access-Control-Allow-Private-Network", "true")
			}
			w.WriteHeader(http.StatusNoContent)
			return false
		}
		if s.Store.browserToken == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-RetroTech-Token")), []byte(s.Store.browserToken)) != 1 {
			fail(w, 403, errors.New("보고서 연결 정보가 다릅니다. 이 폴더의 HTML을 새로 고쳐 주세요"))
			return false
		}
	} else if origin != "" && origin != s.Store.Origin {
		fail(w, 403, errors.New("보고서 화면에서 요청해 주세요"))
		return false
	}
	if r.Method != "GET" && ((!fileAPI && origin != s.Store.Origin) || r.Header.Get("X-RetroTech-Request") != "1" || r.Header.Get("Content-Type") != "application/json") {
		fail(w, 403, errors.New("보고서 화면에서 요청해 주세요"))
		return false
	}
	return true
}
