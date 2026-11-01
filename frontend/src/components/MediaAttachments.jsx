import { useEffect, useRef, useState } from "react"
import { apiRequest } from '../lib/api.js'
import { useJournal } from '../lib/journal.js'
import { checkFile, formatBytes, mediaAccept, mediaContentURL, uploadMedia } from '../lib/media.js'
import VoiceRecorder from './VoiceRecorder.jsx'
import '../styles/media.css'

function MediaPreview ({ meoryID, item}) {
    const url = mediaContentURL(memoryID, item.id)
    const [failed, setFailed] = useState(false)
    if (failed) return <p className="field-hint" role="status">Preview unavailable. Refresh your journal or try a browser that supports this format.</p>
    if (item.kind === 'image') return <img src ={url} alt={item.name} loading="lazy" onError={() => setFailed(true)} />
    if (item.kind === 'video') return <video src ={url} controls preload ="none" playsInline aria-label={item.name} onError={() => setFailed(true)} />
    return <audio src={url} controls preload="none" aria-label={item.name} onError={() => setFailed(true)} />
}

export default function MediaAttachments({ memoryID, editable }) {
    const { beginMedia, sessionExpired, pending } = useJournal()
    const [items, setItems] = useState([])
    const [config, setConfig] = useState(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [notice, setNotice] = useState('')
    const [progress, setProgress] = useState(null)
    const [confirming, setConfirming] = useState(null)
    const [deleting, setDeleting] = useState(false)
    const [retry, setRetry] = useState(0)
    const upload = useRef(null)
    const active = useRef(true)
    const panel = useRef(null)

    useEffect(() => {
        active.current = true
        const controller = new AbortController()
        Promise.all([apiRequest(`/memories/${memoryID}/media`, { signal: controller.signal }), apiRequest('/media/config', { signal: controller.signal})])
            .then(([list, options]) => { if (!controller.signal.aborted) { setItems(list); setConfig(options); setError('')}})
            .catch((failure) => { if (!controller.signal.aborted) { setError(failure.message); if (failure.status === 401) sessionExpired() }})
            .finally(() => {if (!controller.signal.aborted) setLoading(false)})
        if (window.location.hash === '#attachments') panel.current?.scrollIntoView({ block: 'start' })
        return () => { active.current = false; controller.abort(); upload.current?.abort() }

    }, [memoryID, retry, sessionExpired])


    async function addFiles(files, source = 'upload') {
        if(!config || upload.current) return false
        setError(''); setNotice('')
        if (files.length + items.length > config.max_memory_items) { setError(`A memory holds up to ${config.max_memory_items} attachments. Remove one before adding more. `); return false }
        for ( const file of files ) { const issue = checkFile(file, config); if (issue) { setError(issue); return false } }
        const controller = new AbortController()
        upload.current = controller
        const release = beginMedia()
        let saved = 0
        try {
            for (const file of files) {
                if (controller.signal.aborted) throw new DOMException('Upload cancelled', 'AbortError')
                setProgress({ name: file.name, percent: 0})
                const item = await uploadMedia(memoryID, file, { source, signal: controller.signal, onProgress: (percent) => { if (active.current) setProgress({ name: file.name, percent})}})
                if (active.current) setItems((current) => current.some((entry) => entry.id === item.id) ? current : [...current, item])
                saved += 1
            }
            if (active.current) setNotice(`${saved} ${saved === 1 ? 'attachment saved' : 'attachements saved'}. Your files will be seald with this memory.`)
            return true
        }   catch (failure) {
            if (active.current) {
                setError(failure.name === 'AbortError' ? `Upload cancelled. ${saved  ? `${saved} completed attachment(s) remain saved.` : 'Your saved memory is unchanged.'}` : failure.message)
                if (failure.status === 401) sessionExpired()
                try { const list = await apiRequest(`/memories/${memoryID}/media`); if (active.current) setItems(list) } catch { /* Keep the original error. */ }
            }
            return false 
        } finally { upload.current = null; release(); if (active.current) setProgress(null) }
    }
    async function remove() {
        if (!confirming || deleting ) return 
        const id = confirming.id
        setDeleting(true); setError('')
        const release = beginMedia()
        try {
            await apiRequest(`/memories/${memoryID}/media/${id}`, { method: 'DELETE'})
            if (active.current) { setItems((current) => current.filter((item) => item.id)); setConfirming(null); setNotice('Attachment removed.')}
        } catch (failure) { if (active.current) { setError(failure.message); if (failure.status === 401) sessionExpired() }}
        finally { release(); if (active.current) setDeleting(false)}
    }
    const disabled = Boolean(pending) || loading || !config || items.length >= (config?.max_memory_items || 10)
    return <section className="media-section" id="attachments" ref={panel} aria-labelledby="attachments-title">
        <div className="media-heading"><h2 id="attachments-title">Keep a little more.</h2><span>{items.length} / 10 attachements</span></div>
        <p className="media-intro">{editable ? 'Add a picture, a sound, or a moment in motion. Attachments are saved immediately and seald with this memory.' : 'The pictures and sounds you kept with this memory.'}</p>
        {loading && <p role="status">Opening your attachements...</p>}
        {error && <div className="media-error"><p className="form-error" role="alert">{error}</p>{!progress && <button type="button" className="text-link" onClick={() => setRetry((value) => value + 1)}>Refresh attachements</button>}</div>}
        {notice && <p className="media-notice" role="status">{notice}</p>}
        {editable && <>
            <div className="media-actions"><label className={`secondary-button media-picker${disabled ? ' is-disabled' : ''}`}>Add photos, audio, or video<input type="file" accept={mediaAccept} multiple disabled={disabled} onChange={(event) => { const files = Array.from(event.target.files || []); event.target.value = ''; if (files.length) void addFiles(files)}}/></label><VoiceRecorder disabled ={disabled} beginMedia={beginMedia} onUpload={(file, source) => addFiles([file], source)}/></div>
            <p className="field-hint">Pictures: 10 MB · Audio: 25 MB · Video: 100 MB. Voice recording stops at 5 minutes. JPG, PNG, WebP, MP3, WAV, OGG, M4A, MP4 and WebM.</p>
        </>}
        {progress && <div className="upload-status"><label htmlFor="media-progress">{progress.percent === 100 ? 'Checking and saving' : 'Uploading'} {progress.name}</label><progress id="media-progress" max="100" value={progress.percent} /><span role="status">{progress.percent}% transferred</span><button type="button" className="text-button" onClick={() => upload.current?.abort()}>Cancel upload</button></div>}
        {items.length > 0 && <ul className="attachment-list">{items.map((item) => <li className="attachment" key={item.id}>
            <div className="attachmet-heading"><div><h3>{item.name}</h3><p>{item.source === 'voice' ? 'Voice note' : item.kind === 'image' ? 'Picture' : item.kind === 'audio' ? 'Audio' : 'Video'} · {formatBytes(items.verified_bytes)}</p></div>{editable && <button type="button" className="text-button" disabled={Boolean(pending)} onClick={() => setConfirming(item)} aria-label={`Remove ${item.name}`}>Remove</button>}</div>
            <MediaPreview memoryID={memoryID} item={item} />
            {confirming?.id === item.id && <div className="clear-confirmation"><p>Remove this attachment permanently? Your written memory will remain.</p><div className="confirmation-actions"><button type="button" className="danger-button" disabled={deleting} onClick={remove}>{deleting ? 'Removing…' : 'Remove attachment'}</button><button type="button" className="secondary-button" disabled={deleting} onClick={() => setConfirming(null)}>Keep attachment</button></div></div>}
        </li>)}</ul>}
        {!loading && !items.length && !error && <p className="media-empty">{editable ? 'No attachments yet. Your words can stand on their own, too.' : 'There are no attachments with this memory.'}</p>}
    </section>
}