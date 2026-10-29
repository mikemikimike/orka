// Live view frames for the demo's browser Tasks. No browser runs in the demo,
// so each frame is drawn: a made-up storefront checkout that loops every
// minute, with still stretches where the picture does not change and the
// dashboard gets 304, as with a real page that sits still.

/** The image the demo's browser Tasks run, so the live view applies to them. */
export const DEMO_BROWSER_IMAGE = 'ghcr.io/orka-agents/orka/browser-worker:0.4.0'

const LOOP_MS = 60_000
const STEP_MS = 500
const WIDTH = 1280
const HEIGHT = 800
const SITE = 'staging.shop.northwind.example'

const NAME = 'Orka Test'
const EMAIL = 'orka@northwind.example'
const ZIP = '98101'
const TYPED = NAME.length + EMAIL.length + ZIP.length

export type LiveStep = 'product' | 'cart' | 'checkout' | 'review'

export interface LiveScene {
  step: LiveStep
  url: string
  /** Characters typed into the checkout form so far. */
  typed: number
  cursor: { x: number; y: number }
  /** The product page shows "Added to cart". */
  added: boolean
  /** Names the picture: equal keys draw the same frame. */
  key: string
  /** Milliseconds from the Task's start to when this picture first appeared. */
  since: number
}

const lerp = (from: number, to: number, t: number) => Math.round(from + (to - from) * Math.min(1, Math.max(0, t)))
const steps = (ms: number) => Math.floor(ms / STEP_MS)

/** What the demo browser shows elapsedMs after its Task started. */
export function liveScene(elapsedMs: number): LiveScene {
  const elapsed = Math.max(0, elapsedMs)
  const loop = Math.floor(elapsed / LOOP_MS)
  const t = elapsed - loop * LOOP_MS
  const base = loop * LOOP_MS
  let scene: Omit<LiveScene, 'key' | 'since'>
  let since: number

  if (t < 4_000) {
    // Move to "Add to cart", one step at a time.
    const at = steps(t) * STEP_MS
    scene = { step: 'product', url: `https://${SITE}/products/trail-backpack-28l`, typed: 0, added: false,
      cursor: { x: lerp(1060, 860, at / 4_000), y: lerp(640, 470, at / 4_000) } }
    since = base + at
  } else if (t < 9_000) {
    scene = { step: 'product', url: `https://${SITE}/products/trail-backpack-28l`, typed: 0, added: true, cursor: { x: 860, y: 470 } }
    since = base + 4_000
  } else if (t < 12_000) {
    const at = steps(t - 9_000) * STEP_MS
    scene = { step: 'cart', url: `https://${SITE}/cart`, typed: 0, added: true,
      cursor: { x: lerp(860, 1010, at / 3_000), y: lerp(470, 560, at / 3_000) } }
    since = base + 9_000 + at
  } else if (t < 30_000) {
    // Type one character every step.
    const typed = Math.min(TYPED, steps(t - 12_000) + 1)
    scene = { step: 'checkout', url: `https://${SITE}/checkout`, typed, added: true, cursor: { x: 760, y: 300 } }
    since = base + 12_000 + (typed - 1) * STEP_MS
  } else if (t < 33_000) {
    const at = steps(t - 30_000) * STEP_MS
    scene = { step: 'checkout', url: `https://${SITE}/checkout`, typed: TYPED, added: true,
      cursor: { x: lerp(760, 380, at / 3_000), y: lerp(300, 640, at / 3_000) } }
    since = base + 30_000 + at
  } else {
    // The review page: the run stops here and the picture stays still.
    scene = { step: 'review', url: `https://${SITE}/checkout/review`, typed: TYPED, added: true, cursor: { x: 380, y: 640 } }
    since = base + 33_000
  }
  const key = `${loop}-${scene.step}-${scene.typed}-${scene.added ? 1 : 0}-${scene.cursor.x}-${scene.cursor.y}`
  return { ...scene, key, since }
}

type Ctx = CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D

const FONT = 'system-ui, -apple-system, "Segoe UI", sans-serif'

