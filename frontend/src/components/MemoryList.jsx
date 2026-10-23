import { Link } from 'react-router'
import Icon from './Icon.jsx'

export default function MemoryList({ memories }) {
  return (
    <ul className="memory-list">
      {memories.map((memory) => (
        <li key={memory.id} className="memory">
          <Link className="memory-row" to={`/memories/${memory.id}`}>
            <time className="memory-date" dateTime={memory.date}>
              <strong>{memory.day}</strong><span>{memory.month}</span>
            </time>
            <span className="memory-copy">
              <span className="memory-title">{memory.title}</span>
              <span className="memory-excerpt">{memory.excerpt}</span>
              <span className="memory-tag">{memory.tag}</span>
            </span>
            <Icon name="arrow" className="row-arrow" />
          </Link>
        </li>
      ))}
    </ul>
  )
}
