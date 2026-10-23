import { Link } from 'react-router'
import { APP_CONFIG } from '../config/app.js'
import Icon from '../components/Icon.jsx'
import MemoryList from '../components/MemoryList.jsx'
import { sampleMemories } from '../mocks/journal.js'

export default function TodayPage({ hasDraft }) {
  return (
    <>
      <section className="welcome" aria-labelledby="welcome-title">
        <h1 id="welcome-title">{APP_CONFIG.welcomeTitle}<br /><em>{APP_CONFIG.welcomeEmphasis}</em></h1>
        <p>{APP_CONFIG.welcomeDescription}</p>
      </section>
      <div className="composer">
        <Link className="write-action" to="/memories/new"><Icon name="pen" />{hasDraft ? 'Continue writing' : 'Write a memory'}</Link>
      </div>
      <div className="journal-grid">
        <section aria-labelledby="recent-title">
          <div className="section-heading"><h2 id="recent-title">Pages from your life</h2><span>Sample entries</span></div>
          <MemoryList memories={sampleMemories} />
          <Link className="text-link browse-link" to="/memories">Browse memories <Icon name="arrow" /></Link>
        </section>
        <aside className="reflection" aria-labelledby="reflection-title">
          <Icon name="sun" />
          <h2 id="reflection-title">Something to{' '}<br />sit with.</h2>
          <p className="reflection-question">What’s one ordinary thing you hope you never forget?</p>
          <span className="brass-rule" />
          <p className="reflection-note">There’s no right answer.<br />Only yours.</p>
        </aside>
      </div>
      <section className="capsule" aria-labelledby="capsule-title">
        <div className="capsule-heading"><Icon name="lock" /><h2 id="capsule-title">A little time between<br />then and now.</h2></div>
        <div className="capsule-detail">
          <span className="capsule-status">Sealed · Sample capsule</span>
          <p>Your writing stays hidden<br />until you open the capsule.</p>
          <Link className="text-link capsule-link" to="/capsules">View time capsules <Icon name="arrow" /></Link>
        </div>
      </section>
    </>
  )
}