function rect(ctx: Ctx, x: number, y: number, w: number, h: number, fill: string, radius = 8) {
  ctx.fillStyle = fill
  ctx.beginPath()
  ctx.roundRect(x, y, w, h, radius)
  ctx.fill()
}

function text(ctx: Ctx, value: string, x: number, y: number, size: number, color = '#111827', weight = 400) {
  ctx.fillStyle = color
  ctx.font = `${weight} ${size}px ${FONT}`
  ctx.fillText(value, x, y)
}

function button(ctx: Ctx, label: string, x: number, y: number, w: number, color = '#1f6f4a') {
  rect(ctx, x, y, w, 52, color, 10)
  text(ctx, label, x + 24, y + 33, 20, '#ffffff', 600)
}

function field(ctx: Ctx, label: string, value: string, x: number, y: number, active: boolean) {
  text(ctx, label, x, y, 16, '#4b5563', 500)
  rect(ctx, x, y + 10, 520, 48, active ? '#2563eb' : '#d1d5db', 8)
  rect(ctx, x + 2, y + 12, 516, 44, '#ffffff', 7)
  text(ctx, value, x + 16, y + 42, 19)
  if (active) {
    ctx.font = `400 19px ${FONT}`
    rect(ctx, x + 17 + ctx.measureText(value).width, y + 22, 2, 24, '#111827', 0)
  }
}

function cursor(ctx: Ctx, x: number, y: number) {
  ctx.fillStyle = '#111827'
  ctx.strokeStyle = '#ffffff'
  ctx.lineWidth = 2
  ctx.beginPath()
  ctx.moveTo(x, y)
  ctx.lineTo(x, y + 26)
  ctx.lineTo(x + 7, y + 20)
  ctx.lineTo(x + 12, y + 31)
  ctx.lineTo(x + 17, y + 29)
  ctx.lineTo(x + 12, y + 18)
  ctx.lineTo(x + 21, y + 18)
  ctx.closePath()
  ctx.stroke()
  ctx.fill()
}

function backpack(ctx: Ctx, x: number, y: number) {
  rect(ctx, x, y, 360, 360, '#eef2ec', 16)
  rect(ctx, x + 110, y + 70, 140, 220, '#2f5d46', 40)
  rect(ctx, x + 135, y + 150, 90, 90, '#3d7a5b', 18)
  rect(ctx, x + 150, y + 40, 60, 40, '#2f5d46', 20)
}

