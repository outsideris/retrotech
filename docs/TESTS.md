# Tests — RetroTech

## 실행

```bash
npm test          # vitest run (1회 실행)
npx vitest        # watch 모드
```

- 러너: **Vitest** (`vitest.config.ts`, `@vitejs/plugin-react`). 기본 환경 `node`, 컴포넌트 테스트는 파일 상단 `// @vitest-environment jsdom` 로 jsdom 사용.
- 테스트 파일(`**/*.test.ts(x)`)과 `vitest.config.ts` 는 `tsconfig.json` `exclude` 에 있어 `next build` 타입체크 대상이 아니다.
- **CI:** GitHub Actions(`.github/workflows/ci.yml`)가 push(main)/PR 마다 `npm test` + `npm run build` 를 실행해 RSS 포맷·데이터 회귀와 빌드 깨짐을 자동 검증한다.

## 현황

| 테스트 파일 | 대상 | 검증 범위 |
| --- | --- | --- |
| `scripts/gen-rss.test.js` | `gen-rss.js` 의 `episodeToItem`·`shouldSkip`·`sortByDateDesc`·`buildFeedXml`·`readEpisodes` | 프론트매터→RSS 매핑·`index.*` 제외·최신순 결정적 정렬, **포맷 스냅샷**(고정 픽스처로 XML 구조·iTunes 네임스페이스·CDATA escape 회귀 감지), **실제 에피소드 데이터 유효성**(전 회차 title/date/duration/description/`.mp3` + 항목 수) |
| `components/Badges.test.tsx` | `components/Badges.tsx` | 항상 노출되는 Apple/YouTube/Spotify 배지, `google` prop 유무에 따른 Google↔RSS 토글, props 로 넘긴 링크 사용 |

- 합계: 2개 파일, 16개 테스트 (현재 모두 통과).
- `gen-rss.js` 는 순수 로직(`episodeToItem`/`shouldSkip`/`sortByDateDesc`/`buildFeedXml`)과 I/O(`readEpisodes`/`generate`)를 분리·export. `generate()` 는 `require.main === module` 일 때만 실행되어 테스트 import 시 부작용 없음.
- 포맷 스냅샷은 `scripts/__snapshots__/` 에 커밋된다. **의도된** 포맷 변경 시 `npx vitest -u` 로 갱신한다.
- 외부 의존성은 테스트하지 않는다: `Badges` 테스트는 `next/image`·`next/link` 를 mock 해 우리 컴포넌트의 분기 로직만 검증한다(CLAUDE.md 규칙).

## 미검증 영역

- 정적 익스포트 산출물의 페이지별 렌더(현재 수동 확인 + 빌드 시 타입체크).
- `generate()` 의 `public/feed.xml` 파일 쓰기 자체(읽기는 `readEpisodes`, XML 생성은 `buildFeedXml` 로 검증됨).
- 운영 호스트 동작(압축/캐시/HTTPS) — 별도 확인.

## 후보 (향후)

- 실제 에피소드 유효성·항목 수·포맷 스냅샷까지 검증함. `generate()` 의 writeFile 경로 정도가 남음.
