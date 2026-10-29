import { useEffect, useRef, useState } from "react"
import { Link, Navigate, useLocation, useNavigate } from "react-router"
import { useAuth } from "../lib/auth.js"
import { APP_CONFIG } from "../config/app.js"
import AppearanceSettings from "../components/AppearanceSettings.jsx"

const byteLength = (value) => new TextEncoder().encode(value).length

function validate(values, signingUp) {
    const errors = {}
    const email = values.email.trim()

    if (signingUp && !values.displayName.trim()) {
        errors.displayName = 'Enter the name you would like to use.'
    } else if (signingUp && Array.from(values.displayName.trim()).length > 100) {
        errors.displayName = 'Use a name with 100 characters or fewer.'
    }

    if (!email || !/^[^\s@]+@[^\s@]+$/.test(email) || byteLength(email) > 254 ) {
        errors.email = 'Enter a valid email address'
    }

    if (!values.password) {
        errors.password = 'Enter your password'
    } else if (signingUp && Array.from(values.password).length < 12) {
        errors.password = 'Use at least 12 characters for your password.'
    } else if (byteLength(values.password) > 72) {
        errors.password = 'This password is too long. Use fewer characters; emoji and accented characters can reach the limit sooner.'
    } 
    return errors
}

function requestError(error, signingUp) {
    if (error.status === 409 && signingUp) return 'This email already has an account. Sign in instead.'
    if (error.status === 401) return 'This email or password is incorrect. Check both and try again.'
    if (error.status === 400) return 'Check your name, email, and password, then try again.'
    if (error.status === 403) return 'This request could not be verified. Reload the page and try again.'
    if (error.status === 429) return 'Too many attempts. Please wait a little before trying again.'
    if (error.status >= 500) return 'The account service is unavailable. Please try again shortly.'
    return error.message
}

export default function AuthPage({mode}) {
    const signingUp = mode ==='register'
    const {status, authenticate} = useAuth()
    const navigate = useNavigate()
    const location = useLocation()
    const requested = location.state?.from
    const destination = typeof requested === 'string' && requested.startsWith('/') && !requested.startsWith('//') ? requested : '/account'
    const [values,setValues] = useState({displayName:'', email:'', password:''})
    const [errors, setErrors] = useState({})
    const [error, setError] = useState('')
    const [submitting, setSubmitting] = useState(false)
    const [showPassword, setShowPassword] = useState(false)
    const formRef = useRef(null)
    const errorRef = useRef(null)
    const request = useRef(null)

    useEffect(() => () => request.current?.abort(), [])

    if (status === 'signed-in') return  <Navigate to={destination} replace />

    function change(event) {
        const {name, value} = event.target
        setValues((current) => ({ ...current, [name]: value}))
        setErrors((current) => ({ ...current, [name]: ''}))
        setError('')
    }

    function blur(event) {
        const name = event.target.name
        setErrors((current) => ({ ...current, [name]: validate(values, signingUp)[name] || '' }))
    }

    async function submit(event) {
        event.preventDefault()
        if (request.current) return 
        const nextErrors = validate(values, signingUp)
        setErrors(nextErrors)
        setError('')
        const first = Object.keys(nextErrors)[0]
        if (first) {
            formRef.current.elements.namedItem(first)?.focus()
            return
        }
        const controller = new AbortController()
        request.current = controller
        setSubmitting(true)
        try {
            await authenticate(mode, {
                email: values.email.trim(),
                password: values.password,
                ...(signingUp ? {display_name: values.displayName.trim() }: {}),
            }, controller.signal)
            navigate(destination, { replace: true})
        } catch (failure) {
          if (controller.signal.aborted) return
          setError(requestError(failure, signingUp))
          requestAnimationFrame(() => errorRef.current?.focus())
        } finally {
          if (!controller.signal.aborted) setSubmitting(false)
          request.current = null
        }
    }

    return (
        <section className="account-page" aria-labelledby="account-title">
            <div className="account-form-column">
                <h1 id="account-title">{signingUp ? 'Create your account': 'Welcome back.'}</h1>
                <p className="account-intro">
                    {signingUp ? 'A name, an email, and a place to begin.':`Sign in to your ${APP_CONFIG.name} account.`}
                </p>
                <form ref={formRef} className="account-form" onSubmit={submit} noValidate aria-busy={submitting}>
                    <p className="account-required">All fields are required.</p>
                    {error && <p className="form-error" role="alert" tabIndex={-1} ref={errorRef}>{error}</p>}
                    {signingUp && (
                        <div className="account-field">
                            <label htmlFor="display-name">Your name</label>
                            <input id="display-name" name="displayName" autoComplete="nickname" required
                                value={values.displayName} onChange={change} onBlur={blur} disabled={submitting}
                                aria-invalid={Boolean(errors.displayName)} aria-describedby={errors.displayName ? 'name-error' : undefined} />
                            {errors.displayName && <p className="account-field-error" id="name-error">{errors.displayName}</p>}
                        </div>
                    )}

                    <div className="account-field">
                        <label htmlFor="account-email">Email address</label>
                        <input id="account-email" name="email" type="email" autoComplete="username" inputMode="email"
                            autoCapitalize="none" spellCheck={false} required value={values.email} onChange={change} onBlur={blur}
                            disabled={submitting} aria-invalid={Boolean(errors.email)}
                            aria-describedby={errors.email ? 'email-error' : undefined} />
                        {errors.email && <p className="account-field-error" id="email-error">{errors.email}</p>}
                    </div>

                    <div className="account-field">
                        <label htmlFor="account-password">Password</label>
                        <div className="account-password">
                            <input id="account-password" name="password" type={showPassword ? 'text' : 'password'}
                                autoComplete={signingUp ? 'new-password' : 'current-password'} required value={values.password}
                                onChange={change} onBlur={blur} disabled={submitting} aria-invalid={Boolean(errors.password)}
                                aria-describedby={`${signingUp ? 'password-hint ' : ''}${errors.password ? 'password-error':''}`.trim() || undefined} />
                            <button type="button" onClick={() => setShowPassword((current) => !current)}
                                aria-pressed={showPassword} aria-label="Show password" disabled={submitting}>
                                {showPassword ? 'Hide' : 'Show'}
                            </button>
                        </div>
                        {signingUp && <p className="account-hint" id="password-hint">Use at least 12 characters. A few memorable words work well.</p>}
                        {errors.password && <p className="account-field-error" id="password-error">{errors.password}</p>}
                    </div>
                    <button className="write-action account-submit" type="submit" disabled={submitting || status === 'checking'}>
                        {submitting ? (signingUp ? 'Creating-account...':'Signing in...') : (signingUp ? 'Create account':'Sign in')}
                    </button>
                    <p className="account-progress" role="status">{status === 'checking' ? 'Checking your sign-in...': submitting ? 'Please wait a moment.':''}</p>
                </form>

                <p className="account-switch">
                    {signingUp ? 'Already have an account? ' : `New to ${APP_CONFIG.name}? `}
                    <Link to={signingUp ? '/sign-in':'/sign-up'} state={location.state}>{signingUp ? 'Sign in':'Create an account'}</Link>
                </p>
                <Link className="account-back" to="/today">Return to your journal</Link>
            </div>
            <aside className="account-aside" aria-labelledby="account-note-title">
                <h2 id="account-note-title">{APP_CONFIG.tagline}</h2>
                <p>There is no perfect first sentence. Start wherever you are.</p>
                <p className="account-storage-note">Save your writing to your private journal. Seal a memory for a future date, then  return to it when the time is right. </p>
                <AppearanceSettings/>
            </aside>
        </section>
    )
}