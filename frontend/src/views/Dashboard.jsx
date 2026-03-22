import { useState, useEffect, useCallback } from 'react'
import { getNext, consumeNext, ackMessage } from '../api'

const POLL_INTERVAL_MS = 2000
const QUEUE_LIMIT = 5

const URGENCY_META = {
  1: { label: 'Critical',  color: '#dc2626', bg: '#fef2f2', border: '#fca5a5', emoji: '🔴' },
  2: { label: 'Urgent',    color: '#ea580c', bg: '#fff7ed', border: '#fdba74', emoji: '🟠' },
  3: { label: 'Moderate',  color: '#d97706', bg: '#fffbeb', border: '#fcd34d', emoji: '🟡' },
  4: { label: 'Minor',     color: '#65a30d', bg: '#f7fee7', border: '#bef264', emoji: '🟢' },
  5: { label: 'Low',       color: '#16a34a', bg: '#f0fdf4', border: '#86efac', emoji: '🟢' },
}

export default function Dashboard() {
  const [queue, setQueue] = useState([])
  const [error, setError] = useState(null)
  const [calling, setCalling] = useState(false)
  const [currentPatient, setCurrentPatient] = useState(null)
  const [finishing, setFinishing] = useState(false)

  const fetchQueue = useCallback(async () => {
    try {
      const res = await getNext(QUEUE_LIMIT)
      setQueue(res?.queue ?? [])
      setError(null)
    } catch (err) {
      setError(err.message || 'Failed to load queue.')
    }
  }, [])

  useEffect(() => {
    fetchQueue()
    const id = setInterval(fetchQueue, POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [fetchQueue])

  async function handleCallNext() {
    if (!queue[0] || calling || currentPatient) return
    setCalling(true)
    try {
      const patient = await consumeNext()
      await fetchQueue()
      if (!patient) {
        setError('⚠️ Patient already taken by another nurse.')
      } else {
        setCurrentPatient(patient)
        setError(null)
      }
    } catch (err) {
      setError(err.message || 'Failed to call patient.')
    } finally {
      setCalling(false)
    }
  }

  async function handleDone() {
    if (!currentPatient || finishing) return
    setFinishing(true)
    try {
      await ackMessage(currentPatient.id)
      setCurrentPatient(null)
      await fetchQueue()
      setError(null)
    } catch (err) {
      setError(err.message || 'Failed to complete patient.')
    } finally {
      setFinishing(false)
    }
  }

  const nextEntry = queue[0]
  const upNext = queue.slice(1, QUEUE_LIMIT)

  return (
    <div style={{ maxWidth: 620 }}>
      {/* Page header */}
      <div style={{ marginBottom: '1.75rem' }}>
        <h1 style={{ margin: 0, fontSize: '1.6rem', fontWeight: 700, color: '#1e293b' }}>
          👩‍⚕️ Nurse Station
        </h1>
        <p style={{ margin: '0.4rem 0 0', color: '#64748b', fontSize: '0.95rem' }}>
          Call patients from the queue and mark them as complete when done.
        </p>
      </div>

      {error && (
        <div className="card-enter" style={{
          marginBottom: '1rem',
          padding: '0.75rem 1rem',
          borderRadius: 8,
          background: '#fef2f2',
          border: '1px solid #fecaca',
          color: '#991b1b',
          fontWeight: 500,
        }}>
          {error}
        </div>
      )}

      {/* Currently seeing */}
      {currentPatient && (() => {
        const m = URGENCY_META[currentPatient.urgency] ?? URGENCY_META[3]
        return (
          <div className="card-enter pulse" style={{
            marginBottom: '1.5rem',
            padding: '1.25rem 1.5rem',
            borderRadius: 12,
            background: '#fff',
            border: `2px solid ${m.color}`,
            boxShadow: `0 0 20px ${m.color}33, 0 2px 8px rgba(0,0,0,0.08)`,
          }}>
            <div style={{ fontSize: '0.8rem', fontWeight: 700, color: m.color, textTransform: 'uppercase', letterSpacing: '0.08em', marginBottom: '0.4rem' }}>
              🩺 Currently Seeing
            </div>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div>
                <div style={{ fontSize: '1.5rem', fontWeight: 800, color: '#1e293b' }}>
                  {currentPatient.patientId}
                </div>
                <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', marginTop: '0.3rem' }}>
                  <span style={{
                    padding: '0.2rem 0.6rem',
                    borderRadius: 99,
                    background: m.bg,
                    color: m.color,
                    fontWeight: 700,
                    fontSize: '0.8rem',
                    border: `1px solid ${m.border}`,
                  }}>
                    {m.emoji} {m.label}
                  </span>
                  <span style={{ fontSize: '0.8rem', color: '#94a3b8' }}>Urgency {currentPatient.urgency}</span>
                </div>
              </div>
              <button
                onClick={handleDone}
                disabled={finishing}
                style={{
                  padding: '0.6rem 1.25rem',
                  fontWeight: 700,
                  border: 'none',
                  borderRadius: 8,
                  background: finishing ? '#e2e8f0' : 'linear-gradient(135deg, #16a34a, #15803d)',
                  color: '#fff',
                  cursor: finishing ? 'not-allowed' : 'pointer',
                  boxShadow: finishing ? 'none' : '0 4px 12px rgba(22,163,74,0.35)',
                  fontSize: '0.9rem',
                  transition: 'all 0.15s',
                }}
              >
                {finishing ? '⏳ Completing…' : '✅ Done with Patient'}
              </button>
            </div>
          </div>
        )
      })()}

      {/* Next in queue */}
      <div style={{
        background: '#fff',
        borderRadius: 12,
        padding: '1.25rem 1.5rem',
        boxShadow: '0 2px 12px rgba(0,0,0,0.07)',
        border: '1px solid #e2e8f0',
        marginBottom: '1.25rem',
      }}>
        <div style={{ fontSize: '0.8rem', fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.08em', marginBottom: '0.75rem' }}>
          🚨 Next Patient
        </div>

        {nextEntry ? (() => {
          const m = URGENCY_META[nextEntry.urgency] ?? URGENCY_META[3]
          return (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div>
                <div style={{ fontSize: '1.4rem', fontWeight: 800, color: '#1e293b' }}>
                  {nextEntry.patientId}
                </div>
                <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', marginTop: '0.3rem' }}>
                  <span style={{
                    padding: '0.2rem 0.6rem',
                    borderRadius: 99,
                    background: m.bg,
                    color: m.color,
                    fontWeight: 700,
                    fontSize: '0.8rem',
                    border: `1px solid ${m.border}`,
                  }}>
                    {m.emoji} {m.label}
                  </span>
                </div>
              </div>
              <div>
                <button
                  onClick={handleCallNext}
                  disabled={calling || !!currentPatient}
                  style={{
                    padding: '0.6rem 1.25rem',
                    fontWeight: 700,
                    border: 'none',
                    borderRadius: 8,
                    background: (calling || currentPatient) ? '#e2e8f0' : 'linear-gradient(135deg, #dc2626, #991b1b)',
                    color: (calling || currentPatient) ? '#94a3b8' : '#fff',
                    cursor: (calling || currentPatient) ? 'not-allowed' : 'pointer',
                    boxShadow: (calling || currentPatient) ? 'none' : '0 4px 14px rgba(220,38,38,0.35)',
                    fontSize: '0.9rem',
                    transition: 'all 0.15s',
                  }}
                >
                  {calling ? '⏳ Calling…' : '📣 Call Next'}
                </button>
                {currentPatient && (
                  <div style={{ marginTop: '0.4rem', fontSize: '0.75rem', color: '#94a3b8', textAlign: 'center' }}>
                    Finish current patient first
                  </div>
                )}
              </div>
            </div>
          )
        })() : (
          <div style={{ textAlign: 'center', padding: '1.5rem 0', color: '#94a3b8' }}>
            <div style={{ fontSize: '2rem', marginBottom: '0.5rem' }}>🎉</div>
            <div style={{ fontWeight: 600 }}>Queue is empty</div>
            <div style={{ fontSize: '0.85rem' }}>No patients waiting</div>
          </div>
        )}
      </div>

      {/* Up next list */}
      <div style={{
        background: '#fff',
        borderRadius: 12,
        padding: '1.25rem 1.5rem',
        boxShadow: '0 2px 12px rgba(0,0,0,0.07)',
        border: '1px solid #e2e8f0',
      }}>
        <div style={{ fontSize: '0.8rem', fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.08em', marginBottom: '0.75rem' }}>
          ⏳ Up Next ({upNext.length})
        </div>

        {upNext.length === 0 ? (
          <p style={{ margin: 0, color: '#94a3b8', fontSize: '0.9rem' }}>No further patients in queue.</p>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
            {upNext.map((entry, i) => {
              const m = URGENCY_META[entry.urgency] ?? URGENCY_META[3]
              return (
                <div key={entry.id} className="card-enter" style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.75rem',
                  padding: '0.6rem 0.9rem',
                  borderRadius: 8,
                  background: m.bg,
                  border: `1px solid ${m.border}`,
                }}>
                  <span style={{
                    width: 24, height: 24,
                    borderRadius: '50%',
                    background: m.color,
                    color: '#fff',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: '0.75rem',
                    fontWeight: 700,
                    flexShrink: 0,
                  }}>
                    {i + 2}
                  </span>
                  <span style={{ fontWeight: 600, color: '#1e293b' }}>{entry.patientId}</span>
                  <span style={{
                    marginLeft: 'auto',
                    padding: '0.15rem 0.5rem',
                    borderRadius: 99,
                    background: '#fff',
                    color: m.color,
                    fontWeight: 700,
                    fontSize: '0.75rem',
                    border: `1px solid ${m.border}`,
                  }}>
                    {m.emoji} {m.label}
                  </span>
                </div>
              )
            })}
          </div>
        )}
      </div>

      <p style={{ marginTop: '1rem', fontSize: '0.8rem', color: '#94a3b8', textAlign: 'center' }}>
        🔄 Auto-refreshes every {POLL_INTERVAL_MS / 1000}s
      </p>
    </div>
  )
}
