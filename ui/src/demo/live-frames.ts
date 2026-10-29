// Live view frames for the demo's browser Task: a recording of a real one.
// A browser Task ran Playwright through the storefront checkout in
// CHECKOUT_SMOKE_SCRIPT on a kind cluster with --browser-live-view; the
// storefront was served to its browser by the script. These are the frames
// GET /api/v1/tasks/{id}/live returned while it ran, polled as the dashboard
// polls, at the controller's frame times. Back-to-back repeats are dropped and
// each picture is stored once. The demo loops the recording, holding the
// review page still as the run did, so the dashboard gets 304 there.

/** The image the demo's browser Tasks run, so the live view applies to them. */
export const DEMO_BROWSER_IMAGE = 'ghcr.io/orka-agents/orka/browser-worker:0.4.0'

const FILES = import.meta.glob<string>('./live-recording/*.jpg', { eager: true, query: '?url', import: 'default' })

/** Each frame and when it arrived, in milliseconds from the first. */
const RECORDING: readonly (readonly [file: string, at: number])[] = [
  ['frame-00.jpg', 0], ['frame-01.jpg', 2519], ['frame-02.jpg', 4012], ['frame-00.jpg', 5506],
  ['frame-03.jpg', 7024], ['frame-04.jpg', 7513], ['frame-05.jpg', 8000], ['frame-06.jpg', 10002],
  ['frame-07.jpg', 12506], ['frame-08.jpg', 14021], ['frame-09.jpg', 14505], ['frame-10.jpg', 15001],
  ['frame-11.jpg', 15504], ['frame-12.jpg', 16001], ['frame-13.jpg', 16500], ['frame-14.jpg', 17004],
  ['frame-15.jpg', 17517], ['frame-16.jpg', 18013], ['frame-17.jpg', 18499], ['frame-18.jpg', 19018],
  ['frame-19.jpg', 19503], ['frame-20.jpg', 20015], ['frame-21.jpg', 20503], ['frame-22.jpg', 21005],
  ['frame-23.jpg', 21513], ['frame-24.jpg', 22012], ['frame-25.jpg', 22504], ['frame-26.jpg', 23012],
  ['frame-27.jpg', 23515], ['frame-28.jpg', 25009], ['frame-29.jpg', 25514],
]

/** The run stood on the review page this long before it closed the browser. */
const REVIEW_STILL_MS = 15_000
const LOOP_MS = RECORDING[RECORDING.length - 1][1] + REVIEW_STILL_MS

export interface LiveScene {
  /** The recorded frame to show. */
  file: string
  /** Names the picture: equal keys are the same frame. */
  key: string
  /** Milliseconds from the Task's start to when this picture first appeared. */
  since: number
}

/** What the demo browser shows elapsedMs after its Task started. */
export function liveScene(elapsedMs: number): LiveScene {
  const elapsed = Math.max(0, elapsedMs)
  const loop = Math.floor(elapsed / LOOP_MS)
  const t = elapsed - loop * LOOP_MS
  let index = 0
  while (index + 1 < RECORDING.length && RECORDING[index + 1][1] <= t) index++
  const [file, at] = RECORDING[index]
  return { file, key: `${loop}-${index}`, since: loop * LOOP_MS + at }
}

// Fetched frames, so repeated polls of the same picture are not fetched again.
const cache = new Map<string, Promise<Blob | null>>()

/** The scene's recorded JPEG, or null when it could not be loaded. */
export function liveFrame(scene: LiveScene): Promise<Blob | null> {
  const hit = cache.get(scene.file)
  if (hit) return hit
  const url = FILES[`./live-recording/${scene.file}`]
  const frame = url
    ? fetch(url).then((r) => (r.ok ? r.blob() : null)).catch(() => null)
    : Promise.resolve(null)
  // A failed load is tried again on the next poll.
  void frame.then((blob) => { if (!blob) cache.delete(scene.file) })
  cache.set(scene.file, frame)
  return frame
}
