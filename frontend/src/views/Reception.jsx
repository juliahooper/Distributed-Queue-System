import { useState } from 'react'
import { registerPatient } from '../api'

export default function Reception() {
  const [urgency, setUrgency] = useState(3)
  const [status, setStatus] = useState(null) // { type: 'success'|'error', message }

  async function handleSubmit(e) {
    e.preventDefault()
    setStatus(null)
    try {
      const res = await registerPatient(urgency)
      setStatus({ type: 'success', message: `Patient ${res.patientId} registered.` })
    } catch (err) {
      setStatus({ type: 'error', message: err.message || 'Failed to register.' })
    }
  }

  return (
    <div style={{ maxWidth: 420 }}>
      <h1 style={{ marginTop: 0, marginBottom: '1.5rem' }}>Register patient</h1>
      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
        <label style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
          Urgency (1 = most urgent)
          <select
            value={urgency}
            onChange={(e) => setUrgency(Number(e.target.value))}
            style={{
              padding: '0.5rem 0.75rem',
              fontSize: '1rem',
              border: '1px solid #475569',
              borderRadius: 6,
              background: '#1e293b',
              color: '#f8fafc',
            }}
          >
            {[1, 2, 3, 4, 5].map((n) => (
              <option key={n} value={n}>{n}</option>
            ))}
          </select>
        </label>
        <button
          type="submit"
          style={{
            padding: '0.6rem 1rem',
            fontSize: '1rem',
            fontWeight: 600,
            border: 'none',
            borderRadius: 6,
            background: '#0ea5e9',
            color: '#fff',
            cursor: 'pointer',
          }}
        >
          Register
        </button>
      </form>
      {status && (
        <p
          style={{
            marginTop: '1rem',
            color: status.type === 'success' ? '#86efac' : '#fca5a5',
          }}
        >
          {status.message}
        </p>
      )}
    </div>
  )
}
