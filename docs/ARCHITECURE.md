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
│  ├─ serve/main.go        # 로컬 미리보기 서버(clean URL 해석)
│  └─ app/main.go          # 에디터 데스크톱 앱의 HTTP 사이드카(loopback, EDITOR_PORT 출력)
├─ internal/
│  ├─ parser/              # 프론트매터 분리·YAML 파싱·에피소드 로드/정렬
│  │  └─ parser.go         #   Frontmatter/Episode 모델, SortEpisodes
│  ├─ builder/
│  │  ├─ feed.go           # iTunes RSS 피드 생성(encoding 문자열)
│  │  ├─ feed_test.go      #   골든 테스트(testdata/feed.golden.xml)
│  │  ├─ badges.go         # 구독 배지(Apple/YouTube/Spotify/Google|RSS) HTML
│  │  ├─ render.go         # 페이지 빌더 + goldmark + 프로즈 후처리
│  │  ├─ render_layout.go  # 페이지 셸·head·메타·footer·날짜
│  │  └─ render_assets.go  # 다크모드 스크립트·아이콘 SVG·인라인 스타일
│  └─ editor/              # 에피소드 관리 앱 백엔드(아래 "에피소드 관리 데스크톱 앱")
│     ├─ form.go           #   EpisodeForm↔에피소드 변환(본문 무손실 구조화)
│     ├─ compose.go        #   EpisodeForm → 마크다운(블록 스칼라·고정 키 순서)
│     ├─ store.go          #   에피소드 파일 CRUD·슬러그 검증·atomic write
│     ├─ drafts.go         #   초안(JSON) 저장·발행(content/drafts → content/episodes)
│     ├─ editor.go         #   HTTP mux·JSON API·미리보기, //go:embed assets
│     ├─ assist/           #   AI CLI(Claude/Codex/Gemini) 셸 아웃 — Assist 사이드바 백엔드
│     └─ assets/           #   임베드 폼 SPA(index.html·app.js·app.css)
├─ desktop/                # Electron 래퍼(앱 셸). server-bin/node_modules/dist 는 gitignore
│  ├─ main.js              #   창·폴더 선택·서버 spawn·생명주기
│  ├─ package.json         #   electron + electron-builder(build:server/start/dist)
│  └─ icons/               #   앱 아이콘(public/images/cover 에서 생성)
├─ content/
│  ├─ episodes/            # *.md (프론트매터 + 본문). 0, 1a…1n, 2a…2g, 250127-breaks
│  └─ drafts/              # 에디터 초안 *.json (gitignore·로컬 작업 상태, 발행 전까지 비공개)
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
  description: |              # 요약(여러 줄). 목록·본문 상단에 사용되는 평문 소스
      ...
  description2: |             # (선택) 피드에만 덧붙는 보조 설명(레퍼런스 링크·배경음악 크레딧)
      ...
  feedDescription: |          # (선택) 위 둘을 합쳐 피드로 나가는 HTML. 에디터가 저장 시 파생.
      <p>...</p><p>...</p>    #   없으면 빌드 시 description/description2 에서 변환(기존 회차)
  # author 는 프론트매터에 없다 — 호스트(Outsider)는 항상 동일해 빌더에 하드코딩
  # (피드 dc:creator/itunes:author + 에피소드 바이라인). builder.showAuthor 상수.
  enclosure:                  # 팟캐스트 오디오 첨부
    url: https://retrotech-episodes.outsider.dev/2g.mp3
    size: 66997696            # 바이트 단위 파일 크기
  duration: "55:50"           # "MM:SS" — RSS의 duration / itunes:duration
  badges:                     # 회차별 구독 딥링크 — 필드별로 비우면 그 플랫폼은
    apple: "..."              #   쇼/채널 링크로 폴백(홈 루트 아이콘과 동일).
    youtube: "..."            #   발행 직후엔 통째로 비워두고, 플랫폼에 에피소드가
    spotify: "..."            #   등록되면 딥링크를 하나씩 채운다.
    # google 이 있으면 Google 배지, 없으면 RSS 배지
  chapters:                   # (선택) 챕터 마커. 재생 순서대로, 첫 챕터는 "00:00" 권장
    - start: "00:00"          #   "MM:SS" 또는 "HH:MM:SS"(선두 필드는 미패딩·60 이상 허용)
      title: 인트로           #   잘못된 start·빈 title 은 빌드 실패(LoadEpisode 검증)
  ---
  ```

  - 본문에서는 제목 h1 을 쓰지 않는다(템플릿이 프론트매터 title 로 emit). 구독 배지는 `<!--badges-->` 마커 위치에 주입되고, 레퍼런스는 `## 레퍼런스:` 헤딩 + 일반 마크다운 리스트로 작성한다(본문에 raw HTML 불필요 — 빌더가 그 리스트를 `<div class="refs">` 로 감싸 작은 글씨로 렌더).
