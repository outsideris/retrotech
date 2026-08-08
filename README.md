# RetroTech

기술의 역사를 자세히 설명하는 한국어 팟캐스트 **RetroTech** 의 웹사이트.
특정 기술이 어떤 배경에서 등장하고 발전했는지, 왜 어떤 기술은 사라졌는지를 다룬다.

- 운영: <https://retrotech.outsider.dev>
- RSS(팟캐스트 피드): `/feed.xml`

## 기술 스택

**의존성을 최소화한 자체 제작 Go 정적 사이트 생성기.** 외부 의존성은 두 개뿐이다 —
[goldmark](https://github.com/yuin/goldmark)(마크다운)와 [yaml.v3](https://gopkg.in/yaml.v3)(프론트매터).
나머지(HTML 템플릿, RSS XML, 파일 처리, 테스트)는 모두 Go 표준 라이브러리로 처리한다.
브라우저로 전달되는 프레임워크 JS 는 없다(다크모드 토글용 인라인 스크립트뿐).

에피소드 한 편은 `content/episodes/*.md` 파일 하나이며, 프론트매터가 메타데이터·본문이 쇼노트다.

## 개발 / 빌드

```bash
go run ./cmd/build    # content/ + public/ → dist/ (HTML + feed.xml + 자산)
go run ./cmd/serve    # dist/ 미리보기. 빈 포트를 자동 선택해(8080 회피) URL 을 출력한다. clean URL 지원
go test ./...         # 단위 + 피드 골든 테스트
```

- 빌드 산출물은 `dist/` 에 생성되며(`.gitignore` 대상), **Cloudflare Pages**로 배포한다. 자세한 배포·알림은 [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md).
- 별도의 lint 도구는 두지 않는다. `go vet ./...` 으로 정적 검사한다.
- 분석(GA4)은 `ANALYTICS_ID` 환경변수가 있을 때만 주입된다 — 배포 빌드에서만 설정한다.

## 환경 / 외부 의존성

- 분석: GA4(`G-PVJ12C7HR6`) — 배포 빌드에서 `ANALYTICS_ID` 로 주입.
- 오디오(mp3): `retrotech-episodes.outsider.dev` 에 별도 호스팅(에피소드 프론트매터 `enclosure`).
- 푸터 아이콘: 인라인 SVG(Font Awesome Free, CC BY 4.0). 외부 요청 없음.
- 빌드용 `.env` 환경변수는 사용하지 않는다(주요 값은 코드에 상수로 존재).

## 새 에피소드 추가

마크다운을 직접 쓰는 대신 **에디터 데스크톱 앱**(아래)으로 폼에서 관리하길 권한다. 직접 작성한다면:

1. `content/episodes/<id>.md` 생성(예: `2h.md`). 프론트매터 스키마는 [docs/ARCHITECURE.md](docs/ARCHITECURE.md#라우팅--콘텐츠-모델).
2. 본문 끝부분에 `<!--badges-->` 마커(구독 배지 위치)를 두고, 레퍼런스는 `## 레퍼런스:` 헤딩 +
   일반 마크다운 리스트로 작성한다(빌더가 자동으로 `.refs` 스타일 적용).
3. 회차별 구독 딥링크는 프론트매터 `badges:` 에 넣는다. **발행 직후엔 비워둬도 된다** — 비운
   플랫폼은 쇼/채널 링크로 연결되고(홈 루트 아이콘과 동일), 이후 Apple/YouTube/Spotify 에
   에피소드가 등록되면 그 딥링크를 `badges:` 에 필드별로 채워 넣으면 해당 배지만 딥링크로 바뀐다.
4. `go run ./cmd/build` → `feed.xml` 갱신 및 정적 페이지 생성.

## 에피소드 관리 데스크톱 앱 (RetroTech Editor)

에피소드를 마크다운 직접 편집 없이 폼으로 관리하는 Electron 앱. Electron 은 창·폴더 선택·생명주기만
맡고, 모든 로직은 Go 사이드카(`cmd/app`)가 처리한다(UI 는 `internal/editor` 에 임베드).

```bash
go run ./cmd/app -repo .          # 사이드카만 기동 → http://127.0.0.1:49218/_write/ (브라우저에서 확인 가능)

cd desktop
npm install
npm start                         # 개발 실행(서버 빌드 + Electron 창)
npm run dist                      # 패키징 → dist/mac-arm64/RetroTech Editor.app (코드사이닝 없음 — 우클릭 열기)
```

- 폼으로 메타데이터·구독 뱃지·레퍼런스를 편집하고, 오디오 드롭존에 로컬 mp3 를 끌어놓으면(또는 클릭
  선택) `enclosure.size`·`duration` 이 자동으로 채워지고, 분석이 끝나면 **R2 에 업로드** 버튼으로
  Cloudflare R2 버킷(`retrotech` → `retrotech-episodes.outsider.dev/<ID>.mp3`)에 바로 올릴 수 있다
  (wrangler CLI 경유 — `wrangler login` 상태 필요). 미리보기로 실제 에피소드 페이지를 확인한다.
- **발행** 을 누르면 먼저 enclosure URL 의 mp3 가 실제로 다운로드되는지(그리고 크기가 폼 값과 맞는지)
  확인하고, 실패하면 경고를 띄운다 — 확인을 거쳐야만 발행된다.
- **초안→발행:** "새 에피소드"는 바로 공개되지 않고 **초안**(`content/drafts/`, gitignore·자동 저장)으로
  시작해 사이드바 초안 섹션에서 관리하다가, **발행**을 누르면 `content/episodes/<id>.md` 로 기록된다.
  (발행은 파일만 기록하고, 배포는 기존처럼 `go run ./cmd/build` + 푸시.)
- 합성기가 프론트매터 값을 정확히 보존해 RSS 피드를 바이트 동일하게 유지한다. 상세·설계는
  [docs/plan/episode-editor-app.md](docs/plan/episode-editor-app.md).
- **AI Assist:** 우측 `✦ Assist` 사이드바에서 로컬 CLI(Claude/Codex/Gemini)를 골라 프롬프트를 실행한다
  (설치된 CLI 만 활성화). CLI 가 스스로 인증하므로 앱은 키를 보관하지 않는다. 데스크톱 앱은 시작 시
  로그인 셸 PATH 를 채택해 CLI 들을 찾는다.

## 문서

| 문서 | 내용 |
| --- | --- |
| [docs/ARCHITECURE.md](docs/ARCHITECURE.md) | 아키텍처·구성·빌드 파이프라인·외부 통합·주의사항 |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | 배포(Cloudflare Pages)·빌드 설정·텔레그램 알림 |
| [docs/DESIGN.md](docs/DESIGN.md) | 제품/기획 의도, UX, 도메인 규칙, 의사결정 |
| [docs/PERFORMANCE.md](docs/PERFORMANCE.md) | 성능 감사 결과와 개선 우선순위 |
| [docs/TODO.md](docs/TODO.md) | 개선 백로그(Phase/Todo) |
| [docs/QUALITY_GATE.md](docs/QUALITY_GATE.md) | 빌드/검증 기준 |
| [docs/TESTS.md](docs/TESTS.md) | 테스트 현황과 후보 |
| [docs/plan/](docs/plan/) | 상세 구현 계획(Go 마이그레이션 등) |
| [docs/worklog/](docs/worklog/) | 월별 작업 로그 |
