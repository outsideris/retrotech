const { promises: fs } = require('node:fs')
const path = require('node:path')
const RSS = require('rss')
const matter = require('gray-matter')

const SITE_URL = 'https://retrotech.outsider.dev'

// Index listing pages are not episodes and must be left out of the feed.
function shouldSkip(name) {
  return name.startsWith('index.')
}

// Map a parsed episode frontmatter (matter().data) to an RSS feed item.
function episodeToItem(name, data) {
  let description = data.description
  if (data.description2) {
    description += `\n${data.description2}`
  }

  return {
    title: data.title,
    url: `${SITE_URL}/episodes/${name.replace(/\.mdx?/, '')}`,
    date: `${data.date} 09:00`,
    description,
    author: data.author,
    enclosure: data.enclosure,
    duration: data.duration,
    custom_elements: [
      { duration: data.duration },
      { 'itunes:duration': data.duration },
      { 'itunes:explicit': 'no' },
      { 'itunes:author': 'Outsider' },
    ],
  }
}

// Order feed items newest-first, deterministically. Same-date items fall back
// to name (descending) so the order is stable across builds and filesystems.
function sortByDateDesc(episodes) {
  return [...episodes].sort((a, b) => {
    const diff = new Date(b.data.date) - new Date(a.data.date)
    return diff !== 0 ? diff : b.name.localeCompare(a.name)
  })
}

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
          {'itunes:email': 'outsideris@gmail.com'}
      ]},
      {'itunes:author': 'Outsider'},
      {'itunes:image': {
          _attr: {
            href: `${SITE_URL}/images/cover.jpg`
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

  // Read all episodes concurrently, then add them in a deterministic
  // newest-first order. feed.item() must run AFTER sorting — calling it inside
  // the concurrent reads makes item order depend on file-read completion timing.
  const parsed = await Promise.all(
    episodes
      .filter((name) => !shouldSkip(name))
      .map(async (name) => {
        const content = await fs.readFile(
          path.join(__dirname, '..', 'pages', 'episodes', name)
        )
        return { name, data: matter(content).data }
      })
  )

  sortByDateDesc(parsed).forEach(({ name, data }) => feed.item(episodeToItem(name, data)))

  await fs.writeFile('./public/feed.xml', feed.xml({ indent: true }))
  console.log('RSS feed generated!')
}

if (require.main === module) {
  generate()
}

module.exports = { episodeToItem, shouldSkip, sortByDateDesc, generate }
