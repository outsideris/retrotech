# Plan — Go 정적 생성기로 마이그레이션 (Nextra/Next 제거)

> 상태: **계획 (착수 전)** · 결정일 2026-06-16
> 관련 문서: [ARCHITECURE.md](../ARCHITECURE.md), [DESIGN.md](../DESIGN.md), [DEPLOYMENT.md](../DEPLOYMENT.md), [QUALITY_GATE.md](../QUALITY_GATE.md), [TODO.md](../TODO.md) Phase 6
> 참고 구현: `blog.outsider.ne.kr` — 같은 저자의 Go 정적 블로그 빌더(동일 패턴을 차용한다)

---

## 1. 배경 / 동기 (ADR — Context)

현재 사이트는 Next.js 13 + Nextra 2 **beta** + nextra-theme-blog 위에 올라가 있다. 콘텐츠는 사실상 정적이고 인터랙션은 다크모드 토글 하나뿐인데도:

- **프레임워크 버전 추적 부담.** Next 13→14→15, Nextra 2-beta→3(App Router 전환 포함)은 호환성 작업을 강요한다. beta 핀이라 더 불안정하다.
- **bit-rot 위험.** 시간이 지나면 `npm install` + 빌드가 깨져 **새 에피소드를 추가하지 못하게** 될 수 있다(배포된 정적 HTML 자체는 계속 서빙되지만, 재빌드가 막힌다).
- **불필요한 런타임 비용.** 정적 콘텐츠에 First Load JS ~104KB(React+Next 런타임)를 전 페이지에 싣는다. ([PERFORMANCE.md](../PERFORMANCE.md))
- **실제로 쓰는 MDX 기능이 거의 없다.** 에피소드 24편이 MDX에서 쓰는 건 `import Badges` 하나와 `<div className="refs">`뿐. 코드블록 0개, Nextra 콜아웃/탭/`_meta` 내비게이션 미사용. `next/image`는 akamai 로더 passthrough라 이미 no-op.

→ 이 사이트는 프레임워크를 걷어내기에 이상적인 케이스다.

## 2. 결정 (ADR — Decision)

**오픈소스 SSG 프레임워크(Astro·Eleventy 등)도 쓰지 않고, Go 표준 라이브러리 중심의 자체 정적 생성기를 만든다.** 같은 저자가 이미 운영 중인 `blog.outsider.ne.kr`의 Go 빌더 패턴을 그대로 차용한다.

- **언어:** Go (로컬 `go1.26.2` 확인, 참고 프로젝트와 동일).
- **범위:** **빌드/프레임워크 의존성만 제거.** GA4 분석과 GitHub Sponsors iframe은 **유지**(런타임 외부 자원 제거는 이번 범위 밖).
- **longevity 전략:** 의존성 최소화(2개) + `go.mod`/`go.sum` 고정 + Go 1 호환성 보장. 추가로 빌드를 GitHub Actions에서 수행하고 산출물만 업로드(아래 6장).

### 불변식 (절대 깨지면 안 되는 것)

| # | 불변식 | 이유 |
| --- | --- | --- |
| I1 | **URL 동일** — `/`, `/episodes`, `/episodes/{id}` | SEO·기존 링크·내부 참조 유지 |
| I2 | **`feed.xml` 가입자 안정성** — 각 항목의 `guid`·`enclosure.url`·`pubDate` 불변 | 실제 팟캐스트 피드. Apple/Spotify 등록 + 기존 구독자가 중복/유실되면 안 됨 |
| I3 | **"똑같은 형태"** — 시각적으로 현재와 동일 | 사용자 요구 |
| I4 | **정적 자산 경로 동일** — `/images/*`, `/badges/*`, `/favicon.*`, `/site.webmanifest`, `/robots.txt`, `/ads.txt`, `/_headers` | 캐시·매니페스트·OG·피드의 `itunes:image`가 절대경로로 참조 |

## 3. 왜 Go인가 (요약)

Node와 비교한 상세 논의는 대화 기록 참고. 핵심만:

- **longevity가 언어 차원에서 보장**(Go 1 호환성). npm 트리 bit-rot에서 자유롭다.
- **표준 라이브러리가 넓어** "최소 의존성"이 자연스럽다 — 템플릿(`html/template`)·XML(`encoding/xml`)·테스트(`testing`)가 전부 내장. 외부 의존성은 **마크다운·YAML 2개뿐**.
- **참고 구현이 이미 있다.** 같은 저자의 검증된 패턴·라이브러리 선택을 그대로 차용 → 위험·시간 절감.
- 단점(새 언어, 기존 `gen-rss.js`/vitest 폐기)은 수용. 27페이지 규모라 재작성 비용이 작다.

