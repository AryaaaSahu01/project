import { useEffect, useState } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router'
import AppLayout from './layouts/AppLayout.jsx'
import TodayPage from './pages/TodayPage.jsx'
import MemoriesPage from './pages/MemoriesPage.jsx'
import MemoryPage from './pages/MemoryPage.jsx'
import WritePage from './pages/WritePage.jsx'
import CapsulesPage from './pages/CapsulesPage.jsx'
import NotFoundPage from './pages/NotFoundPage.jsx'
import './styles/pages.css'

export default function App() {
  // Session-only writing survives route changes, never refresh or tab closure.
  const [draft, setDraft] = useState({ title: '', body: '' })
  const hasDraft = Boolean(draft.title || draft.body)

  useEffect(() => {
    if (!hasDraft) return
    function warnBeforeLeaving(event) {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', warnBeforeLeaving)
    return () => window.removeEventListener('beforeunload', warnBeforeLeaving)
  }, [hasDraft])

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout hasDraft={hasDraft} />}>
          <Route index element={<Navigate to="/today" replace />} />
          <Route path="today" element={<TodayPage hasDraft={hasDraft} />} />
          <Route path="memories" element={<MemoriesPage hasDraft={hasDraft} />} />
          <Route path="memories/new" element={<WritePage draft={draft} onChange={setDraft} />} />
          <Route path="memories/:id" element={<MemoryPage />} />
          <Route path="capsules" element={<CapsulesPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
