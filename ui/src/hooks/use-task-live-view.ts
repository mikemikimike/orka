import { useEffect, useState } from 'react'
import { API_BASE_URL } from '@/lib/constants'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'

export type TaskLiveView =
  | { status: 'off' }
  | { status: 'waiting' }
  | { status: 'live'; src: string; frameTime?: string }

/** How often the latest frame is fetched; the worker sends at most two a second. */
export const LIVE_VIEW_POLL_MS = 1000

export function taskLiveViewUrl(taskId: string, namespace: string) {
  const ns = namespace ? `?namespace=${encodeURIComponent(namespace)}` : ''
  return `${API_BASE_URL}/tasks/${encodeURIComponent(taskId)}/live${ns}`
}

// frameDataUrl turns a frame into a data: URL. The dashboard's Content
// Security Policy allows data: images but not blob: URLs.
function frameDataUrl(frame: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(new Blob([frame], { type: 'image/jpeg' }))
  })
}

// Polls the latest screencast frame of a running browser Task. A frame is a
// JPEG fetched with the bearer token (auth is header-only, so a bare <img src>
// would get 401) and shown as a data: URL. 204 means live view is on
// but no page has repainted yet; 404 means this Task has no live view, which
// ends the polling. An unchanged frame answers 304 and keeps the current one.
export function useTaskLiveView(taskId: string, enabled: boolean): TaskLiveView {
  const namespace = useUIStore((s) => s.namespace)
  const [view, setView] = useState<TaskLiveView>({ status: 'waiting' })

  useEffect(() => {
    if (!enabled || !taskId) return
    let stopped = false
    let etag = ''
    let timer: ReturnType<typeof setTimeout> | undefined
    const abort = new AbortController()

    const poll = async () => {
      try {
        const token = useAuthStore.getState().token
        const headers: Record<string, string> = {}
        if (token) headers.Authorization = `Bearer ${token}`
        if (etag) headers['If-None-Match'] = etag
        const res = await fetch(taskLiveViewUrl(taskId, namespace), { headers, signal: abort.signal, cache: 'no-store' })
        if (stopped) return
        if (res.status === 404 || res.status === 501) {
          setView({ status: 'off' })
          return
        }
        if (res.status === 200) {
          const src = await frameDataUrl(await res.blob())
          if (stopped) return
          etag = res.headers.get('ETag') ?? ''
          setView({ status: 'live', src, frameTime: res.headers.get('X-Orka-Frame-Time') ?? undefined })
        } else if (res.status === 204) {
          setView((current) => (current.status === 'live' ? current : { status: 'waiting' }))
        }
      } catch {
        // A dropped request is retried on the next tick.
      }
      if (!stopped) timer = setTimeout(poll, LIVE_VIEW_POLL_MS)
    }
    void poll()

    return () => {
      stopped = true
      abort.abort()
      clearTimeout(timer)
    }
  }, [taskId, namespace, enabled])

  return enabled ? view : { status: 'off' }
}
