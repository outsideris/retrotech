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
| `internal/editor/compose_test.go` | `compose.go`·`form.go` | **에디터 라운드트립 안전망**: `content/episodes/` 23편 전수를 폼으로 변환→재합성→재파싱해 ① 프론트매터 값 동일(`reflect.DeepEqual`) ② 구조화 본문 바이트 동일 ③ 재합성 에피소드의 `BuildFeed` 가 원본과 **바이트 동일**(피드 골든 계약 보호). 추가: 블록 스칼라 chomping(strip/clip/keep) 값 보존, `parseLinkItem` 가역성(괄호 포함 URL·plain 텍스트), 빈 섹션 생략(레퍼런스 없는 회차), **챕터 라운드트립**(`TestChaptersSurviveFrontmatterRoundTrip`): 합성한 `chapters:` 가 값 동일하게 재파싱되는지 — bare `"02:41"` 의 sexagesimal 해석(→161) 방지용 강제 따옴표, 비ASCII 시작·`": "`·`#`·따옴표 포함 제목, `HH:MM:SS`, 예약어(`true`) 제목, `StartSeconds()` 초 변환. 챕터 없는 회차가 빈 `chapters:` 키를 얻지 않는지(`TestComposeOmitsChaptersWhenEmpty`) |
| `internal/editor/derive_test.go` | `derive.go` | `deriveDescription`: 마크다운 링크/이미지 → 텍스트 축약(괄호 포함 URL), 다문단·trailing newline 규약(1개), 빈/공백 입력 → 빈 값, 멱등성(재파생 무변화). **`deriveFeedDescription`**: description+description2 → 피드 HTML(줄바꿈 `<br/>`·description2 별도 `<p>`·description2 내부 빈 줄 분리·description2 없음·빈/공백 입력 → 빈 값, trailing newline 1개) |
| `internal/editor/store_test.go` | `store.go` | 에피소드 파일 CRUD(생성/조회/수정/삭제/목록), 중복 생성 거부(`ErrExists`)·미존재(`ErrNotFound`), 슬러그 검증(`ValidID`)·unsafe id(path traversal) 거부(`ErrInvalidID`), 수정이 path id 기준(본문 id 무시 → URL/guid 불변), **description 파생**(Create 가 클라이언트 값 무시·intro 에서 링크 제거, Update 는 intro 미변경 시 레거시 description 바이트 보존·변경 시 재파생), **`feedDescription` 저장**(Create 가 파일에 블록 스칼라로 기록·무관한 수정은 보존·intro 변경 시 재생성, 필드 없는 레거시 파일은 무관한 수정으로 필드가 생기지 않음) |
| `internal/editor/editor_test.go` | `editor.go` | HTTP 핸들러 전수(httptest): 목록·생성(201, 파일 기록)·중복(409)·조회(필드 보존)·수정(200)·삭제(204)·재조회(404), 에러 코드(미존재 404·잘못된 슬러그 400·깨진 JSON 400), `POST /preview`(text/html·제목·레퍼런스 링크 렌더), UI/자산 서빙(임베드 index·app.js + `Cache-Control: no-store`, `public/` styles.css, `/`→302), `New` 가 content/episodes 없는 repo 거부, **초안 API 라이프사이클**(create→list→save→get→publish→episode 200·draft 404, id 없는 발행 400), **초안 find-or-create**(`POST /drafts` body `{id}` — 동일 id 초안 존재 시 200 재사용·미존재 시 201 생성, 초안 개수 불변), **HTTP description 파생**(POST/PUT 응답이 파생 값 반영, intro 미변경 + 낡은 클라이언트 description 이 저장값을 오염시키지 않음), **`writeAssistError`**(타임아웃 → 504+한국어 안내·"signal: killed" 은폐, ErrUnavailable → 503, 기타 → 502), **`GET /assist/providers` 페이로드**(provider 마다 `models[{id,label,efforts}]`, claude/codex 는 비어 있지 않고 id·label 필수·effort 를 가진 모델이 최소 하나, 노브 없는 gemini 는 `null` 아닌 `[]` 로 직렬화, **`requiresModel`** 은 claude 만 true·codex/gemini false), **assist run/analyze 의 모델·effort 검증**(카탈로그 밖 모델·모델이 못 받는 effort·**claude 의 빈 모델·Fable(`claude-fable-5-1`)** → CLI 설치 여부와 무관하게 400. PATH 를 가짜 CLI 로 격리해 **어떤 CLI 도 실행되지 않았음**을 함께 확인 — 검증이 빠지면 실제 CLI 에 돈이 나가고, Fable 은 묻지 않고 사용 크레딧으로 청구될 수 있다. 빈 프롬프트·빈 대본 케이스는 모델을 채워 그 사유만 검사한다), **effort 노브 id 계약**(`index.html` 의 `id="assist-effort-knob"` 과 `app.js` 의 조회가 함께 존재 — 한쪽만 바뀌면 사이드바가 로드 시 죽는다), **`requiresModel` 필드 계약**(`app.js` 가 `p.requiresModel` 을 읽는다 — 한쪽만 이름이 바뀌면 claude 목록에 서버가 매번 거부하는 `(기본)` 이 돌아온다) |
| `internal/editor/drafts_test.go` | `drafts.go`·`compose.go` | 초안 스토어: 생성(빈 폼·결정적 slug)·조회·저장(자동저장, 미존재 ErrNotFound, **구조화 초안의 description 을 intro 에서 파생**)·**`FindByEpisodeID`**(동일 id 초안 검색·빈/미존재 id 미매치)·목록(빈 디렉터리=빈 목록)·삭제, slug 동초 충돌 `-2` 접미사, 수정시각 내림차순 정렬, **발행**(episode 기록·**발행 날짜 스탬프**·초안 삭제·필드 보존·description 파생)·잘못된(빈)/중복 id 거부(초안 생존), 거의-빈 폼의 `ComposeFile` 가 유효·재파싱 가능 |
| `internal/editor/audio_test.go` | `audio.go` | **mp3 R2 업로드·enclosure 검증**: `POST /audio/upload`(fake 업로더 주입 — 키·바이트 전달 검증, 공개 URL 응답, 잘못된 키(traversal·비 mp3·공백)/파일 누락/빈 파일 400, 업로더 실패 502+원인), `wranglerPutArgs`(bucket/key/`--remote`/content-type 플래그), `POST /audio/check`(httptest 대상 서버 — Range 206 크기 일치 OK, 크기 미지정 OK, **크기 불일치 실패**, 404 실패, 연결 불가 실패(200+결과, 500 아님), 비 http(s) URL 400, Range 미지원 200 서버도 크기 검증). (실제 wrangler exec 는 환경 의존이라 미검증 — 수동 E2E 로 확인) |
| `internal/editor/assist/assist_test.go` | `assist.go` | AI CLI 제공자: `Providers()`/`Find`(claude/codex/gemini), claude envelope 파싱(텍스트+**텔레메트리**: duration/cost/tokens·is_error·raw fallback·빈), codex JSONL 파싱(agent_message + usage 토큰·비JSON 라인 무시·error 이벤트), `resolveBinary`(PATH 이름 후보 + fallback 경로). (모델/effort 검증은 `catalog_test.go` 로 분리. 실제 CLI exec 은 환경 의존이라 미검증) |
| `internal/editor/assist/catalog_test.go` | `catalog.go` | **모델 카탈로그와 검증**: 항목 형식(빈 id/label 금지 — `""` 는 UI 의 "(기본)" 자리, id·label 중복 금지, effort 는 알려진 어휘 low/medium/high/xhigh/max/ultra 안에서 중복 없이), **현재 모델 목록 고정**(claude = Opus 5.5·Sonnet 5.5·Haiku 5.5, codex = GPT-6.1 Sol·GPT-6 Astra·GPT-6.1 Terra·GPT-6 Sol·Luna·GPT-5.6 Sol·Terra·Luna, gemini 없음 — CLI 갱신 시 같이 바뀌어야 하는 의도적 핀), **Fable 제외 고정**(`TestCatalogLeavesOutFable`: 어느 제공자도 id·label 에 fable 이 든 모델이나 `best` 별칭을 내놓지 않는다 — 목록 핀은 CLI 갱신마다 바뀌므로, 갱신하면서 Fable 이 슬그머니 돌아오는 것을 따로 막는다), **모델 명시 필수 제공자 고정**(`TestOnlyClaudeRequiresAModel`: `RequiresModel` 이 claude 만 true), `Validate`(claude 는 빈 모델을 effort 와 무관하게 거부, codex 기본 모델은 provider 합집합 기준 — ultra 수용·어휘 밖 `minimal` 거부, 모델별 상한 초과 거부(Sonnet 5.5 에 ultra, GPT-6 Luna·GPT-5.6 Luna 에 ultra), 상한 자체는 수용(Sonnet 5.5·Haiku 5.5 에 max, Astra·GPT-6.1 Sol 에 ultra), effort 안 받는 모델에 effort 거부(2026-10-13 부터 카탈로그에 그런 모델이 없어 `TestValidateOptionsRefusesALevelOnAModelThatTakesNone` 이 가상 목록으로 확인한다), **Fable(`claude-fable-5-1`)·별칭 `fable`·`best` 거부**, 은퇴한 별칭/모델(`opus`·`gpt-5-codex`)·이전 세대(`claude-opus-5`·`claude-haiku-4-5-20251001`)·은퇴(`gpt-5.5`, 2026-10-14)·은퇴한 effort(`minimal`)·숨김 모델(`gpt-reserve`) 거부, 오류가 provider 이름으로 시작), **거부가 CLI 실행 전에 일어남**(Fable 과 claude 의 빈 모델 포함. PATH 를 가짜 CLI 만 있는 디렉터리로 바꾸고, 실행되면 남는 흔적이 없음을 확인 — 흔적은 셸 리다이렉션으로 남긴다. PATH 를 바꾼 뒤에는 `touch` 같은 외부 명령이 해석되지 않아 가드가 헛돌기 때문). 이 가드는 "거부가 exec 보다 먼저"만 증명하며, 실제 CLI 호출 자체는 미검증이다 |
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
- **Assist 사이드바 JS**(`app.js` 의 `renderTuning`/`renderEfforts`/`unionEfforts`/`fillSelect`): 모듈로 분리되어 있지 않아 단위 테스트가 없다. 모델↔effort 연동은 실행 중인 앱을 브라우저로 직접 확인한다(2026-09-20 확인: 모델별 effort 목록, effort 없는 모델의 노브 숨김, gemini 의 줄 숨김, 저장값 복원/폐기. 2026-09-30 카탈로그 갱신 후 재확인: 새 모델 목록, GPT-6 Luna 는 max 까지·Astra 는 ultra 까지, Haiku 4.5 의 노브 숨김. 같은 날 Fable 가드 후 재확인: claude 목록은 Opus 5.5·Sonnet 5.5·Haiku 4.5 이고 `(기본)` 이 없다, 저장값이 Fable·`(기본)`·없음이면 Opus 5.5 로, Sonnet 5.5 면 그대로, codex 는 `(기본)` 유지, 콘솔 오류 없음. 2026-10-13 카탈로그 갱신 후 재확인: claude 목록은 Opus 5.5·Sonnet 5.5·Haiku 5.5 이고 Haiku 5.5 도 effort 노브(low…max)가 보인다, 저장값이 Haiku 4.5 면 Opus 5.5 로, codex 목록 맨 위에 GPT-6.1 Sol(ultra 까지), 콘솔 오류 없음. 이날부터 effort 를 안 받는 모델이 카탈로그에 없어 **노브 숨김은 실행 중인 앱으로 확인할 수 없다** — 그런 모델이 다시 생기면 그때 확인한다. 2026-10-29 재확인: codex 목록의 GPT-6 Astra 다음에 GPT-6.1 Terra(ultra 까지)). Go 쪽은 DOM id 계약과 `requiresModel` 필드 계약만 지킨다.
- **실제 CLI 호출:** 카탈로그의 모델이 설치된 CLI 로 실제로 도는지는 단위 테스트가 아니라 실행 중인 앱의 `POST /_write/api/assist/run` 으로 모델마다 짧게 호출해 확인한다([QUALITY_GATE.md](./QUALITY_GATE.md) 마지막 검토). CLI 는 모델이 못 받는 effort 를 거절하지 않고 그대로 실행하므로(claude 는 effort 를 안 받던 Haiku 4.5 의 effort 를 조용히 버렸고, codex 는 GPT-6 Luna 에 ultra 를 넘겨도 오류가 없다) 모델별 상한은 호출로는 검증되지 않는다 — 원본은 CLI 모델 레지스트리와 codex 계정 카탈로그다.

