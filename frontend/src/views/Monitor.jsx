import { useState, useEffect, useCallback, useRef } from 'react'
import { getMetrics, getDeadLetterCount, deadLetterRetry, getDeadLetterList, deleteDeadLetter, getActivityLog } from '../api'

const POLL_INTERVAL_MS = 5000
const ACTIVITY_LIMIT = 50
const HISTORY_SIZE = 30 // keep last 30 samples (~2.5 min at 5s interval)

export default function Monitor() {
  const [metrics, setMetrics] = useState(null)
  const [dlqCount, setDlqCount] = useState(null)
  const [dlqEntries, setDlqEntries] = useState([])
  const [activityLog, setActivityLog] = useState([])
  const [eventFilter, setEventFilter] = useState('')
  const [error, setError] = useState(null)
  const [retrying, setRetrying] = useState(false)
  const [discarding, setDiscarding] = useState(null)
  const [metricsHistory, setMetricsHistory] = useState([])
  const prevMetrics = useRef(null)

  const fetchMetrics = useCallback(async () => {
    try {
      const m = await getMetrics()
      setMetrics(m)
      setMetricsHistory(prev => {
        const prev_ = prevMetrics.current
        const producedRate = prev_ ? Math.max(0, m.messages_produced_total - prev_.messages_produced_total) : 0
        const consumedRate = prev_ ? Math.max(0, m.messages_consumed_total - prev_.messages_consumed_total) : 0
        prevMetrics.current = m
        const entry = {
          t: Date.now(),
          queueDepth: m.queue_depth ?? 0,
          pending: m.pending_count ?? 0,
          dlq: m.dead_letter_count ?? 0,
          producedRate,
          consumedRate,
        }
        return [...prev.slice(-(HISTORY_SIZE - 1)), entry]
      })
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

  const fetchDlqEntries = useCallback(async () => {
    try {
      const res = await getDeadLetterList()
      setDlqEntries(res?.entries ?? [])
    } catch {
      setDlqEntries([])
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
    fetchDlqEntries()
    fetchActivityLog()
    const id = setInterval(() => {
      fetchMetrics()
      fetchDlqCount()
      fetchDlqEntries()
      fetchActivityLog()
    }, POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [fetchMetrics, fetchDlqCount, fetchDlqEntries, fetchActivityLog])

  async function handleRetry() {
    setRetrying(true)
    try {
      await deadLetterRetry()
      await fetchDlqCount()
      await fetchDlqEntries()
      await fetchMetrics()
      await fetchActivityLog()
    } catch (err) {
      setError(err.message || 'Retry failed')
    } finally {
      setRetrying(false)
    }
  }

  async function handleDiscard(messageID) {
    setDiscarding(messageID)
    try {
      await deleteDeadLetter(messageID)
      await fetchDlqCount()
      await fetchDlqEntries()
    } catch (err) {
      setError(err.message || 'Failed to discard message')
    } finally {
      setDiscarding(null)
    }
  }

  const formatTime = (t) => {
    if (!t) return '-'
    const d = new Date(t)
    return d.toLocaleString()
  }

  const decodeBody = (b64) => {
    if (!b64) return null
    try {
      return JSON.parse(decodeURIComponent(escape(atob(b64))))
    } catch {
      return null
    }
  }

  const EVENT_META = {
    publish:     { emoji: '📨', color: '#16a34a', bg: '#f0fdf4' },
    consume:     { emoji: '👆', color: '#2563eb', bg: '#eff6ff' },
    ack:         { emoji: '✅', color: '#0891b2', bg: '#ecfeff' },
    requeue:     { emoji: '🔄', color: '#d97706', bg: '#fffbeb' },
    dlq:         { emoji: '💀', color: '#dc2626', bg: '#fef2f2' },
    dlq_retry:   { emoji: '🔁', color: '#7c3aed', bg: '#f5f3ff' },
    dlq_deleted: { emoji: '🗑️', color: '#64748b', bg: '#f8fafc' },
  }

  return (
    <div style={{ maxWidth: 1000 }}>
      {/* Page header */}
      <div style={{ marginBottom: '1.75rem' }}>
        <h1 style={{ margin: 0, fontSize: '1.6rem', fontWeight: 700, color: '#1e293b' }}>
          📊 System Monitor
        </h1>
        <p style={{ margin: '0.4rem 0 0', color: '#64748b', fontSize: '0.95rem' }}>
          Live queue metrics, throughput charts, and dead letter management.
        </p>
      </div>

      {error && (
        <div className="card-enter" style={{
          marginBottom: '1rem', padding: '0.75rem 1rem', borderRadius: 8,
          background: '#fef2f2', border: '1px solid #fecaca', color: '#991b1b', fontWeight: 500,
        }}>
          ❌ {error}
        </div>
      )}

      {/* Metric cards */}
      <section style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(5, 1fr)',
        gap: '1rem',
        marginBottom: '1.5rem',
      }}>
        <MetricCard label="Queue Depth" value={metrics?.queue_depth ?? '-'} emoji="📋" color="#dc2626" />
        <MetricCard label="In-Flight" value={metrics?.pending_count ?? '-'} emoji="⏳" color="#d97706" />
        <MetricCard label="Produced" value={metrics?.messages_produced_total ?? '-'} emoji="📨" color="#16a34a" />
        <MetricCard label="Consumed" value={metrics?.messages_consumed_total ?? '-'} emoji="✅" color="#2563eb" />
        <MetricCard label="Dead Letter" value={dlqCount ?? metrics?.dead_letter_count ?? '-'} emoji="💀" color={dlqCount > 0 ? '#dc2626' : '#64748b'} alert={dlqCount > 0} />
      </section>

      {/* Charts + gauges */}
      <section style={{
        display: 'grid',
        gridTemplateColumns: '1fr 1fr',
        gap: '1rem',
        marginBottom: '1.5rem',
      }}>
        <ChartCard label="📋 Queue depth" data={metricsHistory.map(h => h.queueDepth)} color="#dc2626" fillColor="rgba(220,38,38,0.08)" />
        <ChartCard label="⏳ In-flight (pending)" data={metricsHistory.map(h => h.pending)} color="#d97706" fillColor="rgba(217,119,6,0.08)" />
        <ChartCard label="📨 Produced / interval" data={metricsHistory.map(h => h.producedRate)} color="#16a34a" fillColor="rgba(22,163,74,0.08)" />
        <ChartCard label="✅ Consumed / interval" data={metricsHistory.map(h => h.consumedRate)} color="#2563eb" fillColor="rgba(37,99,235,0.08)" />
      </section>

      {/* Efficiency + throughput */}
      <section style={{
        marginBottom: '1.5rem',
        padding: '1.25rem 1.5rem',
        borderRadius: 12,
        background: '#fff',
        border: '1px solid #e2e8f0',
        boxShadow: '0 2px 12px rgba(0,0,0,0.06)',
        display: 'flex',
        alignItems: 'center',
        gap: '3rem',
        flexWrap: 'wrap',
      }}>
        <EfficiencyGauge
          produced={metrics?.messages_produced_total ?? 0}
          consumed={metrics?.messages_consumed_total ?? 0}
        />
        <div style={{ flex: 1, minWidth: 220 }}>
          <ThroughputGauge history={metricsHistory} />
        </div>
      </section>

      {/* Dead letter queue */}
      <section style={{
        marginBottom: '1.5rem',
        padding: '1.25rem 1.5rem',
        borderRadius: 12,
        background: '#fff',
        border: `2px solid ${dlqEntries.length > 0 ? '#fca5a5' : '#e2e8f0'}`,
        boxShadow: dlqEntries.length > 0 ? '0 0 20px rgba(220,38,38,0.1)' : '0 2px 12px rgba(0,0,0,0.06)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
          <div>
            <span style={{ fontWeight: 700, fontSize: '1rem', color: dlqEntries.length > 0 ? '#dc2626' : '#374151' }}>
              💀 Dead Letter Queue
            </span>
            {dlqEntries.length > 0 && (
              <span style={{
                marginLeft: '0.6rem',
                background: '#dc2626',
                color: '#fff',
                borderRadius: 99,
                padding: '0.1rem 0.55rem',
                fontSize: '0.75rem',
                fontWeight: 700,
              }}>
                {dlqEntries.length}
              </span>
            )}
          </div>
          {dlqEntries.length > 0 && (
            <button
              onClick={handleRetry}
              disabled={retrying}
              style={{
                padding: '0.45rem 1rem',
                fontSize: '0.85rem',
                fontWeight: 600,
                border: 'none',
                borderRadius: 7,
                background: retrying ? '#e2e8f0' : 'linear-gradient(135deg, #16a34a, #15803d)',
                color: retrying ? '#94a3b8' : '#fff',
                cursor: retrying ? 'not-allowed' : 'pointer',
                boxShadow: retrying ? 'none' : '0 3px 10px rgba(22,163,74,0.3)',
              }}
            >
              {retrying ? '⏳ Retrying…' : '🔁 Retry All'}
            </button>
          )}
        </div>

        {dlqEntries.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '1.5rem 0', color: '#94a3b8' }}>
            <div style={{ fontSize: '1.8rem', marginBottom: '0.4rem' }}>✅</div>
            <div style={{ fontWeight: 600 }}>Dead letter queue is empty</div>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.65rem' }}>
            {dlqEntries.map((e) => {
              const patient = decodeBody(typeof e.body === 'string' ? e.body : null)
              return (
                <div key={e.message_id} className="card-enter" style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '0.75rem 1rem',
                  borderRadius: 8,
                  background: '#fef2f2',
                  border: '1px solid #fecaca',
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                    <span style={{ fontSize: '1.4rem' }}>⚠️</span>
                    <div>
                      <div style={{ fontWeight: 700, color: '#1e293b' }}>
                        {patient?.patientId ?? e.message_id.slice(0, 8) + '…'}
                      </div>
                      <div style={{ display: 'flex', gap: '0.6rem', marginTop: '0.2rem', fontSize: '0.78rem', color: '#64748b', flexWrap: 'wrap' }}>
                        {patient?.urgency && <span>Urgency {patient.urgency}</span>}
                        <span style={{ color: '#dc2626', fontWeight: 600 }}>Failed {e.retry_count}x</span>
                        <span>{formatTime(e.failed_at)}</span>
                        <span style={{ fontFamily: 'monospace', color: '#94a3b8' }}>{e.message_id.slice(0, 8)}…</span>
                      </div>
                    </div>
                  </div>
                  <button
                    onClick={() => handleDiscard(e.message_id)}
                    disabled={discarding === e.message_id}
                    title="Permanently discard this poison message"
                    style={{
                      padding: '0.4rem 0.9rem',
                      fontSize: '0.82rem',
                      fontWeight: 600,
                      border: '1px solid #fca5a5',
                      borderRadius: 7,
                      background: discarding === e.message_id ? '#f1f5f9' : '#fff',
                      color: discarding === e.message_id ? '#94a3b8' : '#dc2626',
                      cursor: discarding === e.message_id ? 'not-allowed' : 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {discarding === e.message_id ? '⏳ Discarding…' : '🗑️ Discard'}
                  </button>
                </div>
              )
            })}
          </div>
        )}
      </section>

      {/* Activity log */}
      <section style={{
        padding: '1.25rem 1.5rem',
        borderRadius: 12,
        background: '#fff',
        border: '1px solid #e2e8f0',
        boxShadow: '0 2px 12px rgba(0,0,0,0.06)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
          <span style={{ fontWeight: 700, fontSize: '1rem', color: '#374151' }}>📜 Activity Log</span>
          <select
            value={eventFilter}
            onChange={(e) => setEventFilter(e.target.value)}
            style={{
              padding: '0.35rem 0.6rem',
              background: '#f8fafc',
              border: '1px solid #e2e8f0',
              borderRadius: 6,
              color: '#374151',
              fontSize: '0.85rem',
            }}
          >
            <option value="">All events</option>
            <option value="publish">📨 Publish</option>
            <option value="consume">👆 Consume</option>
            <option value="ack">✅ Ack</option>
            <option value="requeue">🔄 Requeue</option>
            <option value="dlq">💀 Dead letter</option>
            <option value="dlq_retry">🔁 DLQ retry</option>
            <option value="dlq_deleted">🗑️ DLQ discarded</option>
          </select>
        </div>
        <div style={{ maxHeight: 360, overflowY: 'auto', fontSize: '0.83rem' }}>
          {activityLog.length === 0 ? (
            <p style={{ color: '#94a3b8', textAlign: 'center', padding: '2rem 0' }}>No activity yet.</p>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid #f1f5f9' }}>
                  {['Time', 'Event', 'Message ID', 'Topic', 'Producer', 'Consumer'].map(h => (
                    <th key={h} style={{ textAlign: 'left', padding: '0.5rem 0.75rem', color: '#64748b', fontWeight: 600, fontSize: '0.78rem', textTransform: 'uppercase', letterSpacing: '0.05em' }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {activityLog.map((e, i) => {
                  const em = EVENT_META[e.event_type] ?? { emoji: '•', color: '#64748b', bg: '#f8fafc' }
                  return (
                    <tr key={i} style={{ borderBottom: '1px solid #f1f5f9' }}>
                      <td style={{ padding: '0.5rem 0.75rem', color: '#64748b', whiteSpace: 'nowrap' }}>{formatTime(e.created_at)}</td>
                      <td style={{ padding: '0.5rem 0.75rem' }}>
                        <span style={{
                          display: 'inline-flex', alignItems: 'center', gap: '0.3rem',
                          padding: '0.15rem 0.5rem', borderRadius: 99,
                          background: em.bg, color: em.color, fontWeight: 600, fontSize: '0.78rem',
                        }}>
                          {em.emoji} {e.event_type}
                        </span>
                      </td>
                      <td style={{ padding: '0.5rem 0.75rem', fontFamily: 'monospace', color: '#94a3b8' }}>
                        {e.message_id ? e.message_id.slice(0, 8) + '…' : '-'}
                      </td>
                      <td style={{ padding: '0.5rem 0.75rem', color: '#374151' }}>{e.topic || '-'}</td>
                      <td style={{ padding: '0.5rem 0.75rem', color: '#374151' }}>{e.producer_id || '-'}</td>
                      <td style={{ padding: '0.5rem 0.75rem', color: '#374151' }}>{e.consumer_id || '-'}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </div>
      </section>

      <p style={{ marginTop: '1rem', fontSize: '0.8rem', color: '#94a3b8', textAlign: 'center' }}>
        🔄 Auto-refreshes every {POLL_INTERVAL_MS / 1000}s
      </p>
    </div>
  )
}

function MetricCard({ label, value, emoji, color, alert }) {
  return (
    <div style={{
      padding: '1rem 1.1rem',
      borderRadius: 10,
      background: '#fff',
      border: `1px solid ${alert ? '#fca5a5' : '#e2e8f0'}`,
      boxShadow: alert ? `0 0 14px rgba(220,38,38,0.12)` : '0 2px 8px rgba(0,0,0,0.05)',
    }}>
      <div style={{ fontSize: '0.75rem', color: '#64748b', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: '0.4rem' }}>
        {emoji} {label}
      </div>
      <div style={{ fontSize: '1.75rem', fontWeight: 800, color }}>{value}</div>
    </div>
  )
}

function ChartCard({ label, data, color, fillColor }) {
  const width = 400
  const height = 80
  const padding = 8

  const max = Math.max(...data, 1)
  const points = data.map((v, i) => {
    const x = padding + (i / Math.max(data.length - 1, 1)) * (width - padding * 2)
    const y = height - padding - (v / max) * (height - padding * 2)
    return [x, y]
  })

  const polyline = points.map(([x, y]) => `${x},${y}`).join(' ')
  const fillPath = points.length > 1
    ? `M${points[0][0]},${height - padding} ` +
      points.map(([x, y]) => `L${x},${y}`).join(' ') +
      ` L${points[points.length - 1][0]},${height - padding} Z`
    : ''

  const latest = data[data.length - 1] ?? 0

  return (
    <div style={{
      padding: '1rem 1.1rem',
      borderRadius: 10,
      background: '#fff',
      border: '1px solid #e2e8f0',
      boxShadow: '0 2px 8px rgba(0,0,0,0.05)',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.5rem' }}>
        <span style={{ fontSize: '0.8rem', color: '#64748b', fontWeight: 600 }}>{label}</span>
        <span style={{ fontSize: '0.95rem', fontWeight: 800, color }}>{latest}</span>
      </div>
      <svg viewBox={`0 0 ${width} ${height}`} style={{ width: '100%', height: 80, display: 'block' }}>
        {fillPath && <path d={fillPath} fill={fillColor} />}
        {points.length > 1 && (
          <polyline points={polyline} fill="none" stroke={color} strokeWidth="2.5" strokeLinejoin="round" />
        )}
        {points.length > 0 && (
          <circle cx={points[points.length - 1][0]} cy={points[points.length - 1][1]} r="4" fill={color} />
        )}
      </svg>
      {data.length < 2 && (
        <p style={{ margin: 0, fontSize: '0.75rem', color: '#94a3b8' }}>Collecting data…</p>
      )}
    </div>
  )
}

function EfficiencyGauge({ produced, consumed }) {
  const ratio = produced > 0 ? Math.min(consumed / produced, 1) : 0
  const pct = Math.round(ratio * 100)
  const r = 50, cx = 70, cy = 68
  const startAngle = -210, sweep = 240
  const angle = startAngle + sweep * ratio
  const trackPath = describeArc(cx, cy, r, startAngle, startAngle + sweep)
  const fillPath = ratio > 0 ? describeArc(cx, cy, r, startAngle, angle) : null
  const color = pct >= 80 ? '#16a34a' : pct >= 50 ? '#d97706' : '#dc2626'
  const label = pct >= 80 ? 'Healthy' : pct >= 50 ? 'Moderate' : 'Low'

  return (
    <div style={{ textAlign: 'center' }}>
      <div style={{ fontSize: '0.75rem', fontWeight: 700, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: '0.25rem' }}>
        ⚡ Processing Efficiency
      </div>
      <svg width={140} height={105} viewBox="0 0 140 105">
        <path d={trackPath} fill="none" stroke="#e2e8f0" strokeWidth="10" strokeLinecap="round" />
        {fillPath && <path d={fillPath} fill="none" stroke={color} strokeWidth="10" strokeLinecap="round" />}
        <text x={cx} y={cy + 6} textAnchor="middle" fontSize="20" fontWeight="800" fill={color}>{pct}%</text>
        <text x={cx} y={cy + 22} textAnchor="middle" fontSize="9" fill="#94a3b8">{label}</text>
      </svg>
    </div>
  )
}

function ThroughputGauge({ history }) {
  const win = history.slice(-6)
  const avgProduced = win.length > 0 ? win.reduce((s, h) => s + h.producedRate, 0) / win.length : 0
  const avgConsumed = win.length > 0 ? win.reduce((s, h) => s + h.consumedRate, 0) / win.length : 0
  const perMin = (v) => (v * (60000 / POLL_INTERVAL_MS)).toFixed(1)

  return (
    <div>
      <div style={{ fontSize: '0.75rem', fontWeight: 700, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: '0.75rem' }}>
        📈 Throughput (30s avg)
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
        <ThroughputBar label="📨 Produced" value={avgProduced} color="#16a34a" perMin={perMin(avgProduced)} />
        <ThroughputBar label="✅ Consumed" value={avgConsumed} color="#2563eb" perMin={perMin(avgConsumed)} />
      </div>
    </div>
  )
}

function ThroughputBar({ label, value, color, perMin }) {
  const maxDisplay = 5
  const pct = Math.min((value / maxDisplay) * 100, 100)
  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.3rem', fontSize: '0.82rem' }}>
        <span style={{ color: '#374151', fontWeight: 500 }}>{label}</span>
        <span style={{ color, fontWeight: 700 }}>{perMin}/min</span>
      </div>
      <div style={{ height: 8, borderRadius: 4, background: '#f1f5f9', overflow: 'hidden' }}>
        <div style={{
          height: '100%', width: `${pct}%`, borderRadius: 4,
          background: color, transition: 'width 0.5s ease',
        }} />
      </div>
    </div>
  )
}

function describeArc(cx, cy, r, startDeg, endDeg) {
  const toRad = (d) => (d * Math.PI) / 180
  const x1 = cx + r * Math.cos(toRad(startDeg))
  const y1 = cy + r * Math.sin(toRad(startDeg))
  const x2 = cx + r * Math.cos(toRad(endDeg))
  const y2 = cy + r * Math.sin(toRad(endDeg))
  const largeArc = endDeg - startDeg > 180 ? 1 : 0
  return `M${x1},${y1} A${r},${r} 0 ${largeArc} 1 ${x2},${y2}`
}
