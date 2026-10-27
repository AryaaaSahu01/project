import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router'
import AppLayout from './layouts/AppLayout.jsx'
import AuthProvider from './components/AuthProvider.jsx'
import AppearanceProvider from './components/AppearanceProvider.jsx'
import JournalProvider from './components/JournalProvider.jsx'
import { useAuth } from './lib/auth.js'
import { useJournal } from './lib/journal.js'
import TodayPage from './pages/TodayPage.jsx'
import MemoriesPage from './pages/MemoriesPage.jsx'
import MemoryPage from './pages/MemoryPage.jsx'
import WritePage from './pages/WritePage.jsx'
import CapsulesPage from './pages/CapsulesPage.jsx'
import CapsulePlannerPage from './pages/CapsulePlannerPage.jsx'
import AccountPage from './pages/AccountPage.jsx'
import AuthPage from './pages/AuthPage.jsx'
import NotFoundPage from './pages/NotFoundPage.jsx'
import './styles/pages.css'
import './styles/account.css'
import './styles/journal.css'

function PrivateJournal({ children }) {
  const { status, error, refreshSession } = useAuth()
  const journal = useJournal()
  const location = useLocation()
  if (status === 'checking') return <p className="account-loading" role="status">Opening your journal…</p>
  if (status === 'signed-out') return <Navigate to="/sign-in" state={{ from: location.pathname + location.search }} replace />
  if (status === 'unavailable') return <div className="empty-state"><h1>Let’s reconnect.</h1><p role="alert">{error}</p><button className="secondary-button" onClick={() => refreshSession()}>Try again</button></div>
  if (journal.status === 'loading') return <p className="account-loading" role="status">Opening your saved memories…</p>
  if (journal.status === 'error') return <div className="empty-state"><h1>Your journal is temporarily unavailable.</h1><p role="alert">{journal.error}</p><button className="secondary-button" onClick={journal.retryWorkspace}>Try again</button></div>
  return children
}
function JournalRoutes() {
  const { account } = useAuth()
  const location = useLocation()
  return <JournalProvider key={account?.user?.id || 'guest'}>
    <Routes><Route element={<AppLayout />}>
      <Route index element={<Navigate to="/today" replace />} />
      <Route path="today" element={<TodayPage />} />
      <Route path="memories" element={<PrivateJournal><MemoriesPage /></PrivateJournal>} />
      <Route path="memories/new" element={<PrivateJournal><WritePage /></PrivateJournal>} />
      <Route path="memories/:id" element={<PrivateJournal><MemoryPage key={location.pathname} /></PrivateJournal>} />
      <Route path="capsules" element={<PrivateJournal><CapsulesPage /></PrivateJournal>} />
      <Route path="capsules/new" element={<PrivateJournal><CapsulePlannerPage key={location.pathname + location.search} /></PrivateJournal>} />
      <Route path="sign-in" element={<AuthPage key="login" mode="login" />} />
      <Route path="sign-up" element={<AuthPage key="register" mode="register" />} />
      <Route path="account" element={<AccountPage />} />
      <Route path="*" element={<NotFoundPage />} />
    </Route></Routes>
  </JournalProvider>
}
export default function App() {
  return <BrowserRouter><AppearanceProvider><AuthProvider><JournalRoutes /></AuthProvider></AppearanceProvider></BrowserRouter>
}
