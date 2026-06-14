// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'

// Mock Next's Image/Link so the test exercises only Badges' own logic
// (which platforms render) rather than Next internals.
vi.mock('next/image', () => ({
  default: (props: any) => (
    // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
    <img src={typeof props.src === 'string' ? props.src : ''} alt={props.alt} />
  ),
}))
vi.mock('next/link', () => ({
  default: (props: any) => <a href={props.href}>{props.children}</a>,
}))

import Badges from './Badges'

describe('Badges', () => {
  it('always renders Apple, YouTube, and Spotify badges', () => {
    render(<Badges />)
    expect(screen.getByAltText('Listen on Apple Podcasts')).toBeTruthy()
    expect(screen.getByAltText('Available on YouTube')).toBeTruthy()
    expect(screen.getByAltText('Listen on Spotify')).toBeTruthy()
  })

  it('shows the RSS badge (not Google) when no google prop is given', () => {
    render(<Badges />)
    expect(screen.getByAltText('Get the RSS Feed')).toBeTruthy()
    expect(screen.queryByAltText('Listen on Google Podcasts')).toBeNull()
  })

  it('shows the Google badge (not RSS) when a google link is given', () => {
    render(<Badges google="https://podcasts.google.com/show" />)
    expect(screen.getByAltText('Listen on Google Podcasts')).toBeTruthy()
    expect(screen.queryByAltText('Get the RSS Feed')).toBeNull()
  })

  it('uses the provided platform links', () => {
    render(<Badges apple="https://apple.test/ep" />)
    const apple = screen.getByAltText('Listen on Apple Podcasts').closest('a')
    expect(apple?.getAttribute('href')).toBe('https://apple.test/ep')
  })
})
