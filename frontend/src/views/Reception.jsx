import { useState } from 'react'
import { registerPatient } from '../api'

const URGENCY_META = {
  1: { label: 'Critical',  color: '#dc2626', bg: '#fef2f2', emoji: '🔴' },
  2: { label: 'Urgent',    color: '#ea580c', bg: '#fff7ed', emoji: '🟠' },
  3: { label: 'Moderate',  color: '#d97706', bg: '#fffbeb', emoji: '🟡' },
  4: { label: 'Minor',     color: '#65a30d', bg: '#f7fee7', emoji: '🟢' },
  5: { label: 'Low',       color: '#16a34a', bg: '#f0fdf4', emoji: '🟢' },
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
    <div style={{ maxWidth: 480 }}>
      {/* Page header */}
      <div style={{ marginBottom: '1.75rem' }}>
        <h1 style={{ margin: 0, fontSize: '1.6rem', fontWeight: 700, color: '#1e293b' }}>
          📋 Patient Registration
        </h1>
        <p style={{ margin: '0.4rem 0 0', color: '#64748b', fontSize: '0.95rem' }}>
          Register a new patient and assign an urgency level.
        </p>
      </div>

      <div style={{
        background: '#fff',
        borderRadius: 12,
        padding: '1.75rem',
        boxShadow: '0 2px 12px rgba(0,0,0,0.08)',
        border: '1px solid #e2e8f0',
      }}>
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>

          {/* Urgency selector */}
          <div>
            <label style={{ display: 'block', fontWeight: 600, marginBottom: '0.5rem', color: '#374151' }}>
              Urgency Level
            </label>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
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
                      padding: '0.65rem 1rem',
                      borderRadius: 8,
                      border: `2px solid ${selected ? m.color : '#e2e8f0'}`,
                      background: selected ? m.bg : '#fafafa',
                      cursor: 'pointer',
                      transition: 'all 0.15s',
                      boxShadow: selected ? `0 0 0 3px ${m.color}22` : 'none',
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
                    <span style={{ fontSize: '1.1rem' }}>{m.emoji}</span>
                    <span style={{ fontWeight: selected ? 700 : 400, color: selected ? m.color : '#374151' }}>
                      {n} — {m.label}
                    </span>
                    {n === 1 && (
                      <span style={{
                        marginLeft: 'auto',
                        fontSize: '0.7rem',
                        background: '#dc2626',
                        color: '#fff',
                        padding: '0.1rem 0.5rem',
                        borderRadius: 99,
                        fontWeight: 700,
                      }}>IMMEDIATE</span>
                    )}
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
              padding: '0.75rem 1.25rem',
              fontSize: '1rem',
              fontWeight: 700,
              border: 'none',
              borderRadius: 8,
              background: loading ? '#fca5a5' : 'linear-gradient(135deg, #dc2626, #991b1b)',
              color: '#fff',
              cursor: loading ? 'not-allowed' : 'pointer',
              boxShadow: loading ? 'none' : '0 4px 14px rgba(220,38,38,0.4)',
              transition: 'all 0.15s',
              letterSpacing: '0.02em',
            }}
          >
            {loading ? '⏳ Registering…' : '➕ Register Patient'}
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

      {/* Info box */}
      <div style={{
        marginTop: '1.25rem',
        padding: '1rem',
        borderRadius: 10,
        background: '#fff',
        border: '1px solid #e2e8f0',
        boxShadow: '0 1px 4px rgba(0,0,0,0.05)',
      }}>
        <div style={{ fontWeight: 600, color: '#374151', marginBottom: '0.5rem', fontSize: '0.85rem' }}>
          ℹ️ Urgency Guide
        </div>
        {Object.entries(URGENCY_META).map(([n, m]) => (
          <div key={n} style={{ display: 'flex', gap: '0.5rem', fontSize: '0.82rem', color: '#64748b', marginBottom: '0.2rem' }}>
            <span>{m.emoji}</span>
            <span style={{ color: m.color, fontWeight: 600 }}>{n} — {m.label}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
