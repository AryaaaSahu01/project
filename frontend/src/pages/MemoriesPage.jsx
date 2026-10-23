import { Link, useSearchParams } from 'react-router'
import MemoryList from '../components/MemoryList.jsx'
import Icon from '../components/Icon.jsx'
import { sampleMemories } from '../mocks/journal.js'

export default function MemoriesPage({ hasDraft }) {
  const [params, setParams] = useSearchParams()
  const query = params.get('q') || ''
  const sort = params.get('sort') === 'oldest' ? 'oldest' : 'newest'
  const terms = query.trim().toLocaleLowerCase()
  const memories = sampleMemories.filter((memory) =>
    [memory.title, memory.body, memory.tag].some((value) => value.toLocaleLowerCase().includes(terms)),
  ).sort((a, b) => sort === 'oldest' ? a.date.localeCompare(b.date) : b.date.localeCompare(a.date))

  function updateParam(key, value) {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    setParams(next, { replace: true })
  }

  return (
    <>
      <section className="page-heading" aria-labelledby="memories-heading">
        <div><h1 id="memories-heading">Your memories.</h1><p>Ordinary days. Words worth coming back to.</p></div>
        <Link className="write-action" to="/memories/new"><Icon name="pen" />{hasDraft ? 'Continue writing' : 'Write a memory'}</Link>
      </section>
      <div className="library-tools">
        <div className="field search-field"><label htmlFor="memory-search">Search sample memories</label>
          <input id="memory-search" type="search" value={query} placeholder="A title, a moment, a word…" onChange={(event) => updateParam('q', event.target.value)} />
        </div>
        <div className="field"><label htmlFor="memory-sort">Order</label>
          <select id="memory-sort" value={sort} onChange={(event) => updateParam('sort', event.target.value)}><option value="newest">Newest first</option><option value="oldest">Oldest first</option></select>
        </div>
      </div>
      <p className="results-note" role="status">{memories.length} sample {memories.length === 1 ? 'memory' : 'memories'}{terms ? ' found' : ' · October 2026'}</p>
      {memories.length ? <MemoryList memories={memories} /> : (
        <section className="empty-state" aria-labelledby="empty-heading">
          <h2 id="empty-heading">No pages match those words.</h2><p>Try another word, or clear your search to see all sample memories.</p>
          <button className="secondary-button" onClick={() => updateParam('q', '')}>Clear search</button>
        </section>
      )}
      <p className="page-note">These two entries are examples. Your account’s memories will appear here after sign-in is connected.</p>
    </>
  )
}
