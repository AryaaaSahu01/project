import { useState } from "react"
import { Link, Navigate, useNavigate } from 'react-router'
import {useAuth} from '../lib/auth.js'
import { useJournal } from "../lib/journal.js"
import AppearanceSettings from '../components/AppearanceSettings.jsx'

export default function AccountPage() {
    const { status, account, error: sessionError, refreshSession, signOut } = useAuth()
    const [confirming, setConfirming] = useState(false)
    const [submitting, setSubmitting] = useState(false)
    const [error, setError] = useState('')
    const navigate = useNavigate()
    const { hasUnsaved, pending } = useJournal()

    async function finishSignOut() {
        if (submitting || pending) return
        setSubmitting(true)
        setError('')
        try {
            await signOut()
            navigate('/sign-in', {replace: true})
        } catch {
            setError('Could not sign out. Your sign-in is still active. Please try again.')
        } finally {
            setSubmitting(false)
        }
    }

    if (status === 'checking') return <p className="account-loading" role="status">Checking your sign-in...</p>
    if (status === 'unavailable') {
        return (
            <section className="account-form-column" aria-labelledby="account-title">
                <h1 id="account-title">Let's reconnect.</h1>
                <p className="form-error" role="alert">{sessionError}</p>
                <button className="write-action" onClick={() => refreshSession()}> Try again </button>
                <Link className="account-back" to="/sign-in">Go to sign in</Link>
            </section>
        )
    }

    if (!account) return <Navigate to="/sign-in" replace/>

    return (
        <section className="account-page" aria-labelledby="account-title">
            <div className="account-form-column">
                <h1 id="account-title">Your account.</h1>
                <p className="account-intro">Welcome back, {account.profile.display_name}.</p>
                <dl className="account-details">
                    <div><dt>Your name</dt><dd>{account.profile.display_name}</dd></div>
                    <div><dt>Email address</dt><dd>{account.user.email}</dd></div>
                    <div><dt>Joined</dt><dd><time dateTime={account.user.created_at}>
                        {new Date(account.user.created_at).toLocaleDateString(undefined, {year: 'numeric', month: 'long', day: 'numeric'})}
                    </time></dd></div>
                </dl>

                <Link className="write-action" to="/today">Back to your journal</Link>
                {error && <p className="form-error account-signout-error" role="alert">{error}</p>}
                { confirming ? (
                    <div className="clear-confirmation account-signout-confirmation">
                        <p>You have unsaved changes. Save them before signing out, or discard them. Your saved memories and capsules will remain in your account.</p>
                        <div className="confirmation-actions">
                            <Link className="secondary-button" to="/memories/new">
                                Return to writing
                            </Link>
                            <button
                                className="text-button"
                                disabled={submitting || Boolean(pending)}
                                onClick={finishSignOut}
                            >
                                {submitting ? 'Signing out...' : 'Discard changes and sign out'}
                            </button>
                            <button
                                className="text-button"
                                disabled={submitting}
                                onClick={() => setConfirming(false)}
                            >
                                Stay Signed in
                            </button>
                        </div>
                    </div>
                ) : (
                    <button 
                    className="text-button account-signout" 
                    disabled={submitting || Boolean(pending)}
                    onClick={() => hasUnsaved ? setConfirming(true) : finishSignOut()}
                    >
                    {submitting ? 'Signing out...' : 'Sign out'}
                    </button>
                )}
                {pending > 0 && <p className="page-note" role="status">Finish saving before signing out.</p>}
            </div>
            <aside className="account-aside" aria-labelledby="account-note-title">
                <h2 id="account-note-title">Make yourself at home.</h2>
                <p>Your saved memories and capsules stay in your account when you sign out.</p>
                <AppearanceSettings/>
            </aside>
        </section>
    )
}