# TODO — RetroTech

> 작업 단위는 Phase > Todo. 상세 구현 계획이 필요해지면 `docs/plan/` 에 문서를 만들고 해당 Todo에서 링크한다.
> 근거: [PERFORMANCE.md](./PERFORMANCE.md), [ARCHITECURE.md](./ARCHITECURE.md), [DESIGN.md](./DESIGN.md).

## Phase 1 — 성능: 이미지 / CLS

- [x] **`cover.svg` 경량화.** SVGO 적용 — 402KB→143KB(64.5%↓, path 436→306, 렌더 동일 확인). 콜드 첫 방문 LCP·파싱/CPU 개선.
- [x] **히어로 이미지 CLS 제거.** `index.mdx` 히어로를 `width={3000} height={3000}` + `height:auto` 로(1:1 비율 예약). 트레이스 CLS 0.00 확인.
- [x] **`outsider.png` 최적화.** 240px WebP 로 교체(110KB→5KB, 95%↓). png 제거, `theme.config.js` 에서 `outsider.webp` 참조 + width/height 지정.
- [ ] 배지 SVG(apple/youtube/spotify/google/rss) SVGO 최적화.
- [x] **배지 이미지 preload 경쟁 제거 (LCP).** `Badges.tsx` 의 `<Image priority>` 제거(배지는 히어로 아래라 LCP 무관). 히어로만 프리로드되도록 해 LCP 개선.

## Phase 2 — 성능: 캐시 / 서드파티 (전 페이지 공통)

- [x] **정적 자산 캐시 연장 (최대 ROI).** `public/_headers` 로 `/_next/static/*` 를 1년 `immutable` 적용(배포 시 반영). 배포 후 `curl -I` 로 확인 권장. → [DEPLOYMENT.md](./DEPLOYMENT.md)
- [x] **FontAwesome Pro 킷 풀로드 제거.** 4개 아이콘(twitter/github/blog/rss)을 `components/Icons.tsx` 인라인 SVG(FA Free, CC BY 4.0)로 대체, `_document.tsx` 의 kit 스크립트 제거. 외부 요청 ~9개 + 웹폰트 3개 절감.
- [ ] **GitHub Sponsors iframe** 지연 로드 또는 정적 링크/버튼으로 대체(전 페이지 iframe 비용 제거).
- [ ] GTM + GA4 동시 사용 필요성 재검토(중복 시 하나로 통합).
- [ ] (경미) 홈의 `episodes-*.js` 중복 프리페치 원인 확인.

## Phase 3 — 접근성 ✅

- [x] 아이콘 전용 링크에 `aria-label` 부여(푸터 RSS) + 장식 아이콘 `aria-hidden`. (`link-name`)
- [x] `<main>` 랜드마크 확보 — `_app.tsx` 에서 `<article>` 에 `role="main"` 부여(테마가 main 미렌더). (`landmark-one-main`)
- 결과: Lighthouse 접근성 94→100, Agentic 25→100.

## Phase 4 — 유지보수 / 구성

- [ ] `public/feed.xml` 을 `.gitignore` 에 추가(빌드 산출물 — 매 빌드마다 untracked 로 생성됨).
- [ ] Nextra 권고대로 `_app.tsx → _app.mdx` 검토.
- [ ] `npx update-browserslist-db@latest` (caniuse-lite 갱신).
- [ ] `gen-rss.js` 의 `SITE_URL` 하드코딩을 환경변수/공유 상수로 추출(여러 곳에 도메인 중복).
- [ ] (선택) 배포 성공 시 텔레그램 알림 설정 — 방법은 [DEPLOYMENT.md](./DEPLOYMENT.md#배포-알림--텔레그램) 참고.
- [ ] (장기) Next 13/Nextra 2-beta → 최신 메이저 업그레이드 호환성 검토.

## Phase 5 — 테스트 도입

- [ ] `scripts/gen-rss.js` 의 프론트매터→RSS 변환 로직 단위 테스트(가장 테스트 가치 높음).
- [ ] `Badges` 의 google 유무에 따른 Google↔RSS 배지 토글 렌더 테스트.
- 상세: [TESTS.md](./TESTS.md)

## 운영(미검증, 확인 필요)

- [ ] 운영 호스트의 gzip/brotli 압축·정적 자산 캐시 헤더 설정 확인(로컬에선 검증 불가 — [PERFORMANCE.md](./PERFORMANCE.md#측정-방법--한계-먼저-읽을-것)).
