import 'nextra-theme-blog/style.css'
import type { AppProps } from 'next/app'
import Head from 'next/head'
import Script from 'next/script'
import { useEffect } from 'react'
import { useRouter } from 'next/router'
import '../styles/main.css'

export default function App({ Component, pageProps }: AppProps) {
  // nextra-theme-blog wraps content in <article> but renders no <main> landmark.
  // Mark the article as the main landmark for accessibility (re-applied on route change).
  const { asPath } = useRouter()
  useEffect(() => {
    if (!document.querySelector('main')) {
      document.querySelector('article')?.setAttribute('role', 'main')
    }
  }, [asPath])

  return (
    <>
      <Head>
        <link
          rel="alternate"
          type="application/rss+xml"
          title="RSS"
          href="/feed.xml"
        />
      </Head>
      <Component {...pageProps} />
      <Script strategy="lazyOnload" src="https://www.googletagmanager.com/gtag/js?id=G-PVJ12C7HR6"/>
      <Script strategy="lazyOnload">
        {`
            window.dataLayer = window.dataLayer || [];
            function gtag(){dataLayer.push(arguments);}
            gtag('js', new Date());
            gtag('config', 'G-PVJ12C7HR6');
          `}
      </Script>
    </>
  )
}