- **목록 페이지.** 홈(`/`)과 `/episodes` 는 `parser.LoadEpisodes` 가 반환한 날짜 내림차순 목록을 `post-item` 으로 렌더한다.

## 빌드 파이프라인

```
go run ./cmd/build
  └─ 1) dist/ 청소
  └─ 2) public/ → dist/ 복사(이미지·배지·파비콘·_headers·styles.css 등)
  └─ 3) styles.css → dist/assets/styles.<hash>.css 로 핑거프린트(immutable 캐시)
  └─ 4) content/episodes/*.md 로드 → []Episode (날짜 내림차순)
  └─ 5) 페이지 렌더 → dist/ (index, episodes, episodes/<id>, 404)
        └─ chapters: 선언 에피소드는 episodes/<id>.chapters.json 도 생성
  └─ 6) feed.xml 생성 → dist/feed.xml
  └─ 7) sitemap.xml 생성 → dist/sitemap.xml
```

- 산출물: `dist/` (HTML 26개 = 홈 + /episodes + 에피소드 23개 + /404, + feed.xml + sitemap.xml + 자산). `chapters:` 를 선언한 에피소드가 있으면 `episodes/<id>.chapters.json` 이 추가된다. 빌드 ~45ms.
- **sitemap.xml**(`internal/builder/sitemap.go`): 홈·/episodes·각 에피소드 URL 을 `encoding/xml` 로 생성. 랜딩(홈·/episodes)의 `lastmod` 는 최신 에피소드 날짜라 새 에피소드 추가 시 자동 반영. `public/robots.txt` 가 이 사이트맵을 가리킨다. 404·feed 는 제외.
- SSR/ISR/API 가 없는 순수 정적 산출이다.

## 프로즈 렌더링(`internal/builder/render.go`)

마크다운 본문은 goldmark(GFM + raw-HTML 통과)로 렌더한 뒤, 이전 Nextra 테마와 동작을 맞추기 위해 후처리한다:

- **레퍼런스 래핑**: `## 레퍼런스:` 헤딩 뒤의 리스트를 `<div class="refs">` 로 감싼다 — 본문은 순수 마크다운으로 두고 `.refs`(작은 글씨) 스타일은 빌더가 입힌다.
- **외부 링크**(`http(s)://`): `target="_blank" rel="noreferrer"` + 스크린리더용 "(opens in a new tab)" span.
- **마크다운 heading**(h2–h6): `subheading-h{n}` 클래스 + 퍼머링크 anchor(id 는 github-slugger 규칙).
- **`<!--badges-->` 마커**: `badges:` 프론트매터로 구성한 배지 블록으로 치환.

페이지 셸은 재사용한 테마 CSS(`/assets/styles.<hash>.css`)를 참조하고, footer 를 `nx-prose` article 안에 둔다(테마와 동일). 다크모드는 프레임워크 없이 인라인 스크립트 두 개(첫 페인트 전 테마 적용 + 토글 영속화)와 해/달 아이콘 스왑 CSS 로 구현한다.

## RSS / 팟캐스트 피드(`internal/builder/feed.go`)

