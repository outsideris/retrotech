# 팟캐스트 챕터(타임스탬프) — 구현 계획/기록

> 상태: **구현 완료(2026-08-01)** — 챕터 데이터 입력은 회차별로 저자가 진행.
> 관련: [DESIGN.md](../DESIGN.md)(온사이트 재생기 미제공 결정), [ARCHITECURE.md](../ARCHITECURE.md)(피드 챕터 반영 경로), [TODO.md](../TODO.md) Phase 9.

## 목표

청취자가 에피소드의 소제목(구간) 단위로 이동할 수 있게 한다. 사이트에는 재생기를 두지 않는다는 기존 설계 결정을 유지하고, **청취가 실제로 일어나는 팟캐스트 앱에서 동작**하는 방식을 택했다.

## 검토했던 대안

| 방식 | 판단 |
| --- | --- |
| ① 피드 경로(챕터 JSON + description 타임스탬프) | **채택.** 설계 결정 유지, 실청취 환경에서 동작 |
| ② 온사이트 `<audio>` 재생기 + 클릭 타임스탬프 | 보류 — "온사이트 재생기 미제공" 결정을 뒤집어야 하고, mp3 별도 도메인의 CORS/Range·a11y 게이트 추가 비용 |
| ③ 페이지에 정적 챕터 목차만 표시 | 미채택(필요 시 ①에 추가 가능) |

## 구현

- **데이터 소스 — 프론트매터 `chapters:`** (에피소드당, 선택):

  ```yaml
  chapters:
    - start: "00:00"      # "MM:SS" 또는 "HH:MM:SS". 선두 필드는 미패딩·60 이상 허용("75:12")
      title: 인트로
    - start: "03:15"
      title: SourceForge의 시작
  ```

  `internal/parser/parser.go` — `Chapter{Start,Title}` + `StartSeconds()`. `LoadEpisode` 가 잘못된 start·빈 title 을 즉시 에러로 만들어 빌드를 실패시킨다(파일명 포함).

- **Podcasting 2.0 chapters JSON** — `internal/builder/chapters.go`의 `BuildChaptersJSON`: `{"version":"1.2.0","chapters":[{"startTime":초,"title":…}]}`. `cmd/build` 가 챕터 선언 에피소드에 한해 `dist/episodes/<id>.chapters.json` 으로 출력(에피소드 페이지 옆 평면 파일 — 추가 라우팅 불필요).
- **피드 반영** — `internal/builder/feed.go`:
  - 아이템에 `<podcast:chapters url="…/episodes/<id>.chapters.json" type="application/json+chapters"/>` (Overcast·Pocket Casts 등 지원 앱용).
  - 아이템 `<description>` 끝에 `MM:SS 제목` 줄 병기 (Apple Podcasts·Spotify·YouTube 가 자동으로 클릭 가능 타임스탬프로 인식하는 폴백).
  - `xmlns:podcast="https://podcastindex.org/namespace/1.0"` 는 **챕터 선언 에피소드가 하나라도 있을 때만** 루트에 선언 — 챕터를 쓰기 전까지 배포 피드는 골든과 바이트 동일(구독자 계약 보수적 유지).

## 완료 기준 (모두 충족)

- [x] 챕터 미선언 시 `feed.xml`·`dist/` 산출물 무변화(피드 골든 테스트 통과로 확인).
- [x] 챕터 선언 시 chapters JSON 생성 + `<podcast:chapters>` + description 타임스탬프 병기(단위 테스트 + 임시 챕터로 실빌드 확인 후 원복).
- [x] 잘못된 챕터(형식 오류·빈 제목)는 빌드 실패.
- [x] `go build`·`go vet`·`go test`·`go run ./cmd/build` 통과.

## 운영 가이드 (챕터 넣는 법)

1. 에피소드 md 프론트매터에 `chapters:` 목록을 재생 순서대로 추가한다.
2. **첫 챕터는 `00:00`** 으로 시작한다(YouTube 가 챕터로 인식하는 조건; 앱 호환성에도 안전).
3. 빌드/배포는 평소와 동일 — 별도 절차 없음.

## 남은 질문 / 후속

- 회차별 실제 챕터 타임스탬프 입력(저자 작업, [TODO.md](../TODO.md) Phase 9).
- (선택) 에피소드 페이지에 정적 챕터 목차 표시 — 필요해지면 `render.go` 에 추가.
- (선택) MP3 자체에 ID3 챕터 삽입 — 사이트 빌드 밖(오디오 제작 단계)의 작업.
