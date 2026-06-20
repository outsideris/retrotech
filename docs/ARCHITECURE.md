# Architecture — RetroTech

> 기술의 역사를 다루는 한국어 팟캐스트 **RetroTech** 의 웹사이트.
> 이 문서는 코드를 모두 읽지 않고도 구조와 구성을 파악할 수 있도록 정리한 참조 문서다.
> 성능 관련 상세는 [PERFORMANCE.md](./PERFORMANCE.md), 기획/UX 의도는 [DESIGN.md](./DESIGN.md), Go 마이그레이션 배경은 [plan/go-static-migration.md](./plan/go-static-migration.md) 참고.

## 한눈에 보기

- **자체 제작 Go 정적 생성기.** `go run ./cmd/build` 가 `content/` 와 `public/` 를 읽어 순수 HTML/CSS 산출물(`dist/`)을 만든다. 런타임 서버·API 가 없고, **브라우저로 가는 프레임워크 JS 도 없다**(다크모드 토글용 인라인 스크립트뿐).
- **콘텐츠 = 마크다운 파일.** 에피소드 한 편이 `content/episodes/*.md` 파일 하나에 대응한다. 프론트매터가 메타데이터, 본문이 쇼노트다.
- **외부 의존성 2개.** `goldmark`(마크다운)·`yaml.v3`(프론트매터)뿐. 템플릿·RSS·파일 처리·테스트는 Go 표준 라이브러리.
- **RSS 피드를 빌드 시 생성.** `internal/builder/feed.go` 가 에피소드 프론트매터를 읽어 iTunes 팟캐스트 규격 `feed.xml` 을 만든다. 이 피드가 Apple/Spotify 등에 등록되는 실제 팟캐스트 피드다.

> 이 구조는 Next.js 13 + Nextra 2(beta) 블로그 테마에서 마이그레이션한 결과다. 시각·동작은 이전과 동일하게 유지하되 프레임워크 의존성과 런타임 JS 를 제거했다. 마이그레이션 계획·불변식은 [plan/go-static-migration.md](./plan/go-static-migration.md).

## 기술 스택

| 영역 | 사용 기술 | 비고 |
| --- | --- | --- |
| 언어/런타임 | Go | `go.mod` 의 `go 1.26.2` |
| 마크다운 | `github.com/yuin/goldmark` (+ GFM 확장) | 외부 의존성 0인 단일 모듈 |
| 프론트매터 | `gopkg.in/yaml.v3` | |
| 템플릿/HTML | 문자열 빌드 + `html` 표준 라이브러리 | 정밀한 출력 제어 |
| RSS | `encoding/xml` 규격을 문자열로 재현 | 기존 피드와 바이트 패리티 |
| 테스트 | `testing`(표준) | 피드 골든 + 단위 |

## 디렉터리 구조

```
retrotech/
├─ cmd/
│  ├─ build/main.go        # 빌드 진입점: content+public → dist (페이지·피드·자산)
│  └─ serve/main.go        # 로컬 미리보기 서버(clean URL 해석)
├─ internal/
│  ├─ parser/              # 프론트매터 분리·YAML 파싱·에피소드 로드/정렬
│  │  └─ parser.go         #   Frontmatter/Episode 모델, SortEpisodes
│  └─ builder/
│     ├─ feed.go           # iTunes RSS 피드 생성(encoding 문자열)
│     ├─ feed_test.go      #   골든 테스트(testdata/feed.golden.xml)
│     ├─ badges.go         # 구독 배지(Apple/YouTube/Spotify/Google|RSS) HTML
│     ├─ render.go         # 페이지 빌더 + goldmark + 프로즈 후처리
│     ├─ render_layout.go  # 페이지 셸·head·메타·footer·날짜
│     └─ render_assets.go  # 다크모드 스크립트·아이콘 SVG·인라인 스타일
├─ content/
│  └─ episodes/            # *.md (프론트매터 + 본문). 0, 1a…1n, 2a…2g, 250127-breaks
├─ public/                 # 정적 자산. 빌드가 dist/ 루트로 복사
│  ├─ images/ badges/ favicon.* site.webmanifest robots.txt ads.txt
│  ├─ styles.css           # 테마+보정 CSS 컴파일본(빌드가 /assets/styles.<hash>.css 로 핑거프린트)
│  └─ _headers             # Cloudflare Pages 응답 헤더(/assets/* 캐시) — DEPLOYMENT.md
├─ scripts/
│  └─ cf-build.sh          # Cloudflare 빌드 래퍼(go run ./cmd/build + 텔레그램 알림)
├─ go.mod / go.sum
└─ dist/                   # 빌드 산출물(gitignore). 배포 대상.
```

