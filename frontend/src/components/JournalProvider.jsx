import { useCallback, useEffect, useRef, useState } from "react";
import { apiRequest } from '../lib/api.js'
import { useAuth } from '../lib/auth.js'
import { JournalContext,blankDraft } from '../lib/journal.js'

export default function JournalProvider({ children }) {
    const { status: authStatus, refreshSesssion } =useAuth()
    const [workspaceState, setWorkspaceState] = useState({ status: 'loading', workspace: null, error: ''})
    const [draft, setDraft] = useState(blankDraft)
    const [revision, setRevision] = useState(0)
    const [tick, setTick] = useState(0)
    const [workspaceRetry, setWorkspaceRetry] = useState(0)
    const [pending, setPending] = useState(0)
    const [summary, setSummary] = useState({ draft: 0, sealed:0, ready: 0, returned:0})
    const channel = useRef(null)
    const hasUnsaved = draft.existing
        ? draft.title !== draft.baseTitle || draft.body !== draft.baseBody
        : Boolean(draft.title.trim() || draft.body.trim())
    const refresh = useCallback(() => setWorkspaceRetry((value) => value + 1), [])
    const invalidate = useCallback(() => {
        setRevision((value) => value + 1)
        channel.current?.postMessage('changed')
    },[])

    const sessionExpired = useCallback(() => {void refreshSesssion() }, [refreshSesssion] )
    useEffect (() => {
        if (authStatus !== 'signed-in') return
        const controller = new AbortController()
        apiRequest('/me/workspaces', { signal: controller.signal }).then((workspaces) => {
            if (controller.signal.aborted) return
            const workspace = workspaces.find((entry) => entry.kind === 'personal' && entry.role === 'owner')
            if (!workspace) throw new Error('Your personal journal could not be found. Please try signing in again.')
            setWorkspaceState({status: 'ready', workspace, error:''})
        }).catch((error) => {
            if (!controller.signal.aborted) setWorkspaceState({ status: 'error', workspace: null, error: error.message })
        })
        return () => controller.abort()
    },[authStatus, workspaceRetry]
    useEffect(() => {
        const check = () => { if (document.visibilityState === 'visible')refresh() }
        const timer = setInterval(check , 15000)
        window.addEventListener('focus', check)
        document.addEventListener('visibilitychange', check)
        if ('BroadcastChannel' in Window) {
            channel.current = new BroadcastChannel('chrono-journal')
            channel.current.onmessage = () => { setRevision((value) => value + 1); refresh()}
        }
        return () => { clearInterval(timer); window.removeEventListener('focus', check); document.removeEventListener('visibilitychange', check); channel.current?.close(); channel.current= null}
    }, [refresh])
    useEffect(() => {
        if (!workspaceState.workspace) return
        const controller = new AbortController()
        apiRequest(`/memories/summary?workspace_id=${workspaceState.workspace.id}`, {signal: controller.signal})
            .then((value) => {if (!controller.signal.aborted) setSummary(value) }).catch((error) => {if (error.status === 401) sessionExpired() })
        return () => controller.abort()
    }, [workspaceState.workspace, tick, revision, sessionExpired])
    useEffect(() => {
        if (!hasUnsaved) return
        const warn = (event) => { event.preventDefault(); event.returnValue=''}
        window.addEventListener('beforeunload', warn)
        return () => window.removeEventListener('beforeunload', warn)
    }, [hasUnsaved])
    async function mutate(path, options) {
        setPending((value) => value+1)
        try {
            const value = await apiRequest(path, options)
            if (options.method === 'DELETE' && path === `/memories/${draft.id}`) setDraft(blankDraft())
            invalidate()
            return value
        }
        catch(error) {if (error.status === 401) sessionExpired(); invalidate(); throw error }
        finally { setPending((value) => value - 1)}
    }
    async function saveDraft(signal) {
        const input = { title: draft.title.trim(), body: draft.body }
        let memory 
        if (draft.existing) {
            memory = await mutate(`/memories/${draft.id}`, {method: 'PATCH', body: { ...input, expected_updated_at: draft.updatedAt}, signal})
        } else {
            memory = await mutate('/memories', { method: 'POST', body: {...input, id: draft.id, workspace_id: workspaceState.workspace.id}, signal})
        }
        setDraft(blankDraft())
        return memory
    }
    return <JournalContext.Provider value={{ ...workspaceState, draft, setDraft, hasUnsaved, revision, tick, pending, summary, refresh, retryWorkspace, invalidate, sessionExpired, saveDraft ,mutate }}>{children}</JournalContext.Provider>
}        
