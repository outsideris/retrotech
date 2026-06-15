const { promises: fs } = require('node:fs')
const path = require('node:path')
const RSS = require('rss')
const matter = require('gray-matter')

const SITE_URL = 'https://retrotech.outsider.dev'
const EPISODES_DIR = path.join(__dirname, '..', 'pages', 'episodes')

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

// Pure: build the feed XML string from parsed episodes (`{ name, data }[]`).
function buildFeedXml(episodes) {
  const feed = new RSS(feedOption)
  sortByDateDesc(episodes).forEach(({ name, data }) => feed.item(episodeToItem(name, data)))
  return feed.xml({ indent: true })
}

// Read and parse every episode file under pages/episodes (skipping index pages).
async function readEpisodes() {
  const names = (await Promise.all(
    (await fs.readdir(EPISODES_DIR)).map(async (name) => {
      if (!name.endsWith('.mdx') && !name.endsWith('.md')) {
        return (await fs.readdir(path.join(EPISODES_DIR, name))).map((file) => `${name}/${file}`)
      }
      return name
    })
  )).flat()

  return Promise.all(
    names
      .filter((name) => !shouldSkip(name))
      .map(async (name) => {
        const content = await fs.readFile(path.join(EPISODES_DIR, name))
        return { name, data: matter(content).data }
      })
  )
}

async function generate() {
  const episodes = await readEpisodes()
  await fs.writeFile('./public/feed.xml', buildFeedXml(episodes))
  console.log('RSS feed generated!')
}

if (require.main === module) {
  generate()
}

module.exports = { episodeToItem, shouldSkip, sortByDateDesc, buildFeedXml, readEpisodes, generate }
