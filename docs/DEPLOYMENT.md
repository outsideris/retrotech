# Deployment — RetroTech

> 배포 방식과 배포 알림(텔레그램) 정리. 구조/구성은 [ARCHITECURE.md](./ARCHITECURE.md).

## 배포 개요

- **호스팅: Cloudflare Pages.** 정적 익스포트 결과(`dist/`)를 배포한다(서버리스 정적 호스팅).
- **운영 도메인:** <https://retrotech.outsider.dev>
- **오디오(mp3):** 사이트와 분리된 `retrotech-episodes.outsider.dev` 에 별도 호스팅(에피소드 프론트매터 `enclosure.url`).
- **CI:** 별도 GitHub Actions 등은 없다. 빌드/배포는 Cloudflare Pages 가 처리한다.
- `dist/` 는 `.gitignore` 대상 — 저장소에 포함되지 않고 매 배포 시 빌드한다.
- **정적 자산 캐시:** 저장소의 `public/_headers` 가 `/_next/static/*` 를 1년 `immutable` 로 지정한다(Cloudflare Pages 가 대시보드 설정 없이 자동 적용, 기본값 override). 비해시 자산(`/images` 등)은 URL 이 고정이라 의도적으로 기본 TTL 유지. 배경/대안은 [PERFORMANCE.md](./PERFORMANCE.md) 참고.

## 빌드 설정 (Cloudflare Pages 대시보드)

| 항목 | 값 | 비고 |
| --- | --- | --- |
| 빌드 명령 | `bash scripts/cf-build.sh` | `npm test`(게이트) → `npm run build`(gen-rss → next build) → 배포 결과를 텔레그램 Worker 로 통지. **테스트/빌드 실패 시 non-zero 종료 → 배포 차단** |
| 출력 디렉터리 | `dist` | ⚠️ Cloudflare 의 Next.js 프리셋 기본값은 `out` 이지만, 이 프로젝트는 `next.config.js` 에서 `distDir: 'dist'` 로 바꿨으므로 **`dist` 로 지정해야 한다** |
| Node 버전 | Next 13 / Nextra 2 빌드 가능한 버전 | 필요 시 빌드 환경변수 `NODE_VERSION` 으로 고정 |

## 배포 알림 → 텔레그램

> 상태: **구현됨** — 빌드 래퍼 `scripts/cf-build.sh` 가 배포 결과를 Worker 로 POST.

텔레그램 전송 자체는 별도 Cloudflare Worker(`cf-webhook.outsideris.workers.dev`)가 담당하고, 이 사이트는 **빌드 종료 코드(0=성공)에 따라** 그 Worker 로 결과를 POST 한다. (Cloudflare 네이티브 알림 웹훅은 자체 스키마라 이 Worker 의 커스텀 페이로드와 맞지 않아 빌드 래퍼 방식을 쓴다.)

### 동작 — `scripts/cf-build.sh`
1. **`npm test` (게이트)** 실행 → 통과하면 `npm run build` 실행. 둘 중 하나라도 실패하면 멈추고 non-zero 로 종료한다.
2. 종료 코드 = Cloudflare 의 배포 판정. **non-zero → 빌드 실패 → 배포 차단(직전 버전 유지)**, 0 → 배포.
3. 결과(성공/실패)에 따라 Worker 의 **`/webhook/generic`** 으로 POST(상태 이모지·라벨은 Worker 가 붙임). 실패 메시지엔 실패 단계 표시:
   ```json
   {"status":"success|failure","project":"retrotech","branch":"<CF_PAGES_BRANCH>","commitSha":"<CF_PAGES_COMMIT_SHA>","url":"<CF_PAGES_URL>","message":"실패 시 'test|build failed (exit N)'"}
   ```
4. 종료 코드로 그대로 종료 → Pages 가 성공/실패를 정확히 표시.

- 웹훅 전송 실패는 무시한다(배포 결과에 영향 없음). `DEPLOY_WEBHOOK_URL` 미설정이면 알림만 건너뛴다.
- 빈 필드는 Worker 가 자동 생략하므로 항상 포함해 보낸다. generic 어댑터가 `status`(`failure`→`failed`)·`project`·`branch`·`commitSha`·`url`·`message` 를 최상위 키에서 읽는다.
- **게이트 동작 조건:** 빌드 환경에 devDependencies(`vitest` 등)가 설치돼야 한다. Cloudflare Pages 는 기본 설치하지만 `NODE_ENV=production` 을 빌드 환경변수로 두면 빠지니 주의. Node 18+ 필요.

### Cloudflare Pages 설정 (대시보드에서 1회)
- **Build command:** `bash scripts/cf-build.sh`
- **Build output directory:** `dist`
- **환경변수(암호화) `DEPLOY_WEBHOOK_URL`:**
  `https://cf-webhook.outsideris.workers.dev/webhook/generic?token=<SECRET_TOKEN>`
  ⚠️ **`/webhook/generic`** 을 쓴다 — `/webhook/cloudflare` 는 Cloudflare **네이티브** 알림 페이로드용이라 이 커스텀 페이로드(`status/project/...` 최상위 키)를 못 읽는다.
  토큰은 이 secret 에만 두고 저장소엔 넣지 않는다. (Production / Preview 각각 설정 가능)

### 한계 / 대안
- 래퍼는 **빌드 단계**의 성공/실패를 잡는다. 빌드 성공 후 Cloudflare 업로드 단계 실패나 빌드 컨테이너 기동 실패 같은 **플랫폼 레벨 장애는 못 잡는다**(드묾).
- 그것까지 받으려면 Dashboard → **Notifications → Destinations → Webhooks** 로 Worker 의 **`/webhook/cloudflare`** 를 등록하고(인증은 secret 을 `cf-webhook-auth` 헤더로 전달) 배포 알림을 생성한다. 이 엔드포인트는 Worker 의 cloudflare 어댑터가 Cloudflare 네이티브 페이로드를 파싱한다(공식 배포 `data` 스키마가 비공개라 `text`/`alert_type` 기반 **베스트 에포트** 상태 추론). 즉 빌드 래퍼(generic)와 네이티브 알림(cloudflare)은 **서로 다른 엔드포인트**로 공존 가능하다.

## 참고 링크
- [Cloudflare Pages 빌드 설정(종료 코드로 성공/실패 판정)](https://developers.cloudflare.com/pages/configuration/build-configuration/)
- [Cloudflare Notifications 웹훅 설정](https://developers.cloudflare.com/notifications/get-started/configure-webhooks/)
- [Cloudflare 웹훅 payload 스키마](https://developers.cloudflare.com/notifications/reference/webhook-payload-schema/)
