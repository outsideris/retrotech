# 문단별 추가 조사 리더

기존 로컬 HTML의 본문을 읽으면서 문단별로 질문하고, 원자료를 확인한 보충 내용을 같은 위치에 쌓는다. 공개 사이트·RSS·에피소드 대본과 별개인 로컬 도구다.

## 실행

Go 1.26.2 이상으로 **지속되는 경로**에 바이너리를 만든다. `cmd/build`는 `dist`를 정리하므로 연구 바이너리는 `dist` 밖에 둔다.

```sh
mkdir -p .local-bin
go build -o .local-bin/retrotech-research ./cmd/research
.local-bin/retrotech-research -report '/absolute/path/to/report' -open
```

보고서 폴더에 짝이 맞는 `index.html`과 `research.md`가 있어야 한다. 현재 RetroTech HTML의 `<article class="prose">` 구조와 Markdown 문단을 대응한다. 임의의 웹페이지를 변환하는 기능은 아니다. 경로에 `episodes`가 있으면 거부한다.

첫 실행은 원본을 `.research/base.html`, `.research/base.md`에 보존하고 UI를 HTML에 삽입한다. 폴더의 `start-research.command`를 다시 실행하면 서버를 시작하거나 이미 열린 서버로 이동한다. 실행 터미널을 닫으면 서버가 종료된다. 서버 없이 HTML을 열어도 본문과 저장된 추가 내용은 읽을 수 있다.

기본 포트는 `49327`이고 점유 중이면 운영체제가 빈 포트를 고른다. `-port 0`으로 처음부터 자동 배정할 수 있다. `7800–7899`는 terrarium 예약 대역이라 거부한다. `127.0.0.1`에만 바인딩하며 상시 서비스로 등록하지 않는다.

## 읽기와 조사

1. 본문 문단을 클릭하거나 오른쪽에서 장을 고른다.
2. 질문·모델·깊이를 선택하고 **조사하기**를 누른다. 보고서당 한 건씩 실행한다.
3. 결과는 해당 문단 뒤에 **추가 조사**로 저장된다. 질문, 날짜, 모델, 근거 위치와 출처가 함께 남고 정정이면 별도 표시된다. 결과가 도착해도 본문 위치를 유지한다.
4. **원래 장면**, **본문의 추가 내용**, **이어서 질문**으로 이동한다. 대화는 기본적으로 선택한 장만 보여 주며 전체 보기로 바꿀 수 있다.
5. **추가 내용** 탭에서 본문에 보일 항목을 숨기거나 복원한다. 조사 기록은 삭제하지 않는다.
6. 충분히 읽고 정리한 뒤 **스킬에 남길 점**에서 규칙 초안을 편집·선택하고 반영한다. `references/follow-up-learnings.md`에 일반화된 점검 질문이 쌓이고 SKILL.md가 이를 연결한다. 스킬 변경 전 사본은 `.research/skill-backups`에 보관한다. 동일 규칙은 중복 저장하지 않는다.

## 모델과 로그인

- 기본: **Codex GPT-5.6 Sol / high**. 원자료를 대조하고 이야기의 빈틈을 채우는 질문에 사용한다.
- 짧은 사실·날짜 확인: **GPT-5.6 Terra / medium**. 출처 확인 기준은 동일하다.
- 상충하는 증언·메일 스레드·이전 버전 설계의 복원: **Sol / xhigh**.
- Claude 대안: **Sonnet 5 / high**. 더 까다로운 질문은 **Opus 5 / high 또는 xhigh**.
- Astra는 이 리더의 선택지에 두지 않는다. 이 추천은 작업 성격에 따른 선택이며 실제 품질·소요 시간은 질문에 따라 다르다.

앱은 설치된 `codex`·`claude` 실행 파일을 찾고 기존 CLI 인증을 사용한다. API 키를 읽거나 저장하지 않는다. 사용자가 터미널에서 `codex login` 또는 `claude auth login`으로 로그인한다. 구독·모델 접근 권한은 각 CLI 계정에 따른다. 모델 목록은 2026-09-16의 공식 안내를 기준으로 했으며 후속 변경 시 CLI와 함께 갱신한다.

근거: [Codex 모델](https://learn.chatgpt.com/docs/models), [Codex 비대화형 실행](https://learn.chatgpt.com/docs/non-interactive-mode), [Claude 모델·effort](https://code.claude.com/docs/en/model-config), [Claude 비대화형 실행](https://code.claude.com/docs/en/headless).

## 저장과 오류 처리

- `.research/state.json`: 질문, 상태, 결과, 출처, 숨김 여부, 반영한 규칙. 보고서와 함께 폴더 전체를 백업한다.
- `.research/jobs/<id>/`: 구조화 출력 스키마와 CLI 결과. 기존 대화 세션을 이어 쓰는 대신 본문과 관련된 이전 조사 최대 4건을 매 요청에 전달한다.
- HTML·Markdown은 보존된 원문과 표시할 추가 항목에서 다시 생성한다. 원문은 수정하지 않는다. **정정**도 출처와 함께 별도 블록으로 읽는다.
- 다른 프로그램이 HTML 또는 Markdown을 수정하면 해시 충돌로 저장을 중단한다. 외부 변경을 덮어쓰지 않는다. 수동 본문 재편집 후 기존 조사 앵커를 옮기는 기능은 없다. 먼저 폴더를 복사해 보존하고, 새 본문은 새 보고서 폴더로 시작한다.
- 일반적인 쓰기 실패에서는 직전 HTML·Markdown 쌍을 복구한다. 쓰기 도중 프로세스·전원이 강제 종료되어 상태와 파일이 어긋나면 자동 덮어쓰기 대신 충돌로 중단한다. `.research/base.*`와 상태·백업을 확인해 복구해야 한다.
- 완료 결과를 저장하지 못하면 `.research/unsaved-<id>.json`에 결과를 별도로 남긴다. 서버 재시작 시 중단된 요청은 실패로 표시하며 질문을 다시 작성할 수 있다.
- 조사 제한은 20분이다. 취소 시 CLI 프로세스를 종료한다. 출력 스키마가 잘못되면 본문에 넣지 않고 실패 사유를 표시한다.

## 구조와 접근 범위

`cmd/research` → `internal/research`(HTTP·상태·앵커·HTML/Markdown 렌더·스킬 반영) → `internal/editor/assist/research.go`(CLI). 브라우저 UI는 Go 바이너리에 임베드되고 보고서에 인라인으로 들어간다. 새 런타임 패키지 의존성은 없다.

Codex는 `exec`, 읽기 전용 샌드박스, 실시간 웹 검색, 임시 세션, 구조화 출력으로 호출한다. 셸·앱·하위 에이전트를 비활성화한다. Claude는 구독 인증을 유지하는 safe mode와 WebSearch/WebFetch만 사용한다. CLI의 쓰기 도구에 HTML이나 스킬을 맡기지 않는다. 결과의 텍스트와 URL을 검증·이스케이프한 후 앱이 고정 경로에 기록한다.

HTTP는 보고서와 지정 API만 제공한다. Host를 검증하고 변경 요청은 같은 Origin·JSON·전용 헤더를 요구한다. 임의 파일 읽기·셸 명령·모델 이름을 API로 받지 않는다. 스킬 반영은 사용자의 선택 버튼으로만 실행하며, 주제별 사실을 공통 규칙으로 자동 승격하지 않는다.
