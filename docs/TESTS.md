# Tests — RetroTech

## 실행

```bash
go test ./...            # 전체
go test ./internal/...   # 패키지별
go test ./internal/builder/ -run TestBuildFeedMatchesGolden -v
```

- 러너: Go 표준 `testing`. 외부 테스트 의존성 없음.
- **CI:** GitHub Actions(`.github/workflows/ci.yml`)가 push(main)/PR 마다 `go vet`·`go test`·`go run ./cmd/build` 를 실행해 피드 회귀와 빌드 깨짐을 자동 검증한다.

## 현황

| 테스트 파일 | 대상 | 검증 범위 |
| --- | --- | --- |
| `internal/parser/parser_test.go` | `parser` | 프론트매터/본문 분리, 폴드(`>`)·따옴표 title 의 trailing newline 보존(피드 패리티 핵심), 블록 스칼라 description, 로드·날짜 내림차순 정렬·`index.*` 제외 |
| `internal/builder/feed_test.go` | `feed.go` | **피드 골든**: `BuildFeed` 출력이 이전 `gen-rss.js` 산출물(`testdata/feed.golden.xml`)과 바이트 동일(휘발성 `lastBuildDate` 정규화)임을 23편 전체로 검증 |
| `internal/builder/badges_test.go` | `badges.go` | 항상 노출되는 Apple/YouTube/Spotify, `google` 유무에 따른 Google↔RSS 토글, 회차 딥링크 사용·`&`→`&amp;` href 이스케이프, 배지별 예약 높이(`height` SVG 비율, `height="0"` 부재 → CLS 방지) |
| `internal/builder/render_test.go` | `render.go` | 홈·episodes 페이지 내비 링크(상호 연결), 에피소드 title·`<!--badges-->` 치환·footer, `## 레퍼런스:` 리스트의 `.refs` 자동 래핑, 한 개의 `role="main"` 랜드마크·skip 링크, **title 규칙**(`<title>`=맨이름·og:title=접미사, 전 페이지 타입) |
| `internal/builder/sitemap_test.go` | `sitemap.go` | sitemap.xml 구조(urlset/xmlns)·홈/episodes/에피소드 URL 포함·404 제외·랜딩 `lastmod`=최신 에피소드 날짜·유효 XML |
| `internal/builder/a11y_perf_test.go` | `render.go`·`badges.go`·`render_layout.go` (전 페이지 타입) | **접근성/성능 불변식**(브라우저 없이 `go test`로): 제목 계층 건너뜀 없음·단일 h1, 모든 `img` `alt`, 단일 `role="main"`+skip 링크, 다크 토글 키보드 조작(role/tabindex/Enter·Space), `html lang`·`title`, iframe `loading="lazy"`·`title`, `height="0"` 부재(CLS), 커버 preload는 home·404 한정·`fetchpriority`, 커버 `width/height` |
| `internal/editor/compose_test.go` | `compose.go`·`form.go` | **에디터 라운드트립 안전망**: `content/episodes/` 23편 전수를 폼으로 변환→재합성→재파싱해 ① 프론트매터 값 동일(`reflect.DeepEqual`) ② 구조화 본문 바이트 동일 ③ 재합성 에피소드의 `BuildFeed` 가 원본과 **바이트 동일**(피드 골든 계약 보호). 추가: 블록 스칼라 chomping(strip/clip/keep) 값 보존, `parseLinkItem` 가역성(괄호 포함 URL·plain 텍스트), 빈 섹션 생략(레퍼런스 없는 회차) |
| `internal/editor/store_test.go` | `store.go` | 에피소드 파일 CRUD(생성/조회/수정/삭제/목록), 중복 생성 거부(`ErrExists`)·미존재(`ErrNotFound`), 슬러그 검증(`ValidID`)·unsafe id(path traversal) 거부(`ErrInvalidID`), 수정이 path id 기준(본문 id 무시 → URL/guid 불변) |
| `internal/editor/editor_test.go` | `editor.go` | HTTP 핸들러 전수(httptest): 목록·생성(201, 파일 기록)·중복(409)·조회(필드 보존)·수정(200)·삭제(204)·재조회(404), 에러 코드(미존재 404·잘못된 슬러그 400·깨진 JSON 400), `POST /preview`(text/html·제목·레퍼런스 링크 렌더), UI/자산 서빙(임베드 index·app.js, `public/` styles.css, `/`→302), `New` 가 content/episodes 없는 repo 거부 |

- 피드 골든(`testdata/feed.golden.xml`)은 마이그레이션 전 `gen-rss.js` 출력에서 운영 기준(pubDate 09:00 UTC)으로 고정해 커밋했다. **의도된** 피드 변경 시 이 파일을 갱신한다. 구독자 계약(guid/enclosure/pubDate)을 지키는 회귀 가드다.
- 피드 테스트는 `content/episodes/` 의 프론트매터만 로드해 빌드한다(본문 불필요). `episodeSourceDir` 상수로 경로 지정.
- 외부 의존성(goldmark·yaml) 자체는 테스트하지 않고, 우리 코드의 입출력·계약만 검증한다(CLAUDE.md 규칙).

## 접근성/성능 검증 (2층)

`go test` 의 단위 테스트 외에, 접근성·성능은 두 층으로 CI에서 검증한다([PERFORMANCE.md](./PERFORMANCE.md) 참고).

1. **Go 마크업 불변식**(`a11y_perf_test.go`) — 위 표 참고. 무의존성·결정적, 기존 `test` 잡에 포함.
2. **Lighthouse CI**(`.lighthouserc.json`, `lighthouse` 잡) — 실제 브라우저 감사. 접근성/SEO/Best-Practices=100 하드 게이트. 로컬 실행: `npx @lhci/cli@0.14.x autorun`(`cmd/serve` 를 자동 기동, Node+Chrome 필요).

## 미검증 영역

- 페이지 HTML 의 시각 렌더는 **스크린샷 비교**(참고 빌드 `_ref_dist` 대비)로 수동 검증했다. 자동 스냅샷은 없음.
- `cmd/build` 의 파일 쓰기·정적 복사·CSS 핑거프린트 경로 자체(빌드 성공으로 간접 확인).
- 운영 호스트 동작(압축/캐시/HTTPS) — 별도 확인.

## 후보 (향후)

- `render.go` 의 프로즈 후처리(외부 링크·heading anchor·badges 마커 치환) 단위 테스트.
- 페이지 생성 골든(주요 페이지 HTML 스냅샷) — 단, 의도적 마크업 변경 시 갱신 부담 고려.
