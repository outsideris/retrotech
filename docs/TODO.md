# TODO — RetroTech

> 작업 단위는 Phase > Todo. 상세 구현 계획이 필요해지면 `docs/plan/` 에 문서를 만들고 해당 Todo에서 링크한다.
> 근거: [PERFORMANCE.md](./PERFORMANCE.md), [ARCHITECURE.md](./ARCHITECURE.md), [DESIGN.md](./DESIGN.md).

## Phase 1 — 성능: 이미지 / CLS

- [x] **`cover.svg` 경량화.** SVGO 적용 — 402KB→143KB(64.5%↓, path 436→306, 렌더 동일 확인). 콜드 첫 방문 LCP·파싱/CPU 개선.
- [x] **히어로 이미지 CLS 제거.** `index.mdx` 히어로를 `width={3000} height={3000}` + `height:auto` 로(1:1 비율 예약). 트레이스 CLS 0.00 확인.
- [x] **`outsider.png` 최적화.** 240px WebP 로 교체(110KB→5KB, 95%↓). png 제거, `theme.config.js` 에서 `outsider.webp` 참조 + width/height 지정.
- [x] 배지 SVG SVGO 최적화 — apple/google/rss/spotify/youtube 총 ~80KB→35KB(viewBox 보존, 렌더 동일 확인).
- [x] **배지 이미지 preload 경쟁 제거 (LCP).** `Badges.tsx` 의 `<Image priority>` 제거(배지는 히어로 아래라 LCP 무관). 히어로만 프리로드되도록 해 LCP 개선.

## Phase 2 — 성능: 캐시 / 서드파티 (전 페이지 공통)

- [x] **정적 자산 캐시 연장 (최대 ROI).** `public/_headers` 로 `/_next/static/*` 를 1년 `immutable` 적용(배포 시 반영). 배포 후 `curl -I` 로 확인 권장. → [DEPLOYMENT.md](./DEPLOYMENT.md)
- [x] **FontAwesome Pro 킷 풀로드 제거.** 4개 아이콘(twitter/github/blog/rss)을 `components/Icons.tsx` 인라인 SVG(FA Free, CC BY 4.0)로 대체, `_document.tsx` 의 kit 스크립트 제거. 외부 요청 ~9개 + 웹폰트 3개 절감.
- [ ] **GitHub Sponsors iframe** 지연 로드 또는 정적 링크/버튼으로 대체(전 페이지 iframe 비용 제거).
- [x] **GTM 제거, GA4 직접 로드만 유지.** `_app.tsx` 의 GTM 컨테이너 + `_document.tsx` 의 GTM `noscript` 제거(사용자 결정: GA4만 직접). 미사용 JS 중복·이중 집계 위험 해소.
- [x] ~~(경미) 홈의 `episodes-*.js` 중복 프리페치 원인 확인.~~ → Go 마이그레이션으로 해소(프레임워크 JS 자체가 사라짐).

## Phase 3 — 접근성 ✅

- [x] 아이콘 전용 링크에 `aria-label` 부여(푸터 RSS) + 장식 아이콘 `aria-hidden`. (`link-name`)
- [x] `<main>` 랜드마크 확보 — `_app.tsx` 에서 `<article>` 에 `role="main"` 부여(테마가 main 미렌더). (`landmark-one-main`)
- 결과: Lighthouse 접근성 94→100, Agentic 25→100.

## Phase 4 — 유지보수 / 구성