## 4. 참고 프로젝트에서 가져올 것 / 버릴 것

`blog.outsider.ne.kr`는 RetroTech보다 훨씬 크다(블로그 엔진·에디터·뉴스레터·데스크톱 앱 포함). **코어 SSG 파이프라인만** 가져온다.

| 차용 ✅ | 버림 ❌ (RetroTech 불필요) |
| --- | --- |
| `cmd/build` 진입점 + `BuildConfig` 패턴 | `cmd/app`, `cmd/chromacss`, 에디터/뉴스레터/데스크톱 |
| `internal/parser`: 프론트매터 분리 + goldmark 렌더 | chroma·goldmark-highlighting (**코드블록 0개**) |
| `internal/builder`: 템플릿 실행 + 페이지 생성 | `fogleman/gg`·`x/image` OG 이미지 생성 (정적 `cover.jpg` 사용) |
| `encoding/xml` 직접 마샬링으로 피드 생성 | 페이지네이션·카테고리·태그·댓글·사이드바 |
| 다크모드 인라인 스크립트(FOUC 방지) 패턴 | `tdewolff/minify` (선택 — 초기엔 생략 가능) |
| `ANALYTICS_ID` 환경변수로 GA 마커 주입 패턴 | `_redirects`/R2/`functions` (대용량 첨부 없음) |
| GitHub Actions 빌드 → `wrangler pages deploy dist` | |

## 5. 목표 아키텍처

### 5.1 의존성 (최종)

| 모듈 | 용도 | 비고 |
| --- | --- | --- |
| `github.com/yuin/goldmark` | 마크다운 → HTML | 외부 의존성 0인 단일 모듈 |
| `gopkg.in/yaml.v3` | 프론트매터 파싱 | |
| *(선택)* `github.com/tdewolff/minify/v2` | HTML/CSS 최소화 | Phase G에서 검토. 초기 생략 가능 |

그 외 전부 **Go 표준 라이브러리**(`html/template`, `encoding/xml`, `os`, `path/filepath`, `testing`). → 직접 의존성 **2개**.

### 5.2 디렉토리 구조 (목표)

```
retrotech/
├─ go.mod / go.sum
├─ cmd/
│  ├─ build/main.go        # 빌드 진입점: 설정 구성 → builder.Build(cfg) → dist/
│  └─ serve/main.go        # (선택) 로컬 미리보기 정적 서버
├─ internal/
│  ├─ parser/              # 프론트매터 분리 + goldmark 렌더 + Episode 모델
│  │  ├─ parser.go
│  │  └─ parser_test.go
│  └─ builder/
│     ├─ build.go          # Build(cfg) 오케스트레이션(dist 청소·페이지·피드·정적 복사)
│     ├─ render.go         # html/template 실행(home/episodes/episode/404)
│     ├─ badges.go         # Badges 컴포넌트 → HTML (props → <div class="badges">)
│     ├─ feed.go           # encoding/xml RSS2.0 + itunes (gen-rss.js 대체)
│     ├─ feed_test.go      # 골든: 현 feed.xml 과 항목 단위 동일
│     └─ *_test.go
├─ content/
│  └─ episodes/            # *.md (프론트매터 + 본문) — 기존 pages/episodes/*.mdx 이관
├─ templates/
│  ├─ home.html            # '/'  (커버 + 소개 + Badges + 에피소드 목록)
│  ├─ episodes.html        # '/episodes' (목록)
│  ├─ episode.html         # '/episodes/{id}'
│  └─ 404.html
├─ public/                 # 정적 자산(현 public/ 유지) — 내용을 dist/ 루트로 복사
│  ├─ images/ badges/ favicon.* site.webmanifest robots.txt ads.txt _headers
│  └─ css/style.css        # nextra-theme-blog 외형 복제 + 다크모드 변수 (신규)
├─ dist/                   # 빌드 산출물 (gitignore)
└─ docs/ …
```

> **자산 경로 주의(I4):** `public/` 의 내용은 `dist/` **루트**로 복사한다(`public/images/cover.svg` → `dist/images/cover.svg`). 참고 프로젝트처럼 `/static/` 접두사를 두지 **않는다** — 현재 URL(`/images/…`, `/badges/…`, 루트 파비콘)을 그대로 유지해야 하기 때문.

