import { createContext, useContext, useEffect, useRef, useState } from 'react'
import { apiRequest } from './api.js'

export const JournalContext = createContext(null)
export const blankDraft = () => ({ id: crypto.randomUUID(), title: '', body: '', existing: false, updatedAt: null })
export function useJournal() { return useContext(JournalContext) }
export function memoryLabel(memory) {
  if (memory.status === 'returned') return 'Opened'
  if (memory.status === 'sealed') return memory.can_open ? 'Ready to open' : 'Sealed'
  return 'Saved'
}
export function readableMemory(memory) {
  const date = new Date(memory.created_at)
  return { ...memory, rawTitle: memory.title || '', date: memory.created_at, day: String(date.getDate()).padStart(2, '0'),
    month: date.toLocaleDateString('en', { month: 'short' }).toUpperCase(),
    title: memory.title || (memory.status === 'sealed' ? 'A sealed memory' : 'Untitled memory'),
    excerpt: memory.status === 'sealed' ? 'Your words stay private until you open this capsule.' : (memory.body || '').trim().slice(0, 120),
    tag: memoryLabel(memory) }
}
export function journalError(error) {
  if (error.status === 401) return 'Your sign-in has expired. Sign in again to continue.'
  if (error.status === 400) return 'Check the details. The return time must be at least one minute in the future.'
  if (error.status === 404) return 'This memory is unavailable or has been deleted.'
  if (error.status === 409) return 'This memory changed, or has linked writing or media. Refresh before trying again.'
  if (error.status === 423) return 'This capsule is still sealed. It can only be opened after its return time.'
  if (error.status >= 500) return 'Your journal could not be reached. Your saved writing is still safe. Please try again.'
  return error.message
}


export function useMemoryCollection({ q = '', state = 'all', sort = 'newest' } = {}) {
  const { workspace, revision, tick, sessionExpired } = useJournal()
  const params = new URLSearchParams({ workspace_id: workspace?.id || '', q, state, sort }).toString()
  const key = `${params}|${revision}`
  const [result, setResult] = useState({ key: '', items: [], status: 'loading', error: '', more: false })
  const request = useRef(null)
  useEffect(() => {
    if (!workspace) return
    const controller = new AbortController()
    const pages = request.current?.key === key ? request.current.pages : 1
    const job = { key, controller, paging: false, pages }
    request.current = job
    const readPages = async () => {
      let items = [], page = []
      for (let number = 0; number < pages; number += 1) {
        const before = items.length ? `&before=${items.at(-1).id}` : ''
        page = await apiRequest(`/memories?${params}${before}`, { signal: controller.signal })
        items = items.concat(page)
        if (page.length < 100) break
      }
      return { items, more: page.length === 100 }
    }
    readPages().then(({ items, more }) => {
      if (controller.signal.aborted) return
      setResult({ key, items: items.map(readableMemory), status: 'ready', error: '', more })
    }).catch((error) => {
      if (controller.signal.aborted) return
      if (error.status === 401) sessionExpired()
      setResult((current) => ({ ...(current.key === key ? current : { items: [], more: false }), key, status: 'error', error: journalError(error) }))
    })
    return () => controller.abort()
  }, [workspace, key, params, tick, sessionExpired])
  const current = result.key === key ? result : { items: [], status: 'loading', error: '', more: false }
  async function loadMore() {
    const job = request.current
    if (!job || job.key !== key || job.paging || !current.more) return
    job.paging = true
    setResult((value) => ({ ...value, status: 'paging' }))
    try {
      const items = await apiRequest(`/memories?${params}&before=${current.items.at(-1).id}`, { signal: job.controller.signal })
      if (job.controller.signal.aborted) return
      job.pages += 1
      setResult((value) => ({ ...value, items: [...value.items, ...items.map(readableMemory)], status: 'ready', error: '', more: items.length === 100 }))
    } catch (error) {
      if (!job.controller.signal.aborted) setResult((value) => ({ ...value, status: 'error', error: journalError(error) }))
    } finally { job.paging = false }
  }
  return { ...current, loadMore }
}

export function useMemory(id) {
  const { workspace, revision, tick, sessionExpired } = useJournal()
  const key = `${id}|${revision}`
  const [result, setResult] = useState({ key: '', memory: null, error: '', status: 'loading' })
  useEffect(() => {
    if (!workspace || !id) return
    const controller = new AbortController()
    apiRequest(`/memories/${encodeURIComponent(id)}`, { signal: controller.signal }).then((memory) => {
      if (!controller.signal.aborted) setResult({ key, memory: readableMemory(memory), error: '', status: 'ready' })
    }).catch((error) => {
      if (controller.signal.aborted) return
      if (error.status === 401) sessionExpired()
      setResult({ key, memory: null, error: journalError(error), status: 'error' })
    })
    return () => controller.abort()
  }, [id, workspace, key, tick, sessionExpired])
  return result.key === key ? result : { memory: null, error: '', status: 'loading' }
}
