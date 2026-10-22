import { APP_CONFIG } from "./config/app";

export default function App() {
  return (
    <>
      <title>{`${APP_CONFIG.name} - Your journal`}</title>
      <meta name = "description" content={APP_CONFIG.description} />

      <a className="skip-link" href="#main-content">
        Skip to content
      </a>

      <div className="app-shell">
        <header className="site-header">
          <a className="brand" href="/" aria-label={`${APP_CONFIG.name} home`}>
          {APP_CONFIG.name}
          </a>

          <span className="preview-label">Design Preview</span>
        </header>

        <main id="main-content" tabIndex={-1}>
          <section className="introduction" aria-labelledby="page-heading">
            <h1 id="page-heading">
              Some things
              <br />
              deserve to <em>stay.</em>
            </h1>

            <p className="introduction-copy">
              A passing thought. An ordinary afternoon.Something you want 
              your future self to remember.
              <br/>
              <br/>
              Give it a place here.
            </p>
          </section>

          <section className="journal-section" aria-labelledby="journal-heading">
            <div className="section-heading">
              <h2 id="journal-heading"> Your journal </h2>
              <span>A little space for you</span>
            </div>

            <div className="journal-placeholder">
              <p className="placeholder-title">
                Every story starts somewhere.
              </p>

              <p>
                This is where your memories will live.
                We're shaping this space, one detail at a time.
              </p>
            </div>
          </section>
        </main>

        <footer className="site-footer">
          <span>{APP_CONFIG.name}</span>
          <span>Interface preview - No account data loaded</span>
        </footer>
      </div>
    </>
  )
}