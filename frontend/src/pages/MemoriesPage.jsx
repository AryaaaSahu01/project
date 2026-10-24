import { Link, useSearchParams } from 'react-router'
import MemoryList from '../components/MemoryList.jsx'
import Icon from '../components/Icon.jsx'

export default function MemoriesPage({ hasDraft, memories }) {
  const [params, setParams] = useSearchParams()
  const query = params.get('q') || ''
  const sort = params.get('sort') === 'oldest' ? 'oldest' : 'newest'
  const terms = query.trim().toLocaleLowerCase()
  
  const visible = memories
    .filter((memory) =>
      [memory.title, memory.body].some((value) =>
        value.toLocaleLowerCase().includes(terms),
      ),
    )
    .sort((a,b) =>
      sort ==='oldest'
        ? a.date.localeCompare(b.date)
        : b.date.localeCompare(a.date),
    )
  function updateParam(key,value) {
    const next = new URLSearchParams(params)

    if (value) next.set(key,value)
    else next.delete(key)
    setParams(next, {replace: true})
  }

  return (
    <>
      <section className='page-heading' aria-labelledby='memories-heading'>
        <div>
          <h1 id='memories-heading'>Your memories.</h1>
          <p>Ordinary days. Words worth coming back to.</p>
        </div>

        <Link className='write-action' to='/memories/new'>
          <Icon name='pen'/>
          {hasDraft ? 'Continue writing':'Write a memory'}
        </Link>
      </section>

      {memories.length > 0 && (
        <div className='library-tools'>
          <div className='field search-field'>
            <label htmlFor='memory-search'>Search memories</label>
            <input
              id="memory-search"
              type="search"
              value={query}
              placeholder='A title, a moment, a word...'
              onChange={(event) =>
                updateParam('q', event.target.value)
              }
            />
          </div>
          
          <div className='field'>
            <label htmlFor='memory-sort'>Order</label>
            <select 
              id="memory-test"
              value={sort}
              onChange={(event) =>
                updateParam('sort', event.target.value)
              }
            >
              <option value="newest">Newest first</option>
              <option value="oldest">Oldest first</option>
            </select>
          </div>
        </div>
      )}

      <p className='results-note' role='status'>
        {visible.length} {visible.length === 1 ? 'memory' : 'memories'}
        {terms && memories.length ? ' found' : ' in this tab'}
      </p>

      {visible.length ? (
        <MemoryList memories={visible} />
      ) : memories.length ? (
        <section className='empty-state'>
          <h2>No pages match those words.</h2>
          <p>
            Try another word, or clear your search to see your memories.
          </p>
          <button
            className='secondary-button'
            onClick={() => updateParam('q', '')}
          >
            Clear Search
          </button>
        </section>
      ) : (
        <section className='empty-state'>
          <h2>Every story starts somewhere.</h2>
          <p>
            Write a memory and choose "Keep in this tab" to see it here.
          </p>
          <Link className='secondary-button' to='/memories/new'>
            {hasDraft ? 'Continue writing' : 'Write your first memory'}
          </Link>
        </section>
      )}
    </>
  )
}
