/**
 * API client for the broker backend.
 * Uses: POST /publish, GET /peek, GET /consume, POST /ack
 */

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080'
const ER_TOPIC = 'er-queue'

async function request(path, options = {}) {
  const url = `${API_BASE}${path}`
  const res = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `HTTP ${res.status}`)
  }
  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return null
  }
  return res.json()
}

/** Encode JSON to base64 for broker body ([]byte) */
function encodeBody(obj) {
  return btoa(unescape(encodeURIComponent(JSON.stringify(obj))))
}

/** Decode broker body (base64) to object */
function decodeBody(base64) {
  if (!base64) return null
  try {
    return JSON.parse(decodeURIComponent(escape(atob(base64))))
  } catch {
    return null
  }
}

/**
 * Register a patient (publish to er-queue).
 * @param {number} urgency - 1 = most urgent, 5 = least
 * @returns {{ patientId: string }}
 */
export async function registerPatient(urgency) {
  const patientId = `P-${Date.now().toString(36).toUpperCase()}`
  await request('/publish', {
    method: 'POST',
    body: JSON.stringify({
      topic: ER_TOPIC,
      body: encodeBody({ patientId, urgency }),
    }),
  })
  return { patientId }
}

/**
 * Peek at the next N patients in the queue (does not remove).
 * @param {number} limit - max entries (default 5)
 * @returns {{ queue: Array<{ id: string, patientId: string, urgency: number }> }}
 */
export async function getNext(limit = 5) {
  const res = await request(`/peek?topic=${encodeURIComponent(ER_TOPIC)}&limit=${limit}`)
  const queue = (res?.queue ?? []).map((m) => {
    const data = decodeBody(m.body)
    return data ? { id: m.id, ...data } : null
  }).filter(Boolean)
  return { queue }
}

/**
 * Consume the first message (for "Call next").
 * @returns {{ id: string, patientId: string, urgency: number } | null}
 */
export async function consumeNext() {
  try {
    const msg = await request(`/consume?topic=${encodeURIComponent(ER_TOPIC)}`)
    if (!msg) return null
    const data = decodeBody(msg.body)
    return data ? { id: msg.id, ...data } : null
  } catch (err) {
    if (err.message?.includes('404') || err.message?.includes('no message')) {
      return null
    }
    throw err
  }
}

/**
 * Acknowledge a consumed message.
 * @param {string} id - message ID from consume
 */
export async function ackMessage(id) {
  await request('/ack', {
    method: 'POST',
    body: JSON.stringify({ id }),
  })
}

/**
 * Call next patient: consume the first message, then ack it.
 * @returns {boolean} true if a patient was called
 */
export async function callPatient() {
  const msg = await consumeNext()
  if (!msg) return false
  await ackMessage(msg.id)
  return true
}

/**
 * Get broker metrics (queue depth, pending, produced, consumed, DLQ count).
 * @returns {Promise<Object>}
 */
export async function getMetrics() {
  return request('/metrics')
}

/**
 * Get dead letter queue count.
 * @returns {Promise<{ count: number }>}
 */
export async function getDeadLetterCount() {
  return request('/dead-letter/count')
}

/**
 * Retry all messages from dead letter queue.
 * @returns {Promise<{ retried: number, failed: number }>}
 */
export async function deadLetterRetry() {
  return request('/dead-letter/retry', { method: 'POST' })
}

/**
 * Get activity log entries.
 * @param {Object} opts - { limit, offset, event_type }
 * @returns {Promise<{ entries: Array }>}
 */
export async function getActivityLog(opts = {}) {
  const params = new URLSearchParams()
  if (opts.limit) params.set('limit', opts.limit)
  if (opts.offset) params.set('offset', opts.offset)
  if (opts.event_type) params.set('event_type', opts.event_type)
  const qs = params.toString()
  return request(`/activity-log${qs ? '?' + qs : ''}`)
}
