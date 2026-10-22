import { APP_CONFIG } from './config/app.js'

function Icon({ name, ...props }) {
  const paths = {
    book: <><path d="M12 5v15M3 4h5a4 4 0 0 1 4 2 4 4 0 0 1 4-2h5v15h-5a4 4 0 0 0-4 2 4 4 0 0 0-4-2H3Z" /></>,
    sun: <><circle cx="12" cy="12" r="4" /><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5" /></>,
    clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
    pen: <><path d="m15 4 5 5M4 20l5-1L21 7a2 2 0 0 0-4-4L5 15Z" /></>,
    lock: <><rect x="5" y="10" width="14" height="11" rx="2" /><path d="M8 10V7a4 4 0 0 1 8 0v3m-4 4v3" /></>,
    arrow: <path d="M5 12h14m-5-5 5 5-5 5" />,
    plus: <path d="M12 5v14M5 12h14" />,
  }

  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none"
      stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"
      strokeLinejoin="round" aria-hidden="true" {...props}>
      {paths[name]}
    </svg>
  )
}

const sampleMemories = [
  {
    day: '21', month: 'OCT', title: 'The long way home', tag: 'Everyday life',
    excerpt: 'I took the quieter street today. Nothing remarkable happened…',
    body: 'But I noticed the light on the old buildings, and the little bakery I always walk past. I think I needed an afternoon without a destination.',
  },
  {
    day: '18', month: 'OCT', title: 'A Sunday worth keeping', tag: 'Small joys',
    excerpt: 'Coffee went cold while we talked. Neither of us seemed to mind.',
    body: 'I want to remember how easy the conversation felt. No occasion, no plans. Just a few hours that belonged entirely to us.',
  },
]

export default function App() {
  return (
    <>
      <title>{`${APP_CONFIG.name} — Today`}</title>
      <meta name="description" content={APP_CONFIG.description} />
      <a className="skip-link" href="#today">Skip to journal</a>

      <div className="journal-app">
        <aside className="sidebar" aria-label="Journal navigation">
          <a className="brand" href="#today">
            <Icon name="book" />
            <span>{APP_CONFIG.name}</span>
          </a>
          <p className="brand-note">Life, in your own words.</p>

          <nav className="navigation" aria-label="On this page">
            <a href="#today"><Icon name="sun" />Today</a>
            <a href="#memories"><Icon name="book" />Memories</a>
            <a href="#capsules"><Icon name="clock" />Time capsules</a>
          </nav>

          <div className="sidebar-note">
            <span className="brass-rule" />
            <p>For the days you’ll want to remember.</p>
            <span>A little space. Entirely yours.</span>
          </div>
        </aside>

        <main id="today" className="workspace" tabIndex={-1}>
          <header className="workspace-header">
            <span>Your journal <span className="breadcrumb-slash">/</span> Today</span>
            <span className="demo-label">Sample journal</span>
          </header>

          <div className="page-content">
            <section className="welcome" aria-labelledby="welcome-title">
              <h1 id="welcome-title">A moment,<br /><em>just for you.</em></h1>
              <p>You don’t need a remarkable day to have something worth keeping.</p>
            </section>

            <details className="composer">
              <summary className="write-action"><Icon name="pen" />Write a memory</summary>
              <div className="writing-sheet">
                <p id="writing-note" className="writing-note">Writing preview only. Text is not saved and will be lost on refresh.</p>
                <label htmlFor="memory-title">Title</label>
                <input id="memory-title" type="text" placeholder="Give this moment a name" autoComplete="off" aria-describedby="writing-note" />
                <label htmlFor="memory-body">Your words</label>
                <textarea id="memory-body" rows="7" placeholder="Start wherever you are…" aria-describedby="writing-note" />
              </div>
            </details>

            <div className="journal-grid">
              <section id="memories" className="memories" aria-labelledby="memories-title">
                <div className="section-heading">
                  <h2 id="memories-title">Pages from your life</h2>
                  <span>Sample entries</span>
                </div>

                {sampleMemories.map((memory) => (
                  <details className="memory" key={memory.title}>
                    <summary>
                      <span className="memory-date"><strong>{memory.day}</strong><span>{memory.month}</span></span>
                      <span className="memory-copy">
                        <span className="memory-title">{memory.title}</span>
                        <span className="memory-excerpt">{memory.excerpt}</span>
                        <span className="memory-tag">{memory.tag}</span>
                      </span>
                      <Icon name="plus" className="expand-icon" />
                    </summary>
                    <p className="memory-body">{memory.body}</p>
                  </details>
                ))}

                <p className="reading-hint">Select a page to read a little more.</p>
              </section>

              <aside className="reflection" aria-labelledby="reflection-title">
                <Icon name="sun" />
                <h2 id="reflection-title">Something to{' '}<br />sit with.</h2>
                <p className="reflection-question">What’s one ordinary thing you hope you never forget?</p>
                <span className="brass-rule" />
                <p className="reflection-note">There’s no right answer.<br />Only yours.</p>
              </aside>
            </div>

            <section id="capsules" className="capsule" aria-labelledby="capsule-title">
              <div className="capsule-heading">
                <Icon name="lock" />
                <h2 id="capsule-title">A little time between<br />then and now.</h2>
              </div>
              <div className="capsule-detail">
                <span className="capsule-status">Sealed · Sample capsule</span>
                <p>Your writing stays hidden<br />until you open the capsule.</p>
                <div className="capsule-date">Scheduled for <time dateTime="2026-11-12">12 November 2026</time></div>
              </div>
            </section>

            <footer className="workspace-footer">
              <span>{APP_CONFIG.name}</span>
              <span>Design preview · Sample content · Nothing is saved</span>
            </footer>
          </div>
        </main>
      </div>
    </>
  )
}
