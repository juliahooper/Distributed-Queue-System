/**
 * API client for the ER queue backend.
 * Endpoints: POST /er/register, GET /er/next, POST /er/call
 */

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080'

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

/**
 * Register a patient in the ER queue.
 * @param {number} urgency - 1 = most urgent, 5 = least
 * @returns {{ patientId: string }}
 */
export async function registerPatient(urgency) {
  const res = await request('/er/register', {
    method: 'POST',
    body: JSON.stringify({ urgency: Number(urgency) }),
  })
  return res
}

/**
 * Peek at the next N patients in the queue (does not remove them).
 * @param {number} limit - max entries to return (default 5)
 * @returns {{ queue: Array<{ id: string, patientId: string, urgency: number }> }}
 */
export async function getNext(limit = 5) {
  return request(`/er/next?limit=${limit}`)
}

/**
 * Call (remove) a patient from the queue by id.
 * @param {string} id - patient id from getNext
 */
export async function callPatient(id) {
  await request('/er/call', {
    method: 'POST',
    body: JSON.stringify({ id }),
  })
}
