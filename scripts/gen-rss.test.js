import { describe, it, expect } from 'vitest'
import { episodeToItem, shouldSkip } from './gen-rss.js'

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
