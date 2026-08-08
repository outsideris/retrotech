# Tests — RetroTech

## 실행

```bash
go test ./...            # 전체
go test ./internal/...   # 패키지별
go test ./internal/builder/ -run TestBuildFeedMatchesGolden -v
cd desktop && npm test   # 데스크톱 앱(Electron 메인 프로세스) JS 테스트
```

- 러너: Go 표준 `testing`. 외부 테스트 의존성 없음. 데스크톱 JS 는 Node 내장 `node --test`(추가 의존성 없음).
- **CI:** GitHub Actions(`.github/workflows/ci.yml`)가 push(main)/PR 마다 `go vet`·`go test`·`go run ./cmd/build` 를 실행해 피드 회귀와 빌드 깨짐을 자동 검증한다.

## 현황

| 테스트 파일 | 대상 | 검증 범위 |
| --- | --- | --- |
| `internal/parser/parser_test.go` | `parser` | 프론트매터/본문 분리, 폴드(`>`)·따옴표 title 의 trailing newline 보존(피드 패리티 핵심), 블록 스칼라 description, 로드·날짜 내림차순 정렬·`index.*` 제외, **`chapters:` 파싱·`StartSeconds`("MM:SS"/"HH:MM:SS"/60분 초과/유효성)·불량 챕터 로드 거부**(빈 title·잘못된 start → `LoadEpisode` 에러) |
| `internal/builder/feed_test.go` | `feed.go` | **피드 골든**: `BuildFeed` 출력이 `testdata/feed.golden.xml` 과 바이트 동일(휘발성 `lastBuildDate` 정규화)임을 전체 에피소드로 검증. **챕터**: 챕터 선언 시 `xmlns:podcast` + `<podcast:chapters>` + description 타임스탬프 줄 병기, 미선언 시 피드 무변화(네임스페이스·태그 부재). **description HTML**(`DescriptionHTML`): 줄바꿈→`<br/>`·빈 줄→`<p>`·연속 빈 줄 병합·앞뒤 개행 제거·맨 URL→`<a href>`·문장부호는 앵커 밖·`&`/`<`/`>` 이스케이프(URL 쿼리 `&` 포함)·빈 입력→빈 결과, 아이템 description 이 CDATA 안에서 이스케이프되지 않은 HTML 로 나가는지, **저장된 `feedDescription`** 이 있으면 그대로 실리고 평문 description 은 무시되는지, 저장된 HTML 뒤에도 챕터 문단이 덧붙는지 |
| `internal/builder/chapters_test.go` | `chapters.go` | Podcasting 2.0 chapters JSON 생성(버전 "1.2.0", `startTime` 초 변환, title trim), 챕터 없는 에피소드는 nil, 잘못된 start 에러, `ChaptersRelPath` 경로 |
| `internal/builder/badges_test.go` | `badges.go` | 항상 노출되는 Apple/YouTube/Spotify, `google` 유무에 따른 Google↔RSS 토글, 회차 딥링크 사용·`&`→`&amp;` href 이스케이프, 배지별 예약 높이(`height` SVG 비율, `height="0"` 부재 → CLS 방지) |
| `internal/builder/render_test.go` | `render.go` | 홈·episodes 페이지 내비 링크(상호 연결), 에피소드 title·`<!--badges-->` 치환·footer, `## 레퍼런스:` 리스트의 `.refs` 자동 래핑, 한 개의 `role="main"` 랜드마크·skip 링크, **title 규칙**(`<title>`=맨이름·og:title=접미사, 전 페이지 타입) |
| `internal/builder/sitemap_test.go` | `sitemap.go` | sitemap.xml 구조(urlset/xmlns)·홈/episodes/에피소드 URL 포함·404 제외·랜딩 `lastmod`=최신 에피소드 날짜·유효 XML |
| `internal/builder/a11y_perf_test.go` | `render.go`·`badges.go`·`render_layout.go` (전 페이지 타입) | **접근성/성능 불변식**(브라우저 없이 `go test`로): 제목 계층 건너뜀 없음·단일 h1, 모든 `img` `alt`, 단일 `role="main"`+skip 링크, 다크 토글 키보드 조작(role/tabindex/Enter·Space), `html lang`·`title`, iframe `loading="lazy"`·`title`, `height="0"` 부재(CLS), 커버 preload는 home·404 한정·`fetchpriority`, 커버 `width/height` |
| `internal/editor/compose_test.go` | `compose.go`·`form.go` | **에디터 라운드트립 안전망**: `content/episodes/` 23편 전수를 폼으로 변환→재합성→재파싱해 ① 프론트매터 값 동일(`reflect.DeepEqual`) ② 구조화 본문 바이트 동일 ③ 재합성 에피소드의 `BuildFeed` 가 원본과 **바이트 동일**(피드 골든 계약 보호). 추가: 블록 스칼라 chomping(strip/clip/keep) 값 보존, `parseLinkItem` 가역성(괄호 포함 URL·plain 텍스트), 빈 섹션 생략(레퍼런스 없는 회차) |
| `internal/editor/derive_test.go` | `derive.go` | `deriveDescription`: 마크다운 링크/이미지 → 텍스트 축약(괄호 포함 URL), 다문단·trailing newline 규약(1개), 빈/공백 입력 → 빈 값, 멱등성(재파생 무변화). **`deriveFeedDescription`**: description+description2 → 피드 HTML(줄바꿈 `<br/>`·description2 별도 `<p>`·description2 내부 빈 줄 분리·description2 없음·빈/공백 입력 → 빈 값, trailing newline 1개) |
| `internal/editor/store_test.go` | `store.go` | 에피소드 파일 CRUD(생성/조회/수정/삭제/목록), 중복 생성 거부(`ErrExists`)·미존재(`ErrNotFound`), 슬러그 검증(`ValidID`)·unsafe id(path traversal) 거부(`ErrInvalidID`), 수정이 path id 기준(본문 id 무시 → URL/guid 불변), **description 파생**(Create 가 클라이언트 값 무시·intro 에서 링크 제거, Update 는 intro 미변경 시 레거시 description 바이트 보존·변경 시 재파생), **`feedDescription` 저장**(Create 가 파일에 블록 스칼라로 기록·무관한 수정은 보존·intro 변경 시 재생성, 필드 없는 레거시 파일은 무관한 수정으로 필드가 생기지 않음) |
| `internal/editor/editor_test.go` | `editor.go` | HTTP 핸들러 전수(httptest): 목록·생성(201, 파일 기록)·중복(409)·조회(필드 보존)·수정(200)·삭제(204)·재조회(404), 에러 코드(미존재 404·잘못된 슬러그 400·깨진 JSON 400), `POST /preview`(text/html·제목·레퍼런스 링크 렌더), UI/자산 서빙(임베드 index·app.js + `Cache-Control: no-store`, `public/` styles.css, `/`→302), `New` 가 content/episodes 없는 repo 거부, **초안 API 라이프사이클**(create→list→save→get→publish→episode 200·draft 404, id 없는 발행 400), **초안 find-or-create**(`POST /drafts` body `{id}` — 동일 id 초안 존재 시 200 재사용·미존재 시 201 생성, 초안 개수 불변), **HTTP description 파생**(POST/PUT 응답이 파생 값 반영, intro 미변경 + 낡은 클라이언트 description 이 저장값을 오염시키지 않음), **`writeAssistError`**(타임아웃 → 504+한국어 안내·"signal: killed" 은폐, ErrUnavailable → 503, 기타 → 502) |
| `internal/editor/drafts_test.go` | `drafts.go`·`compose.go` | 초안 스토어: 생성(빈 폼·결정적 slug)·조회·저장(자동저장, 미존재 ErrNotFound, **구조화 초안의 description 을 intro 에서 파생**)·**`FindByEpisodeID`**(동일 id 초안 검색·빈/미존재 id 미매치)·목록(빈 디렉터리=빈 목록)·삭제, slug 동초 충돌 `-2` 접미사, 수정시각 내림차순 정렬, **발행**(episode 기록·**발행 날짜 스탬프**·초안 삭제·필드 보존·description 파생)·잘못된(빈)/중복 id 거부(초안 생존), 거의-빈 폼의 `ComposeFile` 가 유효·재파싱 가능 |
| `internal/editor/audio_test.go` | `audio.go` | **mp3 R2 업로드·enclosure 검증**: `POST /audio/upload`(fake 업로더 주입 — 키·바이트 전달 검증, 공개 URL 응답, 잘못된 키(traversal·비 mp3·공백)/파일 누락/빈 파일 400, 업로더 실패 502+원인), `wranglerPutArgs`(bucket/key/`--remote`/content-type 플래그), `POST /audio/check`(httptest 대상 서버 — Range 206 크기 일치 OK, 크기 미지정 OK, **크기 불일치 실패**, 404 실패, 연결 불가 실패(200+결과, 500 아님), 비 http(s) URL 400, Range 미지원 200 서버도 크기 검증). (실제 wrangler exec 는 환경 의존이라 미검증 — 수동 E2E 로 확인) |
| `internal/editor/assist/assist_test.go` | `assist.go` | AI CLI 제공자: `Providers()`/`Find`(claude/codex/gemini), claude envelope 파싱(텍스트+**텔레메트리**: duration/cost/tokens·is_error·raw fallback·빈), codex JSONL 파싱(agent_message + usage 토큰·비JSON 라인 무시·error 이벤트), `resolveBinary`(PATH 이름 후보 + fallback 경로). (실제 CLI exec·effort/model 플래그는 환경 의존이라 미검증) |
| `internal/editor/assist/analyze_test.go` | `analyze.go` | 대본 분석 응답 파싱: `parseScriptMeta`(순수 JSON·```json 펜스+prose 견딤·공백 trim·비JSON 오류·title/description 모두 빈 오류·references 파싱), **`normalizeScriptTitle`**(제목 앞 "Episode" 라벨 제거 — 대소문자·구분자(`:`/`-`) 동반 제거, "Episodes…"·중간의 "Episode"·라벨뿐인 제목·빈 문자열은 불변, `parseScriptMeta` 경유 확인), `extractJSONObject`(첫 `{`~마지막 `}` 추출·패스스루), **`reconcileRefs`**(추출 링크 전부·문서 순서 보장, 모델 누락 → 앵커/URL 폴백, 목록 밖 URL 폐기), **`analyzePrompt`**(모든 추출 URL·제목 규칙 포함, 링크 없으면 references 미요청) |
| `internal/editor/assist/links_test.go` | `links.go` | `ExtractLinks`: 마크다운 링크+맨 URL 혼합 문서 순서, URL 중복 제거(첫 등장 우선), 이미지(`![]()`) 제외, 괄호 포함 URL(Wikipedia), 문장부호 트림, 링크 없는 대본 → 빈 목록, **`utm_source=chatgpt.com` 제거**(단독/앞/뒤 파라미터·프래그먼트 유지·다른 utm 보존·클린 URL 과 변형 dedupe) |
| `cmd/app/shellpath_test.go` | `shellpath.go` | `injectLoginPath`: 로그인 셸 PATH 채택(셸 exec 스텁), 실패 시 기존 PATH 보존 |
| `desktop/restart-policy.test.js` | `desktop/restart-policy.js` | 사이드카 자동 재시작의 크래시 루프 가드(순수 모듈·클록 주입): 안정 사망은 항상 재시작, 연속 quick failure 3회 초과 시 포기, 안정 구동(≥10s) 시 카운터 리셋, spawn 즉시 실패도 계수(무한 재시작 방지). 실행: `cd desktop && npm test` |

- 피드 골든(`testdata/feed.golden.xml`)은 마이그레이션 전 `gen-rss.js` 출력에서 운영 기준(pubDate 09:00 UTC)으로 고정해 커밋했다. **의도된** 피드 변경 시 이 파일을 갱신한다. 구독자 계약(guid/enclosure/pubDate)을 지키는 회귀 가드다. (2026-08-08: 아이템 `<description>` 을 HTML 로 내보내면서 골든을 갱신했다 — `gen-rss.js` 대비 divergence 는 description 뿐이고 나머지는 그대로다.)
- **새 에피소드 추가 등 의도된 피드 변경 시 골든 갱신 절차:** `go run ./cmd/build` 후 `cp dist/feed.xml internal/builder/testdata/feed.golden.xml`. 커밋 전에 `git diff` 로 새 `<item>` 추가(및 `lastBuildDate`) 외에 기존 항목의 guid/enclosure/pubDate 가 바뀌지 않았는지 확인하고, 에피소드 md 와 골든을 **같은 커밋**에 포함한다(따로 커밋하면 CI 의 `TestBuildFeedMatchesGolden` 이 실패한다).
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
