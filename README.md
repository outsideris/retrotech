# RetroTech

기술의 역사를 자세히 설명하는 한국어 팟캐스트 **RetroTech** 의 웹사이트.
특정 기술이 어떤 배경에서 등장하고 발전했는지, 왜 어떤 기술은 사라졌는지를 다룬다.

- 운영: <https://retrotech.outsider.dev>
- RSS(팟캐스트 피드): `/feed.xml`

## 기술 스택

[Next.js 13](https://nextjs.org/) (정적 익스포트) + [Nextra](https://nextra.site/) 블로그 테마 기반의 정적 사이트.
에피소드 한 편은 `pages/episodes/*.mdx` 파일 하나이며, 프론트매터가 메타데이터·본문이 쇼노트다.

## 개발 / 빌드

```bash
npm install

npm run dev      # 개발 서버 (next)
npm run build    # RSS 생성(scripts/gen-rss.js) → 정적 익스포트 (dist/)
```

- 빌드 산출물은 `dist/` 에 생성되며(`.gitignore` 대상), 정적 호스트에 배포한다.
- 별도의 test/lint/typecheck 스크립트는 없다. 타입체크는 `npm run build` 가 수행한다.

## 환경 / 외부 의존성

- 분석: Google Tag Manager(`GTM-P368DQ3M`) + GA4(`G-PVJ12C7HR6`) — `pages/_app.tsx`.
- 아이콘: FontAwesome Kit — `pages/_document.tsx`.
- 오디오(mp3): `retrotech-episodes.outsider.dev` 에 별도 호스팅(에피소드 프론트매터 `enclosure`).
- 별도의 `.env` 환경변수는 사용하지 않는다(주요 값은 코드에 상수로 존재).

## 새 에피소드 추가

1. `pages/episodes/<id>.mdx` 생성(예: `2h.mdx`). 프론트매터 스키마는 [docs/ARCHITECURE.md](docs/ARCHITECURE.md#라우팅--콘텐츠-모델).
2. 본문에 `<Badges .../>` 로 플랫폼별 구독 링크, `<div className="refs">` 로 레퍼런스 작성.
3. `npm run build` → `feed.xml` 갱신 및 정적 페이지 생성.

## 문서

| 문서 | 내용 |
| --- | --- |
| [docs/ARCHITECURE.md](docs/ARCHITECURE.md) | 아키텍처·구성·빌드 파이프라인·외부 통합·주의사항 |
| [docs/DESIGN.md](docs/DESIGN.md) | 제품/기획 의도, UX, 도메인 규칙, 의사결정 |
| [docs/PERFORMANCE.md](docs/PERFORMANCE.md) | 성능 감사 결과(2026-06-14)와 개선 우선순위 |
| [docs/TODO.md](docs/TODO.md) | 개선 백로그(Phase/Todo) |
| [docs/QUALITY_GATE.md](docs/QUALITY_GATE.md) | 빌드/검증 기준 |
| [docs/TESTS.md](docs/TESTS.md) | 테스트 현황과 후보 |
| [docs/worklog/](docs/worklog/) | 월별 작업 로그 |
