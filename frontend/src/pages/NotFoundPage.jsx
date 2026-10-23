import { Link } from 'react-router'

export default function NotFoundPage({ memoryMissing = false }) {
  return (
    <section className="empty-state not-found">
      <h1>{memoryMissing ? 'This memory is not here.' : 'A page out of place.'}</h1>
      <p>{memoryMissing ? 'That link does not match an available sample memory.' : 'The address may have changed, or the page does not exist.'}</p>
      <Link className="write-action" to={memoryMissing ? '/memories' : '/today'}>{memoryMissing ? 'Back to memories' : 'Back to Today'}</Link>
    </section>
  )
}
