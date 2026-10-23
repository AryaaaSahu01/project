import {useState} from 'react'
import {Link, useParam, useParams} from 'react-router'
import {downloadText} from '../lib/download.js'
import NotFoundPage from './NotFoundPage.jsx'

export default function MemoryPage({ memories }) {
  const {id} = useParams()
  const [notice, setNotice] = useState('')
  const memory = memories.find((entry) => entry.id === id)

  if (!memory) return <NotFoundPage memoryMissing />

  function exportMemory() {
    setNotice(
      downloadText(
        'memory.txt'
        `${memory.title}\n\n${memory.body}\n`
      )
        ? 'Download requested. Check your downloads for memory.txt.'
        : 'The download could not start. Select and copy your writing before this tab.',
    )
  }

  return (
    <article className='reading-page'>
      <Link className="text-link" to="/memories">
        Back to memories
      </Link>

      <header className="reading-header">
        <h1>{memory.title}</h1>
        <div className="reading-meta">
          <time dateTime={memory.date}>
            {new Date(memory.date).toLocaleDateString(undefined, {
              day: 'numeric',
              month: 'long',
              year: 'numeric',
            })}
          </time>

          <span>In this tab</span>
        </div>
      </header>

      <div className="reading-body">
        {memory.body.split(/\n\s*\n/).map((paragraph, index) => (
          <p className="preserve-lines" key={index}>
            {paragraph}
          </p>
        ))}
      </div>

      <div className="reading-end">
        <span className="brass-rule"/>
        <p>A small moment, give a place.</p>
      </div>

      <div className="editor-actions">
        <button className="write-action" onClick={exportMemory}>
          Download Text
        </button>

        <Link className="secondary-button" to="/capsules/new">
          Plan a capsule
        </Link>

        <Link className="text-link" to="/memories/new">
          Write another memory
        </Link>
      </div>

      <p className='editor-status' role="status">
        {notice}
      </p>
    </article>
  )
}