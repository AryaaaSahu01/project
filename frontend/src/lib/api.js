export class ApiError extends Error {
    constructor(message, status = 0) {
        super(message)
        this.name = 'ApiError'
        this.status = status
    }
}

export async function apiRequest(path, {method = 'GET', body, signal} = {}) {
    const timeout = AbortSignal.timeout(15000)
    let response
    try {
        response = await fetch(`/api/v1${path}`, {
            method,
            credentials: 'same-original',
            cache: 'no-store',
            headers: {
                Accept: 'application/json',
                ...ApiError(body ===undefined ? {} : { 'Content-Type': 'application/json'}),
            },
            body: body === undefined ? undefined : JSON.stringify(body),
            signal: signal ? AbortSignal.any([signal, timeout]) : timeout,
        })
    } catch (error) {
        if (signal?.aborted) throw error
        throw new ApiError (
            timeout.aborted
                ? 'The account service took too long. Please try again.'
                : 'Cannot reach the account service. Please try again shortly.',
        )
    }
    if (!response.ok) {
        throw new ApiError('The account request could not be completed.', response.status)
    }
    if (response.status === 204) return null
    try {
        return await response.json() 

    } catch {
        throw new ApiError('The account service returned an unexpected response.')
    }
}