- [x] `public/feed.xml` 을 `.gitignore` 에 추가(빌드 산출물).
- [x] ~~Nextra 권고대로 `_app.tsx → _app.mdx` 검토.~~ → 무의미(Go 마이그레이션으로 `_app.tsx` 제거).
- [x] `npx update-browserslist-db@latest` (caniuse-lite 1.0.30001517→…1799). 빌드의 "caniuse-lite is outdated" 경고 제거.
- ℹ️ **레거시 JS(12KiB)는 설정으로 못 줄임.** Next 의 framework/main/polyfills 내장 청크라 `browserslist`/`tsconfig target` 변경에도 청크 해시 동일. 모던 browserslist 는 호환성만 좁혀 되돌림. → Next 업그레이드 시 재검토.
- [ ] `SITE_URL`(`internal/builder/feed.go`·`cmd/build`) 하드코딩을 공유 상수로 추출(도메인 중복).
- [x] 배포 성공/실패 텔레그램 알림 — `scripts/cf-build.sh` 빌드 래퍼가 결과를 Worker(`cf-webhook…`)로 POST. **대시보드에서** Build command=`bash scripts/cf-build.sh` + 암호화 환경변수 `DEPLOY_WEBHOOK_URL`(워커의 **`/webhook/generic`** 엔드포인트) 설정 필요. → [DEPLOYMENT.md](./DEPLOYMENT.md#배포-알림--텔레그램)
- [ ] ~~(장기) Next 13/Nextra 2-beta → 최신 메이저 업그레이드 호환성 검토.~~ → **Phase 6(Go 마이그레이션)으로 대체.** 프레임워크 자체를 걷어내므로 업그레이드 트레드밀이 사라진다.

## Phase 5 — 테스트 도입 ✅

- [x] Vitest 도입(`npm test`). `vitest.config.ts`, `gen-rss.js` 를 테스트 가능하게 리팩터(`episodeToItem`/`shouldSkip` export).
- [x] `scripts/gen-rss.test.js` — 프론트매터→RSS 변환(`episodeToItem`) + `index.*` 제외(`shouldSkip`).
- [x] `components/Badges.test.tsx` — Google↔RSS 토글 + 플랫폼 배지 렌더(next/image·link mock).
- 합계 2파일·10테스트 통과. 상세: [TESTS.md](./TESTS.md)
- [ ] (향후) 에피소드 디렉터리→피드 생성 end-to-end 테스트.

## Phase 6 — Go 정적 생성기 마이그레이션 (Nextra/Next 제거)

> 자체 제작 Go 정적 생성기로 전환해 프레임워크 의존성을 제거한다(범위: 빌드/프레임워크만, GA4·Sponsors 유지).
> 상세 계획·불변식·위험: **[plan/go-static-migration.md](./plan/go-static-migration.md)**. 참고 구현: `blog.outsider.ne.kr`.
> 안전 원칙: 기존 Next를 건드리지 않고 Go 생성기를 **병행 구축** → 패리티 통과 후 배포 전환·Next 제거.

- [x] **A. 스캐폴딩** — `go.mod`(goldmark+yaml.v3), `cmd/build`, `internal/parser`. 단위 테스트.
- [x] **B. RSS 패리티 (최우선 리스크).** `feed.go` + 골든 테스트 — 현 `feed.xml`과 23편 항목 바이트 동일(guid·enclosure·pubDate). pubDate 는 운영 기준 09:00 UTC 로 고정.
- [x] **C. Badges + 콘텐츠 변환 규칙 확정** — `badges.go` + 프론트매터 `badges:` + 본문 `<!--badges-->` 마커. `scripts/convert` 도구.
- [x] **D. 템플릿 & 페이지 생성** — home/episodes/episode/404. 평면 `.html`, URL 동일.
- [x] **E. "똑같은 형태"** — 테마 CSS 재사용 + 다크모드 인라인 스크립트 + 해/달 스왑 + GA 마커. 스크린샷 픽셀 일치.
- [x] **F. 콘텐츠 일괄 이관** — 23편 `.mdx`→`content/episodes/*.md`. 골든·시각 회귀 통과.
- [x] **G. 빌드/배포/CI 전환** — `ci.yml`(Go), `cf-build.sh`(Go), CSS 핑거프린트(`/assets/*` immutable). Cloudflare Pages git-build 유지. → [DEPLOYMENT.md](./DEPLOYMENT.md)
- [x] **H. Next 제거 & 문서 정리** — `pages/`·`components/`·`theme.config.js`·`next.config.js`·`package.json`·vitest·`gen-rss*` 제거. README/ARCHITECURE/DESIGN/QUALITY_GATE/TESTS 갱신.

**완료(2026-06-16).** 외부 의존성 2개로 축소, 브라우저 프레임워크 JS 0, 시각·동작·피드 동일. 상세: [plan/go-static-migration.md](./plan/go-static-migration.md).

## Phase 7 — 마이그레이션 후속 정리 (보류, 사용자 재확인 예정)

> 마이그레이션 완료 후 정리 후보. 동작·시각엔 영향 없음. 작업 후 함께 검토하기로 함(2026-06-16).

- [ ] **Nextra 잔재 네이밍 정리.** 페이지 셸의 `<div id="__next">` 래퍼와 CSS 의 `nx-*` 클래스명은 Nextra 흔적이다. 중립 이름으로 바꾸려면 재사용 중인 테마 CSS(`public/styles.css`) 전체를 다시 써야 해 비용이 있다. 동작엔 무관.
- [ ] **홈 본문을 마크다운으로.** 커버·소개·이슈 문구가 `internal/builder/render.go`(`BuildHomePage`)에 하드코딩돼 있다. 마크다운으로 편집하고 싶으면 `content/index.md` 도입 검토.
- [x] **`scripts/convert` 삭제(2026-06-16).** mdx→md 1회성 마이그레이션 도구. 입력(`pages/`)이 제거돼 더는 동작하지 않아 제거. 변환 방식은 worklog·plan 에 기록됨.
- [ ] **피드 `<generator>` 문자열.** 현재 `RSS for Node`(옛 rss 라이브러리 잔재, 부정확). `RetroTech` 등으로 바꾸거나 둘지 결정. (채널 `<description>` 은 2026-06-16 에 실제 설명으로 교체 완료.)
- ℹ️ **비가시 차이(조치 불필요, 렌더 동일):** 에피소드 h1 후행 개행 없음, `<time dateTime>` 속성이 UTC(표시는 동일), 본문 아포스트로피 `'`↔`&#x27;`(둘 다 `'` 로 렌더), next/image 내부 속성(`data-nimg` 등) 생략.

## 운영(미검증, 확인 필요)

- [ ] 운영 호스트의 gzip/brotli 압축·정적 자산 캐시 헤더 설정 확인(로컬에선 검증 불가 — [PERFORMANCE.md](./PERFORMANCE.md#측정-방법--한계-먼저-읽을-것)).