- `content/episodes/` 의 프론트매터를 읽어 RSS 2.0 + `itunes` 네임스페이스 `feed.xml` 을 만든다(Apple Podcasts 규격).
- **이전 `scripts/gen-rss.js`(rss npm 라이브러리) 출력과 바이트 패리티**를 목표로 문자열로 재현한다(`encoding/xml` 은 CDATA·네임스페이스 순서·self-closing 을 그대로 못 냄). 휘발성 `lastBuildDate` 만 매 빌드 갱신.
- 구독자 계약(불변): 각 항목 `guid`(=`/episodes/{id}`)·`enclosure`·`pubDate`. `pubDate` 는 날짜 09:00 UTC(빌드 머신 TZ 무관, 결정적).
- 항목은 발행일 내림차순(동일 날짜 id 내림차순). `internal/builder/testdata/feed.golden.xml` 골든 테스트로 회귀 방지.
- **아이템 `<description>` 은 HTML 이고, 그 HTML 은 md 에 저장된다.** 팟캐스트 앱은 description 을 HTML 로 렌더하므로 평문 줄바꿈이 공백으로 뭉개진다(Apple Podcasts 에서 description2 블록이 요약 문단에 붙어 나오던 원인).
  - 프론트매터 **`feedDescription`** 이 피드로 나갈 HTML 을 그대로 담는다 — 에디터가 저장 시 `description` + `description2` 에서 파생해 기록하므로(`internal/editor/derive.go` 의 `deriveFeedDescription`) md 만 봐도 구독자가 받는 내용이 보인다. `description`·`description2` 는 사람이 읽는 평문 소스로 남는다(사이트 목록 페이지는 계속 `description` 을 쓴다).
  - 변환기는 `builder.DescriptionHTML`: 빈 줄로 나뉜 블록 → `<p>`, 블록 안 줄바꿈 → `<br/>`, 맨 URL → `<a href>`. 텍스트는 `&`·`<`·`>` 만 이스케이프하고(따옴표·아포스트로피는 그대로 — 태그만 걷어내는 앱에서 `&#39;` 로 보이는 것 방지) 전체는 CDATA 안에 그대로 실린다.
  - `feedDescription` 이 없는 파일(필드 도입 전에 쓰인 기존 회차)은 빌드 시 `description`+`description2` 를 같은 방식으로 변환해 폴백한다 — 기존 24편은 손대지 않아도 동일한 결과가 나온다.
  - 챕터 줄은 저장된 HTML 이 아니라 **항상 빌더가** 뒤에 문단으로 덧붙인다(챕터는 별도 프론트매터 목록이므로 챕터만 고쳐도 `feedDescription` 을 다시 만들 필요가 없다).
  - **`gen-rss.js` 대비 의도적 divergence** — 구독자 계약(guid/enclosure/pubDate)은 불변.
- **챕터(타임스탬프).** 프론트매터 `chapters:` 가 있는 에피소드는 두 경로로 피드에 반영된다(`internal/builder/chapters.go`):
  - **Podcasting 2.0**: `episodes/<id>.chapters.json`(`{"version":"1.2.0","chapters":[{"startTime":초,"title":…}]}`) 생성 + 아이템에 `<podcast:chapters url=… type="application/json+chapters"/>`. 지원 앱(Overcast·Pocket Casts 등)은 챕터 목록/탭 이동 UI 를 보여준다.
  - **폴백**: 아이템 `<description>` 끝에 `MM:SS 제목` 줄을 덧붙인다(HTML 변환 후엔 자체 `<p>` 안의 `<br/>` 구분 줄) — Apple Podcasts·Spotify·YouTube 가 자동으로 클릭 가능한 타임스탬프로 인식.
  - `xmlns:podcast` 네임스페이스는 **챕터 선언 에피소드가 하나라도 있을 때만** 선언 — 그 전까지 피드는 골든과 바이트 동일하게 유지된다.
- **하드코딩:** `SITE_URL = 'https://retrotech.outsider.dev'`(`feed.go`/`cmd/build`).

## 에피소드 관리 데스크톱 앱 (RetroTech Editor)

에피소드를 마크다운 직접 편집 없이 폼으로 관리하는 데스크톱 앱. 사이트 빌드와 **독립**이며(별도 cmd),
`internal/parser`(데이터 모델)·`builder.BuildEpisodePage`(미리보기)를 재사용한다. 상세: **[plan/episode-editor-app.md](./plan/episode-editor-app.md)**.

- **구조(참고 앱 `blog.outsider.ne.kr` 와 동일):** Electron 은 얇은 셸(창·폴더 선택·생명주기만), 모든
  로직은 Go 사이드카 HTTP 서버, UI 는 `//go:embed` 로 바이너리에 포함된 폼 SPA. "IPC" 는 실제로는
  `fetch` 로 호출하는 로컬 HTTP API.
