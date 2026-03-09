import { useState, useEffect, useCallback } from 'react'
import { getNext, callPatient } from '../api'

const POLL_INTERVAL_MS = 2000
const QUEUE_LIMIT = 5

export default function Dashboard() {
  const [queue, setQueue] = useState([])
  const [error, setError] = useState(null)
  const [calling, setCalling] = useState(false)

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
    const first = queue[0]
    if (!first || calling) return
    setCalling(true)
    try {
      await callPatient(first.id)
      await fetchQueue()
      setError(null)
    } catch (err) {
      setError(err.message || 'Failed to call patient.')
    } finally {
      setCalling(false)
    }
  }

  const nextEntry = queue[0]
  const upNext = queue.slice(1, QUEUE_LIMIT)

  return (
    <div style={{ maxWidth: 560 }}>
      <h1 style={{ marginTop: 0, marginBottom: '1.5rem' }}>ER Queue</h1>
      {error && (
        <p style={{ color: '#fca5a5', marginBottom: '1rem' }}>{error}</p>
      )}

      <section style={{
        marginBottom: '1.5rem',
        padding: '1rem',
        borderRadius: 8,
        background: '#1e293b',
        border: '1px solid #334155',
      }}>
        <h2 style={{ margin: '0 0 0.5rem', fontSize: '0.9rem', color: '#94a3b8' }}>Next</h2>
        {nextEntry ? (
          <p style={{ margin: 0, fontSize: '1.25rem', fontWeight: 600 }}>
            {nextEntry.patientId}
            <span style={{ marginLeft: '0.5rem', fontWeight: 400, color: '#94a3b8' }}>
              (urgency {nextEntry.urgency})
            </span>
          </p>
        ) : (
          <p style={{ margin: 0, color: '#64748b' }}>No one in queue.</p>
        )}
        {nextEntry && (
          <button
            onClick={handleCallNext}
            disabled={calling}
            style={{
              marginTop: '0.75rem',
              padding: '0.5rem 1rem',
              fontSize: '0.95rem',
              border: 'none',
              borderRadius: 6,
              background: '#22c55e',
              color: '#fff',
              cursor: calling ? 'not-allowed' : 'pointer',
              opacity: calling ? 0.7 : 1,
            }}
          >
            {calling ? 'Calling…' : 'Call next'}
          </button>
        )}
      </section>

      <section style={{
        padding: '1rem',
        borderRadius: 8,
        background: '#1e293b',
        border: '1px solid #334155',
      }}>
        <h2 style={{ margin: '0 0 0.75rem', fontSize: '0.9rem', color: '#94a3b8' }}>
          Up next
        </h2>
        {upNext.length > 0 ? (
          <ul style={{ margin: 0, paddingLeft: '1.25rem' }}>
            {upNext.map((entry, i) => (
              <li
                key={entry.id}
                style={{
                  marginBottom: '0.5rem',
                  padding: '0.35rem 0',
                  listStyle: 'decimal',
                  transition: 'all 0.2s ease',
                }}
              >
                {entry.patientId}
                <span style={{ marginLeft: '0.5rem', color: '#94a3b8' }}>
                  (urgency {entry.urgency})
                </span>
              </li>
            ))}
          </ul>
        ) : (
          <p style={{ margin: 0, color: '#64748b' }}>No further patients in queue.</p>
        )}
      </section>

      <p style={{ marginTop: '1rem', fontSize: '0.85rem', color: '#64748b' }}>
        Updates every {POLL_INTERVAL_MS / 1000} seconds. Open another tab to register patients and watch the queue change.
      </p>
    </div>
  )
}