function drawScene(ctx: Ctx, scene: LiveScene) {
  ctx.fillStyle = '#ffffff'
  ctx.fillRect(0, 0, WIDTH, HEIGHT)
  // Browser chrome.
  ctx.fillStyle = '#e5e7eb'
  ctx.fillRect(0, 0, WIDTH, 64)
  for (const [i, color] of ['#ef4444', '#f59e0b', '#22c55e'].entries()) {
    ctx.fillStyle = color
    ctx.beginPath()
    ctx.arc(28 + i * 22, 32, 7, 0, Math.PI * 2)
    ctx.fill()
  }
  rect(ctx, 110, 14, 1040, 36, '#ffffff', 18)
  text(ctx, scene.url, 132, 38, 16, '#374151')
  // Site header.
  ctx.fillStyle = '#1f2937'
  ctx.fillRect(0, 64, WIDTH, 64)
  text(ctx, 'Northwind Outfitters', 40, 104, 24, '#ffffff', 700)
  text(ctx, scene.step === 'product' && !scene.added ? 'Cart (0)' : 'Cart (1)', 1130, 104, 18, '#d1d5db', 500)

  if (scene.step === 'product') {
    backpack(ctx, 80, 170)
    text(ctx, 'Trail Backpack 28L', 500, 220, 36, '#111827', 700)
    text(ctx, '$89.00', 500, 270, 28, '#1f6f4a', 700)
    text(ctx, 'Water-resistant, 28 liters, padded laptop sleeve.', 500, 320, 19, '#4b5563')
    button(ctx, scene.added ? 'Added to cart' : 'Add to cart', 760, 440, 220, scene.added ? '#4b5563' : '#1f6f4a')
  } else if (scene.step === 'cart') {
    text(ctx, 'Your cart', 80, 200, 34, '#111827', 700)
    rect(ctx, 80, 230, 1120, 110, '#f9fafb', 12)
    text(ctx, 'Trail Backpack 28L', 110, 280, 22, '#111827', 600)
    text(ctx, 'Qty 1', 110, 312, 17, '#6b7280')
    text(ctx, '$89.00', 1080, 295, 22, '#111827', 600)
    text(ctx, 'Subtotal: $89.00', 80, 420, 22, '#111827', 600)
    button(ctx, 'Checkout', 960, 540, 200)
  } else if (scene.step === 'checkout') {
    text(ctx, 'Checkout', 80, 190, 34, '#111827', 700)
    const name = NAME.slice(0, scene.typed)
    const email = EMAIL.slice(0, Math.max(0, scene.typed - NAME.length))
    const zip = ZIP.slice(0, Math.max(0, scene.typed - NAME.length - EMAIL.length))
    const at = scene.typed <= NAME.length ? 0 : scene.typed <= NAME.length + EMAIL.length ? 1 : 2
    const typing = scene.typed < TYPED
    field(ctx, 'Full name', name, 80, 240, typing && at === 0)
    field(ctx, 'Email', email, 80, 350, typing && at === 1)
    field(ctx, 'Postal code', zip, 80, 460, typing && at === 2)
    button(ctx, 'Continue to review', 80, 600, 290)
    rect(ctx, 700, 240, 500, 200, '#f9fafb', 12)
    text(ctx, 'Trail Backpack 28L', 730, 290, 20, '#111827', 600)
    text(ctx, 'Subtotal $89.00', 730, 330, 18, '#4b5563')
  } else {
    text(ctx, 'Review your order', 80, 190, 34, '#111827', 700)
    rect(ctx, 80, 220, 700, 300, '#f9fafb', 12)
    const rows: [string, string][] = [['Trail Backpack 28L', '$89.00'], ['Shipping', '$0.00'], ['Tax', '$7.12']]
    rows.forEach(([label, value], i) => {
      text(ctx, label, 110, 275 + i * 50, 20, '#374151')
      text(ctx, value, 660, 275 + i * 50, 20, '#374151')
    })
    text(ctx, 'Total', 110, 450, 24, '#111827', 700)
    text(ctx, '$96.12', 640, 450, 24, '#111827', 700)
    text(ctx, `Ship to ${NAME}, ${ZIP}`, 110, 495, 17, '#6b7280')
    button(ctx, 'Place order', 80, 560, 220, '#9ca3af')
    rect(ctx, 340, 562, 840, 48, '#fef3c7', 10)
    text(ctx, 'Stopped here: this run prepares the order, it does not place it.', 360, 593, 18, '#92400e', 500)
  }
  cursor(ctx, scene.cursor.x, scene.cursor.y)
}

// The last few drawn frames, so repeated polls of the same picture are not redrawn.
const cache = new Map<string, Promise<Blob | null>>()

/** Draws scene as a JPEG, or null where the browser has no canvas (tests). */
export function renderLiveFrame(scene: LiveScene): Promise<Blob | null> {
  const hit = cache.get(scene.key)
  if (hit) return hit
  const frame = draw(scene)
  cache.set(scene.key, frame)
  while (cache.size > 12) cache.delete(cache.keys().next().value as string)
  return frame
}

async function draw(scene: LiveScene): Promise<Blob | null> {
  try {
    if (typeof OffscreenCanvas !== 'undefined') {
      const canvas = new OffscreenCanvas(WIDTH, HEIGHT)
      const ctx = canvas.getContext('2d')
      if (!ctx) return null
      drawScene(ctx, scene)
      return await canvas.convertToBlob({ type: 'image/jpeg', quality: 0.8 })
    }
    if (typeof document === 'undefined') return null
    const canvas = document.createElement('canvas')
    canvas.width = WIDTH
    canvas.height = HEIGHT
    const ctx = canvas.getContext('2d')
    if (!ctx) return null
    drawScene(ctx, scene)
    return await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.8))
  } catch {
    return null
  }
}