## 후보 (향후)

- `render.go` 의 프로즈 후처리(외부 링크·heading anchor·badges 마커 치환) 단위 테스트.
- 페이지 생성 골든(주요 페이지 HTML 스냅샷) — 단, 의도적 마크업 변경 시 갱신 부담 고려.

## 로컬 조사 리더 (2026-09-16)

- `internal/research/research_test.go`: 문단 위치별 HTML/Markdown 삽입, 원문 보존, 정정·이스케이프, 숨김/복원/재시작, 외부 변경 충돌, 상태 저장 실패의 출력 복구, 결과 스키마·출처 검증, HTTP Host/Origin/경로 제한, 비동기 완료·취소·동시 요청 제한, 명시적 스킬 반영·중복 방지. **메뉴↔서버 일치**(`TestModelMenuIsWhatTheServerAccepts`): 임베드된 `panel.html`의 모델·깊이 메뉴를 읽고, Assist 카탈로그의 모든 모델과 모든 effort(low…ultra)를 `POST /api/jobs`로 보낸다. 메뉴에 있는 조합만 202로 받고 나머지는 사유별 400("조사 effort를 확인해 주세요"/"지원하지 않는 조사 모델입니다")이다. CLI에 넘어간 조합이 메뉴의 조합과 정확히 같은지, 메뉴의 모델이 모두 카탈로그에 있는지, 메뉴 기본값이 GPT-6.1 Sol/high인지도 확인한다. **목록에서 빠진 모델의 기록**(`TestRetiredModelHistoryStaysUsable`): `gpt-6-sol`로 저장된 완료 조사가 재시작 후에도 열리고 그 모델명으로 렌더되며, 그 조사에 대한 후속 질문은 새 모델로 받고 빠진 모델로는 거부한다. **스킬 없음**(`TestMissingSkillStopsTheReader`, 2026-10-31): 스킬 폴더가 없거나, 링크가 끊겼거나, 폴더에 `SKILL.md`가 없거나, `SKILL.md`가 폴더이면 `NewStore`가 `ErrSkillMissing`과 그 경로로 멈추고, 스킬 폴더를 가리키는 링크는 받는다. 확인을 끄는 변이와 일반 파일 확인만 끄는 변이를 모두 잡는다.
- `internal/editor/assist/research_test.go`: **조사 목록 고정**(의도적 핀: codex = GPT-6.1 Sol·Astra, claude = Sonnet 5.5·Opus 5.5, effort = medium/high/xhigh. 목록을 바꾸는 것은 [RESEARCH_WORKBENCH.md](RESEARCH_WORKBENCH.md#모델과-로그인)의 큐레이션 결정이다)과 각 조합이 Assist 카탈로그의 `Validate`를 통과하는지 확인한다. 조합마다 CLI 인자(`--model`, codex `model_reasoning_effort`·읽기 전용 샌드박스·실시간 웹 검색·하위 에이전트 끔, claude `--effort`·safe mode·WebSearch/WebFetch만)를 검사한다. 사유별 거부도 확인한다 — Luna·GPT-6.1 Terra·Fable 5.1·Haiku 5.5·이전 세대(GPT-6 Sol 포함)·다른 CLI의 모델·기본 모델(`""`)은 `ErrResearchModel`, gemini는 `ErrResearchProvider`, low(CLI 기본 깊이가 low인 GPT-6.1 Sol에서도)·max·ultra·빈 값은 `ErrResearchEffort`. **거부가 CLI 실행 전에 일어남**: 가짜 CLI가 실행되면 흔적을 남기는데, Fable·Luna·Astra ultra에서 흔적이 없어야 한다(Fable은 묻지 않고 크레딧을 청구할 수 있어 특히 중요하다). fake 실행 파일로 Codex/Claude 구조화 출력과 로그인 오류 전달도 확인한다. 외부 서비스의 동작을 mock 검증하지 않는다.
- `internal/research/browser_test.go` + `scripts/test-research-browser.mjs`: 선택 실행하는 Chrome 통합 검사. 실제 arch/Bazaar 보고서의 사본을 사용하고, 조사 결과만 결정적인 fixture로 주입한다. 기존 119개 문단 보존, 위치 선택·전송·읽던 위치 유지·정정, 숨김/복원/새로고침, 임시 스킬 반영, 오프라인·인쇄·390/320px를 검증한다.

```sh
go test -race ./internal/research ./internal/editor/assist
RETROTECH_BROWSER_QA_NODE=/absolute/path/to/node \
RETROTECH_BROWSER_QA_REPORT=/absolute/path/to/pristine-report \
PLAYWRIGHT_MODULE=/absolute/path/to/playwright/index.mjs \
go test ./internal/research -run TestBrowserWorkflow -v -count=1
```

Chrome가 필요하며 Node/Playwright는 검증 전용이다. 브라우저 검증 설정이 없는 기본 Go 실행에서는 이 선택 검사만 건너뛴다. 2026-09-16 로컬에서 전체 브라우저 검사와 실제 Codex Sol/high 호출을 별도로 실행했다. Claude는 로컬 로그인이 없어 실제 모델 호출은 미검증이며 fake CLI의 구조화 출력·오류 처리를 확인했다. 강제 전원 종료와 클라우드 서비스 가용성은 자동 테스트 범위 밖이다.

### 파일 HTML 연결 회귀 (2026-09-24)

- `TestFileOriginConnection`: `Origin: null`의 토큰 없는 읽기·잘못된 토큰·외부 웹 출처 거부, API preflight, 올바른 토큰의 읽기/쓰기, 재시작 후 연결 토큰 보존 및 다른 보고서와의 분리.
- `cmd/research/main_test.go`: 실행기가 경로를 인용하고 `-open` 없이 서버만 실행하는 계약. `-skill` 기본값이 Codex 사용자 스킬 링크(`~/.agents/skills/research-retrotech`)인지(2026-10-31).
- Chrome 통합 검사는 이제 주 흐름을 `file://…/index.html`에서 시작한다.

### 조사 모델 갱신 (2026-09-30)

- 단위·HTTP 테스트는 위 두 파일의 메뉴↔서버 일치, 목록 고정, 사유별 거부, exec 전 거부, 빠진 모델 기록 항목이다. 변이 6건(메뉴에만 모델 추가, 목록에만 모델 추가, `researchArgs` 검증 제거, `Start` 검증 제거, 메뉴 기본값 변경, 메뉴에 max 추가)을 모두 이 테스트들이 잡는 것을 확인했다.
- Chrome 통합 검사(`TestBrowserWorkflow`)를 다시 실행했다. 깨끗한 보고서는 설치본의 `.research/base.html`·`base.md` 사본을 쓰고 Playwright 1.59.1(`PLAYWRIGHT_MODULE`)을 지정했다. 119문단 보존과 기본 메뉴(GPT-6 Sol/high)로의 file:// 제출 등 12항목이 통과했다. 저장값 복원은 헤드리스 Chrome으로 따로 확인했다: 빠진 모델 → 기본 Sol, 목록에 없는 깊이 → high, 유효한 값은 유지.
- 실제 CLI: 설치본 보고서 사본 4개와 스킬 사본에 새 바이너리를 띄우고 모델마다 짧은 사실 확인 질문을 보냈다. GPT-6 Astra/medium·Sonnet 5.5/high·Opus 5.5/high는 1~2분 안에 `verified`로 끝났다. GPT-6 Sol/high는 약 4분 뒤 원문 페이지(lists.gnu.org, 응답 약 9초)를 열지 못했다고 밝히고 `partial`로 저장했다. 같은 Sol의 medium은 다른 질문에서 55초에 `verified`였다. Fable 요청은 400으로 거부됐다. 검색 결과와 소요 시간은 질문·사이트에 따라 다르므로 품질 비교로 보지 않는다. 실제 CORS를 통한 질문·삽입·스킬 반영과 새 창/URL 이동 없음, 통신 중단·복구·질문 초안 유지, 끊긴 순간 전송했을 때 오류 알림을 닫고 재시도, 완전 오프라인을 검사한다. 테스트의 모델 응답은 기존과 같이 fixture이며 실제 로컬 AI CLI 호출 계약은 이번 변경 범위 밖이다.

### 조사 기본 모델 교체 (2026-10-29)

- 단위·HTTP 테스트의 목록 핀과 메뉴 기본값을 GPT-6.1 Sol로 바꿨다. `TestRetiredModelHistoryStaysUsable`은 이번에 빠진 `gpt-6-sol` 기록으로 확인한다(설치본에 그 모델의 조사 5건이 있다). 거부 표에 이전 세대 GPT-6 Sol과 Assist 카탈로그에 새로 들어온 GPT-6.1 Terra를 더했고, CLI 기본 깊이가 low인 GPT-6.1 Sol에서도 low를 거부하는지 본다. 변이 4건(메뉴에만 옛 Sol, 목록에만 옛 Sol, 목록·메뉴 둘 다에 옛 Sol 추가, 메뉴 첫 항목을 Astra로)을 모두 테스트가 잡았다.
- Chrome 통합 검사(`TestBrowserWorkflow`)를 다시 실행했다. 깨끗한 보고서는 설치본의 `.research/base.html`·`base.md` 사본이고 Playwright 1.59.1을 지정했다. 119문단 보존과 기본 메뉴(GPT-6.1 Sol/high)로의 file:// 제출 등 12항목이 통과했다.
- 저장값 복원은 새 바이너리를 띄운 보고서 사본을 내장 브라우저로 열어 확인했다. `codex:gpt-6-sol`/xhigh를 저장해 두면 GPT-6.1 Sol/xhigh로 열리고, `codex:gpt-6-astra`/medium은 그대로 복원된다. 콘솔 오류는 없다. 같은 보고서 사본에 설치된 이전 바이너리와 새 바이너리를 띄워 메뉴 전후 스크린샷도 찍었다.
- 실제 CLI: 설치본 보고서 사본에 새 바이너리를 띄우고(codex 0.160.0) GPT-6.1 Sol로 두 번 조사했다. high는 2분 31초 뒤 `partial`로 저장됐다. 메일 발송 시각과 수신 목록은 LWN 재수록본으로 확인했지만 원래 목록의 개별 메시지 주소는 찾지 못했다고 밝혔다. medium은 45초 뒤 `verified`로 저장됐다(출처 2, 동시대 논문 포함). 두 결과 모두 HTML·Markdown에 모델명과 함께 렌더됐고, 원본 보고서의 해시는 그대로다. `~/.codex/skills/research-retrotech`가 이 Mac에 없어 스킬은 보고서 `versions/`의 9/16 SKILL.md 백업 사본을 썼다. Claude 모델은 목록이 바뀌지 않아 다시 호출하지 않았다.