### 5.3 데이터 모델 (`internal/parser`)

```go
type Enclosure struct {
    URL  string `yaml:"url"`
    Size int64  `yaml:"size"`
}
type Frontmatter struct {
    Title        string    `yaml:"title"`        // 폴드 스칼라 ">" — feed/페이지에서 trim 정책 통일
    Date         string    `yaml:"date"`         // "2026/03/07" 문자열 그대로 보존(피드 pubDate 재현용)
    Description  string    `yaml:"description"`
    Description2 string    `yaml:"description2,omitempty"`
    Author       string    `yaml:"author"`
    Enclosure    Enclosure `yaml:"enclosure"`
    Duration     string    `yaml:"duration"`     // "MM:SS"
}
type Episode struct {
    Frontmatter
    ID          string // 파일명에서 확장자 제거 ("2g", "0", "250127-breaks")
    ContentHTML string // goldmark 렌더 결과
}
```

- `LoadEpisodes(dir)` → `index.*` 제외, 날짜 내림차순 정렬(동일 날짜는 ID 내림차순 — `gen-rss.js`의 `sortByDateDesc` 와 동일 규칙).
- `RenderMarkdown`: goldmark 기본 + GFM(필요 시) + `WithUnsafe()`(본문에 raw `<div class="refs">`·링크 보존). **하이라이팅·heading demoter는 불필요**(코드블록 없음, 본문 `#` 제목은 그대로 둠 — 6.2 변환 규칙 참고).

### 5.4 빌드 파이프라인 (`builder.Build`)

```
go run ./cmd/build
  1) dist/ 청소
  2) content/episodes/*.md 로드 → []Episode (날짜 내림차순)
  3) 페이지 렌더 → dist/
       /                → dist/index.html
       /episodes        → dist/episodes/index.html
       /episodes/{id}   → dist/episodes/{id}/index.html   ← 트레일링슬래시 동작은 Phase D에서 현 산출물과 대조
       404              → dist/404.html
  4) feed.xml 생성     → dist/feed.xml   (encoding/xml)
  5) public/ → dist/ 루트로 복사(정적 자산)
  6) (선택) HTML/CSS minify
  7) 링크/자산 무결성 점검(참고: builder.CheckAssetIntegrity)
```

### 5.5 RSS 패리티 (`feed.go`) — **최우선 리스크 (I2)**

`encoding/xml` 구조체로 RSS2.0 + `itunes` 네임스페이스를 직접 마샬링해 현 `gen-rss.js` 출력을 재현한다. **항목 단위 동일성**이 목표(바이트 단위 완전 동일은 nice-to-have, 가입자 안정성엔 불필요).

현 `gen-rss.js`가 정한 계약(반드시 보존):

| 필드 | 현재 규칙 | 비고 |
| --- | --- | --- |
| `guid` | 항목 `url` = `https://retrotech.outsider.dev/episodes/{id}` | **구독자 식별자 — 절대 변경 금지** |
| `pubDate` | `new Date("{date} 09:00")` → UTC 문자열 | TZ 의존성 존재 → Go도 **고정 TZ로 파싱**해 현 CI 출력과 일치시킴(골든으로 확정) |
| `enclosure` | `{url, size}` + `.mp3`→`audio/mpeg` 타입 추론 | url/length/type 보존 |
| `description` | `description` (+ `description2` 있으면 `\n` 연결) | |
| 채널 | `itunes:owner/author/image/explicit=no/category=Technology`, `language=ko` | |
| 항목 커스텀 | `duration`, `itunes:duration`, `itunes:explicit=no`, `itunes:author` | |
| 정렬 | 날짜 내림차순, 동일 날짜는 name 내림차순 | |

- **골든 테스트:** 마이그레이션 전 `node scripts/gen-rss.js`로 생성한 `feed.xml`을 픽스처로 커밋 → Go 생성 결과를 항목 단위(guid·enclosure·pubDate·title·itunes 필드)로 비교. 차이가 나면 가입자 영향 여부를 판단해 의도적 변경만 허용.

### 5.6 템플릿 & 페이지 (`html/template`)

