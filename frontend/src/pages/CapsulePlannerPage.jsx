import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { blankDraft, journalError, useJournal, useMemory, useMemoryCollection } from '../lib/journal.js'
import Icon from '../components/Icon.jsx'
import { formatSchedule, validateSchedule } from '../lib/capsuleSchedule.js'

export default function CapsulePlannerPage() {
  const [params] = useSearchParams()
  const [plan, setPlan] = useState({
    memoryId: params.get('memory') || '',
    localDateTime: ''
  })
  const [query, setQuery] = useState('')
  const [review, setReview] = useState(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const { draft, hasUnsaved, mutate, setDraft, refresh } = useJournal()
  const collection = useMemoryCollection({ state: 'draft', q: query })
  const selected = useMemory(plan.memoryId)

  const source = 
    selected.memory?.status === 'draft' ? selected.memory : null

  const choices = 
    source && !collection.items.some((entry) => entry.id === source.id)
      ? [source, ...collection.items]
      : collection.items

  const request = useRef(null)
  const reviewRef= useRef(null)
  const formRef = useRef(null)
  const navigate = useNavigate()
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'Device local time'

  useEffect(() => () => request.current?.abort(), [])
  useEffect(() => { if (review) reviewRef.current?.focus() }, [review])

  function change(name, value) {
    setPlan((current) => ({ ...current, [name]: value }))
    setReview(null)
    setError('')
  }
  
  function validate() {
    const result = validateSchedule(
      plan.memoryId,
      plan.localDateTime,
      source ? [source] : []
    )
    if (result.error) {
      setError(result.error)
      formRef.current?.elements.namedItem(result.field)?.focus()
      return  null
    }
    if (hasUnsaved && draft.id === plan.memoryId) {
      setError('You have unsaved changes to this memory. Save them before sealing.')
      return null
    }
    return result.schedule
  }

  function reviewPlan(event) {
    event.preventDefault()
    const result = validate()

    if (result) {
      setError('')
      setReview(result)
    }
  }

  async function seal() {
    if (request.current) return

    const result = validate()
    if (!result) return

    const controller = new  AbortController()
    request.current = controller
    setBusy(true)
    setError('')

    try {
      await mutate(`/memories/${plan.memoryId}/seal`, {
        method: 'POST',
        body: { unlock_at: result.scheduledAt },
        signal: controller.signal
      })

      if (draft.id === plan.memoryId) {
        setDraft(blankDraft())
      }

      if (!controller.signal.aborted) {
        navigate(`/memories/${plan.memoryId}`, { replace: true })
      }
    } catch (failure) {
      if (!controller.signal.aborted) {
        setError(journalError(failure))
      }
    } finally {
      request.current = null 
      if (!controller.signal.aborted) setBusy(false)
    }
  }

  return (
    <section className="planner-page" aria-labelledby="planner-heading">
      <Link className="text-link" to="/capsules">Back to time capsules</Link>
      <div className="page-heading planner-heading">
        <div><h1 id="planner-heading">Give it<br /><em>a little time.</em></h1><p>A memory from today. A moment for your future self.</p></div>
        <Icon name="clock" className="editor-emblem" />
      </div>
      <p className="planning-note">
        Sealing hides your memory's title and words until its return  time.
        You can delete a sealed memory, but you cannot read it early or change its date.
      </p>

      <ol className="planning-steps" aria-label="Sealing progress">
        <li aria-current={!review ? 'step' : undefined}>Choose a memory and date</li>
        <li aria-current={review ? 'step' : undefined}>Review and seal</li>
      </ol>

      {(error || collection.error) && (
        <p className='form-error' role='alert'>
          {error || collection.error}
        </p>
      )}
      {review ? (
        <section className="schedule-review" aria-labelledby="review-heading">
          <h2 id="review-heading" ref={reviewRef} tabIndex={-1}>
            A moment to make sure.
          </h2>
          <dl className="review-list">
            <div>
              <dt>Memory</dt>
              <dd>{source?.title || 'Your saved memory'}</dd>
            </div>
            <div>
              <dt>Return time</dt>
              <dd>
                <time dateTime={review.scheduledAt}>
                  {formatSchedule(review)}
                </time>
              </dd>
            </div>
            <div>
              <dt>Time zone</dt>
              <dd>{review.timeZone}</dd>
            </div>
          </dl>
          <p className="page-note">
            Once sealed, this memory stays locked until this time.
            Then it becomes ready for you to open.
          </p>

          <div className="editor-actions">
            <button 
              className="write-action" 
              disabled={busy}
              onClick={seal}
            >
              {busy ? 'Sealing...' : 'Seal memory'}
            </button>

            <button
              className='secondary-button'
              disabled={busy}
              onClick={() => setReview(null)}
            >
              Change details
            </button>
          </div>
        </section>
      ) : (
        <form 
          className="schedule-form" 
          ref = {formRef}
          onSubmit={reviewPlan} 
          noValidate
        >
          <div className='field'>
            <label htmlFor='capsule-search'>Find a saved memory</label>
            <input
              id="capsule-search"
              type="search"
              value={query}
              maxLength={120}
              placeholder="Search your saved writing..."
              onChange={(event) => setQuery(event.target.value)}
            />
          </div>

          <div className='field'>
            <label htmlFor='capsule-memory'>Memory to seal</label>
            <select
              id='capsule-memory'
              name='memoryId'
              value={plan.memoryId}
              onChange={(event) => change('memoryId', event.target.value)}
              required
            >
              <option value = "">Choose a saved memory</option>
              {choices.map((memory) => (
                <option value={memory.id} key={memory.id}>
                  {memory.title}
                </option>
              ))}
            </select>
          </div>
          {collection.status === 'loading' && (
            <p className='page-note' role="status">
              Finding your saved memories...
            </p>
          )}

          { collection.more && (
            <button
              type="button"
              className="secondary-button"
              onClick={collection.loadMore}
              disabled={collection.status === 'paging'}
            >
              Load more memories
            </button>
          )}

          {!choices.length && collection.status !== 'loading' && !collection.error && (
            <p className='page-note'>
              {query
                ? 'No saved memories match. Try another word.'
                : 'Start by saving a memory.'}{' '}
              <Link className='text-link' to="/memories/new">
                Write a memory
              </Link>
            </p>
          )}

          {collection.error && (
            <button
              type='button'
              className='secondary-button'
              onClick={refresh}
            >
              Try again
            </button>
          )}

          <div className='field'>
            <label htmlFor='capsule-date'>
              When would you like to return?
            </label>
            <input 
              id="capsule-date"
              name="localDateTime"
              type="datetime-local"
              step="60"
              required
              value={plan.localDateTime}
              onChange={(event) => change('localDateTime', event.target.value)}
              aria-describedby='zone-note'
            />
            <p className='schedule-zone' id='zone-note'>
              Time zone: {zone}. Choose at least two minutes from now.
            </p>
          </div>

          <div className='editor-actions'>
            <button
              className='write-action'
              type='submit'
              disabled={selected.status === 'loading' && Boolean(plan.memoryId)}
            >
              Review before sealing
              <Icon name="arrow"/>
            </button>

            <Link className='text-link' to='/memories'>
              Return to memories
            </Link>
          </div>
        </form>
      )}
    </section>
  )
}
