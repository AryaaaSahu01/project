import { Link, useSearchParams } from 'react-router'
import { useJournal, useMemoryCollection } from '../lib/journal.js'
import MemoryList from '../components/MemoryList.jsx'
import Icon from '../components/Icon.jsx'

export default function MemoriesPage() {
  const [params, setParams] = useSearchParams()
  const query = params.get('q') || ''
  const sort = params.get('sort') === 'oldest' ? 'oldest' : 'newest'
  const { hasUnsaved, refresh } = useJournal()

  const {
    items,
    status,
    error,
    more,
    loadMore
  } = useMemoryCollection({ q: query, sort })

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
          {hasUnsaved ? 'Continue writing':'Write a memory'}
        </Link>
      </section>

      
        <div className='library-tools'>
          <div className='field search-field'>
            <label htmlFor='memory-search'>Search your writing</label>
            <input
              id="memory-search"
              type="search"
              value={query}
              maxLength={120}
              placeholder='A title, a moment, a world...'
              onChange={(event) =>
                updateParam('q', event.target.value)
              }
              aria-describedby="search-note"
            />
          </div>
          
          <div className='field'>
            <label htmlFor='memory-sort'>Order</label>
            <select 
              id="memory-sort"
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
        
        <p className="results-note" id="search-note">
            Sealed words stay hidden from search until you open them.
        </p>

        {error && (
          <div>
            <p className='form-error' role="alert">
              {error}
            </p>
            <button
              className='secondary-button'
              onClick={refresh}
            >
              Try again
            </button>
          </div>
        )}

        {status === 'loading' ? (
          <p className="page-note" role="status">
            Opening your saved memories...
          </p>
        ) : (
          <>
            <p className="results-note" role="status">
              {items.length} {items.length === 1 ? 'memory':'memories'}
              {more ? ' shown' : query ? ' found' : ''}
            </p>

            {items.length ? (
              <MemoryList memories={items} />
            ) : !error ? (
              <section className='empty-state'>
                <h2>
                  {query
                    ? 'No pages match those words.'
                    : 'Every story starts somewhere.'}
                </h2>
                <p>
                  {query
                    ? 'Try another word, or clear your search.'
                    : 'Write a memory and save it to your journal.'}
                </p>
                
                {query ? (
                  <button
                    className='secondary-button'
                    onClick={() => updateParam('q', '')}
                  >
                    Clear search
                  </button>
                ) : (
                  <Link
                    className='secondary-button'
                    to="/memories/new"
                  >
                    Write your first memory
                  </Link>
                )}
              </section>
            ) : null}

            {more && (
              <button
                className='secondary-button load-more'
                disabled={status === 'paging'}
                onClick={loadMore}
              >
                {status === 'paging' ? 'Loading...' : 'Load more memories'} 
              </button>
            )}
          </>
        )}
    </>
  )
}