- 참고 프로젝트의 `templates/*.html` + `RenderList`/`RenderPost` 패턴 차용. RetroTech는 페이지 종류가 적다(home/episodes/episode/404).
- `Badges`는 `badges.go`의 함수가 `template.HTML`(현 `Badges.tsx`와 동일 마크업: `<div class="badges">` + 플랫폼별 `<a><img></a>`, `google` 없으면 RSS 배지)을 반환 → 템플릿에 주입. 도메인 규칙은 [DESIGN.md](../DESIGN.md) 그대로.
- 공통 funcs: `{{year}}`(푸터 연도) 등.

### 5.7 "똑같은 형태" + 다크모드 — **노력의 핵심 (I3)**

- **CSS 복제:** `nextra-theme-blog/style.css`의 외형(본문 폭·타이포·헤더/푸터·다크 팔레트)을 `public/css/style.css`로 옮긴다. 현 `styles/main.css` 보정(배지 정렬·`.refs`·폰트)도 병합.
- **기준 캡처:** 마이그레이션 전 `npm run build` 산출물(또는 운영 사이트)의 렌더 HTML/computed CSS를 1회 캡처해 **시각 회귀 기준**으로 삼는다(스크린샷 diff).
- **다크모드:** 참고 프로젝트 패턴 — `<head>` 인라인 스크립트로 `localStorage` 테마를 첫 페인트 전 적용(FOUC 방지) + `color-scheme` + 토글 버튼. 프레임워크 불필요(~20줄).
- **푸터:** 호스트 프로필(`outsider.webp`), 인라인 SVG 아이콘(현 `Icons.tsx`를 정적 SVG로), GitHub Sponsors iframe, RSS 링크 — 현 `theme.config.js` 푸터와 동일.

### 5.8 분석(GA4) 주입

- 템플릿에 `<!-- @analytics -->` 마커 → `ANALYTICS_ID` 환경변수가 있을 때만 gtag 스니펫으로 치환(없으면 제거). 배포 빌드에서만 주입 → 로컬/CI 빌드는 GA-free(본인·CI 트래픽 오염 방지). 현 `_app.tsx`의 GA4(`G-PVJ12C7HR6`) 유지.

## 6. MDX → Markdown 변환 규칙 (`content/episodes/*.md`)

| 항목 | 변환 |
| --- | --- |
| `import Badges from 'components/Badges'` | **제거.** Badges는 본문 마커/플레이스홀더로 대체하고 빌더가 주입(예: `<!-- badges: apple=… youtube=… spotify=… -->` 또는 프론트매터 `badges:` 맵). 방식은 Phase C에서 확정 |
| `<Badges .../>` JSX | 위 마커로 대체 |
| `<div className="refs">` | `<div class="refs">` 로(JSX `className`→`class`) |
| refs 내부 마크다운 리스트 | **앞뒤 빈 줄 추가** — CommonMark는 raw HTML 블록 안 마크다운을 빈 줄로 구분해야 파싱(현 들여쓰기 방식과 다름). goldmark `WithUnsafe()`로 div는 통과 |
| 본문 `# 제목` | 그대로(페이지 `<h1>`로 렌더). 참고 프로젝트의 heading demoter는 RetroTech엔 불필요 |
| 프론트매터 | 스키마 동일(아래 enclosure 등). 폴드 `>` title 처리만 정책 통일 |

> raw `<...>` 텍스트(예: 본문의 `<angular…>` 2곳)는 plain Markdown이 MDX보다 너그럽게 처리한다(이점).

## 7. 단계별 계획 (Phase) — TODO.md Phase 6과 연동

각 Phase는 **독립 커밋 단위**다. 안전을 위해 **Go 생성기를 기존 Next와 병행 구축** → 패리티 달성 후 마지막에 Next 제거(11장 병행/롤백).

