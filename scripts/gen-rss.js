const { promises: fs } = require('node:fs')
const path = require('node:path')
const RSS = require('rss')
const matter = require('gray-matter')

const SITE_URL = 'https://retrotech.outsider.dev'

async function generate() {
  const feedOption = {
    title: 'RetroTech 팟캐스트',
    site_url: SITE_URL,
    feed_url: `${SITE_URL}/feed.xml`,
    language: 'ko',
    custom_namespaces: {
      'itunes': 'http://www.itunes.com/dtds/podcast-1.0.dtd'
    },
    custom_elements: [
      {'itunes:owner': [
          {'itunes:name': 'Outsider'},
          {'itunes:email': 'outsideris@gmail.com.com'}
      ]},
      {'itunes:author': 'Outsider'},
      {'itunes:image': {
          _attr: {
            href: `${SITE_URL}/images/cover.png`
          }
      }},
      {'itunes:explicit': 'no'},
      {'itunes:category': [
          {_attr: {
              text: 'Technology'
          }},
      ]}
    ],
  }
  const feed = new RSS(feedOption)

  const episodeFiles = await fs.readdir(path.join(__dirname, '..', 'pages', 'episodes'))
  const episodes = (await Promise.all(episodeFiles.map(async (name) => {
    if (!name.endsWith('.mdx') && !name.endsWith('.md')) {
      return (await fs.readdir(path.join(__dirname, '..', 'pages', 'episodes', name))).map((file) => `${name}/${file}`)
    }

    return name
  }))).flat()

  await Promise.all(
    episodes.map(async (name) => {
      if (name.startsWith('index.')) return

      const content = await fs.readFile(
        path.join(__dirname, '..', 'pages', 'episodes', name)
      )
      const frontmatter = matter(content)

      feed.item({
        title: frontmatter.data.title,
        url: `${SITE_URL}/episodes/${name.replace(/\.mdx?/, '')}`,
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
  console.log('RSS feed generated!')
}

generate()