- **`cmd/app`** — 고정 loopback 49218(점유 시 OS 할당)에 listen → `EDITOR_PORT <n>` 출력(Electron 이
  읽어 URL 결정) → `internal/editor` 서빙. `-repo` 로 프로젝트 루트 지정(`content/episodes` 검증).
- **`internal/editor`** — `editor.go`(HTTP mux: `/_write/` UI, `/_write/api/episodes[/{id}]` CRUD,
  `/_write/api/drafts[/{slug}[/publish]]`, `/_write/api/preview`, `/_write/api/audio/upload`,
  `/_write/api/audio/check`, `/` → `public/` 정적 서빙),
  `store.go`(에피소드 파일 CRUD·슬러그 검증·atomic write), `drafts.go`(초안 JSON 저장·발행),
  `derive.go`(구조화 폼의 `description` 을 도입부에서 파생 — 마크다운 링크 제거),
  `form.go`(본문↔구조 무손실 파싱), `compose.go`(마크다운 합성). **합성 계약:** 피드는 프론트매터 값만
  읽으므로(`feed.go` 본문 미사용), 합성 결과가 재파싱 시 동일 값을 내면 `BuildFeed` 바이트 동일 →
  골든 통과. 기존 파일 저장 시 프론트매터 스타일만 1회 정규화(값·피드 불변, 테스트로 증명). 구조화
  회차의 `description` 은 클라이언트 값이 아니라 서버가 도입부에서 파생한다 — 단, `Update` 는 도입부
  미변경 시 저장된 description 바이트를 보존해 옛 회차 저장이 피드를 바꾸지 않는다.
- **초안→발행:** "새 에피소드"는 `content/drafts/<slug>.json`(폼 전체) 초안을 만들고 자동 저장한다.
  사이트 빌드/피드는 `content/episodes` 만 읽어 초안은 비공개; **발행** 시 폼을 `content/episodes/<id>.md`
  로 합성하고 초안을 지운다(발행 날짜 스탬프). `content/drafts/` 는 gitignore. 작성자는 프론트매터에
  없고 빌더에 하드코딩(`builder.showAuthor`).
- **AI Assist:** `internal/editor/assist` 가 로컬 CLI(Claude/Codex/Gemini)를 비대화 모드로 셸 아웃
  (`/api/assist/providers`·`/api/assist/run`). `cmd/app` 은 `injectLoginPath()` 로 GUI 의 빈 PATH 를
  로그인 셸 PATH 로 교체해 CLI 를 찾는다. 우측 Assist 사이드바의 토대 — 구체 기능은 이후 확장.
- **오디오 업로드·검증(`audio.go`):** 오디오 fieldset 의 드롭존에 mp3 를 끌어놓으면(또는 클릭 선택)
  size·duration 분석 후 "R2 에 업로드" 버튼이 활성화된다. 업로드는 사이드카가 임시 파일로 스풀한 뒤
  **wrangler CLI**(`wrangler` 또는 `npx -y wrangler`; assist CLI 처럼 자체 인증 — 앱은 키를 보관하지
  않음)로 `r2 object put retrotech/<ID>.mp3 --remote` 실행 — 버킷 키가 곧
  `retrotech-episodes.outsider.dev/<ID>.mp3` 공개 경로다. wrangler 는 cwd 에 `.wrangler/` 캐시를
  만들므로 `cmd.Dir=os.TempDir()` 로 실행한다. **발행 게이트:** 발행 버튼은 먼저
  `/api/audio/check`(Range GET 2바이트 + 서버 크기 vs 폼 `enclosureSize` 비교)로 enclosure URL 이
  실제로 다운로드되는지 확인하고, 실패하면 경고 confirm 을 거쳐야 발행된다(죽은/미완료 mp3 가 피드에
  실리는 사고 방지). 업로드 직후에도 같은 check 로 공개 URL 을 즉시 재확인한다.
