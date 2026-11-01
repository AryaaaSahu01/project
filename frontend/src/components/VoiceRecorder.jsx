import { useEffect, useRef, useState } from 'react'
import { formatBytes } from '../lib/media.js'

const maxMilliseconds = 5 * 60 * 1000
const maxBytes = 25 * 1024 * 1024
const clock = (milliseconds) => `${Math.floor(milliseconds / 60000)}:${String(Math.floor(milliseconds / 1000) % 60).padStart(2, '0')}`

export default function VoiceRecorder({ disabled, beginMedia, onUpload }) {
  const [phase, setPhase] = useState('idle')
  const [elapsed, setElapsed] = useState(0)
  const [error, setError] = useState('')
  const [preview, setPreview] = useState(null)
  const resource = useRef({ active: true })
  useEffect(() => {
    resource.current.active = true
    const state = resource.current
    return () => {
      state.active = false
      clearInterval(state.timer)
      if (state.recorder?.state === 'recording') state.recorder.stop()
      state.stream?.getTracks().forEach((track) => track.stop())
      if (state.url) URL.revokeObjectURL(state.url)
      state.release?.()
    }
  }, [])

  async function start() {
    const state = resource.current
    if (phase !== 'idle') return
    setError('')
    if (!navigator.mediaDevices?.getUserMedia || !window.MediaRecorder) {
      setError('Recording needs a supported browser on localhost or HTTPS. You can upload an audio file instead.')
      return
    }
    state.release = beginMedia()
    setPhase('permission')
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      if (!state.active) { stream.getTracks().forEach((track) => track.stop()); state.release?.(); return }
      state.stream = stream
      const type = ['audio/webm;codecs=opus', 'audio/ogg;codecs=opus', 'audio/mp4'].find((value) => MediaRecorder.isTypeSupported(value))
      if (!type) throw new Error('Your browser cannot record a supported format. Upload an audio file instead.')
      const recorder = new MediaRecorder(stream, { mimeType: type, audioBitsPerSecond: 96000 })
      state.recorder = recorder
      const chunks = []
      let bytes = 0
      let failed = false
      recorder.ondataavailable = (event) => {
        if (!event.data.size) return
        chunks.push(event.data); bytes += event.data.size
        if (bytes >= maxBytes && recorder.state === 'recording') recorder.stop()
      }
      recorder.onerror = () => {
        failed = true
        clearInterval(state.timer)
        stream.getTracks().forEach((track) => track.stop())
        state.release?.()
        if (state.active) { setPhase('idle'); setError('Recording stopped unexpectedly. Please try again.') }
      }
      recorder.onstop = () => {
        clearInterval(state.timer)
        stream.getTracks().forEach((track) => track.stop())
        if (!state.active || failed) return
        const blob = new Blob(chunks, { type: type.split(';')[0] })
        if (!blob.size || blob.size > maxBytes) {
          state.release?.(); setPhase('idle'); setError('This recording could not be kept. Try a shorter voice note.'); return
        }
        const extension = type.includes('webm') ? 'webm' : type.includes('ogg') ? 'ogg' : 'm4a'
        const date = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' }).format(new Date()).replaceAll('/', '-').replaceAll(':', '-')
        const file = new File([blob], `Voice note ${date}.${extension}`, { type: blob.type })
        state.url = URL.createObjectURL(file)
        setPreview({ file, url: state.url }); setPhase('preview')
      }
      const started = performance.now()
      setElapsed(0)
      recorder.start(1000)
      setPhase('recording')
      state.timer = setInterval(() => {
        const duration = performance.now() - started
        if (state.active) setElapsed(Math.floor(Math.min(duration, maxMilliseconds) / 1000) * 1000)
        if (duration >= maxMilliseconds && recorder.state === 'recording') recorder.stop()
      }, 250)
    } catch (failure) {
      state.stream?.getTracks().forEach((track) => track.stop())
      state.release?.()
      if (state.active) {
        setPhase('idle')
        setError(failure.name === 'NotAllowedError' ? 'Microphone access was denied. Allow access in your browser settings, or upload an audio file.' : failure.name === 'NotFoundError' ? 'No microphone was found. Connect one or upload an audio file.' : failure.message)
      }
    }
  }

  function discard() {
    const state = resource.current
    if (state.url) URL.revokeObjectURL(state.url)
    state.url = null; state.release?.()
    setPreview(null); setPhase('idle'); setElapsed(0)
  }
  async function keep() {
    setPhase('uploading')
    const saved = await onUpload(preview.file, 'voice')
    if (!resource.current.active) return
    if (saved) discard()
    else setPhase('preview')
  }
  return <div className="voice-recorder">
    {phase === 'idle' && <button type="button" className="secondary-button" disabled={disabled} onClick={start}>Record a voice note</button>}
    {phase === 'permission' && <p role="status">Allow microphone access in your browser to start recording.</p>}
    {phase === 'recording' && <div className="voice-session"><p role="status">Recording <span className="recording-time">{clock(elapsed)}</span> / 5:00</p><button type="button" className="secondary-button" onClick={() => { const recorder = resource.current.recorder; if (recorder?.state === 'recording') recorder.stop() }}>Stop recording</button></div>}
    {preview && <div className="voice-preview"><p>Preview your voice note · {formatBytes(preview.file.size)}</p><audio controls src={preview.url} aria-label="Preview your unsaved voice note" /><p className="field-hint">This recording has not been uploaded yet. Leaving this page discards it.</p><div className="confirmation-actions"><button type="button" className="write-action" disabled={phase === 'uploading'} onClick={keep}>{phase === 'uploading' ? 'Uploading voice note…' : 'Save voice note'}</button><button type="button" className="text-button" disabled={phase === 'uploading'} onClick={discard}>Discard recording</button></div></div>}
    {error && <p className="form-error" role="alert">{error}</p>}
  </div>
}
