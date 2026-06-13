# Tests — RetroTech

## 현재 상태

- **자동화 테스트 없음.** `package.json` 에 테스트 러너·`test` 스크립트가 없다.
- 검증은 현재 `next build` 의 타입체크와 수동 구동 확인에 의존한다([QUALITY_GATE.md](./QUALITY_GATE.md)).

## 테스트 후보 (가치 순)

| 대상 | 무엇을 검증 | 비고 |
| --- | --- | --- |
| `scripts/gen-rss.js` | 프론트매터 → RSS 아이템 변환(제목/url/date `09:00`/description+description2 결합/enclosure/duration/iTunes 필드), `index.*` 제외, 정렬 | **가장 테스트 가치 높음** — 순수 변환 로직. 픽스처 mdx 입력으로 검증. |
| `components/Badges.tsx` | `google` prop 유무에 따른 Google↔RSS 배지 토글, 기본 링크값 | React Testing Library 렌더 테스트 |

## 권장 도입 방향

- 러너: Vitest(또는 Jest). `gen-rss.js` 는 Node 환경, 컴포넌트는 jsdom 환경.
- 외부 의존성(howler, 네트워크, 팟캐스트 플랫폼)은 테스트하지 않는다. 우리 코드의 입출력·호출 계약만 fake/mock 으로 검증한다(CLAUDE.md 규칙).
- 픽스처: `pages/episodes/` 형식을 본뜬 최소 mdx 로 RSS 변환을 검증.

## 미검증 영역

- 정적 익스포트 산출물의 페이지별 렌더(현재 수동 확인).
- 운영 호스트 동작(압축/캐시/HTTPS).
