import { useState, useEffect, useCallback } from 'react'
import { getMetrics, getDeadLetterCount, deadLetterRetry, getActivityLog } from '../api'

const POLL_INTERVAL_MS = 5000
const ACTIVITY_LIMIT = 50

export default function Monitor() {
  const [metrics, setMetrics] = useState(null)
  const [dlqCount, setDlqCount] = useState(null)
  const [activityLog, setActivityLog] = useState([])
  const [eventFilter, setEventFilter] = useState('')
  const [error, setError] = useState(null)
  const [retrying, setRetrying] = useState(false)

  const fetchMetrics = useCallback(async () => {
    try {
      const m = await getMetrics()
      setMetrics(m)
      setError(null)
    } catch (err) {
      setError(err.message || 'Failed to load metrics')
    }
  }, [])

  const fetchDlqCount = useCallback(async () => {
    try {
      const res = await getDeadLetterCount()
      setDlqCount(res?.count ?? 0)
    } catch {
      setDlqCount(0)
    }
  }, [])

  const fetchActivityLog = useCallback(async () => {
    try {
      const res = await getActivityLog({
        limit: ACTIVITY_LIMIT,
        offset: 0,
        event_type: eventFilter || undefined,
      })
      setActivityLog(res?.entries ?? [])
    } catch (err) {
      setActivityLog([])
    }
  }, [eventFilter])

  useEffect(() => {
    fetchMetrics()
    fetchDlqCount()
    fetchActivityLog()
    const id = setInterval(() => {
      fetchMetrics()
      fetchDlqCount()
      fetchActivityLog()
    }, POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [fetchMetrics, fetchDlqCount, fetchActivityLog])

  async function handleRetry() {
    setRetrying(true)
    try {
      await deadLetterRetry()
      await fetchDlqCount()
      await fetchMetrics()
      await fetchActivityLog()
    } catch (err) {
      setError(err.message || 'Retry failed')
    } finally {
      setRetrying(false)
    }
  }

  const formatTime = (t) => {
    if (!t) return '-'
    const d = new Date(t)
    return d.toLocaleString()
  }

  return (
    <div style={{ maxWidth: 900 }}>
      <h1 style={{ marginTop: 0, marginBottom: '1.5rem' }}>Monitoring Dashboard</h1>
      {error && (
        <p style={{ color: '#fca5a5', marginBottom: '1rem' }}>{error}</p>
      )}

      <section style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))',
        gap: '1rem',
        marginBottom: '2rem',
      }}>
        <MetricCard label="Queue depth" value={metrics?.queue_depth ?? '-'} />
        <MetricCard label="Pending" value={metrics?.pending_count ?? '-'} />
        <MetricCard label="Produced" value={metrics?.messages_produced_total ?? '-'} />
        <MetricCard label="Consumed" value={metrics?.messages_consumed_total ?? '-'} />
        <MetricCard label="Dead letter" value={dlqCount ?? metrics?.dead_letter_count ?? '-'} />
      </section>

      {(dlqCount > 0 || (metrics?.dead_letter_count ?? 0) > 0) && (
        <section style={{
          marginBottom: '2rem',
          padding: '1rem',
          borderRadius: 8,
          background: '#7f1d1d',
          border: '1px solid #991b1b',
        }}>
          <h2 style={{ margin: '0 0 0.5rem', fontSize: '0.95rem' }}>
            Dead letter queue has {(dlqCount ?? metrics?.dead_letter_count) ?? 0} items
          </h2>
          <button
            onClick={handleRetry}
            disabled={retrying}
            style={{
              padding: '0.5rem 1rem',
              fontSize: '0.9rem',
              border: 'none',
              borderRadius: 6,
              background: '#22c55e',
              color: '#fff',
              cursor: retrying ? 'not-allowed' : 'pointer',
              opacity: retrying ? 0.7 : 1,
            }}
          >
            {retrying ? 'Retrying…' : 'Retry all'}
          </button>
        </section>
      )}

      <section style={{
        padding: '1rem',
        borderRadius: 8,
        background: '#1e293b',
        border: '1px solid #334155',
      }}>
        <h2 style={{ margin: '0 0 1rem', fontSize: '0.95rem', color: '#94a3b8' }}>
          Activity log
        </h2>
        <div style={{ marginBottom: '0.75rem' }}>
          <label style={{ marginRight: '0.5rem', color: '#94a3b8' }}>Filter:</label>
          <select
            value={eventFilter}
            onChange={(e) => setEventFilter(e.target.value)}
            style={{
              padding: '0.35rem 0.5rem',
              background: '#0f172a',
              border: '1px solid #334155',
              borderRadius: 4,
              color: '#f8fafc',
            }}
          >
            <option value="">All events</option>
            <option value="publish">Publish</option>
            <option value="consume">Consume</option>
            <option value="ack">Ack</option>
            <option value="requeue">Requeue</option>
            <option value="dlq">Dead letter</option>
            <option value="dlq_retry">DLQ retry</option>
          </select>
        </div>
        <div style={{
          maxHeight: 400,
          overflowY: 'auto',
          fontSize: '0.85rem',
        }}>
          {activityLog.length === 0 ? (
            <p style={{ color: '#64748b' }}>No activity (PostgreSQL backend required)</p>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid #334155' }}>
                  <th style={{ textAlign: 'left', padding: '0.5rem', color: '#94a3b8' }}>Time</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem', color: '#94a3b8' }}>Event</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem', color: '#94a3b8' }}>Message ID</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem', color: '#94a3b8' }}>Topic</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem', color: '#94a3b8' }}>Producer</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem', color: '#94a3b8' }}>Consumer</th>
                </tr>
              </thead>
              <tbody>
                {activityLog.map((e, i) => (
                  <tr key={i} style={{ borderBottom: '1px solid #334155' }}>
                    <td style={{ padding: '0.5rem' }}>{formatTime(e.created_at)}</td>
                    <td style={{ padding: '0.5rem' }}>{e.event_type}</td>
                    <td style={{ padding: '0.5rem', fontFamily: 'monospace', fontSize: '0.8rem' }}>
                      {e.message_id ? e.message_id.slice(0, 8) + '…' : '-'}
                    </td>
                    <td style={{ padding: '0.5rem' }}>{e.topic || '-'}</td>
                    <td style={{ padding: '0.5rem' }}>{e.producer_id || '-'}</td>
                    <td style={{ padding: '0.5rem' }}>{e.consumer_id || '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </section>

      <p style={{ marginTop: '1rem', fontSize: '0.85rem', color: '#64748b' }}>
        Updates every {POLL_INTERVAL_MS / 1000} seconds. Requires PostgreSQL backend for metrics and activity log.
      </p>
    </div>
  )
}

function MetricCard({ label, value }) {
  return (
    <div style={{
      padding: '1rem',
      borderRadius: 8,
      background: '#1e293b',
      border: '1px solid #334155',
    }}>
      <div style={{ fontSize: '0.8rem', color: '#94a3b8', marginBottom: '0.25rem' }}>{label}</div>
      <div style={{ fontSize: '1.5rem', fontWeight: 600 }}>{value}</div>
    </div>
  )
}
