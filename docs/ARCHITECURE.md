# Architecture — RetroTech

> 기술의 역사를 다루는 한국어 팟캐스트 **RetroTech** 의 웹사이트.
> 이 문서는 코드를 모두 읽지 않고도 구조와 구성을 파악할 수 있도록 정리한 참조 문서다.
> 성능 관련 상세는 [PERFORMANCE.md](./PERFORMANCE.md), 기획/UX 의도는 [DESIGN.md](./DESIGN.md) 참고.

## 한눈에 보기

- **정적 사이트.** Next.js 의 정적 익스포트(`output: 'export'`)로 빌드되어 순수 HTML/CSS/JS 산출물(`dist/`)만 배포된다. 런타임 서버나 API가 없다.
- **콘텐츠 = MDX 파일.** 에피소드 한 편이 `pages/episodes/*.mdx` 파일 하나에 대응한다. 프론트매터(frontmatter)가 메타데이터, 본문이 쇼노트다.
- **테마는 Nextra 블로그 테마.** 레이아웃·목록·다크모드·스타일 대부분을 `nextra-theme-blog` 가 제공한다. 우리가 직접 만든 건 `theme.config.js`(헤더/푸터), 배지 컴포넌트, 약간의 CSS 뿐이다.
- **RSS 피드를 빌드 시 생성.** `scripts/gen-rss.js` 가 에피소드 프론트매터를 읽어 iTunes 팟캐스트 규격 `feed.xml` 을 만든다. 이 피드가 Apple/Spotify 등에 등록되는 실제 팟캐스트 피드다.

## 기술 스택

| 영역 | 사용 기술 | 버전 |
| --- | --- | --- |
| 프레임워크 | Next.js (Pages Router) | `^13.4.9` |
| 콘텐츠/MDX | Nextra | `2.0.0-beta.5` |
| 테마 | nextra-theme-blog | `2.0.0-beta.5` |
| UI 런타임 | React / React DOM | `^18.2.0` |
| 프론트매터 파싱 | gray-matter | `^4.0.3` *(빌드 스크립트 전용)* |
| RSS 생성 | rss | `^1.2.2` *(빌드 스크립트 전용)* |
| 언어/타입 | TypeScript | `^5.1.6` (`strict: false`, `target: es5`) |

> ⚠️ Next 13 / Nextra 2 **beta** 핀이다. 메이저 업그레이드(Next 14/15, Nextra 3, App Router 전환)는 호환성 작업이 필요하다.

## 디렉터리 구조

```
retrotech/
├─ pages/                     # 파일 기반 라우팅 = 사이트의 모든 페이지
│  ├─ index.mdx               # 홈 ('/'): 커버 + 소개 + 구독 배지 + 에피소드 목록
│  ├─ _app.tsx                # 전역 래퍼: CSS import, RSS <link>, GTM/GA 스크립트
│  ├─ _document.tsx           # <html lang="ko">, 메타태그, GTM noscript, FontAwesome kit
│  └─ episodes/
│     ├─ index.md             # '/episodes' 목록 페이지
│     ├─ 0.mdx, 1a.mdx … 2g.mdx   # 에피소드 본문(쇼노트) + 프론트매터
│     └─ 250127-breaks.mdx    # 비정규 에피소드(쉬어가는 회차)
├─ components/
│  └─ Badges.tsx              # 구독 플랫폼 배지(Apple/YouTube/Spotify/Google|RSS)
├─ scripts/
│  └─ gen-rss.js              # 빌드 시 public/feed.xml 생성
├─ styles/
│  └─ main.css                # 전역 보정 CSS (배지 레이아웃, 폰트, refs 등)
├─ public/                    # 정적 자산. 빌드 시 dist/ 루트로 복사됨
│  ├─ images/                 # cover.svg, cover.jpg, outsider.png
│  ├─ badges/                 # apple/youtube/spotify/google/rss .svg
│  ├─ favicon.ico, favicon-*.png, apple-touch-icon.png, android-chrome-*.png, site.webmanifest  # 파비콘/PWA
│  ├─ robots.txt, ads.txt
│  └─ (feed.xml)              # 빌드가 생성 — 저장소엔 커밋되지 않음
├─ next.config.js             # 정적 익스포트 + Nextra + 이미지 로더 설정
├─ theme.config.js            # Nextra 블로그 테마 헤더/푸터/메타 커스터마이즈
├─ tsconfig.json
└─ dist/                      # 빌드 산출물(gitignore). 배포 대상.
```

## 라우팅 & 콘텐츠 모델

