import { ApiError } from './api.js'

export const mediaAccept = '.jpg,.jpeg,.png,.webp,.mp3,.wav,.ogg,.m4a,.mp4,.webm,audio/webm'
export function mediaContentURL(memoryID, assetID) {
  return `/api/v1/memories/${encodeURIComponent(memoryID)}/media/${encodeURIComponent(assetID)}/content`
}
export function formatBytes(bytes) {
  return bytes < 1048576 ? `${Math.ceil(bytes / 1024)} KB` : `${(bytes / 1048576).toFixed(1)} MB`
}
export function fileType(file) {
  const type = file.type.split(';')[0].toLowerCase()
  const aliases = { 'audio/x-m4a': 'audio/mp4', 'audio/x-wav': 'audio/wav', 'audio/wave': 'audio/wav', 'audio/mp3': 'audio/mpeg' }
  if (type && type !== 'application/octet-stream') return aliases[type] || type
  return { jpg: 'image/jpeg', jpeg: 'image/jpeg', png: 'image/png', webp: 'image/webp', mp3: 'audio/mpeg', wav: 'audio/wav', ogg: 'audio/ogg', m4a: 'audio/mp4', mp4: 'video/mp4', webm: 'video/webm' }[file.name.split('.').at(-1).toLowerCase()] || ''
}
export function checkFile(file, config) {
  const rule = config?.formats[fileType(file)]
  if (!rule) return 'Choose JPG, PNG, WebP, MP3, WAV, OGG, M4A, MP4 or WebM.'
  if (!file.size) return 'This file is empty. Choose another file.'
  if (file.size > rule.max_bytes) return `${file.name} is too large. The ${rule.kind} limit is ${formatBytes(rule.max_bytes)}.`
  return ''
}

// Upload the binary with XHR for real byte progress. Metadata calls still use
// apiRequest. No storage credentials, public bucket or signed bearer URL reaches JS.
export function uploadMedia(memoryID, file, { source = 'upload', id = crypto.randomUUID(), signal, onProgress } = {}) {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) { reject(new DOMException('Upload cancelled', 'AbortError')); return }
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `/api/v1/memories/${encodeURIComponent(memoryID)}/media`)
    xhr.timeout = 240000
    xhr.setRequestHeader('Content-Type', fileType(file))
    xhr.setRequestHeader('X-Upload-Name', encodeURIComponent(file.name))
    xhr.setRequestHeader('X-Upload-Size', String(file.size))
    xhr.setRequestHeader('X-Upload-ID', id)
    xhr.setRequestHeader('X-Media-Source', source)
    const abort = () => xhr.abort()
    const cleanup = () => signal?.removeEventListener('abort', abort)
    signal?.addEventListener('abort', abort, { once: true })
    xhr.upload.onprogress = (event) => { if (event.lengthComputable) onProgress?.(Math.round(event.loaded / event.total * 100)) }
    xhr.onload = () => {
      cleanup()
      let value
      try { value = JSON.parse(xhr.responseText) } catch { reject(new ApiError('Unexpected upload response. Refresh attachments before trying again.', xhr.status)); return }
      if (xhr.status >= 200 && xhr.status < 300) resolve(value)
      else reject(new ApiError(value.error || 'Upload failed. Try again.', xhr.status))
    }
    xhr.onerror = () => { cleanup(); reject(new ApiError('Connection lost. Refresh attachments before retrying; your saved memory is safe.')) }
    xhr.ontimeout = () => { cleanup(); reject(new ApiError('Upload timed out. Refresh attachments and try a smaller file.')) }
    xhr.onabort = () => { cleanup(); reject(new DOMException('Upload cancelled', 'AbortError')) }
    xhr.send(file)
  })
}
