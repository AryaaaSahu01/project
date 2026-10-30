import { useEffect, useRef, useState} from 'react'
import { Link, useParams, useNavigate } from 'react-router'
import { journalError, useJournal, useMemory } from '../lib/journal.js'
import { formatReturnTime, fromatReturnTime } from '../lib/capsuleSchedule.js'
import Icon from '../components/Icon.jsx'


export default function MemoryPage() {
  const {id} = useParams()

  const {
    memory,
    status,
    error: loadError
  } = useMemory(id)

  const {
    mutate,
    refresh,
    draft,
    setDraft,
    hasUnsaved
  } = useJournal()

  const [confirming, setConfirming] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const request = useRef(null)
  const navigate = useNavigate()

  useEffect(() => () => request.current?.abort(), [])

  async function act(action) {
    if (request.current) return
    
    const controller = new AbortController()
    request.current = controller

    setBusy(true)
    setError('')

    try {
      await mutate(
        `/memories/${id}${action === 'open' ? '/open' : ''}`,
        {
          method: action === 'open' ?  'POST' : 'DELETE',
          signal: controller.signal
        }
      )

      if (!controller.signal.aborted && action === 'delete') {
        navigate('/memories', { replace: true })
      }

    } catch (failure) {
      if (!controller.signal.aborted) {
        setError(journalError(failure))
      }
    } finally {
      request.current = null

      if (!controller.signal.aborted) {
        setBusy(false)
        setConfirming('')
      }
    }
  }


  function edit(replace = false) {
    if (hasUnsaved && draft.id !== id && !replace) {
      setConfirming('edit')
      return 
    }

    setDraft({
      id,
      existing: true,
      title: memory.rawTitle,
      body: memory.body || '',
      updatedAt: memory.updated_at,
      baseTitle: memory.rawTitle,
      baseBody: memory.body || ''
    })

    navigate('/memories/new')

  }

  if (status === 'loading') {
    return (
      <p className='account-loading' role='status'>
        Opening your memory
      </p>
    )
  }

  if (!memory) {
    return (
      <section className='empty-state'>
        <h1>This page is unavailable.</h1>

        <p role="alert">
          {loadError}
        </p>

        <button
          className='secondary-button'
          onClick={refresh}
        >
          Try again
        </button>

        <Link className='text-link' to="/memories">
          Back to memories
        </Link>
      </section>
    )
  }

  const sealed = memory.status === 'sealed'

  return (
    <article className='reading-page'>
      <Link 
        className="text-link" 
        to={sealed ? '/capsules' : '/memories' }
      >
        {sealed ? 'Back to time capsules' : 'Back to memories' }
      </Link>

      <header className="reading-header">
        <h1>{memory.title}</h1>

        <div className="reading-meta">
          <time dateTime={memory.created_at}>
            {new Date(memory.created_at).toLocaleDateString(undefined, {
              day: 'numeric',
              month: 'long',
              year: 'numeric',
            })}
          </time>

          <span>{memory.tag}</span>
        </div>
      </header>

      {sealed ? (
        <section
          className="locked-memory"
          aria-labelledby="locked-title"
        >
          <Icon name="lock"/>

          <h2 id="locked-title">
            {memory.can_open
              ? 'The time is yours.'
              : 'A little patience. A future perspective.'}
          </h2>
          
          <p>
            {memory.can_open
              ? 'Your memory is ready. Open it whenever you feel ready.'
              : 'Your writing is sealed and cannot be read or edited yet.'}
          </p>

          <p className='return-time'>
            Return time:{' '}
            <time dateTime={memory.unlock_at}>
              {formatReturnTime(memory.unlock_at)}
            </time>
          </p>
          
          {memory.can_open && (
            <button
              className='write-action'
              disabled={busy}
              onClick={() => act('open')}
            >
              {busy ? 'Opening...' : 'Open capsule'}
            </button>
          )}
        </section>
      ) : (
        <>
          <div className="reading-body">
            {(memory.body || '').split(/\n\s*\n/).map((paragraph, index) => (
              <p className='preserve-lines' key={index}>
                {paragraph}
              </p>
            ))}
          </div>

          <div className='reading-end'>
            <span className='brass-rule'/>

            <p>
              {memory.status === 'returned'
                ? 'A moment returned. A new perspective.'
                : 'A small moment, given a place.'}
            </p>
          </div>
        </>
      )}

      <div className="reading-end">
        <span className="brass-rule"/>
        <p>A small moment, give a place.</p>
      </div>

      {error && (
        <p className='form-error' role='alert'>
          {error}
        </p>
      )}

      <div className='editor-actions'>
        {memory.status === 'draft' && (
          <>
            <Link 
              className='write-action'
              to={`/capsules/new?memory=${id}`}
            >
              Seal for later
            </Link>

            <button
              className='secondary-button'
              disabled={busy}
              onClick={() => edit()}
            >
              Edit memory
            </button>
          </>
        )}

        <button 
          className='text-button'
          disabled={busy}
          onClick={() => setConfirming('delete')}
        >
          Delete memory
        </button>

        <Link className='text-link' to="/memories/new">
          Write another memory
        </Link>
      </div>

      {confirming === 'delete' && (
        <div className='clear-confirmation'>
          <p>
            Delete this memory permanently? This cannot be undone.
          </p>

          <div className='confirmation-actions'>
            <button
              className='danger-button'
              disabled={busy}
              onClick={() => act('delete')}
            >
              {busy ? 'Deleting...' : 'Delete permanently'}
            </button>

            <button
              className='secondary-button'
              disabled={busy}
              onClick={() => setConfirming('')}
            >
              Keep memory
            </button>
          </div>
        </div>
      )}

      {confirming === 'edit' && (
        <div className='clear-confirmation'>
          <p>
            You have other unsaved writing. Discard those changes
            and edit this memory?
          </p>

          <div className='confirmation-actions'>
            <button
              className='secondary-button'
              onClick={() => edit(true)}
            >
              Discard changes and edit
            </button>

            <button
              className='text-link'
              onClick={() => setConfirming('')}
            >
              Keep current writing
            </button>
          </div>
        </div>
      )}
    </article>
  )
}