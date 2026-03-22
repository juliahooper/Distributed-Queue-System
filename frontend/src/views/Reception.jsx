import { useState } from 'react'
import { registerPatient } from '../api'

const URGENCY_META = {
  1: { label: 'Critical',  color: '#dc2626', bg: '#fef2f2', border: '#fca5a5', tag: 'IMMEDIATE' },
  2: { label: 'Urgent',    color: '#ea580c', bg: '#fff7ed', border: '#fdba74', tag: 'HIGH PRIORITY' },
  3: { label: 'Moderate',  color: '#eab308', bg: '#fefce8', border: '#fde047', tag: 'STANDARD' },
  4: { label: 'Minor',     color: '#22c55e', bg: '#f0fdf4', border: '#86efac', tag: 'NON-URGENT' },
  5: { label: 'Low',       color: '#2563eb', bg: '#eff6ff', border: '#93c5fd', tag: 'ROUTINE' },
}

export default function Reception() {
  const [urgency, setUrgency] = useState(3)
  const [status, setStatus] = useState(null)
  const [loading, setLoading] = useState(false)

  const meta = URGENCY_META[urgency]

  async function handleSubmit(e) {
    e.preventDefault()
    setStatus(null)
    setLoading(true)
    try {
      const { patientId } = await registerPatient(urgency)
      setStatus({ type: 'success', message: `✅ Patient ${patientId} registered successfully.` })
    } catch (err) {
      setStatus({ type: 'error', message: `❌ ${err.message || 'Failed to register.'}` })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ maxWidth: 700, position: 'relative' }}>
      {/* Page header */}
      <div style={{
        marginBottom: '1.75rem',
        padding: '2rem 2.5rem 1.75rem',
        borderRadius: 16,
        background: 'linear-gradient(135deg, #111827 0%, #1f2937 100%)',
        boxShadow: '0 0 60px rgba(220,38,38,0.35), 0 0 120px rgba(220,38,38,0.15)',
      }}>
        <h1 style={{
          margin: 0,
          fontSize: '2.8rem',
          color: '#fff',
          fontFamily: "'Bebas Neue', sans-serif",
          letterSpacing: '0.06em',
          textShadow: '0 0 30px rgba(255,255,255,0.25)',
        }}>
          Patient Registration
        </h1>
        <p style={{
          margin: '0.5rem 0 0',
          color: 'rgba(255,255,255,0.55)',
          fontSize: '0.95rem',
          fontFamily: "'Inter', sans-serif",
          letterSpacing: '0.02em',
        }}>
          Register a new patient and assign an urgency level.
        </p>
      </div>

      <div style={{
        background: '#111827',
        borderRadius: 16,
        padding: '2.5rem',
        boxShadow: '0 0 60px rgba(220,38,38,0.35), 0 0 120px rgba(220,38,38,0.15)',
        border: '1px solid rgba(220,38,38,0.15)',
      }}>
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>

          {/* Urgency selector */}
          <div>
            <label style={{ display: 'block', fontWeight: 800, marginBottom: '1rem', color: '#fff', fontFamily: "'Bebas Neue', sans-serif", fontSize: '1.5rem', letterSpacing: '0.04em' }}>
              Urgency Level
            </label>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
              {[1, 2, 3, 4, 5].map((n) => {
                const m = URGENCY_META[n]
                const selected = urgency === n
                return (
                  <label
                    key={n}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '0.75rem',
                      padding: '1.1rem 1.5rem',
                      borderRadius: 10,
                      border: `1px solid ${m.color}55`,
                      background: selected
                        ? `linear-gradient(180deg, rgba(255,255,255,0.25) 0%, rgba(255,255,255,0) 55%), ${m.color}`
                        : `linear-gradient(180deg, rgba(255,255,255,0.85) 0%, rgba(255,255,255,0.25) 100%), ${m.color}`,
                      cursor: 'pointer',
                      transition: 'all 0.15s',
                      boxShadow: selected ? `0 4px 18px ${m.color}88` : `0 2px 10px ${m.color}55`,
                      transform: selected ? 'translateY(1px)' : 'translateY(0)',
                    }}
                  >
                    <input
                      type="radio"
                      name="urgency"
                      value={n}
                      checked={selected}
                      onChange={() => setUrgency(n)}
                      style={{ display: 'none' }}
                    />
                    <span style={{ fontWeight: 800, color: '#1e293b', fontFamily: "'Bebas Neue', sans-serif", fontSize: '1.4rem', letterSpacing: '0.04em' }}>
                      {m.label}
                    </span>
                    <span style={{
                      marginLeft: 'auto',
                      fontSize: '0.8rem',
                      background: selected ? 'rgba(255,255,255,0.25)' : `${m.color}22`,
                      color: selected ? '#fff' : m.color,
                      padding: '0.2rem 0.7rem',
                      borderRadius: 99,
                      fontWeight: 700,
                      whiteSpace: 'nowrap',
                      border: selected ? '1px solid rgba(255,255,255,0.4)' : `1px solid ${m.color}44`,
                    }}>{m.tag}</span>
                  </label>
                )
              })}
            </div>
          </div>

          {/* Submit */}
          <button
            type="submit"
            disabled={loading}
            style={{
              marginTop: '0.5rem',
              padding: '1rem 1.25rem',
              fontSize: '1.4rem',
              fontWeight: 800,
              fontFamily: "'Bebas Neue', sans-serif",
              letterSpacing: '0.06em',
              border: `1px solid #dc262655`,
              borderRadius: 10,
              background: loading
                ? `linear-gradient(180deg, rgba(255,255,255,0.25) 0%, rgba(255,255,255,0) 55%), #dc2626`
                : `linear-gradient(180deg, rgba(255,255,255,0.85) 0%, rgba(255,255,255,0.25) 100%), #dc2626`,
              color: loading ? '#fff' : '#1e293b',
              cursor: loading ? 'not-allowed' : 'pointer',
              transition: 'all 0.15s',
              boxShadow: loading ? '0 4px 18px #dc262688' : '0 2px 10px #dc262655',
            }}
          >
            {loading ? 'Registering…' : 'Register Patient'}
          </button>
        </form>

        {status && (
          <div
            className="card-enter"
            style={{
              marginTop: '1rem',
              padding: '0.75rem 1rem',
              borderRadius: 8,
              background: status.type === 'success' ? '#f0fdf4' : '#fef2f2',
              border: `1px solid ${status.type === 'success' ? '#bbf7d0' : '#fecaca'}`,
              color: status.type === 'success' ? '#166534' : '#991b1b',
              fontWeight: 500,
            }}
          >
            {status.message}
          </div>
        )}
      </div>

    </div>
  )
}
