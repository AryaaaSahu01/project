import { useEffect, useRef } from 'react'
import { Link, NavLink, Outlet, useLocation } from 'react-router'
import { APP_CONFIG } from '../config/app.js'
import Icon from '../components/Icon.jsx'
import Brand from '../components/Brand.jsx'
import { useAuth } from '../lib/auth.js'
import { useJournal } from '../lib/journal.js'

function pageName(pathname) {
  if (pathname === '/today') return 'Today'
  if (pathname === '/memories') return 'Memories'
  if (pathname === '/memories/new') return 'Write a memory'
  if (/^\/memories\/[^/]+$/.test(pathname)) return 'Your memory'
  if (pathname === '/capsules') return 'Time capsules'
  if (pathname === '/capsules/new') return 'Seal a capsule'
  if (pathname === '/sign-in') return 'Sign in'
  if (pathname === '/sign-up') return 'Create an account'
  if (pathname === '/account') return 'Your account'
  return 'Page not found'
}
export default function AppLayout() {
  const { pathname } = useLocation()
  const mainRef = useRef(null)
  const { status } = useAuth()
  const { hasUnsaved, summary } = useJournal()
  const title = pageName(pathname)
  useEffect(() => { window.scrollTo(0, 0); mainRef.current?.focus({ preventScroll: true }) }, [pathname])
  return <>
    <title>{`${APP_CONFIG.name} — ${title}`}</title><meta name="description" content={APP_CONFIG.description} />
    <a className="skip-link" href="#main-content">Skip to content</a>
    <div className="journal-app">
      <aside className="sidebar" aria-label="Journal navigation">
        <Brand /><p className="brand-note">{APP_CONFIG.tagline}</p>
        <nav className="navigation" aria-label="Main navigation">
          <NavLink to="/today"><Icon name="sun" />Today</NavLink>
          <NavLink to="/memories"><Icon name="book" />Memories</NavLink>
          <NavLink to="/capsules"><Icon name="clock" />Time capsules{summary.ready > 0 && <span className="ready-count">{summary.ready}<span className="sr-only"> ready to open</span></span>}</NavLink>
        </nav>
        <div className="sidebar-note"><Icon name="quote" className="quote-mark" /><blockquote>{APP_CONFIG.sidebarQuote}</blockquote>
          <div className="privacy-note"><Icon name="lock" /><div><p>{APP_CONFIG.privacyTagline}</p><span>{APP_CONFIG.privacyDetail}</span></div></div>
        </div>
      </aside>
      <main id="main-content" className="workspace" tabIndex={-1} ref={mainRef}>
        <header className="workspace-header"><span>Your journal<span className="breadcrumb-slash">/</span>{title}</span>
          <div className="workspace-account-actions"><span className="session-label">{status === 'signed-in' ? 'Your private journal' : 'A place for your words'}</span>
            <Link className="account-entry" to={status === 'signed-in' ? '/account' : '/sign-in'}>{status === 'signed-in' ? 'Your account' : 'Sign in'}</Link>
          </div>
        </header>
        <div className="page-content">
          {hasUnsaved && pathname !== '/memories/new' && <div className="draft-reminder"><p>You have unsaved writing.</p><Link className="text-link" to="/memories/new">Continue writing<Icon name="arrow" /></Link></div>}
          <Outlet />
          <footer className="workspace-footer"><span>{APP_CONFIG.name}</span><span>{APP_CONFIG.tagline}</span></footer>
        </div>
      </main>
    </div>
  </>
}