- **파일 기반 라우팅.** `pages/` 아래 파일 경로가 곧 URL이다. `pages/episodes/2g.mdx` → `/episodes/2g`.
- **에피소드 식별자 규칙.** `시즌숫자 + 알파벳` 형태(`1a`~`1n`, `2a`~`2g`). `0.mdx` 는 0화(소개/예고). 날짜 기반(`250127-breaks`)은 정규 시즌 외 회차.
- **에피소드 프론트매터 스키마** (`gen-rss.js` 와 본문이 함께 사용):

  ```yaml
  ---
  title: >                    # 멀티라인 제목 (예: "2g. VCS: SourceForge")
      2g. VCS: SourceForge
  date: 2026/03/07            # YYYY/MM/DD. RSS에서 "<date> 09:00" 으로 발행시각 구성
  description: |              # 요약(여러 줄). 본문 상단과 RSS description에 사용
      ...
  description2: |             # (선택) RSS description에만 줄바꿈으로 덧붙는 보조 설명
      ...
  author: Outsider
  enclosure:                  # 팟캐스트 오디오 첨부
    url: https://retrotech-episodes.outsider.dev/2g.mp3
    size: 66997696            # 바이트 단위 파일 크기
  duration: "55:50"           # "MM:SS" — RSS의 duration / itunes:duration
  ---
  ```

  - 본문에서는 `import Badges from 'components/Badges'` 후 `<Badges .../>` 로 회차별 구독 링크를 렌더링하고, `#### 레퍼런스:` 아래 `<div className="refs">` 로 참고자료 목록을 둔다.
- **목록 페이지.** `index.mdx`(홈)와 `episodes/index.md` 는 프론트매터 `type: posts` 로 지정되어 Nextra 블로그 테마가 에피소드 목록을 자동 렌더링한다.

## 빌드 파이프라인

```
npm run build
  └─ 1) node ./scripts/gen-rss.js   # pages/episodes/* 읽어 public/feed.xml 작성
  └─ 2) next build                  # output:'export' → dist/ 에 정적 HTML 27개 + 자산 익스포트
```

- 산출물: `dist/` (HTML 27개 = 홈 + /episodes + 에피소드 24개 + /404). 자세한 크기는 [PERFORMANCE.md](./PERFORMANCE.md).
- 정적 익스포트이므로 SSR/ISR/API Routes/미들웨어는 사용할 수 없다. 모든 페이지는 빌드 타임에 정적으로 렌더된다(`○ (Static)`).

## 주요 구성 파일

### `next.config.js`
```js
const nextConfig = {
  output: 'export',     // 정적 HTML 익스포트 (서버 없이 배포)
  distDir: 'dist',      // 기본 .next 대신 dist 로 출력
  images: {
    loader: 'akamai',   // next/image 커스텀 로더
    path: '/'
  }
}
module.exports = withNextra(nextConfig)
```
- **`images.loader: 'akamai'` 의 의미가 중요하다.** 정적 익스포트에서는 Next 의 기본 이미지 최적화 서버가 없으므로, `next/image` 는 akamai 로더를 통해 **원본 경로를 그대로** 출력한다(`src="/images/cover.svg"`). 즉 `next/image` 를 써도 **리사이즈·포맷 변환·압축이 일어나지 않으며** 사실상 일반 `<img>` 와 같다. → 이미지 최적화는 우리가 자산을 직접 줄여야만 한다. (성능 영향: [PERFORMANCE.md](./PERFORMANCE.md))

### `theme.config.js` (Nextra 블로그 테마 커스터마이즈)
- `darkMode: true`.
- `head()`: 페이지 제목으로 `og:title` / `twitter:title` 구성. 홈은 "RetroTech 팟캐스트", 그 외는 `"<제목> - RetroTech"`.
- `footer`: **모든 페이지 공통**으로 렌더되는 푸터. GitHub Sponsors 버튼 `<iframe>`, 호스트(Outsider) 프로필 이미지, FontAwesome 아이콘 링크(트위터/깃헙/블로그), 연도 + RSS 링크를 포함한다. → 이 푸터가 외부 의존성(FontAwesome, GitHub iframe)을 전 페이지에 끌어온다.

### `pages/_app.tsx`
- `nextra-theme-blog/style.css` + `styles/main.css` 전역 적용.
- `<Head>` 에 RSS 자동발견용 `<link rel="alternate" type="application/rss+xml" href="/feed.xml">`.
- **Google Tag Manager**(`GTM-P368DQ3M`)와 **GA4**(`G-PVJ12C7HR6`) 스크립트를 `next/script` 의 `strategy="lazyOnload"` 로 주입.

