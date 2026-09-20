# Quality Gate — RetroTech

이 프로젝트의 완료/커밋 가능 판단 기준. 자체 제작 Go 정적 생성기 기준으로 작성했다.

## 사용 가능한 명령

| 명령 | 내용 |
| --- | --- |
| `go run ./cmd/build` | `content/` + `public/` → `dist/` (HTML + feed.xml + 자산) |
| `go run ./cmd/serve` | `dist/` 미리보기. 빈 포트 자동 선택(8080 회피)·URL 출력. `PORT` 로 고정 가능. clean URL |
| `go test ./...` | 단위 + 피드 골든 + **접근성/성능 불변식**(`a11y_perf_test.go`) 테스트 |
| `go vet ./...` | 표준 정적 검사 |
| `go build ./...` | 컴파일 확인 |
| `npx @lhci/cli@0.14.x autorun` | **Lighthouse CI**: `cmd/serve` 자동 기동 → `/`·`/episodes`·에피소드 감사. 설정 `.lighthouserc.json` |
| `go run ./cmd/app -repo .` | 에피소드 에디터 사이드카 단독 기동 → `http://127.0.0.1:49218/_write/`(데스크톱 앱 백엔드) |
| `cd desktop && npm test` | 데스크톱 앱 JS 테스트(`node --test`, 사이드카 재시작 정책 등). `desktop/` 변경 시 필수 |
| `cd desktop && npm run dist` | 에디터 데스크톱 앱 패키징 → `dist/mac-arm64/RetroTech Editor.app`(Go 서버 동봉, 코드사이닝 없음) |

> 별도 lint/format 도구는 두지 않는다(`go vet` + `gofmt` 관례). 사이트 빌드용 npm 스크립트는 없다(에디터 앱의 `desktop/` 만 npm 사용). Lighthouse CI 는 개발/CI 전용(`npx`)이라 산출물 의존성에 영향 없다.
>
> **CI:** GitHub Actions(`.github/workflows/ci.yml`) 가 push(main)/PR 마다 두 잡을 실행한다. ① `test`: `go vet`·`go test`·`go run ./cmd/build`(피드 골든·접근성/성능 불변식·빌드). ② `lighthouse`: 빌드 후 Lighthouse 감사 — **접근성/SEO/Best-Practices=100 은 하드 게이트**(위반 시 실패), 성능·CLS 는 경고(localhost 절대 타이밍은 비대표적이라). 접근성/성능 검증은 [PERFORMANCE.md](./PERFORMANCE.md) 참고.

## 필수 확인 항목

- [ ] **컴파일/정적검사:** `go build ./...` · `go vet ./...` 통과.
- [ ] **테스트:** `go test ./...` 통과. 특히 **피드 골든**(`internal/builder/testdata/feed.golden.xml`)이 구독자 계약(guid/enclosure/pubDate)을 지키는지. **새 에피소드를 추가했다면** 골든도 같은 커밋에서 갱신해야 한다(절차: [TESTS.md](./TESTS.md)).
- [ ] **빌드:** `go run ./cmd/build` 성공.
- [ ] **RSS 생성:** `dist/feed.xml` 이 생성되고 iTunes 필드가 포함되는지.
- [ ] **정적 산출물:** `dist/` 에 HTML(홈/episodes/404 + 에피소드 전체 편수) + `feed.xml` + `sitemap.xml` + 자산(`assets/styles.<hash>.css` 포함)이 생성되는지. `chapters:` 선언 에피소드가 있으면 `episodes/<id>.chapters.json` 도 생성되는지.
- [ ] **수동 구동 확인:** `go run ./cmd/serve` 로 홈·에피소드·다크모드 토글이 정상 렌더되는지.
- [ ] **접근성/성능(불변식):** `go test ./...` 의 `a11y_perf_test.go` 통과(제목 계층·랜드마크·alt·토글 키보드·lazy iframe·배지 높이·preload 등 마크업).
- [ ] **접근성/성능(Lighthouse):** `npx @lhci/cli@0.14.x autorun` — 접근성/SEO/Best-Practices=100(하드 게이트). CI `lighthouse` 잡과 동일.

## 선택 확인 항목

- [ ] **시각 패리티/회귀:** 주요 페이지 스크린샷 비교(마이그레이션 기준은 참고 빌드 `_ref_dist`). 절차·기준은 [PERFORMANCE.md](./PERFORMANCE.md).
- [ ] **운영 성능 절대값:** 배포 후 `https://retrotech.outsider.dev` 에서 DevTools 트레이스로 LCP/TTFB(로컬은 TTFB≈0 이라 낙관적).

## 면제 / 미검증 조건

