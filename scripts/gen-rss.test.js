import { describe, it, expect } from 'vitest'
import { episodeToItem, shouldSkip, sortByDateDesc, buildFeedXml, readEpisodes } from './gen-rss.js'

const baseData = {
  title: '2g. VCS: SourceForge',
  date: '2026/03/07',
  description: '본문 요약',
  author: 'Outsider',
  enclosure: { url: 'https://retrotech-episodes.outsider.dev/2g.mp3', size: 66997696 },
  duration: '55:50',
}

describe('episodeToItem', () => {
  it('maps frontmatter fields to an RSS item', () => {
    const item = episodeToItem('2g.mdx', baseData)
    expect(item.title).toBe('2g. VCS: SourceForge')
    expect(item.url).toBe('https://retrotech.outsider.dev/episodes/2g')
    expect(item.date).toBe('2026/03/07 09:00')
    expect(item.description).toBe('본문 요약')
    expect(item.author).toBe('Outsider')
    expect(item.enclosure).toEqual(baseData.enclosure)
    expect(item.duration).toBe('55:50')
  })

  it('appends description2 with a newline when present', () => {
    const item = episodeToItem('2g.mdx', { ...baseData, description2: '레퍼런스 안내' })
    expect(item.description).toBe('본문 요약\n레퍼런스 안내')
  })

  it('strips the .mdx/.md extension from the url slug', () => {
    expect(episodeToItem('0.mdx', baseData).url).toBe(
      'https://retrotech.outsider.dev/episodes/0'
    )
    expect(episodeToItem('250127-breaks.mdx', baseData).url).toBe(
      'https://retrotech.outsider.dev/episodes/250127-breaks'
    )
  })

  it('includes the iTunes custom elements', () => {
    const item = episodeToItem('2g.mdx', baseData)
    expect(item.custom_elements).toEqual([
      { duration: '55:50' },
      { 'itunes:duration': '55:50' },
      { 'itunes:explicit': 'no' },
      { 'itunes:author': 'Outsider' },
    ])
  })
})

describe('shouldSkip', () => {
  it('skips index listing files', () => {
    expect(shouldSkip('index.md')).toBe(true)
    expect(shouldSkip('index.mdx')).toBe(true)
  })

  it('keeps episode files', () => {
    expect(shouldSkip('2g.mdx')).toBe(false)
    expect(shouldSkip('0.mdx')).toBe(false)
  })
})

describe('sortByDateDesc', () => {
  it('orders episodes newest-first, deterministically regardless of input order', () => {
    const input = [
      { name: '2e.mdx', data: { date: '2025/10/08' } },
      { name: '2g.mdx', data: { date: '2026/03/07' } },
      { name: '2f.mdx', data: { date: '2025/12/26' } },
    ]
    const expected = ['2g.mdx', '2f.mdx', '2e.mdx']
    expect(sortByDateDesc(input).map((e) => e.name)).toEqual(expected)
    // 입력 순서를 섞어도 결과가 같아야 한다(결정적).
    expect(sortByDateDesc([input[2], input[0], input[1]]).map((e) => e.name)).toEqual(expected)
  })

  it('breaks same-date ties by name descending (stable across input order)', () => {
    const a = { name: '1a.mdx', data: { date: '2023/07/31' } }
    const b = { name: '1b.mdx', data: { date: '2023/07/31' } }
    expect(sortByDateDesc([a, b]).map((e) => e.name)).toEqual(['1b.mdx', '1a.mdx'])
    expect(sortByDateDesc([b, a]).map((e) => e.name)).toEqual(['1b.mdx', '1a.mdx'])
  })

  it('does not mutate the input array', () => {
    const input = [
      { name: '0.mdx', data: { date: '2023/07/24' } },
      { name: '2g.mdx', data: { date: '2026/03/07' } },
    ]
    const before = input.map((e) => e.name)
    sortByDateDesc(input)
    expect(input.map((e) => e.name)).toEqual(before)
  })
})

// Format/structure regression guard: render a fixed fixture feed and snapshot it.
// Catches unintended changes to the feed's shape (fields, iTunes namespace,
// ordering, escaping) from refactors or dependency bumps. Stable across real
// episode changes because it uses fixtures, not pages/episodes.
describe('buildFeedXml (format)', () => {
  it('renders channel + items in the expected structure (newest first, escaped)', () => {
    const fixtures = [
      {
        name: '1a.mdx',
        data: {
          title: 'First & Oldest',
          date: '2024/01/01',
          description: 'one < two',
          author: 'Outsider',
          enclosure: { url: 'https://retrotech-episodes.outsider.dev/1a.mp3', size: 1000 },
          duration: '10:00',
        },
      },
      {
        name: '2b.mdx',
        data: {
          title: 'Second',
          date: '2024/02/02',
          description: 'desc',
          description2: 'extra line',
          author: 'Outsider',
          enclosure: { url: 'https://retrotech-episodes.outsider.dev/2b.mp3', size: 2000 },
          duration: '20:00',
        },
      },
    ]
    const xml = buildFeedXml(fixtures).replace(
      /<lastBuildDate>.*?<\/lastBuildDate>/,
      '<lastBuildDate>NORMALIZED</lastBuildDate>'
    )
    expect(xml).toMatchSnapshot()
  })
})

// Data validity guard: run against the REAL episodes. Stable across additions
// (no snapshot of content) but fails if any episode is missing/malformed.
describe('feed from real episodes', () => {
  it('every episode has the fields the feed requires', async () => {
    const episodes = await readEpisodes()
    expect(episodes.length).toBeGreaterThan(20)

    const problems = []
    for (const { name, data } of episodes) {
      if (!data.title) problems.push(`${name}: missing title`)
      if (!data.date || Number.isNaN(Date.parse(data.date))) problems.push(`${name}: bad/missing date`)
      if (!data.duration) problems.push(`${name}: missing duration`)
      if (!data.description) problems.push(`${name}: missing description`)
      if (!/\.mp3$/.test(data.enclosure?.url || '')) problems.push(`${name}: bad enclosure.url`)
    }
    expect(problems).toEqual([])
  })

  it('builds a feed including every episode exactly once', async () => {
    const episodes = await readEpisodes()
    const xml = buildFeedXml(episodes)
    expect(xml).toContain('<rss')
    expect((xml.match(/<item>/g) || []).length).toBe(episodes.length)
    expect(episodes.some(({ name }) => shouldSkip(name))).toBe(false)
  })
})
