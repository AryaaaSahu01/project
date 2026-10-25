import { useState } from "react"
import { useNavigate } from "react-router"
import Icon from '../components/Icon.jsx'
import { downloadText } from "../lib/download.js"

export default function WritePage({draft, onChange, onKeep}) {
  const navigate = useNavigate()
  const [preview, setPreview] = useState(false)
  const [confirmClear, setConfirmClear] = useState(false)
  const [notice, setNotice] = useState('')

  const hasText = Boolean(draft.title || draft.body)
  const words = draft.body.trim()
    ? draft.body.trim().split(/\s+/u).length
    : 0

  function changeField(field, value) {
    onChange({...draft, [field]:value})
    setNotice('')
    setConfirmClear(false)
  }

  function exportDraft() {
    const contents = 
      `${draft.title.trim() || 'Untitled memory'}\n\n${draft.body}\n`
    
    setNotice(
      downloadText('memory.txt', contents)
      ? 'Download requested. Check your downloads for memory.txt.'
      : 'The download could not start. Copy your writing before closing this tab.',
    )
  }

  function keepWriting() {
    const id = onKeep()

    if (id) {
      navigate(`/memories/${id}`)
    } else {
      setNotice('Write a few words before keeping this memory.')
    }
  }

  function clearDraft() {
    onChange({ title: '', body: '' })
    setConfirmClear(false)
    setPreview(false)
    setNotice('Writing cleared form this tab.')
  }

  return (
    <section aria-labelledby="editor-heading" className="editor-page">
      <div className="page-heading editor-heading">
        <div>
          <h1 id="editor-heading">
            Make room 
            <br/>
            for a <em>memory.</em>
          </h1>
          <p>
            No perfect beginning needed. Start with what stayed with you.
          </p>
        </div>

        <Icon name="pen" className="editor-emblem"/>
      </div>

      <p className="editor-notice" id="draft-help">
        <Icon name="pen"/>
        <span>
          <strong>Writing in this tab.</strong> Keep it in your journal
          here, or download a copy. Account saving is coming next.
        </span>
      </p>

      <div className="editor-toolbar">
        <div
          className="editor-mode"
          role="group"
          aria-label="Editor mode"
        >
          <button
            className="mode-button"
            aria-pressed={!preview}
            onClick={() => setPreview(false)}
          >
            Write
          </button>

          <button
            className="mode-button"
            aria-pressed={preview}
            onClick={() => setPreview(true)}
          >
            Preview
          </button>
        </div>
        <span className="word-count">
          {words} {words === 1 ? 'word' : 'words'}
        </span>
      </div>

      {preview ? (
        <article 
          className="editor-paper preview-paper"
          aria-label="Memory preview"
        >
          <h2>{draft.title || 'Untitled memory'}</h2>

          <p className={draft.body ? 'preview-body' : 'preview-empty'}>
            {draft.body ||
              'Your words will appear here. Switch to Write to begin.'}
          </p>
        </article>
      ) : (
        <div className="editor-paper">
          <label htmlFor="draft-title">Memory title</label>

          <input 
            id="draft-title"
            className="draft-title"
            value={draft.title}
            onChange={(event) =>
              changeField('title', event.target.value)
            }
            placeholder="Give this moment a name"
            maxLength={200}
            autoComplete="off"
            aria-describedby="title-limit draft-help"
          />

          <span id="title-limit" className="field-hint">
            {draft.title.length}/200 characters
          </span>

          <label htmlFor="draft-body">Your words</label>

          <textarea
            id="draft-body"
            className="draft-body"
            value={draft.body}
            onChange={(event)=>
              changeField('body', event.target.value)
            }
            placeholder="Today, I want to remember..."
            maxLength={50000}
            aria-describedby="body-limit draft-help"
          />

          <span id="body-limit" className="field-hint">
            {draft.body.length.toLocaleString()}/50,000 characters
          </span>
        </div>
      )}

      <div className="editor-actions">
        <button
          className="write-action"
          disabled={!draft.body.trim()}
          onClick={keepWriting}
        >
          Keep in this tab
        </button>

        <button
          className="secondary-button"
          disabled={!hasText}
          onClick={exportDraft}
        >
          Download text
        </button>

        <button
          className="text-button"
          disabled={!hasText}
          onClick={() => setConfirmClear(true)}
        >
          Clear writing
        </button>
      </div>

      {confirmClear && (
        <div 
          className="clear-confirmation"
          role="group"
          aria-label="Confirm clearing writing"
        >
          <p>
            Clear this writing? You cannot undo this.Download a Copy
            first if you want to keep it.
          </p>
          
          <div className="confirmation-actions">
            <button
              className="secondary-button"
              onClick={() => setConfirmClear(false)}
            >
              Keep writing
            </button>

            <button className="danger-button" onClick={clearDraft}>
              Clear permanently            </button>

          </div>
        </div>
      )}

      <p className="editor-status" role="status">
        {notice}
      </p>

      <p className="page-note">
        Keeping a memory here lasts until refresh or tab closure.
        Downloaded text files are unencrypted.
      </p>
    </section>
  )
}