- **`desktop/`** — Electron 래퍼. `main.js` 가 repo 폴더 결정(env→config.json→네이티브 picker) → 서버
  spawn → `http://127.0.0.1:<port>/_write/` 로드. electron-builder 가 Go 바이너리를 `extraResources`
  로 `.app` 에 동봉(server-bin→editor-server). `npm run dist` → `RetroTech Editor.app`(arm64,
  코드사이닝 없음). 빌드 산출물(node_modules·dist·server-bin)은 gitignore. **주의:** asar 에 들어갈
  파일은 `package.json` 의 build.files **화이트리스트**로 지정한다 — `desktop/` 에 새 JS 파일을 추가하면
  여기에도 추가해야 한다(누락 시 패키징 앱이 시작 require 에서 죽는다; 2026-08-01 worklog 참고).
- **사이드카 감시·자동 재시작:** 실행 중 서버 프로세스가 죽으면(외부 `pkill`·크래시 등) `main.js` 가
  자동 respawn 한다 — 안 그러면 창은 떠 있는데 모든 API 호출이 조용히 실패하는 좀비 UI 가 된다(실제
  발생: 타 프로젝트 앱 빌드의 이름 기반 pkill 이 동명 사이드카를 함께 죽임). 고정 포트 재바인드가
  일반 경로라 페이지는 리로드 없이 회복되고, 포트가 바뀐 경우만 창을 새 URL 로 리로드. 크래시 루프는
  `desktop/restart-policy.js`(10초 미만 연속 사망 3회 초과 시 포기, 단위 테스트 있음)로 차단. 프런트
  `request()` 는 재시작 찰나를 덮기 위해 GET 만 1회 재시도(쓰기는 중복 위험으로 제외).

## 외부 의존성 / 통합

| 통합 | 위치 | 용도 |
| --- | --- | --- |
| **Google Analytics 4** | `render_layout.go`(`<!-- @analytics -->` 주입) | 방문 분석(`G-PVJ12C7HR6`). `ANALYTICS_ID` 설정 시(배포)만 |
| **GitHub Sponsors** | `render_layout.go` footer | 후원 버튼 `<iframe>`(전 페이지) |
| **팟캐스트 플랫폼** | `badges.go`, 각 에피소드 `badges:` | Apple/Spotify/YouTube/Google/RSS 구독 링크 |
| **오디오 호스팅** | 프론트매터 `enclosure.url` | `retrotech-episodes.outsider.dev/*.mp3` (Cloudflare R2 버킷 `retrotech`) |
| **wrangler CLI** | `internal/editor/audio.go`(에디터 앱 전용) | 에디터의 mp3 → R2 업로드(`r2 object put`). `wrangler login` 자체 인증 |

## 배포

- **Cloudflare Pages** git 연동으로 빌드(`bash scripts/cf-build.sh` → `go run ./cmd/build`) 후 `dist/` 를 배포. 운영 도메인 `https://retrotech.outsider.dev`.
- 대시보드에 `GO_VERSION` 환경변수가 필요하고, 프로덕션에 `ANALYTICS_ID` 를 설정한다. 배포 알림(텔레그램) 포함 상세는 → [DEPLOYMENT.md](./DEPLOYMENT.md).
- 오디오(mp3)는 `retrotech-episodes.outsider.dev` 에 분리 호스팅.

## 알려진 제약 / 주의사항

1. **`SITE_URL` 하드코딩.** `feed.go` 와 `cmd/build` 에 도메인이 상수로 존재한다.
2. **이전 빌드 산출물과의 미세 차이**(비가시): 페이지 HTML 은 프레임워크 JS 를 제거했고, 일부 속성(`data-nimg` 등)·엔티티 인코딩(`'`↔`&#x27;`)이 다르다. 시각·동작은 동일(스크린샷 검증). 피드만 바이트 패리티.

## 테스트

- `go test ./...`. `internal/parser`(프론트매터·정렬), `internal/builder`(배지·**피드 골든**·**접근성/성능 불변식** `a11y_perf_test.go`). 상세는 [TESTS.md](./TESTS.md), 검증 기준은 [QUALITY_GATE.md](./QUALITY_GATE.md).
- **접근성/성능 CI**: 마크업 불변식(`go test`, 무의존성)에 더해 Lighthouse CI(`.lighthouserc.json`, `lighthouse` 잡)가 빌드본을 실제 감사한다 — 접근성/SEO/Best-Practices=100 하드 게이트. 측정·근거는 [PERFORMANCE.md](./PERFORMANCE.md).
