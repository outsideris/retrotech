# Tests — RetroTech

## 실행

```bash
npm test          # vitest run (1회 실행)
npx vitest        # watch 모드
```

- 러너: **Vitest** (`vitest.config.ts`, `@vitejs/plugin-react`). 기본 환경 `node`, 컴포넌트 테스트는 파일 상단 `// @vitest-environment jsdom` 로 jsdom 사용.
- 테스트 파일(`**/*.test.ts(x)`)과 `vitest.config.ts` 는 `tsconfig.json` `exclude` 에 있어 `next build` 타입체크 대상이 아니다.

## 현황

| 테스트 파일 | 대상 | 검증 범위 |
| --- | --- | --- |
| `scripts/gen-rss.test.js` | `gen-rss.js` 의 `episodeToItem`, `shouldSkip` | 프론트매터→RSS 아이템 매핑(title, url 슬러그에서 `.mdx/.md` 제거, `date`+`09:00`, `description`+`description2` 결합, enclosure, duration, iTunes custom_elements), `index.*` 제외 |
| `components/Badges.test.tsx` | `components/Badges.tsx` | 항상 노출되는 Apple/YouTube/Spotify 배지, `google` prop 유무에 따른 Google↔RSS 토글, props 로 넘긴 링크 사용 |

- 합계: 2개 파일, 10개 테스트 (현재 모두 통과).
- `gen-rss.js` 는 순수 로직(`episodeToItem`/`shouldSkip`)과 I/O(`generate`)를 분리하고 `module.exports` 로 노출하도록 리팩터링됨. `generate()` 는 `require.main === module` 일 때만 실행되어 테스트 import 시 부작용 없음.
- 외부 의존성은 테스트하지 않는다: `Badges` 테스트는 `next/image`·`next/link` 를 mock 해 우리 컴포넌트의 분기 로직만 검증한다(CLAUDE.md 규칙).

## 미검증 영역

- 정적 익스포트 산출물의 페이지별 렌더(현재 수동 확인 + 빌드 시 타입체크).
- `gen-rss.js` 의 파일 읽기/쓰기 등 I/O 경로(엔드투엔드). 순수 변환 로직만 단위 테스트.
- 운영 호스트 동작(압축/캐시/HTTPS) — 별도 확인.

## 후보 (향후)

- 에피소드 디렉터리 → 피드 생성 end-to-end(픽스처 디렉터리 + 임시 출력) 테스트.
