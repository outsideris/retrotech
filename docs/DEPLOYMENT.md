# Deployment — RetroTech

> 배포 방식과 배포 알림(Chime → Telegram·Buzz) 정리. 구조/구성은 [ARCHITECURE.md](./ARCHITECURE.md).

## 배포 개요

- **호스팅: Cloudflare Pages.** 자체 Go 정적 생성기(`go run ./cmd/build`)의 산출물(`dist/`)을 배포한다(서버리스 정적 호스팅).
- **운영 도메인:** <https://retrotech.outsider.dev>
- **오디오(mp3):** 사이트와 분리된 `retrotech-episodes.outsider.dev` 에 별도 호스팅(에피소드 프론트매터 `enclosure.url`).
- **CI:** GitHub Actions(`.github/workflows/ci.yml`)가 push(main)/PR 마다 두 잡을 실행한다. ① `test`: `go vet`·`go test`(피드 골든 + 접근성/성능 불변식)·`go run ./cmd/build`. ② `lighthouse`: 빌드 후 `cmd/serve` 를 띄워 Lighthouse 감사(`.lighthouserc.json`) — 접근성/SEO/Best-Practices=100 하드 게이트, 성능·CLS 경고. 배포 자체는 Cloudflare Pages 가 git 연동으로 처리한다. 자세한 접근성/성능 검증은 [PERFORMANCE.md](./PERFORMANCE.md).
- `dist/` 는 `.gitignore` 대상 — 저장소에 포함되지 않고 매 배포 시 빌드한다.
- **정적 자산 캐시:** 저장소의 `public/_headers` 가 캐시 정책을 지정한다(Cloudflare Pages 가 대시보드 설정 없이 자동 적용): 콘텐츠 해시 자산 `/assets/*`(해시 스타일시트)는 1년 `immutable`, `/badges/*` 는 30일, `/images/*` 는 1주(URL 고정이라 immutable 대신 적당 TTL — 변경 시 최대 1주 stale 또는 캐시 purge). 파비콘류는 기본 TTL. 배경/대안은 [PERFORMANCE.md](./PERFORMANCE.md) 참고.

## 빌드 설정 (Cloudflare Pages 대시보드)

| 항목 | 값 | 비고 |
| --- | --- | --- |
| 빌드 명령 | `bash scripts/cf-build.sh` | `go test ./...`(게이트) → `go run ./cmd/build` → 배포 결과를 Chime 으로 통지. **테스트/빌드 실패 시 non-zero 종료 → Cloudflare 가 배포 차단(직전 버전 유지)**. 게이트·알림이 필요 없으면 `go run ./cmd/build` 만으로도 됨 |
| 출력 디렉터리 | `dist` | `cmd/build` 가 `dist/` 에 생성 |
| `ANALYTICS_ID` (환경변수, 프로덕션) | GA4 측정 ID(예 `G-PVJ12C7HR6`) | 설정 시에만 GA 스니펫을 주입. 미설정(프리뷰/로컬/CI)이면 분석 코드 미포함 |

> **Go 버전:** Cloudflare Pages 가 `go.mod` 의 `go` 지시자에서 버전을 자동 감지하므로 `GO_VERSION` 환경변수는 **보통 불필요**하다(같은 패턴의 `blog.outsider.ne.kr` 도 미설정으로 빌드된다). 빌드가 Go 버전 문제로 실패할 때만 `GO_VERSION` 으로 고정한다.

## 배포 알림 → Chime

> 상태: **구현됨** — 빌드 래퍼 `scripts/cf-build.sh` 가 배포 결과를 terrarium 수신함을 거쳐 Chime 으로 보낸다.

알림 전송은 알림 서비스 **Chime** 이 담당한다(Telegram 과 등급별 Buzz 채널로 함께 보낸다). Chime 은 사설망 안에 있어 인터넷에서 닿지 않으므로, Cloudflare 빌드는 terrarium 의 **공개 수신함**(`https://terrarium-inbox.outsideris.workers.dev/in/chime`)으로 POST 하고 terrarium 이 그것을 Chime 의 `/notify` 로 넘긴다. 이 사이트는 **빌드 종료 코드(0=성공)에 따라** 결과를 보낸다. (2026-10-11 이전에는 Cloudflare Worker `cf-webhook` 의 `/webhook/generic` 으로 보냈다.)

