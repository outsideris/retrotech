# 에피소드 관리 데스크톱 앱 (RetroTech Editor)

> 에피소드를 마크다운 파일을 직접 편집하지 않고 폼으로 관리하는 데스크톱 앱.
> 구조·구성 요약은 [ARCHITECURE.md](../ARCHITECURE.md#에피소드-관리-데스크톱-앱-retrotech-editor),
> 도메인/UX 의도는 [DESIGN.md](../DESIGN.md#에피소드-관리-데스크톱-앱-에디터), 테스트는 [TESTS.md](../TESTS.md).
> 참고 구현: `blog.outsider.ne.kr` 의 Electron 글쓰기 도구.

## 배경 / 목표

에피소드는 `content/episodes/<id>.md` 한 파일이고, 그동안 손으로 직접 편집했다. 프론트매터 YAML 에는
손이 많이 가는 함정이 많다 — `duration: "17:27"` 따옴표를 빼면 60진수로 잘못 파싱되고,
`enclosure.size`(바이트)를 직접 계산해야 하고, 구독 뱃지 딥링크 5종을 공개 후 하나씩 채워야 하고,
긴 레퍼런스 링크 목록(어떤 회차 ~60개)을 수작업으로 쓴다.

**목표:** 참고 앱의 검증된 구조 — *Electron 은 얇은 셸(창·폴더 선택·생명주기만), 모든 로직은 Go
사이드카 HTTP 서버, UI 는 바이너리에 임베드된 폼 SPA* — 를 그대로 가져와, 마크다운을 건드리지 않고
에피소드를 편하게 관리한다. 기존 `internal/parser`(데이터 모델)·`internal/builder.BuildEpisodePage`
(미리보기)를 재사용한다.

## 아키텍처

```
Electron(desktop/main.js)  ──spawn──▶  Go 사이드카(cmd/app)  ──serve──▶  임베드 폼 UI(internal/editor/assets)
   창·폴더 선택·생명주기            EDITOR_PORT 출력·HTTP API            fetch 로 API 호출(IPC 아님)
```

- **Electron = 얇은 셸.** repo 폴더 결정(env→config.json→네이티브 picker, `content/episodes` 검증)
  → `server-bin` spawn(`-repo`) → stdout `EDITOR_PORT <n>` 파싱 → `http://127.0.0.1:<port>/_write/`
  로드. 단일 인스턴스·창 bounds·외부 링크·메뉴·종료 시 서버 kill. preload/IPC 불필요(UI 는 HTTP 만).
- **Go 사이드카.** 고정 loopback 49218(점유 시 OS 할당). `//go:embed` 로 UI 를 바이너리에 포함 →
  단일 자산. "IPC" 는 실제로는 `fetch` 로 호출하는 로컬 HTTP API.
- **패키징.** electron-builder 가 `build:server` 로 Go 바이너리를 먼저 빌드 → `extraResources` 로
  `.app` 에 동봉(server-bin→editor-server), 런타임에 `process.resourcesPath` 로 탐색.

## 데이터 모델 (`internal/editor/form.go`)

`parser.Frontmatter/Episode/Badges/Enclosure` 를 그대로 쓰고 폼/JSON 뷰만 추가한다.

- `EpisodeForm` — 프론트매터 전 항목 + 본문을 `Intro`(badges 마커 앞) / `References` / `Extra`
  (레퍼런스 뒤 `## 배경음악` 등)로 **무손실** 분해. 구조가 예상과 다르면 `Structured=false`,
  전체 본문을 `RawBody` 에 verbatim 보존(데이터 손실 0).
- `Reference{Text, URL, Indent}` — `[text](url)`(URL 있음) 또는 일반 텍스트(URL 빈값), `Indent` 로
  중첩(4칸/레벨) 표현. `parseLinkItem` 은 `emit(parse)==원본` 이 성립(괄호 포함 URL·`](` 안전).
- **설명2·배경음악 = 「배경음악 라이센스」 한 필드(프런트 전용).** description2 와 본문 `## 배경음악` 은
  형식이 늘 같아(「레퍼런스는 홈페이지 참고: <slug URL>」 + 음악 신용표기), UI 는 음악 한 필드만 받고
  슬러그로 레퍼런스 줄을 자동 생성해 둘을 구성한다. **백엔드 무변경** — hidden `f-description2`/`f-extra`
  가 로드 원본을 들고 있다가 음악/ID 를 실제 편집할 때만 재구성하므로, 미편집 회차는 바이트 동일
  (라운드트립/피드 골든 불변).

## HTTP API (`internal/editor/editor.go`, `/_write/api/...`)

| 메서드·경로 | 동작 |
| --- | --- |
| `GET /api/episodes` | 목록(id/title/date/duration, 날짜 내림차순) |
| `GET /api/episodes/{id}` | `EpisodeForm`(프론트매터 + 본문 파싱) |
| `POST /api/episodes` | 생성(id 필수·중복 409) |
| `PUT /api/episodes/{id}` | 수정(path id 기준 — 본문 id 무시 → URL/guid 불변) |
| `DELETE /api/episodes/{id}` | 삭제 |
| `POST /api/preview` | 폼 → 메모리 Episode 합성 → `builder.BuildEpisodePage` 렌더 HTML(저장 없이) |

- 그 외 경로는 `public/` 정적 서빙(미리보기가 참조하는 `/styles.css`·`/badges/*`·`/images/*`).
  `/` → `/_write/` 리다이렉트. 미리보기는 반환 HTML 을 `<iframe srcdoc>` 에 주입(절대경로 자산이
  에디터 서버 오리진에서 해결).
- 스토어 에러 → HTTP 코드(400/404/409/500). `decodeForm` 은 `DisallowUnknownFields`.

## 초안(draft) → 발행 워크플로우 (`internal/editor/drafts.go`)

"새 에피소드"는 바로 발행하지 않고 **초안**을 만든다. 초안은 편집 중 자동 저장되고, 발행 전까지
사이트·피드에 보이지 않으며, **발행** 시 비로소 에피소드가 된다.

- **저장 위치·형식:** `content/drafts/*.json` (에피소드 마크다운 아님). 초안은 **`EpisodeForm` 전체**
  (목표 episode id·빈 필드·structured/raw·레퍼런스)를 보존해야 하는데, 에피소드 frontmatter 엔 id
  필드가 없으므로(파일명=id) JSON 이 자연스럽다. 사이트 빌드/피드는 `content/episodes` 만 읽어 초안은
  비공개. `content/drafts/` 는 `.gitignore`(로컬 작업 상태).
- **`DraftStore`** (주입식 `now` 로 결정적 테스트): `Create`(빈 폼·오늘 날짜·`draft-YYYYMMDD-HHMMSS`
  slug, 동초 충돌 시 `-2`), `List`(최근 수정 내림차순), `Get`/`Save`(자동저장)/`Delete`,
  `Publish`(폼→`Store.Create` 로 id 검증·중복 거부→초안 삭제).

| 메서드·경로 | 동작 |
| --- | --- |
| `GET /api/drafts` | 초안 목록(slug/title/id/updated) |
| `POST /api/drafts` | 빈 초안 생성 → `{slug, form}` |
| `GET/PUT/DELETE /api/drafts/{slug}` | 조회 / 저장(자동) / 삭제 |
| `POST /api/drafts/{slug}/publish` | 발행: `content/episodes/<id>.md` 기록 + 초안 삭제 → `{id}` |

- **UI:** 사이드바 **초안 섹션**(검색창과 에피소드 목록 사이, 비면 숨김). 초안 편집은 700ms 디바운스
  자동저장("편집 중…→저장 중…→저장됨"), **발행** 버튼은 최신 폼 flush 후 publish→에피소드로 전환.
  모드별 UI: 초안=id 편집·발행·자동저장 / 에피소드=id read-only·명시 저장. 발행은 파일만 기록하며
  배포(빌드/푸시)는 기존처럼 별도(git 미연동).

## 합성기 (`internal/editor/compose.go`) — 핵심 계약

**RSS 피드 골든 테스트는 파싱된 프론트매터 값에만 의존하지 YAML 스타일에는 의존하지 않는다**
(`feed.go` 는 본문을 읽지 않음). 따라서 합성 결과가 **재파싱 시 동일한 `Frontmatter` 값**을 내면
`BuildFeed` 는 바이트 동일 → 골든 통과.

- 프론트매터: `yaml.Marshal` 대신 **고정 키 순서 커스텀 직렬화**.
- title/description/description2: **리터럴 블록 스칼라 `|`** + chomping 지시자로 trailing newline
  정확 재현(`\n` 0개→`|-`, 1개→`|`, 2+개→`|+`). 콜론(`VCS: SCCS`)·여러 문단도 안전.
- `duration` 항상 큰따옴표(60진수 함정 차단). `badges` 는 비어있지 않은 필드만 struct 순서로.
- 본문: `intro + "\n\n<!--badges-->" + (refs? "\n\n## 레퍼런스:\n\n"+목록) + (extra? "\n\n"+extra)`.
  빈 섹션 생략(레퍼런스 없는 "Breaks" 회차에 빈 헤딩 안 생김).

### 정규화 주의 (1회 한정, 무해)

기존 파일을 도구로 저장하면 **프론트매터 스타일만** 1회 정규화된다(`title: >`→`|`, google 뱃지를
struct 순서로 이동). 측정: 23편 재출력 시 총 65줄 변경, **전부 프론트매터 스타일**, 본문(중첩
레퍼런스 포함)은 바이트 동일. **값·피드는 라운드트립/피드 동일성 테스트로 불변 증명**.

## UI (`internal/editor/assets/`, 프레임워크 없음)

- 목록 사이드바(검색·날짜 내림차순) + 구조화 폼(메타/오디오/뱃지/본문).
- 레퍼런스 행 편집(텍스트+URL+하위 들여쓰기 체크, 드래그 정렬).
- **로컬 mp3 선택 → size·duration 자동 채움**: `<input type=file>` 로 `file.size`,
  숨은 `<audio>` 메타데이터로 `MM:SS`. 파일은 업로드하지 않음(mp3 호스팅 별개) — 두 값만 읽음.
- 미리보기 `<iframe srcdoc>`. 신규 시 id→enclosure URL 자동 생성, 수정 시 id read-only(guid 보호).

## AI Assist 사이드바 (`internal/editor/assist/`)

로컬 AI CLI 를 호출하는 우측 사이드바. 참고 앱(blog.outsider.ne.kr)의 assist 구조를 가볍게 가져온
**토대** — 제공자 선택 → 프롬프트 → 실행 → 응답. 구체 AI 기능은 이후 이 위에 얹는다.

- **`Provider`**(Name/Available/Run) 3종, 각 CLI 의 비대화 모드로 셸 아웃:
  - **claude**: `claude -p --output-format=json`(stdin), JSON envelope 의 `result` 추출.
  - **codex**: `codex exec --json --skip-git-repo-check --sandbox read-only`(stdin), JSONL 의 `agent_message` 연결.
  - **gemini**: `agy`(antigravity CLI, blog 와 동일) 또는 `gemini`, `-p <prompt>` 평문 출력.
  - **바이너리 해석**(`resolveBinary`): 이름 후보(예: gemini→`gemini`/`agy`)를 PATH 에서 찾고, 없으면
    알려진 설치 경로(`~/.local/bin/<cli>`)를 stat 으로 fallback. CLI 가 비정상 종료해도 stdout 의
    메시지를 우선(예: claude 401).
- **PATH:** `cmd/app` 이 `injectLoginPath()` 로 로그인 셸 PATH 를 채택 — GUI 앱(Finder/launchd)의 빈
  PATH 에서도 `claude`/`codex`/`gemini` 가 터미널처럼 해석된다.
- **튜닝·텔레메트리:** `Options{Model,Effort}` 를 Run 에 넘긴다 — claude `--model`/`--effort`,
  codex `--model`/`-c model_reasoning_effort=`, gemini(agy) 는 미지원. 응답엔 `Meta`(model/effort/
  durationMs/tokens/costUsd)를 담는다(claude envelope·codex usage·wall-clock). effort 는 provider 별
  닫힌 집합으로 서버 검증.
- **API:** `GET /api/assist/providers`(이름·설치여부), `POST /api/assist/run`
  ({provider,prompt,model,effort}→{output,meta}, 3분 타임아웃, 미설치→503).
- **UI:** 브랜드행 `✦ Assist` 토글 → 우측 패널. 제공자 버튼 + **모델/effort 드롭다운**(provider 별
  옵션, gemini 는 숨김, localStorage 저장) + **디버그 모드 체크박스**(체크해야 대화창=프롬프트/실행/
  출력 열림) + **최근 사용 trace 3개**(`provider · model · effort · 시간 · 비용USD`, localStorage 보존).
  레이아웃 flex 라 미리보기와 공존, 미설치 제공자 비활성, ⌘/Ctrl+Enter 실행.
- **대본 import:** 사이드바 하단 드롭존(.md 드래그앤드롭/클릭). `POST /api/assist/analyze` 가 선택 CLI 로
  대본을 분석해 `{title,id,description}` 추출(`assist.AnalyzeScript` — JSON 추출 프롬프트 + 펜스/prose
  견디는 파싱). 결과로 **새 초안**을 만들어 제목·ID·설명을 채우고 id→enclosure URL 자동.
- **인증·비용:** CLI 가 스스로 인증(키체인/로그인). 이 앱은 API 키를 보관하지 않는다.

## 빌드 / 실행

```bash
# Go 측(완전 검증 가능)
go test ./internal/editor/        # 라운드트립·피드 동일성·핸들러·스토어
go run ./cmd/app -repo .          # 사이드카 단독 기동 → http://127.0.0.1:49218/_write/

# 데스크톱 앱
cd desktop
npm install
npm start                         # build:server + electron .(개발 실행)
npm run dist                      # → dist/mac-arm64/RetroTech Editor.app (코드사이닝 없음)
```

## 테스트 (상세: [TESTS.md](../TESTS.md))

- `compose_test.go` — 23편 전수 라운드트립(프론트매터 값 동일 + 구조화 본문 바이트 동일 +
  `BuildFeed` 바이트 동일), 블록 스칼라 chomping, `parseLinkItem` 가역성, 빈 섹션 생략.
- `store_test.go` — CRUD·중복·미존재·unsafe id(path traversal)·본문 id 무시.
- `editor_test.go` — HTTP 핸들러 전수·에러 코드·미리보기·UI/자산 서빙.

## 미검증 / 후속

- **GUI 픽셀 렌더**: 개발 환경(디스플레이/Electron 헤드리스 제약)에서 미수행. 서버/API/자산/JS·
  패키징은 검증. 실제 화면은 `npm start` 또는 `.app` 으로 확인.
- 후속 후보: 미리보기에 `## 배경음악` 같은 raw 섹션 구조화, 회차 복제, 정렬/그룹 보기.
