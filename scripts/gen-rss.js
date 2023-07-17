const { promises: fs } = require('node:fs')
const path = require('node:path')
const RSS = require('rss')
const matter = require('gray-matter')

async function generate() {
  const feed = new RSS({
    title: 'RetroTech 팟캐스트',
    site_url: 'https://yoursite.com',
    feed_url: 'https://yoursite.com/feed.xml',
  })

  const posts = await fs.readdir(path.join(__dirname, '..', 'pages', 'episodes'))

  await Promise.all(
    posts.map(async (name) => {
      if (name.startsWith('index.')) return

      const content = await fs.readFile(
        path.join(__dirname, '..', 'pages', 'episodes', name)
      )
      const frontmatter = matter(content)

      feed.item({
        title: frontmatter.data.title,
        url: '/episodes/' + name.replace(/\.mdx?/, ''),
        date: frontmatter.data.date,
        description: frontmatter.data.description,
        author: frontmatter.data.author,
      })
    })
  )

  await fs.writeFile('./public/feed.xml', feed.xml({ indent: true }))
}

generate()
