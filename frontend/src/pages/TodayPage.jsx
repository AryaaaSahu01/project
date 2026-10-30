import { Link } from 'react-router'
import { APP_CONFIG } from '../config/app.js'
import { useAuth } from '../lib/auth.js'
import { useJournal, useMemoryCollection } from '../lib/journal.js'
import Icon from '../components/Icon.jsx'
import MemoryList from '../components/MemoryList.jsx'

export default function TodayPage() {
  const { status, refreshSession } = useAuth()
  const journal = useJournal()
  const collection = useMemoryCollection()
  const signedIn = status === 'signed-in'
  return <div className="today-page">
    <section className="welcome"><h1 className="welcome-heading">{APP_CONFIG.welcomeTitle}{' '}<em>{APP_CONFIG.welcomeEmphasis}</em></h1><p>{APP_CONFIG.welcomeDescription}</p></section>
    <section className="composer"><Link className="write-action" to={signedIn ? '/memories/new' : '/sign-up'}><Icon name="pen" />{signedIn ? (journal.hasUnsaved ? 'Continue writing' : 'Write a memory') : 'Start your journal'}</Link></section>
    {!signedIn ? <section className="guest-intro"><h2>Keep the moments that matter.</h2><p>Save your writing to your account. Seal a memory for a future date, then return to it when the time is right.</p>{status === 'unavailable' ? <button className="secondary-button" onClick={() => refreshSession()}>Reconnect to your account</button> : <Link className="text-link" to="/sign-in">Already have an account? Sign in</Link>}</section> : <>
      {journal.summary.ready > 0 && <section className="ready-banner" aria-label="Ready capsules"><Icon name="clock" /><div><h2>A moment has come back.</h2><p>{journal.summary.ready} {journal.summary.ready === 1 ? 'capsule is' : 'capsules are'} ready to open.</p></div><Link className="secondary-button" to="/capsules?state=ready">See ready capsules</Link></section>}
      <div className="journal-grid"><section><div className="section-heading"><h2>Recent memories</h2><Link className="text-link" to="/memories">View all</Link></div>
        {collection.error ? <div><p className="form-error" role="alert">{collection.error}</p><button className="secondary-button" onClick={journal.refresh}>Try again</button></div> : journal.status === 'error' ? <div><p className="form-error" role="alert">{journal.error}</p><button className="secondary-button" onClick={journal.retryWorkspace}>Try again</button></div> : collection.status === 'loading' ? <p role="status" className="page-note">Opening your saved memories…</p> : collection.items.length ? <MemoryList memories={collection.items.slice(0, 5)} /> : <div className="empty-state"><h2>Your first page is waiting.</h2><p>Write a memory and save it. It will be here whenever you return.</p><Link className="text-link" to="/memories/new">Write your first memory</Link></div>}
      </section><aside className="reflection"><Icon name="sun" /><h2>Something to sit with.</h2><p className="reflection-question">What’s one ordinary thing you hope you never forget?</p><span className="brass-rule" /><p className="reflection-note">There’s no right answer. Only yours.</p></aside></div>
      <section className="capsule"><div className="capsule-heading"><Icon name="clock" /><h2>A little time between<br />then and now.</h2></div><div className="capsule-detail"><p>Save a memory today. Choose a time to open it again.</p><Link className="text-link capsule-link" to="/capsules">Your time capsules<Icon name="arrow" /></Link></div></section>
    </>}
  </div>
}
