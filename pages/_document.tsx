import { Html, Head, Main, NextScript } from 'next/document'

export default function Document() {
  const meta = {
    title: 'RetroTech 팟캐스트',
    description: '기술의 역사를 살펴보는 팟캐스트입니다',
    image: 'https://retrotech.outsider.dev/images/cover.jpg',
  }

  return (
    <Html lang="ko">
      <Head>
        <meta name="robots" content="follow, index" />
        <meta name="description" content={meta.description} />
        <meta property="og:site_name" content={meta.title} />
        <meta property="og:description" content={meta.description} />
        <meta property="og:title" content={meta.title} />
        <meta property="og:image" content={meta.image} />
        <meta name="twitter:card" content="summary" />
        <meta name="twitter:site" content="@outsideris" />
        <meta name="twitter:creator" content="@outsideris" />
        <meta name="twitter:title" content={meta.title} />
        <meta name="twitter:description" content={meta.description} />
        <meta name="twitter:image" content={meta.image} />
      </Head>
      <body>
        <noscript>
          <iframe src="https://www.googletagmanager.com/ns.html?id=GTM-P368DQ3M" height="0" width="0" style={{display: "none", visibility: "hidden"}}></iframe>
        </noscript>
        <Main />
        <NextScript />
        <script src="https://kit.fontawesome.com/bba8fa6a15.js" crossOrigin="anonymous"></script>
      </body>
    </Html>
  )
}
