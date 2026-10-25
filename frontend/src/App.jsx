import { useEffect, useState } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router'
import AppLayout from './layouts/AppLayout.jsx'
import TodayPage from './pages/TodayPage.jsx'
import MemoriesPage from './pages/MemoriesPage.jsx'
import MemoryPage from './pages/MemoryPage.jsx'
import WritePage from './pages/WritePage.jsx'
import CapsulesPage from './pages/CapsulesPage.jsx'
import CapsulePlannerPage from './pages/CapsulePlannerPage.jsx'
import NotFoundPage from './pages/NotFoundPage.jsx'
import './styles/pages.css'

export default function App() {
  const [draft, setDraft] = useState({ title: '', body: '' })
  const [memories, setMemories] = useState([])
  const [plans, setPlans] = useState([])
  const [planDraft, setPlanDraft] = useState({
    memoryId: '',
    localDateTime: '',
  })

  const hasDraft = Boolean(draft.title || draft.body)

  const hasWriting =
    hasDraft ||
    memories.length > 0 ||
    plans.length > 0 ||
    Boolean(planDraft.memoryId || planDraft.localDateTime)

  useEffect(() => {
    if (!hasWriting) return 

    function warnBeforeLeaving(event) {
      event.preventDefault()
      event.returnValue = ''
    }

    window.addEventListener('beforeunload', warnBeforeLeaving)

    return () => {
      window.removeEventListener('beforeunload', warnBeforeLeaving)
    }
  }, [hasWriting])

  function keepMemory() {
    if (!draft.body.trim()) return null

    const now = new Date()

    const memory = {
      id: crypto.randomUUID(),
      title: draft.title.trim() || 'Untitled memory',
      body: draft.body,
      excerpt: draft.body.trim().slice(0,120),
      date: now.toISOString(),
      day: String(now.getDate()).padStart(2,'0'),
      month: now
        .toLocaleDateString('en', { month: 'short' })
        .toUpperCase(),
      tag: 'In this tab',
    }

    setMemories((current) => [memory, ...current])
    setDraft({ title: '', body: '' })

    return memory.id
  }

  function keepPlan(schedule) {
    if (!memories.some((memory) => memory.id === schedule.memoryId)) {
      return false
    }

    setPlans((current) => [
      { ...schedule, id: crypto.randomUUID()},
      ...current,
    ])

    setPlanDraft({ memoryId: '', localDateTime: ''})

    return true
  }

  return (
    <BrowserRouter>
      <Routes>
        <Route 
          element={
            <AppLayout hasDraft={hasDraft} memories={memories} />
          }
        >

          <Route index element={<Navigate to="/today" replace/>}/>

          <Route
            path="today"
            element={
              <TodayPage 
                hasDraft={hasDraft}
                memories={memories}
                plans={plans}
              />
            }
          />

          <Route
            path="memories"
            element={
              <MemoriesPage
                hasDraft={hasDraft}
                memories={memories}
              />
            }
          />

          <Route 
            path="memories/new"
            element={
              <WritePage
                draft={draft}
                onChange={setDraft}
                onKeep={keepMemory}
              />
            }
          />

          <Route
            path="memories/:id"
            element={<MemoryPage memories={memories}/>}
          />

          <Route  
            path="capsules"
            element={<CapsulesPage plans={plans}/>}
          />

          <Route
            path="capsules/new"
            element={
              <CapsulePlannerPage
                memories={memories}
                draft={planDraft}
                onChange={setPlanDraft}
                onKeep={keepPlan}
              />
            }   
          />

          <Route path="*" element={<NotFoundPage/>} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
