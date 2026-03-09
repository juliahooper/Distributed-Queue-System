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

export async function registerPatient(urgency) {
  return request('/er/register', {
    method: 'POST',
    body: JSON.stringify({ urgency: Number(urgency) }),
  })
}

export async function getNext(limit = 6) {
  return request(`/er/next?limit=${limit}`)
}

export async function callPatient(id) {
  return request('/er/call', {
    method: 'POST',
    body: JSON.stringify({ id }),
  })
}