## 라우팅 & 콘텐츠 모델

- **빌드가 URL→파일을 결정한다.** `content/episodes/2g.md` → `dist/episodes/2g.html` → URL `/episodes/2g`(Cloudflare Pages 가 `.html` 생략 서빙). 평면 `.html` 파일을 emit 한다(이전 Next 정적 익스포트와 동일 경로).
  - `/` → `dist/index.html`, `/episodes` → `dist/episodes.html`, `404` → `dist/404.html`.
- **에피소드 식별자 규칙.** `시즌숫자 + 알파벳`(`1a`~`1n`, `2a`~`2g`). `0` 은 0화(소개/예고). 날짜 기반(`250127-breaks`)은 정규 시즌 외 회차. 파일명이 곧 id·slug 다.
- **에피소드 프론트매터 스키마**:

  ```yaml
  ---
  title: >                    # 멀티라인 제목 (예: "2g. VCS: SourceForge")
      2g. VCS: SourceForge
  date: 2026/03/07            # YYYY/MM/DD(0 미패딩 허용). 피드 pubDate 는 이 날짜 09:00 UTC
  description: |              # 요약(여러 줄). 목록·본문 상단·RSS description 에 사용
      ...
  description2: |             # (선택) RSS description 에만 줄바꿈으로 덧붙는 보조 설명
      ...
  author: Outsider
  enclosure:                  # 팟캐스트 오디오 첨부
    url: https://retrotech-episodes.outsider.dev/2g.mp3
    size: 66997696            # 바이트 단위 파일 크기
  duration: "55:50"           # "MM:SS" — RSS의 duration / itunes:duration
  badges:                     # 회차별 구독 딥링크 — 필드별로 비우면 그 플랫폼은
    apple: "..."              #   쇼/채널 링크로 폴백(홈 루트 아이콘과 동일).
    youtube: "..."            #   발행 직후엔 통째로 비워두고, 플랫폼에 에피소드가
    spotify: "..."            #   등록되면 딥링크를 하나씩 채운다.
    # google 이 있으면 Google 배지, 없으면 RSS 배지
  ---
  ```

  - 본문에서는 제목 h1 을 쓰지 않는다(템플릿이 프론트매터 title 로 emit). 구독 배지는 `<!--badges-->` 마커 위치에 주입되고, 레퍼런스는 `#### 레퍼런스:` 헤딩 + 일반 마크다운 리스트로 작성한다(본문에 raw HTML 불필요 — 빌더가 그 리스트를 `<div class="refs">` 로 감싸 작은 글씨로 렌더).
- **목록 페이지.** 홈(`/`)과 `/episodes` 는 `parser.LoadEpisodes` 가 반환한 날짜 내림차순 목록을 `post-item` 으로 렌더한다.

## 빌드 파이프라인

```
go run ./cmd/build
  └─ 1) dist/ 청소
  └─ 2) public/ → dist/ 복사(이미지·배지·파비콘·_headers·styles.css 등)
  └─ 3) styles.css → dist/assets/styles.<hash>.css 로 핑거프린트(immutable 캐시)
  └─ 4) content/episodes/*.md 로드 → []Episode (날짜 내림차순)
  └─ 5) 페이지 렌더 → dist/ (index, episodes, episodes/<id>, 404)
  └─ 6) feed.xml 생성 → dist/feed.xml
```

- 산출물: `dist/` (HTML 26개 = 홈 + /episodes + 에피소드 23개 + /404, + feed.xml + 자산). 빌드 ~30ms.
- SSR/ISR/API 가 없는 순수 정적 산출이다.

## 프로즈 렌더링(`internal/builder/render.go`)

마크다운 본문은 goldmark(GFM + raw-HTML 통과)로 렌더한 뒤, 이전 Nextra 테마와 동작을 맞추기 위해 후처리한다:

