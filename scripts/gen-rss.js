const { promises: fs } = require('node:fs')
const path = require('node:path')
const RSS = require('rss')
const matter = require('gray-matter')

async function generate() {
  const feedOption = {
    title: 'RetroTech 팟캐스트',
    site_url: 'https://yoursite.com',
    feed_url: 'https://yoursite.com/feed.xml',
    language: 'ko',
    custom_namespaces: {
      'itunes': 'http://www.itunes.com/dtds/podcast-1.0.dtd'
    },
  }
  const feed = new RSS(feedOption)

  const episodes = await fs.readdir(path.join(__dirname, '..', 'pages', 'episodes'))

  await Promise.all(
    episodes.map(async (name) => {
      if (name.startsWith('index.')) return

      const content = await fs.readFile(
        path.join(__dirname, '..', 'pages', 'episodes', name)
      )
      const frontmatter = matter(content)

      feed.item({
        title: frontmatter.data.title,
        url: `${feedOption.site_url}/episodes/${name.replace(/\.mdx?/, '')}`,
        date: `${frontmatter.data.date} 09:00`,
        description: frontmatter.data.description,
        author: frontmatter.data.author,
        enclosure: frontmatter.data.enclosure,
        duration: frontmatter.data.duration,
        custom_elements: [
          {'duration': frontmatter.data.duration},
          {'itunes:duration': frontmatter.data.duration},
          {'itunes:explicit': 'no'},
          {'itunes:author': 'Outsider'},
        ]
      })
    })
  )

  await fs.writeFile('./public/feed.xml', feed.xml({ indent: true }))
}

generate()
