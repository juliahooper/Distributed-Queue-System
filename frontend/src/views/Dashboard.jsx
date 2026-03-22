import { useState, useEffect, useCallback } from 'react'
import { getNext, consumeNext, ackMessage } from '../api'

const POLL_INTERVAL_MS = 2000
const QUEUE_LIMIT = 5

const URGENCY_META = {
  1: { label: 'Critical',  color: '#dc2626', tag: 'IMMEDIATE' },
  2: { label: 'Urgent',    color: '#ea580c', tag: 'HIGH PRIORITY' },
  3: { label: 'Moderate',  color: '#eab308', tag: 'STANDARD' },
  4: { label: 'Minor',     color: '#22c55e', tag: 'NON-URGENT' },
  5: { label: 'Low',       color: '#2563eb', tag: 'ROUTINE' },
}

const card = {
  background: '#111827',
  borderRadius: 16,
  padding: '1.5rem',
  boxShadow: '0 0 60px rgba(220,38,38,0.35), 0 0 120px rgba(220,38,38,0.15)',
  border: '1px solid rgba(220,38,38,0.15)',
  marginBottom: '1.25rem',
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
          Nurse Station
        </h1>
        <p style={{
          margin: '0.5rem 0 0',
          color: 'rgba(255,255,255,0.55)',
          fontSize: '0.95rem',
          fontFamily: "'Inter', sans-serif",
          letterSpacing: '0.02em',
        }}>
          Call patients from the queue and mark them as complete when done.
        </p>
      </div>

      {/* Error */}
      {error && (
        <div className="card-enter" style={{
          marginBottom: '1rem',
          padding: '0.75rem 1rem',
          borderRadius: 8,
          background: '#1f0a0a',
          border: '1px solid #dc262655',
          color: '#fca5a5',
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
            ...card,
            border: `2px solid ${m.color}`,
            boxShadow: `0 0 30px ${m.color}55, 0 0 60px ${m.color}22`,
          }}>
            <div style={{
              fontSize: '0.75rem',
              fontWeight: 700,
              color: m.color,
              textTransform: 'uppercase',
              letterSpacing: '0.1em',
              fontFamily: "'Bebas Neue', sans-serif",
              fontSize: '1rem',
              marginBottom: '0.75rem',
            }}>
              🩺 Currently Seeing
            </div>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '1rem' }}>
              <div>
                <div style={{
                  fontSize: '1.8rem',
                  fontWeight: 800,
                  color: '#fff',
                  fontFamily: "'Bebas Neue', sans-serif",
                  letterSpacing: '0.04em',
                }}>
                  {currentPatient.patientId}
                </div>
                <span style={{
                  display: 'inline-block',
                  marginTop: '0.4rem',
                  padding: '0.2rem 0.7rem',
                  borderRadius: 99,
                  background: `${m.color}22`,
                  color: m.color,
                  fontWeight: 700,
                  fontSize: '0.8rem',
                  border: `1px solid ${m.color}44`,
                }}>
                  {m.label} — {m.tag}
                </span>
              </div>
              <button
                onClick={handleDone}
                disabled={finishing}
                style={{
                  padding: '0.75rem 1.5rem',
                  fontWeight: 800,
                  fontFamily: "'Bebas Neue', sans-serif",
                  fontSize: '1.2rem',
                  letterSpacing: '0.04em',
                  border: '1px solid #22c55e55',
                  borderRadius: 10,
                  background: finishing
                    ? '#22c55e'
                    : `linear-gradient(180deg, rgba(255,255,255,0.85) 0%, rgba(255,255,255,0.25) 100%), #22c55e`,
                  color: finishing ? '#fff' : '#1e293b',
                  cursor: finishing ? 'not-allowed' : 'pointer',
                  boxShadow: finishing ? '0 4px 18px #22c55e88' : '0 2px 10px #22c55e55',
                  transition: 'all 0.15s',
                  whiteSpace: 'nowrap',
                }}
              >
                {finishing ? 'Completing…' : '✅ Done with Patient'}
              </button>
            </div>
          </div>
        )
      })()}

      {/* Next in queue */}
      <div style={card}>
        <div style={{
          fontFamily: "'Bebas Neue', sans-serif",
          fontSize: '1.3rem',
          letterSpacing: '0.06em',
          color: 'rgba(255,255,255,0.7)',
          marginBottom: '1rem',
        }}>
          🚨 Next Patient
        </div>

        {nextEntry ? (() => {
          const m = URGENCY_META[nextEntry.urgency] ?? URGENCY_META[3]
          const disabled = calling || !!currentPatient
          return (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '1rem' }}>
              <div>
                <div style={{
                  fontSize: '1.8rem',
                  fontWeight: 800,
                  color: '#fff',
                  fontFamily: "'Bebas Neue', sans-serif",
                  letterSpacing: '0.04em',
                }}>
                  {nextEntry.patientId}
                </div>
                <span style={{
                  display: 'inline-block',
                  marginTop: '0.4rem',
                  padding: '0.2rem 0.7rem',
                  borderRadius: 99,
                  background: `${m.color}22`,
                  color: m.color,
                  fontWeight: 700,
                  fontSize: '0.8rem',
                  border: `1px solid ${m.color}44`,
                }}>
                  {m.label} — {m.tag}
                </span>
              </div>
              <div style={{ textAlign: 'center' }}>
                <button
                  onClick={handleCallNext}
                  disabled={disabled}
                  style={{
                    padding: '0.75rem 1.5rem',
                    fontWeight: 800,
                    fontFamily: "'Bebas Neue', sans-serif",
                    fontSize: '1.2rem',
                    letterSpacing: '0.04em',
                    border: disabled ? '1px solid #374151' : '1px solid #dc262655',
                    borderRadius: 10,
                    background: disabled
                      ? '#1f2937'
                      : `linear-gradient(180deg, rgba(255,255,255,0.85) 0%, rgba(255,255,255,0.25) 100%), #dc2626`,
                    color: disabled ? '#6b7280' : '#1e293b',
                    cursor: disabled ? 'not-allowed' : 'pointer',
                    boxShadow: disabled ? 'none' : '0 2px 10px #dc262655',
                    transition: 'all 0.15s',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {calling ? 'Calling…' : '📣 Call Next'}
                </button>
                {currentPatient && (
                  <div style={{ marginTop: '0.4rem', fontSize: '0.75rem', color: '#6b7280' }}>
                    Finish current patient first
                  </div>
                )}
              </div>
            </div>
          )
        })() : (
          <div style={{ textAlign: 'center', padding: '1.5rem 0', color: '#6b7280' }}>
            <div style={{ fontSize: '2rem', marginBottom: '0.5rem' }}>🎉</div>
            <div style={{ fontWeight: 600, color: 'rgba(255,255,255,0.6)' }}>Queue is empty</div>
            <div style={{ fontSize: '0.85rem' }}>No patients waiting</div>
          </div>
        )}
      </div>

      {/* Up next list */}
      <div style={card}>
        <div style={{
          fontFamily: "'Bebas Neue', sans-serif",
          fontSize: '1.3rem',
          letterSpacing: '0.06em',
          color: 'rgba(255,255,255,0.7)',
          marginBottom: '1rem',
        }}>
          ⏳ Up Next ({upNext.length})
        </div>

        {upNext.length === 0 ? (
          <p style={{ margin: 0, color: '#6b7280', fontSize: '0.9rem' }}>No further patients in queue.</p>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
            {upNext.map((entry, i) => {
              const m = URGENCY_META[entry.urgency] ?? URGENCY_META[3]
              return (
                <div key={entry.id} className="card-enter" style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.75rem',
                  padding: '0.75rem 1rem',
                  borderRadius: 10,
                  background: `linear-gradient(180deg, rgba(255,255,255,0.85) 0%, rgba(255,255,255,0.25) 100%), ${m.color}`,
                  border: `1px solid ${m.color}55`,
                  boxShadow: `0 2px 8px ${m.color}33`,
                }}>
                  <span style={{
                    width: 26, height: 26,
                    borderRadius: '50%',
                    background: 'rgba(0,0,0,0.15)',
                    color: '#1e293b',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: '0.8rem',
                    fontWeight: 800,
                    fontFamily: "'Bebas Neue', sans-serif",
                    flexShrink: 0,
                  }}>
                    {i + 2}
                  </span>
                  <span style={{ fontWeight: 700, color: '#1e293b', fontFamily: "'Bebas Neue', sans-serif", fontSize: '1.1rem', letterSpacing: '0.03em' }}>
                    {entry.patientId}
                  </span>
                  <span style={{
                    marginLeft: 'auto',
                    padding: '0.2rem 0.6rem',
                    borderRadius: 99,
                    background: 'rgba(255,255,255,0.4)',
                    color: '#1e293b',
                    fontWeight: 700,
                    fontSize: '0.75rem',
                    border: '1px solid rgba(255,255,255,0.5)',
                  }}>
                    {m.label}
                  </span>
                </div>
              )
            })}
          </div>
        )}
      </div>

      <p style={{ marginTop: '0.5rem', fontSize: '0.8rem', color: '#6b7280', textAlign: 'center' }}>
        🔄 Auto-refreshes every {POLL_INTERVAL_MS / 1000}s
      </p>
    </div>
  )
}
