import { useBlogContext } from 'nextra-theme-blog'

const YEAR = new Date().getFullYear()

export default {
  darkMode: true,
  head: () => {
    const { opts } = useBlogContext()
    let title = '';
    if (opts.title === 'RetroTech') {
      title = 'RetroTech 팟캐스트'
    } else {
      title = `${opts.title.replace('\n', '')} - RetroTech`
    }

    return (
      <>
        <meta property="og:title" content={title} />
        <meta name="twitter:title" content={title} />
      </>
    )
  },
  footer: (
    <footer>
      <iframe src="https://github.com/sponsors/outsideris/button" title="Sponsor outsideris" height="32" width="114"></iframe>
      <h3>Host:</h3>
      <div>
        <img src="/images/outsider.png" alt="Outsider" width="120px" className="profile" />
        <strong>Outsider</strong><br/>
        <i className="fa-brands fa-twitter"></i> <a href="https://twitter.com/outsideris">outsideris</a><br/>
        <i className="fa-brands fa-github"></i> <a href="https://github.com/outsideris">outsideris</a><br/>
        <i className="fa-solid fa-blog"></i> <a href="https://blog.outsider.ne.kr/">blog.outsider.ne.kr</a>
      </div>
      <small>
        <time>{YEAR}</time> © Outsider.
        <a href="/feed.xml">
          <i className="fa-solid fa-rss"></i>
        </a>
      </small>
      <style jsx>{`
        footer {
          margin-top: 8rem;
        }
        footer > small {
          display: block;
          margin-top: 30px;
        }
        footer > small > a {
          float: right;
        }
        footer > div > img {
          float: left;
          margin: 0 10px 0 0;
        }
        footer > iframe {
          border: 0; 
          border-radius: 6px;
          margin: 0 auto;
        }
      `}</style>

    </footer>
  ),
}
