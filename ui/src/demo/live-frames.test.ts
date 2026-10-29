import { describe, expect, it, vi } from 'vitest'

// jsdom has no canvas: stand in for the drawing so the handler's success path runs.
vi.mock('./live-frames', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./live-frames')>()
  return { ...actual, renderLiveFrame: vi.fn(async () => new Blob(['jpeg'], { type: 'image/jpeg' })) }
})

import { liveScene } from './live-frames'
import { handleDemoRequest } from './server'

const live = (name: string, headers?: HeadersInit) =>
  handleDemoRequest(new Request(`http://localhost/api/v1/tasks/${name}/live?namespace=default`, { headers }))

describe('demo live view scenes', () => {
  it('moves through product, cart, checkout, and review each minute', () => {
    expect(liveScene(1_000).step).toBe('product')
    expect(liveScene(10_000).step).toBe('cart')
    expect(liveScene(20_000).step).toBe('checkout')
    expect(liveScene(45_000).step).toBe('review')
    expect(liveScene(61_000).step).toBe('product')
  })

  it('types into checkout one character at a time', () => {
    const early = liveScene(13_000)
    const later = liveScene(20_000)
    expect(later.typed).toBeGreaterThan(early.typed)
    expect(later.key).not.toBe(early.key)
  })

  it('keeps the review page still, so the same picture keeps its key and time', () => {
    const first = liveScene(34_000)
    const last = liveScene(59_000)
    expect(last.key).toBe(first.key)
    expect(last.since).toBe(first.since)
    expect(liveScene(94_000).key).not.toBe(first.key)
  })
})

describe('demo live view API', () => {
  it('serves the seeded browser Task a frame, and 304 for the same picture', async () => {
    const response = await live('storefront-checkout-smoke-g5h2w')
    expect(response.status).toBe(200)
    expect(response.headers.get('Content-Type')).toBe('image/jpeg')
    const etag = response.headers.get('ETag')
    expect(etag).toBeTruthy()
    expect(response.headers.get('X-Orka-Frame-Time')).toBeTruthy()

    const again = await live('storefront-checkout-smoke-g5h2w', { 'If-None-Match': etag ?? '' })
    expect([200, 304]).toContain(again.status)
  })

  it('has no live view for other Tasks', async () => {
    expect((await live('terraform-drift-check-u5r1a')).status).toBe(404)
    expect((await live('checkout-latency-investigation-7k2qp')).status).toBe(404)
    expect((await live('no-such-task')).status).toBe(404)
  })

  it('has no live view for a running container Task on another image', async () => {
    vi.useFakeTimers({ toFake: ['Date'], now: Date.now() })
    try {
      const forked = await handleDemoRequest(new Request('http://localhost/api/v1/tasks/db-migrate-orders-v42-e2l9o/fork?namespace=default', {
        method: 'POST', body: '{}', headers: { 'Content-Type': 'application/json' },
      }))
      const { newTaskName } = (await forked.json()) as { newTaskName: string }
      vi.setSystemTime(Date.now() + 10_000)
      const task = (await (await handleDemoRequest(new Request(`http://localhost/api/v1/tasks/${newTaskName}?namespace=default`))).json()) as {
        status?: { phase?: string }
      }
      expect(task.status?.phase).toBe('Running')
      expect((await live(newTaskName)).status).toBe(404)
    } finally {
      vi.useRealTimers()
    }
  })
})
