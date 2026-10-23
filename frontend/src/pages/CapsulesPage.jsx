import { useState } from 'react'
import Icon from '../components/Icon.jsx'
import { sampleCapsules } from '../mocks/journal.js'

const filters = [{ value: 'all', label: 'All' }, { value: 'sealed', label: 'Sealed' }, { value: 'ready', label: 'Ready to open' }, { value: 'opened', label: 'Opened' }]

export default function CapsulesPage() {
  const [filter, setFilter] = useState('all')
  const capsules = sampleCapsules.filter((capsule) => filter === 'all' || capsule.state === filter)
  return (
    <>
      <section className="page-heading" aria-labelledby="capsules-heading"><div>
        <h1 id="capsules-heading">For another day.</h1><p>Some words need a little distance. Give them time.</p>
      </div></section>
      <div className="filter-bar" role="group" aria-label="Filter capsules">
        {filters.map(({ value, label }) => <button className="filter-button" key={value} aria-pressed={filter === value} onClick={() => setFilter(value)}>{label}</button>)}
      </div>
      <p className="results-note" role="status">{capsules.length} sample {capsules.length === 1 ? 'capsule' : 'capsules'}</p>
      {capsules.length ? capsules.map((capsule) => (
        <article className="capsule capsule-on-page" key={capsule.id}>
          <div className="capsule-heading"><Icon name="lock" /><h2>A memory, sealed.</h2><p className="sealed-note">The title and words stay out of view.</p></div>
          <div className="capsule-detail"><span className="capsule-status">Sealed · Sample capsule</span>
            <p>This is a metadata-only example.</p>
            <div className="capsule-date">Scheduled for <time dateTime={capsule.scheduledDate}>{capsule.scheduledLabel}</time></div>
          </div>
        </article>
      )) : <section className="empty-state"><h2>{filter === 'ready' ? 'Nothing waiting to be opened.' : 'No opened capsules yet.'}</h2><p>This sample journal contains one sealed capsule. Choose All to see it.</p><button className="secondary-button" onClick={() => setFilter('all')}>View all capsules</button></section>}
      <div className="capsule-explainer"><span className="brass-rule" /><h2>Opening is your choice.</h2><p>A reminder that a capsule is ready is separate from opening it. Scheduling and opening controls will arrive with the backend connection.</p></div>
    </>
  )
}