- **운영 호스트 설정(압축·캐시 헤더·HTTPS):** 로컬 정적 서버로는 검증 불가. 호스트에서 별도 확인.
- **GA4 주입:** `ANALYTICS_ID` 미설정 시 분석 코드 미포함 — 로컬/CI 빌드는 의도적으로 GA-free.
- **에디터 앱:** 데이터/HTTP 계층은 `go test ./...`(`internal/editor`)로 검증된다. **GUI 픽셀 렌더**는 디스플레이/Electron 헤드리스 제약으로 자동 검증 대상이 아니다 — 변경 시 `cd desktop && npm start`(또는 `.app`)로 수동 확인. 에디터는 사이트 빌드(`cmd/build`)에 포함되지 않아 위 사이트 게이트에 영향 없다.

## 커밋 전 체크

- [ ] `go build`·`go vet`·`go test ./...` 통과(또는 실패/미실행 사유를 worklog·커밋 메시지에 명시).
- [ ] `go run ./cmd/build` 성공.
- [ ] `docs/worklog/YYYY-MM.md` 에 작업 기록 추가.
- [ ] 테스트를 추가/변경했다면 [TESTS.md](./TESTS.md) 갱신.
- [ ] `git commit --signoff` (CLAUDE.md 규칙), 메시지는 영어.

## 마지막 검토

- **2026-09-20:** Assist 사이드바의 모델/effort 어휘를 `internal/editor/assist/catalog.go` 한 곳으로 모았다. **CLI(claude·codex)를 업그레이드하면 이 카탈로그를 다시 확인한다** — CLI 는 기계가 읽을 모델 목록을 내주지 않아 수기 목록이고, 계정에 있는 모델이라도 설치된 CLI 버전이 못 쓰는 경우가 있다(오늘 GPT-6 Astra 가 그래서 빠져 있다). 확인 방법: 카탈로그의 모델 하나씩 `POST /_write/api/assist/run` 으로 짧게 호출. 에디터 UI 는 `cmd/build` 산출물이 아니므로 사이트 게이트에는 영향이 없다.
- **2026-08-08:** 아이템 `<description>` 을 HTML 로 발행하도록 변경하며 피드 골든을 갱신. 골든은 이제 "`gen-rss.js` 바이트 패리티"가 아니라 **구독자 계약(guid/enclosure/pubDate) + 현재 피드 형태**의 회귀 가드다 — 의도된 피드 변경 시 갱신 후 `git diff` 로 계약 필드 불변을 확인한다.
- **2026-08-01:** 팟캐스트 챕터 도입 — 정적 산출물 항목에 조건부 `episodes/<id>.chapters.json` 추가. 피드 골든은 챕터 미선언 시 그대로 유효(네임스페이스 조건부 선언).
- **2026-06-16:** 접근성/성능을 CI에서 검증하도록 추가 — `go test` 의 마크업 불변식(`a11y_perf_test.go`) + 새 `lighthouse` 잡(Lighthouse CI, a11y/SEO/BP=100 하드 게이트). 로컬 재현: `npx @lhci/cli@0.14.x autorun`.
- **2026-06-16:** Go 정적 생성기로 마이그레이션(Next/Nextra 제거). 검증 기준을 `go build`·`go vet`·`go test`·`go run ./cmd/build` 로 교체. CI 를 Go 로 전환.
- (이전) 2026-06-15: Next 기반 — RSS 포맷 스냅샷 + 데이터 유효성 테스트, GitHub Actions CI 도입.

## 로컬 조사 리더 변경 시 (2026-09-16)

- 필수: `go build ./...`, `go vet ./...`, `go test ./...`, 조사/CLI 패키지 race 검사, 공개 사이트 빌드와 RSS·정적 산출물 확인.
- UI 변경 시: [TESTS.md](TESTS.md)의 Chrome 통합 검사와 desktop/mobile 화면 확인. 원문 문단 수·내용, 삽입 위치·스크롤 유지, 저장 후 재열기, 스킬 임시 사본 반영, 오프라인·인쇄를 확인한다.
- CLI 호출 계약 변경 시: 로그인된 제공자로 보고서 사본에서 실제 호출을 확인한다. 로그인되지 않은 제공자는 테스트 한계와 사용자 로그인 방법을 기록한다.
- 이 리더의 HTML/JS는 공개 사이트 `cmd/build`에 포함되지 않는다. 공개 사이트 코드·콘텐츠·자산에 변경이 없으면 사이트 수동 렌더와 Lighthouse 재측정은 요구하지 않고 기존 사이트 불변식·빌드로 회귀를 확인한다. 공개 사이트 변경이 함께 있으면 위 사이트 게이트를 그대로 적용한다.
