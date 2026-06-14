# Deployment — RetroTech

> 배포 방식과 배포 알림(텔레그램) 옵션 정리. 구조/구성은 [ARCHITECURE.md](./ARCHITECURE.md).

## 배포 개요

- **호스팅: Cloudflare.** 정적 익스포트 결과(`dist/`)를 Cloudflare로 배포한다(서버리스 정적 호스팅).
- **운영 도메인:** <https://retrotech.outsider.dev>
- **오디오(mp3):** 사이트와 분리된 `retrotech-episodes.outsider.dev` 에 별도 호스팅(에피소드 프론트매터 `enclosure.url`).
- **CI:** 현재 GitHub Actions 등 별도 CI 파이프라인은 없다. 배포는 Cloudflare 쪽에서 처리한다.
- `dist/` 는 `.gitignore` 대상 — 저장소에 포함되지 않고 매 배포 시 빌드한다.
- Cloudflare 연결 방식(Pages Git 연동 / 직접 업로드 / Workers 등)의 구체 설정은 운영자가 별도로 관리한다. 아래 빌드 설정만 코드 기준으로 명시한다.
- **정적 자산 캐시:** 저장소의 `public/_headers` 가 `/_next/static/*` 를 1년 `immutable` 로 지정한다(Cloudflare Pages 가 대시보드 설정 없이 자동 적용, 기본값 override). 비해시 자산(`/images` 등)은 URL 이 고정이라 의도적으로 기본 TTL 유지. 배경/대안은 [PERFORMANCE.md](./PERFORMANCE.md) 참고.

## 빌드 설정 (Cloudflare에서 빌드하는 경우)

| 항목 | 값 | 비고 |
| --- | --- | --- |
| 빌드 명령 | `npm run build` | `scripts/gen-rss.js`(feed.xml 생성) → `next build` |
| 출력 디렉터리 | `dist` | ⚠️ Cloudflare의 Next.js 프리셋 기본값은 `out` 이지만, 이 프로젝트는 `next.config.js` 에서 `distDir: 'dist'` 로 바꿨으므로 **`dist` 로 지정해야 한다** |
| Node 버전 | Next 13 / Nextra 2 빌드 가능한 버전 | 필요 시 Cloudflare 빌드 환경변수 `NODE_VERSION` 으로 고정 |

> 직접 업로드(wrangler 등)로 배포한다면 위 빌드는 로컬/스크립트에서 수행하고 `dist/` 를 업로드한다.

## 배포 알림 → 텔레그램

> 상태: **현재 미설정.** 아래는 향후 도입을 위한 참고 정리다.

**Cloudflare는 텔레그램으로 직접 알림을 보내지 못한다.** Cloudflare 알림 전송 수단은 **이메일 · 웹훅 · PagerDuty** 뿐이다. 따라서 텔레그램으로 받으려면 아래 두 방식 중 하나로 "중간 변환"이 필요하다.

### 사전 준비 (공통)
1. 텔레그램 **@BotFather** 로 봇 생성 → **봇 토큰** 발급.
2. 봇에게 아무 메시지나 보낸 뒤 `https://api.telegram.org/bot<TOKEN>/getUpdates` 응답에서 **chat_id** 확인.
3. 토큰/chat_id 는 코드에 하드코딩하지 말고 **secret/환경변수**로 보관한다.

### 옵션 A — 빌드 명령에 `curl` 추가 (가장 간단)
Cloudflare가 Git 연동으로 빌드하는 경우에 적합. Cloudflare Pages는 빌드가 성공(exit 0)해야 배포하므로, 빌드 끝에 텔레그램 전송을 붙이면 사실상 "배포 성공" 신호가 된다.

```bash
npm run build && curl -s "https://api.telegram.org/bot$TELEGRAM_TOKEN/sendMessage" \
  -d chat_id=$TELEGRAM_CHAT_ID \
  -d text="✅ RetroTech 배포 완료"
```

- `TELEGRAM_TOKEN` / `TELEGRAM_CHAT_ID` 는 Cloudflare Pages **빌드 환경변수(secret)** 로 설정.
- 장점: 추가 인프라가 전혀 없다.
- 단점: 자산 업로드 직전(=빌드 성공) 시점이라 엄밀한 "배포 완료"보다 약간 빠르고, **빌드 실패 시에는 전송되지 않는다**(성공 알림 전용).

### 옵션 B — Cloudflare Notification(웹훅) → Worker 중계 → 텔레그램 (더 견고)
실제 배포 이벤트(성공/실패)를 기준으로 알림을 받고 싶을 때.

1. Cloudflare Dashboard → **Notifications → Destinations → Webhooks** 에서 웹훅 대상 추가(중계 Worker URL + secret).
2. **Notifications → Create** 에서 배포 관련 알림을 만들고 위 웹훅으로 전송하도록 연결.
3. Cloudflare 웹훅은 자체 JSON 페이로드를 보내 텔레그램 `sendMessage` 형식과 맞지 않으므로, **작은 Cloudflare Worker** 가 웹훅을 받아 텔레그램 형식으로 변환·전송한다(토큰/chat_id 는 Worker secret).

중계 Worker 예시(개념):
```js
export default {
  async fetch(req, env) {
    const payload = await req.json()                 // Cloudflare 웹훅 JSON (text 등 포함)
    const text = `📦 RetroTech: ${payload.text ?? '배포 이벤트'}`
    await fetch(`https://api.telegram.org/bot${env.TELEGRAM_TOKEN}/sendMessage`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ chat_id: env.TELEGRAM_CHAT_ID, text }),
    })
    return new Response('ok')
  },
}
```

- 장점: 빌드 로그와 분리되고 성공/실패 모두 커버한다.
- 단점: Worker(+secret) 설정이 필요하다.

## 참고 링크
- [Cloudflare Notifications 웹훅 설정](https://developers.cloudflare.com/notifications/get-started/configure-webhooks/)
- [Cloudflare 웹훅 payload 스키마](https://developers.cloudflare.com/notifications/reference/webhook-payload-schema/)
- [Cloudflare → Telegram 봇 알림 튜토리얼(커뮤니티)](https://community.cloudflare.com/t/tutorial-get-notifications-from-cloudflare-to-telegram-bot/755646)
- [Telegram Bot API: sendMessage](https://core.telegram.org/bots/api#sendmessage)
