import { Link } from 'react-router'

export default function NotFoundPage({ memoryMissing = false }) {
  return (
    <section className="empty-state not-found">
      <h1>
        {memoryMissing
          ? 'This memory is not here.'
          : 'A page out of place.'}
      </h1>

      <p>
        {memoryMissing
          ? 'This memory is no longer available in this tab. Refreshing or closing this tab clears writing held here.'
          : 'The address may have changed, or the page does not exist'}
      </p>

      <Link
        className='write-action'
        to={memoryMissing ? '/memories': '/today'}
      >
        {memoryMissing ? 'Back to memories':'Back to Today'}
      </Link>
    </section>
  )
}