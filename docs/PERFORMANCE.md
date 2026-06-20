# Performance — RetroTech (감사 결과)

> 최초 측정 2026-06-14(마이그레이션 전 Next/Nextra). **2026-06-16 전면 갱신** — Go 정적 생성기 전환 + 접근성/성능 최적화 반영.
> 구조·구성은 [ARCHITECURE.md](./ARCHITECURE.md), 배포/호스팅은 [DEPLOYMENT.md](./DEPLOYMENT.md), 개선 백로그는 [TODO.md](./TODO.md).

## 현재 상태 (마이그레이션 후)

Nextra/Next.js → 자체 Go 정적 생성기로 전환하며 이전 감사의 성능 부채 대부분이 **구조적으로 사라졌다**:

- **브라우저 프레임워크 JS 제거** — 이전 홈 First Load ~104KB(framework/main/webpack 청크) → 다크모드 토글 인라인 스크립트 한 조각만 남음.
- **FontAwesome 킷 제거** — 외부 요청 ~9개 + 웹폰트 3개 → 4개 아이콘 인라인 SVG.
- **GTM 제거** — GA4 만 직접 주입(운영 배포에서 `ANALYTICS_ID` 설정 시에만, 프리뷰/로컬/CI 는 분석 코드 미포함).
- **이미지 다이어트(마이그레이션 전 적용분 유지)** — `cover.svg` SVGO 402→143KB, 푸터 사진 PNG 110KB→WebP 5KB.

그 위에 이번 라운드에서 접근성·성능을 마저 끌어올렸다(아래).

## 2026-06-16 적용 — 접근성·성능 최적화

> 사용자 요청 "성능/접근성 최적화" → 분석 후 **P3·P4 제외** 전부 적용. 각 항목 개별 커밋.

| 항목 | 영역 | 변경 | 커밋 |
| --- | --- | --- | --- |
| A1 | 접근성 | `<article id="content" role="main">` — 페이지당 정확히 하나의 main 랜드마크 | `49cd000` |
| A3 | 접근성 | "본문 바로가기" skip 링크(`#content`) | `49cd000` |
| A2 | 접근성 | 다크모드 토글 키보드 조작(Enter/Space, `role`/`tabindex`) | `da6a969` |
| P1 | 성능 | GitHub Sponsors iframe `loading="lazy"` | `e6313a3` |
| P2 | 성능(LCP) | 홈·404 히어로 `cover.svg` `<link rel=preload as=image fetchpriority=high>` | `9d73877` |
| P5 | 성능(캐시) | `_headers` 에 `/images/*`(1주)·`/badges/*`(30일) 추가 | `9d34e88` |
| A4 | 접근성 | 에피소드 제목 계층 정리(`#### 레퍼런스:`/`배경음악` → `##`, h2·h3 건너뜀 제거) | `ac6da63` |
| P6 | 성능(CLS) | 배지 `<img>` `height="0"` → SVG 비율 기반 실제 높이 예약 | `a55cd8d` |

### 보류 (의도적)

- **P3 — `cover.svg` 래스터화(AVIF/WebP).** 콜드 첫 방문 LCP 의 디코드/전송 비용을 더 줄일 수 있으나, 벡터 일러스트를 래스터로 바꾸면 시각 동등성·해상도 트레이드오프가 생기고 현재 LCP 도 양호 등급이라 보류. (SVGO 로 143KB 까지는 이미 줄임.)
- **P4 — 사용하지 않는 테마 CSS purge.** 재사용한 Nextra 테마 스타일시트에서 죽은 규칙을 더 들어낼 수 있으나(이미 `.nextra-*` 27개·dead 규칙 제거함), 추가 purge 는 시각 회귀 위험 대비 이득이 작아 보류. 현 스타일시트는 콘텐츠 해시(`styles.<hash>.css`)로 1년 immutable 캐시.

## Lighthouse (로컬 빌드 `dist/`, 모바일)

> 로컬 정적 서버(`cmd/serve`)가 배포본과 동일한 `dist/` 산출물을 제공. 접근성·모범사례·SEO·Agentic 은 마크업/레이아웃 기반이라 로컬·운영이 동일하다(네트워크 타이밍 무관). 운영 배포 후 한 번 더 확인 권장.

| 카테고리 | 홈 `/` | 에피소드 `/episodes/2g` |
| --- | --- | --- |
| Accessibility | **100** | **100** |
| Best Practices | **100** | **100** |
| SEO | **100** | **100** |
| Agentic Browsing | **100** | **100** |
| (통과/실패) | 54 / 0 | 55 / 0 |

이전(운영, 마이그레이션 전): 접근성 94(`link-name`+`landmark-one-main` 실패), Agentic 25(CLS 0.25 등). 마이그레이션(인라인 SVG 아이콘 → `link-name` 해소) + A1(main 랜드마크) 로 접근성 100, P6(배지 높이 예약) 로 에피소드 CLS 0.093→0 → Agentic 100.

## Core Web Vitals

| 지표 | 값 | 비고 |
| --- | --- | --- |
| **CLS** | **0.00** (홈·에피소드 모두) | 홈은 본래 0, 에피소드는 배지 0.093 → P6 로 0 |
| **LCP** | 양호 등급(<2.5s) | 히어로 `cover.svg`. P2 preload 로 발견 지연 단축. 로컬 TTFB≈0 이라 절대값은 낙관적 — 운영 절대값은 배포 후 측정 |

> LCP 절대값은 로컬(TTFB≈0)에서 낙관적으로 나오므로 운영 재측정 대상이다. 단 **CLS 는 레이아웃 안정성 지표라 로컬·운영이 같고**, P6 이후 두 페이지 모두 0 이다.

## 캐시 정책 (`public/_headers`, Cloudflare Pages 자동 적용)

| 경로 | 정책 | 근거 |
| --- | --- | --- |
| `/assets/*` (해시 스타일시트) | `max-age=31536000, immutable` | 내용 바뀌면 파일명(해시)도 바뀜 → 영구 캐시 안전 |
| `/badges/*` | `max-age=2592000` (30일) | 플랫폼 배지, 사실상 불변 |
| `/images/*` | `max-age=604800` (1주) | URL 고정이라 immutable 대신 적당 TTL(갱신 시 최대 1주 stale 또는 purge) |
| 파비콘류 | 기본 TTL | 작고 브라우저 캐시됨 |

> 마이그레이션 전 문제였던 "`/_next/static/*` 도 4시간뿐"은 해당 경로 자체가 사라져 무효. 콘텐츠 해시 자산은 `/assets/*` 로 옮겨 1년 immutable.

## 이미 좋은 점 (유지)

- 프레임워크 런타임 JS 없음(정적 HTML + 인라인 토글 스크립트 한 조각).
- 정적 산출물 — Cloudflare 엣지에서 Brotli + HTTP/3 전송.
- 콘텐츠 해시 스타일시트 1년 immutable, SEO·Best Practices 100.
- 히어로 `cover.svg` SVGO(143KB), 푸터 WebP(5KB).

## 재현 방법

```bash
# 빌드 후 로컬 서버 기동(빈 포트 자동 선택; $PORT 로 고정 가능)
go run ./cmd/build && PORT=8099 go run ./cmd/serve
# Lighthouse: 모바일, http://localhost:8099/ 및 /episodes/<id>
# 운영 절대 타이밍(LCP/TTFB)은 배포 후 https://retrotech.outsider.dev 에서 DevTools 트레이스로 측정
curl -sI -H 'Accept-Encoding: br, gzip' https://retrotech.outsider.dev/assets/<hash>.css   # 캐시 헤더 확인
```
