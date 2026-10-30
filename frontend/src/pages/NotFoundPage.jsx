import { Link } from 'react-router'

export default function NotFoundPage() {
  return (
    <section className="empty-state not-found">
      <h1>A page out of place.</h1><p>This address does not lead to journal page. Your saved memories are still in your journal</p><Link className='write-action' to="/today">Return to your journal</Link></section>
  )
}