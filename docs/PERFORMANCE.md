# Performance — RetroTech (감사 결과)

> 최초 측정 2026-06-14. **운영 사이트(https://retrotech.outsider.dev, Cloudflare)** 기준으로 갱신.
> 구조·구성은 [ARCHITECURE.md](./ARCHITECURE.md), 배포/호스팅은 [DEPLOYMENT.md](./DEPLOYMENT.md), 개선 백로그는 [TODO.md](./TODO.md).

## 측정 방법

- **대상:** 운영 사이트 `https://retrotech.outsider.dev` (Cloudflare, HTTP/3).
- **도구:** Chrome DevTools 성능 트레이스(Core Web Vitals) + Lighthouse(접근성/모범사례/SEO) + `curl` 응답 헤더 점검.
- **조건:** 모바일 에뮬레이션 412×915, **Slow 4G, CPU 4×** (PageSpeed 모바일과 유사).
- 참고: Lighthouse `lighthouse_audit` 도구는 성능 점수를 제외하므로, 성능 지표는 트레이스에서 측정했다. CrUX 필드 데이터는 트래픽이 적어 없음(lab 측정만).

## 요약 (개선 우선순위)

| 순위 | 영역 | 개선점 | 효과 |
| --- | --- | --- | --- |
| 1 ✅ | 성능(캐시) | `public/_headers` 로 `/_next/static/*` → 1년 immutable (적용; 배포 시 반영) | 재방문 속도 ↑ (최대 ROI) |
| 2 ✅ | 성능(서드파티) | FontAwesome 킷 제거 → 4개 아이콘 인라인 SVG(`components/Icons.tsx`) | 외부 요청 ~9개·웹폰트 3개 제거(전 페이지) |
| 3 🟠 | 접근성 | 아이콘 링크 `aria-label`, `<main>` 랜드마크 | A11y 94→100 |
| 4 🟠 | 성능(CLS) | 히어로 이미지 높이 예약(`width/height` 또는 `aspect-ratio`) | 간헐 CLS 0.25 → 0 |
| 5 🟡 | 성능(LCP) | 히어로 이미지 preload / 렌더블로킹 CSS 축소 | LCP load delay 439ms 단축 |
| 6 ✅ | 성능(이미지) | `cover.svg` SVGO(402→143KB) · `outsider.png`→WebP(110→5KB) | 콜드 첫 방문 LCP·파싱/CPU 절감 |
| 7 🟡 | 성능(서드파티) | GitHub Sponsors iframe 지연, GTM+GA4 중복 검토 | 메인스레드/요청 절감 |

> ✅ **이미 좋은 점:** Brotli 압축 ON(HTML/CSS/JS/SVG), HTTP/3, LCP·CLS 양호 등급, SEO·Best Practices 100, 정적 익스포트.

> **적용 현황 (2026-06-14, 배포 시 반영):** ① `public/_headers` 추가 — `/_next/static/*` 를 `max-age=31536000, immutable` 로(Cloudflare 기본 4시간 override). ② `cover.svg` SVGO 최적화 — 402KB→143KB(브라우저 렌더 동일 확인). **아래 운영 측정값은 이 변경 이전 기준이다.**

## Core Web Vitals (운영, 모바일 Slow 4G / CPU 4×)

| 지표 | 값 | 평가 |
| --- | --- | --- |
| **LCP** | **1,022 ms** | 양호 (<2.5s) |
| **CLS** | 0.00 (트레이스) / **0.25** (Lighthouse) | 간헐적 — 수정 권장 |
| TTFB | 322 ms | Cloudflare 왕복(Slow 4G 포함) |

**LCP 분해** (요소 = 홈 히어로 `cover.svg` IMG):
- TTFB 322ms (31.5%)
- **Resource load delay 439ms (42.9%) ← 가장 큼.** 이미지가 늦게 발견됨(HTML 본문 참조, preload 미적용/렌더블로킹 CSS 뒤). 단, 측정된 이미지 다운로드 **4ms 는 재방문 캐시 적중** 값이고, **콜드 첫 방문**에는 brotli 118KB 를 받아야 하므로(Slow 4G ≈ 0.6s) cover.svg 크기가 첫 방문 LCP 에는 실제로 영향을 준다.
- Load duration 5ms (0.5%)
- Render delay 256ms (25%)

> 로컬(`http.server`) 측정 때 LCP 419ms 로 보였던 것은 TTFB≈1ms 때문에 낙관적이었던 수치다. 운영 LCP 1,022ms 가 실제값.

## Lighthouse (운영, 모바일)

| 카테고리 | 점수 |
| --- | --- |
| Accessibility | **94** |
| Best Practices | 100 |
| SEO | 100 |
| Agentic Browsing | 25 |

**실패 항목:**
- 접근성 `link-name` (가중치 7) — 식별 가능한 이름 없는 링크. 푸터의 아이콘 전용 링크(예: RSS `<a href="/feed.xml"><i class="fa-rss"/></a>`). → `aria-label` 추가.
- 접근성 `landmark-one-main` (가중치 3) — `<main>` 랜드마크 없음.
- Agentic `cumulative-layout-shift` — CLS 0.25(히어로 이미지 높이 미예약).
- Agentic `agent-accessibility-tree` — 접근성 트리 비정형(위 a11y 이슈와 연관).

## 호스팅 동작 (운영 실측 헤더)

| 항목 | 상태 | 비고 |
| --- | --- | --- |
| 압축 | ✅ **Brotli** (`content-encoding: br`) | HTML/CSS/JS/SVG 모두 |
| 프로토콜 | ✅ HTTP/3 (h3) | |
| HTML 캐시 | `max-age=0, must-revalidate` · `cf-cache-status: DYNAMIC` | 엣지 캐시 안 함 |
| 정적/해시 자산 캐시 | ⚠️ `max-age=14400(4h), must-revalidate` · `REVALIDATED` | **문제:** `/_next/static/*` 처럼 파일명에 해시가 있는 자산도 4시간뿐. 1년 `immutable` 이어야 함 |

> 4시간(14400초)·`must-revalidate` 가 HTML 외 모든 자산에 균일 적용된 것은 Cloudflare **Browser Cache TTL 기본값(4h)** 이 적용된 정황이다. → 아래 캐시 개선 참고.

## 빌드 산출물

- 정적 HTML **27p**. 홈 First Load JS **104 kB**(공유 84.4 kB + 페이지). CSS 9 kB(br).
- 공유 청크: framework 45.2 + main 28.5 + _app/webpack ~1.6 (kB, br 기준).

## 자산 용량 (원본; 운영은 brotli 전송)

| 자산 | 용도 | 원본 | 비고 |
| --- | --- | --- | --- |
| `images/cover.svg` | 홈 히어로(LCP) | ~~402 KB~~ → **143 KB** (SVGO 적용) | 벡터 일러스트(path 436→306 병합). SVGO 로 64.5% 감소(렌더 동일 확인), brotli ~50KB. 배포 전 운영값은 402KB/brotli 118KB |
| `images/cover.jpg` | OG/iTunes(비표시) | 233 KB | 페이지 로드와 무관 |
| `images/outsider.webp` | 푸터(120px 표시) | ~~110 KB(png)~~ → **5 KB** | 240px WebP 로 교체(95%↓) |
| 배지 SVG ×4 | 구독 배지 | 9~25 KB | SVGO 가능 |

## 네트워크 (홈, 33 요청)

- 동일 출처 ~16 (HTML/CSS/JS/이미지).
- **FontAwesome ~9** (kit.js + CSS 5 + woff2 3) — 아이콘 4개용.
- **GitHub Sponsors iframe** 2, **GTM+GA4** 4.
- 위 외부 요청 다수가 `theme.config.js` 푸터·`_app.tsx` 분석에서 와 **전 페이지 공통 비용**.
- 서드파티 메인스레드(4× 기준): GTM 176ms.

## 개선 상세

### 1. 캐시 (최대 ROI) — `/_next/static/*` 1년 immutable
> ✅ **적용됨:** `public/_headers` 추가 완료(배포 시 반영). 아래는 배경과 대안.

해시 자산은 내용이 바뀌면 파일명도 바뀌므로 영구 캐시가 안전하다. 두 가지 방법:
- **(권장) `public/_headers` 파일** (Cloudflare Pages가 인식):
  ```
  /_next/static/*
    Cache-Control: public, max-age=31536000, immutable
  ```
  단, Cloudflare 대시보드의 **Browser Cache TTL** 이 "Respect existing headers" 여야 origin 헤더가 반영된다(현재 4h 고정이면 override 중일 수 있음).
- **(또는) Cloudflare Cache Rule (가장 확실)** — 대시보드에서 도메인 선택 → **Caching → Cache Rules → Create rule**:
  - 식(expression): `URI Path` `starts with` `/_next/static/`
  - Cache eligibility: Eligible for cache
  - Edge TTL: "Ignore cache-control header and use this TTL" → **1년(31536000s)**
  - Browser TTL: "Override origin" → **1년**
  - 전역 기본값(현재 4시간)은 **Caching → Configuration → Browser Cache TTL** 에 있다. "Respect Existing Headers" 로 두면 `_headers`/origin 헤더가 반영된다.
- (선택) HTML 은 배포 시 purge 를 전제로 엣지 캐시(cache-everything + 짧은 TTL)도 가능.

### 2. FontAwesome 경량화
푸터 아이콘은 4개(twitter/github/blog/rss). Pro 킷 풀로드 대신 해당 아이콘 **SVG 인라인** 또는 부분 번들로 대체 → 외부 요청 ~9개 + 웹폰트 3개 제거(전 페이지).

### 3. 접근성
- 아이콘 전용 링크에 `aria-label`(특히 푸터 RSS/소셜).
- 페이지 본문을 `<main>` 으로 감싸 랜드마크 확보(테마 제약 확인 필요).

### 4. CLS — 히어로 이미지
`<Image src="/images/cover.svg" width={0} height={0} style={{width:'100%'}}/>` 는 세로 공간을 예약하지 않아 로드 시 밀림(간헐 CLS 0.25). 실제 가로·세로 비율 지정 또는 `aspect-ratio` 적용.

### 5. LCP load delay
히어로 이미지를 `<link rel="preload" as="image">` 로 미리 받거나(`priority` preload 가 실제 emit 되는지 확인), 렌더블로킹 CSS(테마 CSS)를 줄여 발견 시점을 앞당긴다. (LCP 는 이미 양호라 우선순위는 낮음)

### 6. 이미지 다이어트
- **`cover.svg`** (436 path, raw 402KB / **brotli 118KB**): path 좌표가 소수점 6자리(예: `3002.000000`)라 SVGO 로 정밀도를 1~2자리로 낮추면 크게 줄어든다(콜드 첫 방문 LCP + 파싱/CPU 개선). 복잡한 일러스트라 AVIF/WebP 래스터 대안도 비교해볼 만하다.
- **`outsider.png`** (110KB, PNG): 표시 크기(≈240px@2x)로 리사이즈 + WebP.

### 7. 기타 서드파티
GitHub Sponsors iframe 지연 로드(또는 정적 링크), GTM+GA4 동시 사용 필요성 재검토.

## 재현 방법
```bash
# 운영 사이트 헤더 확인
curl -sI -H 'Accept-Encoding: br, gzip' https://retrotech.outsider.dev/_next/static/css/<hash>.css
# Lighthouse / DevTools 트레이스로 https://retrotech.outsider.dev 측정 (모바일, Slow 4G, CPU 4x)
```
