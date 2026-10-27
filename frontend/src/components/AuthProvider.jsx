import { useEffect, useRef, useState, useCallback } from 'react'
import { AuthContext } from '../lib/auth.js'
import { ApiError, apiRequest } from '../lib/api.js'

export default function AuthProvider({ children }) {
    const [session, setSession] = useState({
        status: 'checking', account: null, error: '',
    })
    const sequence = useRef(0)
    const busy = useRef(false)
    const channel = useRef(null)

    const refreshSession = useCallback(async (signal) => {
        if (busy.current) return 
        const current = ++sequence.current
        try {
            const data = await apiRequest('/browser/session', {signal})
            if (current !== sequence.current || signal?.aborted) return
            setSession({
                status: data.account ? 'signed-in' : 'signed-out',
                account: data.account, error: '',
            })
        } catch (error) {
          if (current !== sequence.current || signal?.aborted) return 
          setSession({ status: 'unavailable', account: null, error: error.message })
        }
    }, [])

    useEffect(() => {
        const controller = new AbortController()
        const current = ++sequence.current
        apiRequest('/browser/session', { signal: controller.signal })
            .then((data) => {
                if(current !== sequence.current || controller.signal.aborted) return 
                setSession({status: data.account ? 'signed-in' : 'signed-out', account: data.account, error: ''})
            })
            .catch((error) => {
                if (current !== sequence.current || controller.signal.aborted) return 
                setSession({status: 'unavailable', account: null, error: error.message})
            })
        const checkOnFocus = () => refreshSession(controller.signal)
        window.addEventListener('focus', checkOnFocus)
        if ('BroadcastChannel' in window) {
            channel.current = new BroadcastChannel('chrono-account')
            channel.current.onmessage = checkOnFocus
        }
        return () => {
            controller.abort()
            window.removeEventListener('focus', checkOnFocus)
            channel.current?.close()
            channel.current = null
        }
    },[refreshSession])


    async function authenticate(mode,input,signal) {
        busy.current = true
        ++sequence.current
        try {
            const result = await apiRequest(`/browser/auth/${mode}`,{
                method:"POST", body: input, signal,
            })
            //verify the cookie was accepted before showing a signed-in account
            const verified = await apiRequest('/browser/session', {signal})
            if (!verified.account || verified.account.user.id !== result.account?.user?.id) {
                throw new ApiError('Your browser could not keep the sign-in cookie. Allow cookies for this site and try signing in again.')
            }
            setSession({ status: 'signed-in', account: verified.account, error: ''})
            channel.current?.postMessage('session-changed')
        } finally {
            busy.current = false    
        }       
    }

    async function signOut() {
        busy.current = true
        ++sequence.current
        try {
            await apiRequest('/browser/auth/logout', { method: 'POST'})
            setSession({ status: 'signed-out', account: null, error: ''})
            channel.current?.postMessage('session-changed')
        } finally {
            busy.current = false
        }
    }

    return (
        <AuthContext.Provider value={{ ...session, authenticate, signOut, refreshSession}}>
            {children}
        </AuthContext.Provider>
    )
}