- **레퍼런스 래핑**: `#### 레퍼런스:` 헤딩 뒤의 리스트를 `<div class="refs">` 로 감싼다 — 본문은 순수 마크다운으로 두고 `.refs`(작은 글씨) 스타일은 빌더가 입힌다.
- **외부 링크**(`http(s)://`): `target="_blank" rel="noreferrer"` + 스크린리더용 "(opens in a new tab)" span.
- **마크다운 heading**(h2–h6): `subheading-h{n}` 클래스 + 퍼머링크 anchor(id 는 github-slugger 규칙).
- **`<!--badges-->` 마커**: `badges:` 프론트매터로 구성한 배지 블록으로 치환.

페이지 셸은 재사용한 테마 CSS(`/assets/styles.<hash>.css`)를 참조하고, footer 를 `nx-prose` article 안에 둔다(테마와 동일). 다크모드는 프레임워크 없이 인라인 스크립트 두 개(첫 페인트 전 테마 적용 + 토글 영속화)와 해/달 아이콘 스왑 CSS 로 구현한다.

## RSS / 팟캐스트 피드(`internal/builder/feed.go`)

- `content/episodes/` 의 프론트매터를 읽어 RSS 2.0 + `itunes` 네임스페이스 `feed.xml` 을 만든다(Apple Podcasts 규격).
- **이전 `scripts/gen-rss.js`(rss npm 라이브러리) 출력과 바이트 패리티**를 목표로 문자열로 재현한다(`encoding/xml` 은 CDATA·네임스페이스 순서·self-closing 을 그대로 못 냄). 휘발성 `lastBuildDate` 만 매 빌드 갱신.
- 구독자 계약(불변): 각 항목 `guid`(=`/episodes/{id}`)·`enclosure`·`pubDate`. `pubDate` 는 날짜 09:00 UTC(빌드 머신 TZ 무관, 결정적).
- 항목은 발행일 내림차순(동일 날짜 id 내림차순). `internal/builder/testdata/feed.golden.xml` 골든 테스트로 회귀 방지.
- **하드코딩:** `SITE_URL = 'https://retrotech.outsider.dev'`(`feed.go`/`cmd/build`).

## 외부 의존성 / 통합

| 통합 | 위치 | 용도 |
| --- | --- | --- |
| **Google Analytics 4** | `render_layout.go`(`<!-- @analytics -->` 주입) | 방문 분석(`G-PVJ12C7HR6`). `ANALYTICS_ID` 설정 시(배포)만 |
| **GitHub Sponsors** | `render_layout.go` footer | 후원 버튼 `<iframe>`(전 페이지) |
| **팟캐스트 플랫폼** | `badges.go`, 각 에피소드 `badges:` | Apple/Spotify/YouTube/Google/RSS 구독 링크 |
| **오디오 호스팅** | 프론트매터 `enclosure.url` | `retrotech-episodes.outsider.dev/*.mp3` |

## 배포

- **Cloudflare Pages** git 연동으로 빌드(`bash scripts/cf-build.sh` → `go run ./cmd/build`) 후 `dist/` 를 배포. 운영 도메인 `https://retrotech.outsider.dev`.
- 대시보드에 `GO_VERSION` 환경변수가 필요하고, 프로덕션에 `ANALYTICS_ID` 를 설정한다. 배포 알림(텔레그램) 포함 상세는 → [DEPLOYMENT.md](./DEPLOYMENT.md).
- 오디오(mp3)는 `retrotech-episodes.outsider.dev` 에 분리 호스팅.

## 알려진 제약 / 주의사항

1. **`SITE_URL` 하드코딩.** `feed.go` 와 `cmd/build` 에 도메인이 상수로 존재한다.
2. **이전 빌드 산출물과의 미세 차이**(비가시): 페이지 HTML 은 프레임워크 JS 를 제거했고, 일부 속성(`data-nimg` 등)·엔티티 인코딩(`'`↔`&#x27;`)이 다르다. 시각·동작은 동일(스크린샷 검증). 피드만 바이트 패리티.

## 테스트

- `go test ./...`. `internal/parser`(프론트매터·정렬), `internal/builder`(배지·**피드 골든**). 상세는 [TESTS.md](./TESTS.md), 검증 기준은 [QUALITY_GATE.md](./QUALITY_GATE.md).
