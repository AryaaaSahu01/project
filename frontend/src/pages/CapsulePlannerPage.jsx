import { useEffect, useRef, useState } from "react"
import { Link, useNavigate } from 'react-router'
import Icon from '.../components/Icon.jsx'
import {
    formatSchedule,
    validateSchedule,
} from '../lib/capsuleSchedule.js'

export default function CapsulePlannerPage({
    memories,
    draft,
    onChange,
    onKeep,
}) {
    const navigate = useNavigate()
    const [review, setReview] = useState(null)
    const [error, setError] = userState(null)

    const memoryRef = useRef(null)
    const timeRef = useRef(null)
    const reviewRef = useRef(null)

    const source = memories.find((memory) => memory.id === draft.memoryId)

    const zone = 
        Intl.DateTimeFormat().resolvedOptions().timeZone ||
        'Device local time'
    useEffect(() => {
        if (review) reviewRef.current?.focus()
    }, [review])

    function reportError(result) {
        setError(result)
        setReview(null)

        window.resquestAnimationFrame(() => {
            const field = result.field === 'memoryId' ? memoryRef : timeRef
            field.current?.focus() 
        })
    }

    function changeField(field, value) {
        onChange({ ...draft, [field]:value })
        setError(null)
        setReview(null)
    }

    function reviewPlan(event) {
        event.preventDefault()

        const result = validateSchedule(
            draft.memoryId,
            draft.localDateTime,
            memories,
        )

        if (result.error) {
            reportError(result)
            return
        }

        setError(null)
        setReview(result.schedule)
    }

    function keepPlan() {
        const result = validateSchedule(
            draft.memoryId,
            draft.localDateTime,
            memories,
        )

        if (result.error) {
            reportError(result)
            return
        }

        if (onKeep(result.schedule)) {
            naviagate('/capsule')
        } else {
            reportError ({
                field: 'memoryId',
                error: 'That memory is no longer available. Choose another memory.',
            })
        }
    }

    if (!memories.length) {
        return (
            <section className="planner-page">
                <Link className="text-link" to="/capsules">
                Back to time capsules
                </Link>

                <div className="page-heading">
                    <div>
                        <h1> Begin with a memory.</h1>
                        <p>A capsule start with something worth returning to.</p>
                    </div>
                </div>

                <div className="empty-state">
                    <h2>Your words come first.</h2>
                    <p>
                        Write a memory, choose "Keep in this tab", then return here
                        to plan its date.
                    </p>

                    <Link className="write-action" to="/memories/new">
                        <Icon name="pen" />
                        Write a memory
                    </Link>
                </div>
            </section>
        )
    }

    return (
        <section className="palanner-page" aria-labelledby="planner-heading">
            <Link className="text-link" to="/capsules">
                Back to time capsules
            </Link>

            <div className="page-heading planner-heading">
                <div>
                    <h1 id="planner-heading">
                        Give it
                        <br/>
                        <em>a little time.</em>
                    </h1>

                    <p>A memory from today. A moment for your future self.</p>
                </div>

                <Icon name="clock" className="editor-emblem" />
            </div>

            <p className="planning-note">
                <strong>This is an unsealed plan.</strong> Your memory stays
                readable. Planning does not lock your writing or send a reminder.
            </p>

            <ol className="planning-steps" aria-label="Planning progress">
                <li aria-current={!review ? 'step' : undefined}>
                    Choose a memory and date
                </li>
                <li aria-current={review ? 'step' : undefined}>
                    Review your plan 
                </li>
            </ol>

            {error && (
                <p className="form-error" id="schedule-error" role="alert">
                    {error.error}
                </p>
            )}

            {review ? (
                <section
                    className="schedule-review"
                    aria-labelledby="review-heading"
                >
                    <h2 id="review-heading" ref={reviewRef} tabIndex={-1}>
                        A moment to make sure.
                    </h2>

                    <dl className="review-list">
                        <div>
                            <dt>Memory</dt>
                            <dd>{source?.title}</dd>
                        </div>
                        <div>
                            <dt>Return date</dt>
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
                        <div>
                            <dt>Plan status</dt>
                            <dd>Unsealed - In this tab</dd>
                        </div>
                    </dl>

                    <p className="page-note">
                        Your writing stays in Memories. Keeping this plan will not
                        seal it. Download your writing and plan before refreshing
                        or closing this tab.
                    </p>

                    <div className="editor-actions">
                        <button className="write-action" onClick={keepPlan}>
                            Keep plan in this tab
                        </button>

                        <button 
                            className="secondary-button"
                            onClick={() => setReview(null)}
                        >
                            Change details
                        </button>
                    </div>
                </section>
            ) : (
                <form 
                    className="schedule-form"
                    onSubmit={reviewPlan}
                    noValidate
                >
                    <div className="field">
                        <label htmlFor="capsule-memory">Memory to return to</label>

                        <select 
                            id="capsule-memory"
                            ref={memoryRef}
                            value={draft.memoryId}
                            onChange={(event) =>
                                changeField('memoryId', event.target.value)
                            }
                            required
                            aria-invalid={error?.field === 'memoryId'}
                            aria-describedby={
                                error?.field === 'memoryId'
                                  ? 'schedule-error'
                                  : undefined
                            }
                        >   
                            <option value="">Choose a memory</option>

                            {memories.map((memory) => 
                                <option value={memory.id} key={memory.id}>
                                    {memory.title}
                                </option>
                            )}
                        </select>
                    </div>
                    <div className="field">
                        <label htmlFor="capsule-date">
                            When would you like to return?
                        </label>

                        <input 
                            id="capsule-date"
                            ref={timeRef}
                            type="datetime-local"
                            step="60"
                            value={draft.localDateTime}
                            onChange={(event) =>
                                changeField('localDateTime', event.target.value)
                            }
                            required
                            aria-invalid={error?.field === 'localDateTime'}
                            aria-describedby={`zone-note${
                                error?.field === 'localDateTime'
                                    ? ' schedule-error'
                                    : ''
                            }`}
                        />


                        <p className="schedule-zone" id="zone-note">
                            Time Zone: {zone}. Choose a future date and time.
                        </p>
                    </div>

                    <div className="editor-actions">
                        <button className="write-action" type="submit">
                            Review plan
                            <Icon name="arrow" />
                        </button>

                        <Link className="text-link" to="/capsules">
                            Return to capsules
                        </Link>
                    </div>
                </form>

            )}  
        </section>
    )
}