### 동작 — `scripts/cf-build.sh`
1. **`go test ./...`(게이트)** → 통과하면 `go run ./cmd/build`. 둘 중 하나라도 실패하면 멈추고 non-zero 로 종료(Cloudflare 는 이 종료 코드로 성공/실패를 판정 → 실패 시 배포 차단, 직전 버전 유지).
2. 성공(0)/실패(≠0)에 따라 수신함으로 POST 한다. 인증은 `Authorization: Bearer <수신함 비밀>`, 본문은 Chime 의 알림 필드(최상위 키)다:
   ```json
   {"project":"retrotech","status":"success|failed","level":"info|warn","environment":"production|preview","branch":"<CF_PAGES_BRANCH>","commitSha":"<CF_PAGES_COMMIT_SHA>","commitMessage":"<커밋 제목>","url":"<CF_PAGES_URL, 성공 시에만>","message":"실패 시 <phase> failed (exit N)"}
   ```
3. 종료 코드로 그대로 종료 → Pages 가 성공/실패를 정확히 표시.

- `title` 은 보내지 않는다 — 상태 이모지와 `Deploy succeeded`/`Deploy failed` 제목은 Chime 이 붙인다.
- `level` 은 성공 `info`, 실패 `warn` 이다(Buzz 채널이 등급으로 갈린다). 실패해도 직전 배포가 그대로 서비스되므로 `critical` 이 아니다.
- `environment` 는 `CF_PAGES_BRANCH` 가 `main` 이면 `production`, 그 밖의 브랜치면 `preview` 다.
- `commitMessage` 는 `git log -1 --pretty=%s` 다. 브랜치 이름과 커밋 제목은 따옴표·백슬래시를 이스케이프하고 제어 문자를 지운 뒤 넣는다.
- `url` 은 성공했을 때만 넣는다 — 실패한 배포의 주소는 아무것도 서빙하지 않는다.
- 빈 필드는 Chime 이 생략하므로 항상 포함해 보낸다.
- 수신함의 `200` 은 **큐에 넣었다**는 뜻이지 채널에 게시했다는 뜻이 아니다. 보통 30초 안에 Chime 으로 넘어가고, 운영 기계가 꺼져 있었으면 다시 뜬 뒤에 간다.
- 웹훅 전송 실패는 무시한다(15초 제한, 배포 결과에 영향 없음). `DEPLOY_WEBHOOK_URL`·`DEPLOY_WEBHOOK_TOKEN` 중 하나라도 없으면 알림만 건너뛴다.
- 래퍼는 재시도하지 않고 `Idempotency-Key` 도 보내지 않는다 — 수신함이 이벤트 id 로 붙인다.

### Cloudflare Pages 설정 (대시보드에서 1회)
- **Build command:** `bash scripts/cf-build.sh`
- **Build output directory:** `dist`
- **환경변수(암호화) `DEPLOY_WEBHOOK_URL`:** `https://terrarium-inbox.outsideris.workers.dev/in/chime`
- **환경변수(암호화) `DEPLOY_WEBHOOK_TOKEN`:** 수신함의 `chime` 대상 비밀(43자). 주소와 비밀은 `terrarium inbox url chime` 이 보여 준다.
  ⚠️ 수신함은 **헤더로만** 인증한다 — 비밀을 URL(`?token=…`)에 넣지 않는다. 이 값은 Chime 의 API 키(`WEBHOOK_SECRET`)와 다른 값이다.
  비밀은 이 환경변수에만 두고 저장소엔 넣지 않는다. (Production / Preview 각각 설정 가능)

### 한계 / 대안
- 래퍼는 **빌드 단계**의 성공/실패를 잡는다. 빌드 성공 후 Cloudflare 업로드 단계 실패나 빌드 컨테이너 기동 실패 같은 **플랫폼 레벨 장애는 못 잡는다**(드묾).
- 그것까지 받으려면 Dashboard → **Notifications → Destinations → Webhooks** 에 **같은 주소**(`…/in/chime`)를 등록하고 Secret 칸에 **같은 수신함 비밀**을 넣은 뒤(Cloudflare 가 `cf-webhook-auth` 헤더로 보낸다) 배포 알림을 생성한다. Cloudflare 네이티브 본문은 Chime 의 범용 파서가 읽는다(공식 배포 `data` 스키마가 비공개라 `text`/`alert_type` 기반 **베스트 에포트** 상태 추론). 빌드 래퍼와 네이티브 알림은 주소와 비밀을 함께 쓰며 공존할 수 있다.

## 참고 링크
- Chime 저장소의 `docs/SENDING.md`(알림 필드·등급), terrarium 저장소의 `docs/DEPLOY.md` 16장(수신함 주소·비밀·인증 방법)
- [Cloudflare Pages 빌드 설정(종료 코드로 성공/실패 판정)](https://developers.cloudflare.com/pages/configuration/build-configuration/)
- [Cloudflare Notifications 웹훅 설정](https://developers.cloudflare.com/notifications/get-started/configure-webhooks/)
- [Cloudflare 웹훅 payload 스키마](https://developers.cloudflare.com/notifications/reference/webhook-payload-schema/)
