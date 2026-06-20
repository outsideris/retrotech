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
| `internal/builder/render_test.go` | `render.go` | 홈·episodes 페이지 내비 링크(상호 연결), 에피소드 title·`<!--badges-->` 치환·footer, `## 레퍼런스:` 리스트의 `.refs` 자동 래핑, 한 개의 `role="main"` 랜드마크·skip 링크 |
| `internal/builder/sitemap_test.go` | `sitemap.go` | sitemap.xml 구조(urlset/xmlns)·홈/episodes/에피소드 URL 포함·404 제외·랜딩 `lastmod`=최신 에피소드 날짜·유효 XML |
| `internal/builder/a11y_perf_test.go` | `render.go`·`badges.go`·`render_layout.go` (전 페이지 타입) | **접근성/성능 불변식**(브라우저 없이 `go test`로): 제목 계층 건너뜀 없음·단일 h1, 모든 `img` `alt`, 단일 `role="main"`+skip 링크, 다크 토글 키보드 조작(role/tabindex/Enter·Space), `html lang`·`title`, iframe `loading="lazy"`·`title`, `height="0"` 부재(CLS), 커버 preload는 home·404 한정·`fetchpriority`, 커버 `width/height` |

- 피드 골든(`testdata/feed.golden.xml`)은 마이그레이션 전 `gen-rss.js` 출력에서 운영 기준(pubDate 09:00 UTC)으로 고정해 커밋했다. **의도된** 피드 변경 시 이 파일을 갱신한다. 구독자 계약(guid/enclosure/pubDate)을 지키는 회귀 가드다.
- 피드 테스트는 `content/episodes/` 의 프론트매터만 로드해 빌드한다(본문 불필요). `episodeSourceDir` 상수로 경로 지정.
- 외부 의존성(goldmark·yaml) 자체는 테스트하지 않고, 우리 코드의 입출력·계약만 검증한다(CLAUDE.md 규칙).

## 미검증 영역

- 페이지 HTML 의 시각 렌더는 **스크린샷 비교**(참고 빌드 `_ref_dist` 대비)로 수동 검증했다. 자동 스냅샷은 없음.
- `cmd/build` 의 파일 쓰기·정적 복사·CSS 핑거프린트 경로 자체(빌드 성공으로 간접 확인).
- 운영 호스트 동작(압축/캐시/HTTPS) — 별도 확인.

## 후보 (향후)

- `render.go` 의 프로즈 후처리(외부 링크·heading anchor·badges 마커 치환) 단위 테스트.
- 페이지 생성 골든(주요 페이지 HTML 스냅샷) — 단, 의도적 마크업 변경 시 갱신 부담 고려.
