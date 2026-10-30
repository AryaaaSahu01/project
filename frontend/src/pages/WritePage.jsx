import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import Icon from '../components/Icon.jsx'
import { blankDraft, journalError, useJournal } from '../lib/journal.js'

export default function WritePage() {
  const { draft, setDraft, hasUnsaved, saveDraft } = useJournal()
  const [preview, setPreview] = useState(false)
  const [confirmDiscard, setConfirmDiscard] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const request = useRef(null)
  const navigate = useNavigate()
  useEffect(() => () => request.current?.abort(), [])
  const words = draft.body.trim() ? draft.body.trim().split(/\s+/u).length : 0
  function change(name, value) { setDraft((current) => ({ ...current, [name]: value })); setError(''); setConfirmDiscard(false) }
  async function save(andSeal) {
    if (request.current || !draft.body.trim()) return
    const controller = new AbortController()
    request.current = controller
    setSubmitting(true); setError('')
    try {
      const memory = await saveDraft(controller.signal)
      if (!controller.signal.aborted) navigate(andSeal ? `/capsules/new?memory=${memory.id}` : `/memories/${memory.id}`)
    } catch (failure) { if (!controller.signal.aborted) setError(journalError(failure)) }
    finally { request.current = null; if (!controller.signal.aborted) setSubmitting(false) }
  }
  function discard() {
    const id = draft.existing ? draft.id : null
    setDraft(blankDraft()); setConfirmDiscard(false); setPreview(false)
    if (id) navigate(`/memories/${id}`)
  }
  return <section className="editor-page" aria-labelledby="editor-heading">
    <div className="page-heading editor-heading"><div><h1 id="editor-heading">{draft.existing ? 'A little more to say.' : <>Make room<br />for a <em>memory.</em></>}</h1><p>No perfect beginning needed. Start with what stayed with you.</p></div><Icon name="pen" className="editor-emblem" /></div>
    <p className="editor-notice"><Icon name="lock" /><span>Save your words to your journal, or save and choose a time to seal them.</span></p>
    <div className="editor-toolbar"><div className="editor-mode" role="group" aria-label="Editor mode"><button className="mode-button" aria-pressed={!preview} onClick={() => setPreview(false)}>Write</button><button className="mode-button" aria-pressed={preview} onClick={() => setPreview(true)}>Preview</button></div><span className="word-count">{words} {words === 1 ? 'word' : 'words'}</span></div>
    {preview ? <article className="editor-paper preview-paper" aria-label="Memory preview"><h2>{draft.title || 'Untitled memory'}</h2><p className={draft.body ? 'preview-body' : 'preview-empty'}>{draft.body || 'Your words will appear here. Switch to Write to begin.'}</p></article> : <div className="editor-paper">
      <label htmlFor="draft-title">Memory title</label><input id="draft-title" className="draft-title" value={draft.title} onChange={(event) => change('title', event.target.value)} placeholder="Give this moment a name" maxLength={200} autoComplete="off" aria-describedby="title-limit" disabled={submitting} /><span id="title-limit" className="field-hint">{draft.title.length}/200 characters</span>
      <label htmlFor="draft-body">Your words</label><textarea id="draft-body" className="draft-body" value={draft.body} onChange={(event) => change('body', event.target.value)} placeholder="Today, I want to remember…" maxLength={50000} aria-describedby="body-limit" disabled={submitting} /><span id="body-limit" className="field-hint">{draft.body.length.toLocaleString()}/50,000 characters</span>
    </div>}
    {error && <p className="form-error editor-error" role="alert">{error} Your unsaved words are still here.</p>}
    <div className="editor-actions"><button className="write-action" disabled={submitting || !draft.body.trim()} onClick={() => save(false)}>{submitting ? 'Saving…' : draft.existing ? 'Save changes' : 'Save memory'}</button><button className="secondary-button" disabled={submitting || !draft.body.trim()} onClick={() => save(true)}>Save &amp; seal…</button><button className="text-button" disabled={submitting || !hasUnsaved && !draft.existing} onClick={() => setConfirmDiscard(true)}>Discard changes</button></div>
    <p className="editor-status" role="status">{submitting ? 'Saving your memory…' : hasUnsaved ? 'Unsaved changes. Save before leaving or closing this page.' : 'Ready when you are.'}</p>
    {confirmDiscard && <div className="clear-confirmation"><p>Discard these changes? {draft.existing ? 'Your previously saved memory will remain.' : 'This writing has not been saved.'}</p><div className="confirmation-actions"><button className="danger-button" onClick={discard}>Discard changes</button><button className="secondary-button" onClick={() => setConfirmDiscard(false)}>Keep writing</button></div></div>}
  </section>
}