### `pages/_document.tsx`
- `<html lang="ko">`. SEO/OG/Twitter 메타태그(설명, 커버 이미지 `https://retrotech.outsider.dev/images/cover.jpg`).
- GTM `<noscript>` iframe.
- **FontAwesome Kit** 스크립트(`https://kit.fontawesome.com/bba8fa6a15.js`)를 `<body>` 끝에서 로드. 푸터 아이콘 몇 개를 위해 Pro 킷 전체를 불러온다.
- **파비콘/PWA 아이콘 `<link>`** — favicon(16/32), `apple-touch-icon`, `site.webmanifest`. 아이콘 자산은 `public/` 루트에 위치하며 매니페스트도 루트 경로를 참조한다.

### `tsconfig.json`
- `strict: false`, `target: es5`, `jsx: preserve`, `allowJs: true`. 타입 안전성은 느슨하게 설정됨.
- `include: ["**/*.ts", "**/*.tsx"]` → `components/` 의 미사용 파일도 `next build` 의 타입체크 대상이다(import 되지 않아도 타입에러가 빌드를 깨뜨릴 수 있음).

## 스타일링

- 기본 레이아웃/타이포그래피는 `nextra-theme-blog/style.css`.
- `styles/main.css` 는 보정용: 본문 폰트(`Apple SD Gothic Neo` 계열 한글 우선), `.badges` 배지 정렬, `.refs` 참고자료 글자 크기, `h5` 크기.
- 컴포넌트 단위 스타일은 `theme.config.js` 푸터처럼 styled-jsx(`<style jsx>`) 로 인라인.

## 외부 의존성 / 통합

| 통합 | 위치 | 용도 |
| --- | --- | --- |
| **FontAwesome Kit (Pro)** | `_document.tsx` | 푸터 소셜/RSS 아이콘. 킷 JS + CSS + 웹폰트를 외부 CDN에서 로드 |
| **Google Tag Manager** | `_app.tsx` | 태그 관리 (`GTM-P368DQ3M`) |
| **Google Analytics 4** | `_app.tsx` | 방문 분석 (`G-PVJ12C7HR6`) |
| **GitHub Sponsors** | `theme.config.js` 푸터 | 후원 버튼 `<iframe>` (전 페이지) |
| **팟캐스트 플랫폼** | `components/Badges.tsx`, 각 에피소드 | Apple Podcasts / Spotify / YouTube / Google / RSS 구독 링크 |
| **오디오 호스팅** | 프론트매터 `enclosure.url` | `https://retrotech-episodes.outsider.dev/*.mp3` (mp3 별도 호스팅) |

## RSS / 팟캐스트 피드 (`scripts/gen-rss.js`)

- `pages/episodes/` 를 읽어 `.mdx`/`.md` 프론트매터를 `gray-matter` 로 파싱, `rss` 라이브러리로 `public/feed.xml` 생성.
- iTunes 네임스페이스(`itunes:owner`, `itunes:author`, `itunes:image`, `itunes:category=Technology`, `itunes:explicit=no`, `itunes:duration`) 포함 → Apple Podcasts 등록 규격을 만족.
- 각 아이템: 제목, `url`(웹 에피소드 페이지), `date`(`<date> 09:00`), `description`(+`description2`), `enclosure`(mp3), `duration`.
- `index.*` 파일은 건너뛴다. 에피소드 배열을 `reverse()` 하여 피드 순서를 맞춘다.
- **`SITE_URL = 'https://retrotech.outsider.dev'`** 가 스크립트에 하드코딩되어 있다.

## 배포

- **정적 호스팅.** `dist/` 를 정적 호스트에 업로드하는 형태(서버리스). 운영 도메인: `https://retrotech.outsider.dev`.
- 오디오 파일(mp3)은 사이트와 분리된 `retrotech-episodes.outsider.dev` 에 호스팅된다.
- `dist/` 는 `.gitignore` 대상이라 저장소에 포함되지 않는다(매 배포 시 빌드).

## 알려진 제약 / 주의사항 (작업 전 반드시 확인)

1. **`next/image` 는 이 구성에서 최적화를 하지 않는다**(akamai 로더 passthrough). 이미지 용량은 원본 그대로 전송된다. → [PERFORMANCE.md](./PERFORMANCE.md)
2. **`public/feed.xml` 은 빌드 산출물인데 gitignore 되어 있지 않다.** 로컬 빌드 후 untracked 파일로 남는다. `/dist/feed.xml` 로도 익스포트되므로 `public/feed.xml` 은 `.gitignore` 에 추가하는 편이 깔끔하다.
3. **Nextra 권고:** 빌드 시 `Found "_app.tsx" file, refactor it to "_app.mdx" for better performance.` 힌트가 출력된다.
4. **`caniuse-lite` 가 오래됨** — 빌드 시 browserslist 경고. `npx update-browserslist-db@latest` 권장.

## 테스트

- 현재 자동화 테스트 없음. 상세는 [TESTS.md](./TESTS.md), 검증 기준은 [QUALITY_GATE.md](./QUALITY_GATE.md).
