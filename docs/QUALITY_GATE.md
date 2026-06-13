# Quality Gate — RetroTech

이 프로젝트의 완료/커밋 가능 판단 기준. `package.json` 의 실제 스크립트에 맞춰 작성했다.

## 사용 가능한 스크립트 (`package.json`)

| 명령 | 내용 |
| --- | --- |
| `npm run dev` | `next` 개발 서버 |
| `npm run build` | `node ./scripts/gen-rss.js && next build` (RSS 생성 → 정적 익스포트 `dist/`) |
| `npm run start` | `next start` (정적 익스포트 구성에선 잘 쓰지 않음) |

> 별도의 `test` / `lint` / `format` / `typecheck` 스크립트는 **없다.** 타입체크는 `next build` 가 내부적으로 수행한다(`tsc`).

## 필수 확인 항목

- [ ] **빌드:** `npm run build` 성공.
  - ⚠️ **현재 차단됨.** `components/Player.tsx` 의 타입에러(`Cannot find name 'Link'` 등)로 `next build` 가 실패한다. `tsconfig` 의 `include: ["**/*.tsx"]` 때문에 미사용 파일도 타입체크된다. → [TODO.md Phase 0](./TODO.md)
- [ ] **타입체크:** 위 빌드의 "Linting and checking validity of types" 단계 통과(별도 명령 없음).
- [ ] **RSS 생성:** `public/feed.xml` 이 생성되고 iTunes 필드가 포함되는지 확인(`scripts/gen-rss.js`).
- [ ] **정적 산출물:** `dist/` 에 HTML 27개(홈/episodes/에피소드들/404)와 자산이 생성되는지 확인.
- [ ] **수동 구동 확인:** `cd dist && python3 -m http.server <port>` 로 홈·에피소드 페이지가 정상 렌더되는지 확인.

## 선택 확인 항목

- [ ] **성능:** Lighthouse / DevTools 트레이스. 절차·기준선은 [PERFORMANCE.md](./PERFORMANCE.md).
- [ ] **접근성:** Lighthouse Accessibility(현재 94). 목표: `link-name`/`landmark-one-main` 해소.
- [ ] **번들 크기:** `next build` First Load JS 리포트 회귀 감시(홈 ~104 kB 기준).

## 면제 / 미검증 조건

- **운영 호스트 설정(압축·캐시 헤더·HTTPS):** 로컬 정적 서버로는 검증 불가. 호스트에서 별도 확인.
- `next start`: 정적 익스포트(`output: 'export'`) 구성이라 운영 검증 경로가 아니다(정적 호스팅 사용).

## 커밋 전 체크

- [ ] `npm run build` 성공(또는 실패/미실행 사유를 worklog·커밋 메시지에 명시).
- [ ] `docs/worklog/YYYY-MM.md` 에 작업 기록 추가.
- [ ] 테스트를 추가/변경했다면 [TESTS.md](./TESTS.md) 갱신.
- [ ] `git commit --signoff` (CLAUDE.md 규칙), 메시지는 영어.

## 마지막 검토

- **2026-06-14:** 최초 작성. `package.json` 에 test/lint/typecheck/format 스크립트가 없음을 확인. `Player.tsx` 가 현재 빌드를 차단함을 기록.
