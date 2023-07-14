const YEAR = new Date().getFullYear()

export default {
  darkMode: true,
  footer: (
    <footer>
      <small>
        <time>{YEAR}</time> © Outsider.
        <a href="/feed.xml">
          <i className="fa-solid fa-rss"></i>
        </a>
      </small>
      <h3>Host:</h3>
      <div>
        <img src="/images/outsider.png" alt="Outsider" width="120px" className="profile" />
        <strong>Outsider</strong><br/>
        <i className="fa-brands fa-twitter"></i> <a href="https://twitter.com/outsideris">outsideris</a><br/>
        <i className="fa-brands fa-github"></i> <a href="https://github.com/outsideris">outsideris</a><br/>
        <i className="fa-solid fa-blog"></i> <a href="https://blog.outsider.ne.kr/">blog.outsider.ne.kr</a>
      </div>
      <style jsx>{`
        footer {
          margin-top: 8rem;
        }
        footer > small > a {
          float: right;
        }
        footer > div > img {
          float: left;
          margin: 0 10px 0 0;
        }
      `}</style>

    </footer>
  ),
}
