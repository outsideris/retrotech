# Performance — RetroTech (감사 결과)

> 측정일: **2026-06-14** · 대상: 현재 `main` 구성으로 빌드한 정적 사이트(`dist/`).
> 구조·구성은 [ARCHITECURE.md](./ARCHITECURE.md) 참고. 개선 백로그는 [TODO.md](./TODO.md).

## 측정 방법 & 한계 (먼저 읽을 것)

- **빌드:** `npm run build` (정적 익스포트). 단, `components/Player.tsx` 가 타입에러로 빌드를 깨뜨려서 측정 동안 해당 WIP 파일을 잠시 제외하고 빌드함. ([ARCHITECURE.md 주의사항](./ARCHITECURE.md#알려진-제약--주의사항-작업-전-반드시-확인))
- **서빙:** `dist/` 를 로컬 `python3 -m http.server` 로 서빙(`http://localhost`).
- **측정 도구:** Chrome DevTools — 성능 트레이스(Core Web Vitals)와 Lighthouse(접근성/모범사례/SEO).
- ⚠️ **로컬 서빙의 한계 — 수치 해석 시 반드시 감안:**
  - 로컬 서버라 **TTFB ≈ 1ms**. 실제 호스팅/CDN 왕복 지연은 반영되지 않아 **LCP 등 로딩 지표가 실제보다 낙관적**이다.
  - `http.server` 는 **gzip/brotli 압축도, 캐시 헤더도 적용하지 않는다.** 따라서 "텍스트 미압축(17.6 kB 낭비)"·"캐시 정책 없음" 같은 지적은 운영 호스트 설정에 따라 달라진다(여기선 호스트 설정을 검증하지 못함).
  - HTTP(비-HTTPS) 로 측정되어 Best Practices 점수가 실제보다 후하게 나올 수 있다.
- **결론적으로 의미 있는 신호는 "수치의 절대값"이 아니라 "구조적 비용"** — 자산 용량, 요청 수, 서드파티 부하, 레이아웃 시프트 — 이다.

## 요약 (Top findings)

1. 🔴 **홈 히어로 `cover.svg` 가 402 KB (gzip 153 KB).** 페이지의 모든 JS 를 합친 것보다 무겁고, LCP 영역 자산이다. SVG 한 장으로 단일 최대 페이로드.
2. 🔴 **푸터/분석이 모든 페이지에 무거운 서드파티를 끌어온다.** FontAwesome Pro 킷(아이콘 4개 쓰려고 JS+CSS 5개+웹폰트 3개 ≈ 9요청) + GitHub Sponsors iframe + GTM + GA4. → 홈에서만 외부 요청이 다수.
3. 🟠 **레이아웃 시프트(CLS).** 히어로 이미지가 `width={0} height={0}` 라 높이를 예약하지 않아, 로드되며 콘텐츠를 밀어낸다. Lighthouse 모바일에서 **CLS 0.25(불량)** 측정.
4. 🟠 **`outsider.png` 110 KB 를 120px 로 표시.** 표시 크기 대비 과대 원본.
5. 🟡 **`next/image` 가 최적화하지 않음**(akamai 로더 passthrough) — 위 이미지 문제를 프레임워크가 못 잡아준다. 자산을 직접 줄여야 함.
6. 🟡 **`episodes-*.js` 청크가 홈에서 2회 요청됨**(프리페치 중복, 경미).

## 빌드 산출물 (정적 익스포트)

- 정적 HTML **27개** 페이지. `dist/` 총 **3.7 MB** (JS 청크 1.6 MB).
- `next build` First Load JS 리포트(빌드 시 표기값 = gzip 기준):

| Route | Page Size | First Load JS |
| --- | --- | --- |
| `/` (홈) | 7.87 kB | **104 kB** |
| `/episodes` | 7.12 kB | 103 kB |
| `/episodes/2g` | 10.8 kB | 107 kB |
| `/episodes/1n` (최대) | 13.6 kB | **110 kB** |
| 공유(shared by all) | — | **84.4 kB** |

- 공유 청크: `framework 45.2 kB` + `main 28.5 kB` + `_app 0.8 kB` + `webpack 0.8 kB` + `css 9.09 kB` (모두 gzip 기준).
- 평가: **JS 풋프린트는 정적 콘텐츠 사이트치고 평범~약간 무거움**(글 위주인데 ~104 kB). 단 페이지별 JS 차이는 작고, 진짜 무게는 아래 이미지/서드파티 쪽이다.

## 자산 용량 (실측: 원본 / gzip)

| 자산 | 용도 | 원본 | gzip |
| --- | --- | --- | --- |
| **`images/cover.svg`** | 홈 히어로(화면 표시) | **402 KB** | **153 KB** |
| `images/cover.jpg` | OG/iTunes 커버(페이지 비표시) | 233 KB | 170 KB |
| `images/outsider.png` | 푸터 프로필(120px 표시) | 110 KB | 109 KB |
| `chunks/framework.js` | React/Next | 141 KB | 45 KB |
| `chunks/polyfills.js` | 폴리필 | 91 KB | 31 KB |
| `chunks/main.js` | Next 런타임 | 99 KB | 29 KB |
| `chunks/929.js` | 공통 청크 | 60 KB | 21 KB |
| `css/*.css` | 테마+main | 50 KB | 9 KB |
| `index.html` | 홈 문서 | 26 KB | 7.5 KB |
| 배지 SVG ×4 | 구독 배지 | 9~25 KB/개 | — |

> 홈에서 동일 출처로 내려받는 **이미지(cover.svg 153 + outsider.png 109 ≈ 262 KB gzip)가 전체 JS(≈130 KB gzip)보다 2배 무겁다.** 최적화 1순위.

## 네트워크 워터폴 — 홈 (총 33 요청)

| 그룹 | 요청 | 비고 |
| --- | --- | --- |
| 동일 출처 문서/CSS/JS | ~10 | HTML, CSS, webpack/framework/main/_app/index/929 + 매니페스트 2 |
| 동일 출처 이미지 | 6 | cover.svg, outsider.png, 배지 SVG 4 |
| **FontAwesome (외부)** | **~9** | kit.js → pro/v4-shims/v5-font-face/v4-font-face CSS + kit-upload.css + **woff2 웹폰트 3** |
| **GitHub Sponsors (외부)** | 2 | sponsors 버튼 iframe + github assets CSS |
| **GTM + GA4 (외부)** | 4 | gtm.js, gtag ×2, `g/collect` |
| 기타 | — | `episodes-*.js` 가 2회 요청(프리페치 중복) |

- **구조적 문제:** 위 외부 요청 다수가 **`theme.config.js` 푸터**와 **`_app.tsx` 분석 스크립트**에서 오므로 **전 페이지 공통 비용**이다. 페이지 콘텐츠는 가벼운데 "껍데기"가 무겁다.

## Core Web Vitals (Lab)

| 조건 | LCP | CLS | 비고 |
| --- | --- | --- | --- |
| 데스크톱 / 스로틀 없음 | 363 ms | 0.00 | 렌더 지연 284 ms 가 대부분 |
| 모바일 / Slow 4G / CPU 4× | 419 ms | 0.00(트레이스) | 로드지연 118 ms + 렌더지연 297 ms |
| **Lighthouse 모바일** | — | **0.25 (불량)** | 히어로 이미지 높이 미예약으로 인한 시프트 |

- LCP 자체는 로컬 측정상 낮지만(↑ 한계 참고: TTFB≈1ms), **렌더 지연 비중이 큰 점**과 **CLS 0.25** 가 실질 신호다.
- **CLS 원인:** `<Image src="/images/cover.svg" width={0} height={0} style={{width:'100%'}}/>` — 가로만 100%, 세로 예약이 없어 이미지 디코드 후 레이아웃이 밀린다. → `width`/`height` 에 실제 비율을 주거나 `aspect-ratio` 로 공간 예약 필요.

## Lighthouse (모바일, navigation)

| 카테고리 | 점수 |
| --- | --- |
| Accessibility | **94** |
| Best Practices | 100 *(로컬 HTTP·무압축 환경이라 후하게 나온 값일 수 있음)* |
| SEO | 100 |
| Agentic Browsing | 25 |

**실패한 감사 항목:**
- 접근성 `link-name` (가중치 7) — **식별 가능한 이름이 없는 링크.** 푸터의 RSS 아이콘 링크(`<a href="/feed.xml"><i class="fa-rss"/></a>`) 등 아이콘만 있고 텍스트/`aria-label` 이 없는 링크. → `aria-label` 추가.
- 접근성 `landmark-one-main` (가중치 3) — `<main>` 랜드마크 없음(테마 구조 영향).
- Agentic `cumulative-layout-shift` — CLS 0.25(위 참조).
- Agentic `agent-accessibility-tree` — 접근성 트리 비정형(위 a11y 이슈와 연관).

## 서드파티 영향 (메인 스레드, CPU 4× 기준)

- **Google Tag Manager: 176 ms** (최대) — `lazyOnload` 라 LCP 는 막지 않으나 TBT/INP 에 영향.
- FontAwesome CDN: 25 ms.

## 개선 우선순위 (요약 — 상세 백로그는 [TODO.md](./TODO.md))

1. **이미지 다이어트(최대 효과):** `cover.svg` 최적화(SVGO)하거나 적정 크기 래스터(WebP/AVIF)로 교체. `outsider.png` 를 표시 크기(≈240px@2x)로 리사이즈+WebP.
2. **CLS 제거:** 히어로 이미지에 실제 가로·세로(또는 `aspect-ratio`) 부여.
3. **FontAwesome 경량화:** Pro 킷 풀로드 → 사용하는 아이콘 4개만 SVG 인라인 또는 부분 번들. 외부 요청 ~9개 제거 가능.
4. **서드파티 지연/축소:** GitHub Sponsors iframe 지연 로드(또는 정적 링크 대체), GTM/GA 필요성 재검토.
5. **접근성:** 아이콘 링크에 `aria-label`, `<main>` 랜드마크 확보.
6. **운영 호스트 확인(미검증):** gzip/brotli 압축과 정적 자산 캐시 헤더가 실제 호스트에서 켜져 있는지 확인. (로컬에선 검증 불가)
7. (경미) `episodes-*.js` 중복 프리페치, `_app.tsx → _app.mdx` 권고, `caniuse-lite` 갱신.

## 재현 방법

```bash
# (현재는 Player.tsx 때문에 빌드가 실패하므로 먼저 해결 필요 — QUALITY_GATE.md 참고)
npm run build
cd dist && python3 -m http.server 4321
# 브라우저/DevTools 또는 Lighthouse 로 http://localhost:4321 측정
```