- **A. 스캐폴딩** — `go.mod`(module `github.com/outsideris/retrotech`), `cmd/build` 골격, `internal/parser`(프론트매터 분리 + goldmark). 파일럿으로 에피소드 1편(`2g`)만 `.md` 변환해 파싱 확인. 단위 테스트.
- **B. RSS 패리티 (최우선)** — `feed.go` + 골든 테스트. 현 `gen-rss.js` 출력과 24편 전부 항목 단위 동일 확인. **여기서 막히면 이후 진행 보류.**
- **C. Badges + 콘텐츠 변환 규칙 확정** — `badges.go` + Badges 마커 방식 결정. 에피소드 1편으로 본문/refs/badges 렌더 검증.
- **D. 템플릿 & 페이지 생성** — home/episodes/episode/404 `html/template`. 전 페이지 `dist/` 생성. **현 산출물과 URL·파일 경로·트레일링슬래시 대조(I1).**
- **E. "똑같은 형태"(CSS·다크모드·푸터)** — 테마 CSS 복제 + 다크모드 + GA 마커. 스크린샷 diff로 시각 패리티 확인(I3).
- **F. 콘텐츠 일괄 이관** — 24편 `.mdx`→`content/episodes/*.md` 전량 변환. 골든·시각 회귀 재확인.
- **G. 빌드/배포/CI 전환** — `_headers`/robots/ads 복사, (선택)minify, `ci.yml`을 `go vet`+`go test`+`go run ./cmd/build`로, 배포를 GitHub Actions 빌드 → `wrangler pages deploy dist`로. [DEPLOYMENT.md](../DEPLOYMENT.md) 갱신.
- **H. Next 제거 & 문서 정리** — `pages/`·`components/`·`theme.config.js`·`next.config.js`·`package.json`·`node_modules`·vitest 제거. ARCHITECURE/DESIGN/QUALITY_GATE/TESTS 갱신.

## 8. 검증 / 품질 게이트 (신규 — Go 기준)

이관 후 [QUALITY_GATE.md](../QUALITY_GATE.md)를 Go 기준으로 교체:

- `go build ./...` / `go vet ./...` 통과
- `go test ./...` 통과 — 특히 **feed 골든**(I2)·parser 단위·badges 렌더
- `go run ./cmd/build` 성공 → `dist/` HTML 27p + `feed.xml` + 정적 자산 생성
- **feed 패리티:** 현 `feed.xml`과 항목 단위 동일(또는 의도된 차이만)
- **시각 패리티:** 주요 페이지 스크린샷 diff(I3)
- **링크/자산 무결성:** `dist/` 내 로컬 참조가 실제 파일로 해석되는지 점검
- 수동 구동: `go run ./cmd/serve`(또는 `python3 -m http.server`)로 홈·에피소드·다크모드 확인

## 9. 위험 & 오픈 결정

| 위험/결정 | 대응 / 기본값 |
| --- | --- |
| **R1. feed 가입자 영향(I2)** | guid·enclosure·pubDate 골든 고정. pubDate TZ 차이는 골든으로 확정해 동일 출력 보장 |
| **R2. URL/트레일링슬래시 불일치(I1)** | Phase D에서 현 `dist/` 1회 캡처해 emit 경로를 정확히 맞춤. 필요 시 `_redirects` 보강 |
| **R3. "똑같은 형태" 허용 오차(I3)** | 기본값: **눈으로 동일**(스크린샷 diff 통과). 픽셀 단위 100% 동일까지 요구하면 비용 증가 — 사용자 확인 필요 |
| **D1. Badges 콘텐츠 표현 방식** | 프론트매터 `badges:` 맵 vs 본문 HTML 주석 마커 — Phase C에서 결정 |
| **D2. 배포 방식** | GitHub Actions 빌드 → `wrangler pages deploy dist`(참고 프로젝트와 동일). 현 Cloudflare git-build에서 전환 — 시크릿(`CLOUDFLARE_API_TOKEN`) 필요 |
| **D3. minify 도입 여부** | 선택. 현 brotli로 충분하면 생략 가능(의존성 0 유지) |

## 10. 병행 / 롤백 전략

- A~F는 **기존 Next 사이트를 건드리지 않고** Go 생성기를 같은 저장소에 병행 구축한다(`cmd/`·`internal/`·`content/`·`templates/` 추가, `pages/` 유지).
- 패리티(피드 골든 + 시각 diff)가 통과한 뒤에만 G/H에서 배포를 전환하고 Next를 제거한다.
- 문제 발생 시 배포 설정만 되돌리면 기존 Next 빌드로 즉시 복귀 가능(H 이전까지).

## 11. 완료 기준

- [ ] `go test ./...` 통과(feed 골든 포함), `go run ./cmd/build` 성공
- [ ] `feed.xml` 가입자 계약(I2) 보존 확인
- [ ] URL(I1)·정적 자산 경로(I4) 동일 확인
- [ ] 시각 패리티(I3) 확인
- [ ] 배포 전환 + CI 전환 완료
- [ ] Next/Nextra/React/npm 의존성 제거, 문서 갱신
