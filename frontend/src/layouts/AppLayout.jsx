import { useEffect, useRef } from 'react'
import { Link, NavLink, Outlet, useLocation } from 'react-router'
import { APP_CONFIG } from '../config/app.js'
import Icon from '../components/Icon.jsx'
import { sampleMemories } from '../mocks/journal.js'

function pageName(pathname) {
  if (pathname === '/today') return 'Today'
  if (pathname === '/memories') return 'Memories'
  if (pathname === '/memories/new') return 'Write a memory'
  if (/^\/memories\/[^/]+$/.test(pathname)) return 'Read a memory'
  if (pathname === '/capsules') return 'Time capsules'
  return 'Page not found'
}

export default function AppLayout({ hasDraft }) {
  const { pathname } = useLocation()
  const mainRef = useRef(null)
  const title = pageName(pathname)
  const memory = sampleMemories.find((entry) => pathname === `/memories/${entry.id}`)
  const documentTitle = memory?.title || title

  useEffect(() => {
    window.scrollTo(0, 0)
    mainRef.current?.focus({ preventScroll: true })
  }, [pathname])

  return (
    <>
      <title>{`${APP_CONFIG.name} — ${documentTitle}`}</title>
      <meta name="description" content={APP_CONFIG.description} />
      <a className="skip-link" href="#main-content">Skip to content</a>
      <div className="journal-app">
        <aside className="sidebar" aria-label="Journal navigation">
          <Link className="brand" to="/today"><span>{APP_CONFIG.name}</span></Link>
          <p className="brand-note">{APP_CONFIG.tagline}</p>
          <p className="mobile-privacy"><Icon name="lock" />{APP_CONFIG.privacyTagline}</p>
          <nav className="navigation" aria-label="Main navigation">
            <NavLink to="/today"><Icon name="sun" />Today</NavLink>
            <NavLink to="/memories"><Icon name="book" />Memories</NavLink>
            <NavLink to="/capsules"><Icon name="clock" />Time capsules</NavLink>
          </nav>
          <div className="sidebar-note">
            <Icon name="quote" className="quote-mark" />
            <blockquote>{APP_CONFIG.sidebarQuote}</blockquote>
            <div className="privacy-note"><Icon name="lock" /><div>
              <p>{APP_CONFIG.privacyTagline}</p><span>{APP_CONFIG.privacyDetail}</span>
            </div></div>
          </div>
        </aside>
        <main id="main-content" className="workspace" tabIndex={-1} ref={mainRef}>
          <header className="workspace-header">
            <span>Your journal <span className="breadcrumb-slash">/</span> {title}</span>
            <span className="demo-label">Sample journal</span>
          </header>
          <div className="page-content">
            {hasDraft && pathname !== '/memories/new' && (
              <div className="draft-reminder">
                <p>You have writing in this tab. It is not saved.</p>
                <Link className="text-link" to="/memories/new">Continue writing <Icon name="arrow" /></Link>
              </div>
            )}
            <Outlet />
            <footer className="workspace-footer">
              <span>{APP_CONFIG.name}</span>
              <span>Design preview · Sample content · No account data connected</span>
            </footer>
          </div>
        </main>
      </div>
    </>
  )
}
