# Quality Gate — RetroTech

이 프로젝트의 완료/커밋 가능 판단 기준. 자체 제작 Go 정적 생성기 기준으로 작성했다.

## 사용 가능한 명령

| 명령 | 내용 |
| --- | --- |
| `go run ./cmd/build` | `content/` + `public/` → `dist/` (HTML + feed.xml + 자산) |
| `go run ./cmd/serve` | `dist/` 미리보기. 빈 포트 자동 선택(8080 회피)·URL 출력. `PORT` 로 고정 가능. clean URL |
| `go test ./...` | 단위 + 피드 골든 테스트 |
| `go vet ./...` | 표준 정적 검사 |
| `go build ./...` | 컴파일 확인 |

> 별도 lint/format 도구는 두지 않는다(`go vet` + `gofmt` 관례). 빌드용 npm 스크립트는 없다.
>
> **CI:** GitHub Actions(`.github/workflows/ci.yml`)가 push(main)/PR 마다 `go vet`·`go test`·`go run ./cmd/build` 를 실행한다(피드 골든 + 빌드 자동 검증).

## 필수 확인 항목

- [ ] **컴파일/정적검사:** `go build ./...` · `go vet ./...` 통과.
- [ ] **테스트:** `go test ./...` 통과. 특히 **피드 골든**(`internal/builder/testdata/feed.golden.xml`)이 구독자 계약(guid/enclosure/pubDate)을 지키는지.
- [ ] **빌드:** `go run ./cmd/build` 성공.
- [ ] **RSS 생성:** `dist/feed.xml` 이 생성되고 iTunes 필드가 포함되는지.
- [ ] **정적 산출물:** `dist/` 에 HTML 26개(홈/episodes/에피소드 23/404) + `feed.xml` + `sitemap.xml` + 자산(`assets/styles.<hash>.css` 포함)이 생성되는지.
- [ ] **수동 구동 확인:** `go run ./cmd/serve` 로 홈·에피소드·다크모드 토글이 정상 렌더되는지.

## 선택 확인 항목

- [ ] **시각 패리티/회귀:** 주요 페이지 스크린샷 비교(마이그레이션 기준은 참고 빌드 `_ref_dist`). 절차·기준은 [PERFORMANCE.md](./PERFORMANCE.md).
- [ ] **접근성:** Lighthouse Accessibility(이전 100). 회귀 감시.
- [ ] **성능:** Lighthouse / DevTools 트레이스.

## 면제 / 미검증 조건

- **운영 호스트 설정(압축·캐시 헤더·HTTPS):** 로컬 정적 서버로는 검증 불가. 호스트에서 별도 확인.
- **GA4 주입:** `ANALYTICS_ID` 미설정 시 분석 코드 미포함 — 로컬/CI 빌드는 의도적으로 GA-free.

## 커밋 전 체크

- [ ] `go build`·`go vet`·`go test ./...` 통과(또는 실패/미실행 사유를 worklog·커밋 메시지에 명시).
- [ ] `go run ./cmd/build` 성공.
- [ ] `docs/worklog/YYYY-MM.md` 에 작업 기록 추가.
- [ ] 테스트를 추가/변경했다면 [TESTS.md](./TESTS.md) 갱신.
- [ ] `git commit --signoff` (CLAUDE.md 규칙), 메시지는 영어.

## 마지막 검토

- **2026-06-16:** Go 정적 생성기로 마이그레이션(Next/Nextra 제거). 검증 기준을 `go build`·`go vet`·`go test`·`go run ./cmd/build` 로 교체. CI 를 Go 로 전환.
- (이전) 2026-06-15: Next 기반 — RSS 포맷 스냅샷 + 데이터 유효성 테스트, GitHub Actions CI 도입.
