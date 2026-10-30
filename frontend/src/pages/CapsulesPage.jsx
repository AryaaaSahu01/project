import { Link, useSearchParams } from 'react-router'
import Icon from '../components/Icon.jsx'
import { useJournal, useMemoryCollection } from '../lib/journal.js'
import { formatReturnTime } from '../lib/capsuleSchedule.js'

const filters = [['capsules', 'All'], ['sealed', 'Sealed'], ['ready', 'Ready to open'], ['returned', 'Opened']]
export default function CapsulesPage() {
  const [params, setParams] = useSearchParams()
  const state = filters.some(([value]) => value === params.get('state')) ? params.get('state') : 'capsules'
  const { refresh } = useJournal()
  const { items, status, error, more, loadMore } = useMemoryCollection({ state })
  return <>
    <section className="page-heading"><div><h1>For another day.</h1><p>Some words need a little distance. Give them time.</p></div><Link className="write-action" to="/capsules/new"><Icon name="clock" />Seal a capsule</Link></section>
    <div className="filter-bar" role="group" aria-label="Filter capsules">{filters.map(([value, label]) => <button className="filter-button" key={value} aria-pressed={state === value} onClick={() => setParams(value === 'capsules' ? {} : { state: value })}>{label}</button>)}</div>
    {error && <div><p className="form-error" role="alert">{error}</p><button className="secondary-button" onClick={refresh}>Try again</button></div>}
    {status === 'loading' ? <p className="page-note" role="status">Opening your time capsules…</p> : <><p className="results-note" role="status">{items.length} {items.length === 1 ? 'capsule' : 'capsules'}{more ? ' shown' : ''}</p>
      {items.map((memory) => <article className="capsule capsule-on-page" key={memory.id}><div className="capsule-heading"><Icon name={memory.status === 'returned' ? 'book' : 'lock'} /><h2>{memory.status === 'returned' ? memory.title : memory.can_open ? 'A moment has come back.' : 'A day to return.'}</h2><p className="sealed-note">{memory.tag}</p></div><div className="capsule-detail"><p>{memory.status === 'returned' ? 'Your words are yours to revisit.' : memory.can_open ? 'Your memory is ready whenever you are.' : 'Your writing is sealed until its return time.'}</p><div className="capsule-date">Return time <time dateTime={memory.unlock_at}>{formatReturnTime(memory.unlock_at)}</time></div><Link className="text-link capsule-link" to={`/memories/${memory.id}`}>{memory.status === 'returned' ? 'Read memory' : memory.can_open ? 'Open your capsule' : 'View capsule'}<Icon name="arrow" /></Link></div></article>)}
      {!items.length && !error && <section className="empty-state"><h2>{state === 'ready' ? 'Nothing waiting to be opened.' : state === 'returned' ? 'Your returned words will be here.' : 'A little distance. A different perspective.'}</h2><p>{state === 'ready' ? 'Your capsules become ready at their scheduled time, even while you are away.' : 'Save a memory, choose its return time, and seal it for your future self.'}</p><Link className="secondary-button" to="/capsules/new">Seal a capsule</Link></section>}
      {more && <button className="secondary-button load-more" disabled={status === 'paging'} onClick={loadMore}>{status === 'paging' ? 'Loading…' : 'Load more capsules'}</button>}
    </>}
    <p className="page-note">The return time makes a capsule ready to open. Its words stay hidden until you choose Open.</p>
  </>
}
