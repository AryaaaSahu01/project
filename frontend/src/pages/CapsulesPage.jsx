import { useState } from 'react'
import { Link } from 'react-router'
import Icon from '../components/Icon.jsx'
import { formatSchedule } from '../lib/capsuleSchedule.js'
import { downloadText } from '../lib/download.js'


const filters = [
  { value: 'all', label: 'All' },
  { value: 'planned', label: 'Planned'},
  { value: 'sealed', label: 'Sealed' },
  { value: 'ready', label: 'Ready to open' },
  { value: 'opened', label: 'Opened' }
]


const emptyCopy = {
  all: [
    'A little distance. A different perspective.',
    'Choose a memory and a future date to prepare your first capsule plan.',
  ],
  planned: [
    'No capsule plans yet.',
    'Choose a memory and a future date to prepare a plan.',
  ],
  sealed: [
    'No sealed capsule yet.',
    'Plans stay unsealed until account saving and sealing are connected.',
  ],
  ready: [
    'Nothing waiting to be opened.',
    'A planned date alone does not make a capsule ready. Sealing and opening will be handled through your account.',
  ],
  opened: [
    'No opened capsules yet.',
    'Memories you explicitly open from sealed capsules will appear here.',
  ],
}

export default function CapsulePage({plans}) {
  const [filter, setFilter] = useState('all')
  const [notice, setNotice] = useState('')

  const visible = plans.filter(
    (plan) => filter === 'all' || plan.state === filter,
  )

  function exportPlan(plan) {
    const text = [
      'Capsule plan',
      `Return date: ${formatSchedule(plan)}`,
      `Time zone: ${plan.timeZone}`,
      `UTC : ${plan.scheduleAt}`,
      'Status: Planned, unsealed',
      '',
      'This plan does not include the memory text.',
      'Download your writing separately.',
      '',
    ].join('\n')

    setNotice(
      downloadText('capsule-plan.txt', text)
        ? 'Download requested. Check your downloads for capsule-plan.txt'
        : 'The download could not start. Copy the return date before closing this tab.',
    )
  }

  return (
    <>
      <section className='page-heading' aria-labelledby='capsule-heading'>
        <div>
          <h1 id='capsule-heading'>For another day.</h1>
          <p>Some work need a little distance. Give them time.</p>
        </div>

        <Link className='write-action' to ='/capsules/new'>
          <Icon name='clock' />
          Plan a capsule
        </Link>
      </section>

      <div 
        className='filter-bar'
        role='group'
        aria-label='Filter capsules'
      >
        {filters.map(({value, label}) => (
          <button 
            className='filter-button'
            key={value}
            aria-pressed={filter === value}
            onClick={() => setFilter(value)}
          >
            {label}
          </button>
        ))}
      </div>

      <p className='results-note' roles='status'>
        {visible.length}{''}
        {filter === 'all' || filter === 'planned'
          ? 'capsule plans'
          : 'capsules'}{''}  
        in this tab
      </p> 

      {visible.length ? (
        visible.map((plan) => (
          <article className='capsule capsule-on-page' key={plan.id}>
            <div className='capsule-heading'>
              <Icon name="clock"/>
              <h2>A day to return.</h2>
              <p className='sealed-note'>Planned · Unsealed</p>
            </div>

            <div className='capsule-detail'>
              <p>Your writing is still available in Memories.</p>

              <div className='capsule-date'>
                Return on
                <time dateTime='{plan.scheduledAt}'>
                  {formatSchedule(plan)}
                </time>
              </div>

              <button 
                className='text-link capsule-link text-download'
                onClick={() => exportPlan(plan)}
              >
                Download plan
                <Icon name='arrow'/>
              </button> 
            </div>
          </article>
        ))
      ) : (
        <section className='empty-state'>
          <h2>{emptyCopy[filter][0]}</h2>
          <p>{emptyCopy[filter][1]}</p>
          <Link className='secondary-button' to='/capsules/new'>
            Plan a capsule
          </Link>
        </section>
      )}

      <p className='editor-status' role='status'>
        {notice}
      </p>

      <div className='capsule-explainer'>
        <span className='brass-rule' />
        <h2>A plan now. A return later.</h2>

        <p>
          Planning chooses the memory and return date. Sealing, 
          reminders, and explicit opening will become available
          with account saving.
        </p>
      </div>
    </>
  )
}
