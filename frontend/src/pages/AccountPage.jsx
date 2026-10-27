import { useState } from "react"
import { Link, Navigate, useNavigate } from 'react-router'
import {useAuth} from '../lib/auth.js'

export default function AccountPage({hasWriting, onSignedOut}) {
    const { status, account, error: sessionError, refreshSession, signOut } = useAuth()
    const [confirming, setConfirming] = useState(false)
    const [submitting, setSubmitting] = useState(false)
    const [error, setError] = useState('')
    const navigate = useNavigate()

    async function finishSignOut() {
        if (submitting) return
        setSubmitting(true)
        setError('')
        try {
            await signOut()
            onSignedOut()
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
                <p className="account-intro" role="status">You're signed in, {account.profile.display_name}.</p>
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
                        <p>Signing out clears the writing and capsule plans held in this tab. Download anything you want to keep first.</p>
                        <div className="confirmation-actions">
                            <button className="secondary-button" disabled={submitting} onClick={finishSignOut}>
                                {submitting ? 'Signing out...' : 'Clear tab writing and sign out'}
                            </button>
                            <button className="text-button" disabled={submitting} onClick={() => setConfirming(false)}>Keep writing</button>
                        </div>
                        <Link className="account-back" to="/memories">Review and download memories</Link>
                        <Link className="account-back" to="/memories/new">Review and download unfinished writing</Link>
                        <Link className="account-back" to="/capsules">Review and download capsule plans</Link>
                    </div>
                ) : (
                  <button className="text-button account-signout" disabled={submitting}
                    onClick={() => hasWriting ? setConfirming(true) : finishSignOut()}>
                    {submitting ? 'Signing out...' : 'Sign out'}
                  </button>
                )}
            </div>
            <aside className="account-aside" aria-labelledby="account-note-title">
                <h2 id="account-note-title">A little clarity.</h2>
                <p>Your account is ready to use.</p>
                <p className="account-storage-note">Journal writing and capsule plans are still held in this tab. Signing in does not upload them yet; download a copy before refreshing or closing the page.</p>
            </aside>
        </section>
    )
}