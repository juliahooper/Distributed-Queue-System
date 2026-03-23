import { useState, useEffect, useCallback, useRef } from 'react'
import { getMetrics, getDeadLetterCount, deadLetterRetry, getDeadLetterList, deleteDeadLetter, getActivityLog } from '../api'

const POLL_INTERVAL_MS = 5000
const ACTIVITY_LIMIT = 50
const HISTORY_SIZE = 30

const card = {
  background: '#111827',
  borderRadius: 16,
  padding: '1.25rem 1.5rem',
  boxShadow: '0 0 60px rgba(220,38,38,0.35), 0 0 120px rgba(220,38,38,0.15)',
  border: '1px solid rgba(220,38,38,0.15)',
  marginBottom: '1.25rem',
}

const sectionLabel = {
  fontFamily: "'Bebas Neue', sans-serif",
  fontSize: '1.3rem',
  letterSpacing: '0.06em',
  color: 'rgba(255,255,255,0.7)',
  marginBottom: '1rem',
}

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
      const prev_ = prevMetrics.current
      const producedRate = prev_ ? Math.max(0, m.messages_produced_total - prev_.messages_produced_total) : 0
      const consumedRate = prev_ ? Math.max(0, m.messages_consumed_total - prev_.messages_consumed_total) : 0
      prevMetrics.current = m
      setMetricsHistory(prev => {
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
    } catch { setDlqCount(0) }
  }, [])

  const fetchDlqEntries = useCallback(async () => {
    try {
      const res = await getDeadLetterList()
      setDlqEntries(res?.entries ?? [])
    } catch { setDlqEntries([]) }
  }, [])

  const fetchActivityLog = useCallback(async () => {
    try {
      const res = await getActivityLog({ limit: ACTIVITY_LIMIT, offset: 0, event_type: eventFilter || undefined })
      setActivityLog(res?.entries ?? [])
    } catch { setActivityLog([]) }
  }, [eventFilter])

  useEffect(() => {
    fetchMetrics(); fetchDlqCount(); fetchDlqEntries(); fetchActivityLog()
    const id = setInterval(() => {
      fetchMetrics(); fetchDlqCount(); fetchDlqEntries(); fetchActivityLog()
    }, POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [fetchMetrics, fetchDlqCount, fetchDlqEntries, fetchActivityLog])

  async function handleRetry() {
    setRetrying(true)
    try {
      await deadLetterRetry()
      await fetchDlqCount(); await fetchDlqEntries(); await fetchMetrics(); await fetchActivityLog()
    } catch (err) { setError(err.message || 'Retry failed') }
    finally { setRetrying(false) }
  }

  async function handleDiscard(messageID) {
    setDiscarding(messageID)
    try {
      await deleteDeadLetter(messageID)
      await fetchDlqCount(); await fetchDlqEntries()
    } catch (err) { setError(err.message || 'Failed to discard message') }
    finally { setDiscarding(null) }
  }

  const formatTime = (t) => t ? new Date(t).toLocaleString() : '-'

  const decodeBody = (b64) => {
    if (!b64) return null
    try { return JSON.parse(decodeURIComponent(escape(atob(b64)))) } catch { return null }
  }

  const EVENT_META = {
    publish:     { emoji: '📨', color: '#22c55e',  bg: '#052015' },
    consume:     { emoji: '👆', color: '#60a5fa',  bg: '#051230' },
    ack:         { emoji: '✅', color: '#22d3ee',  bg: '#051a20' },
    requeue:     { emoji: '🔄', color: '#fbbf24',  bg: '#1a1200' },
    dlq:         { emoji: '💀', color: '#f87171',  bg: '#1a0505' },
    dlq_retry:   { emoji: '🔁', color: '#a78bfa',  bg: '#0f0a1a' },
    dlq_deleted: { emoji: '🗑️', color: '#94a3b8', bg: '#1e293b' },
  }

  const hasDlq = dlqEntries.length > 0

  return (
    <div style={{ maxWidth: 1000 }}>

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
          System Monitor
        </h1>
        <p style={{
          margin: '0.5rem 0 0',
          color: 'rgba(255,255,255,0.55)',
          fontSize: '0.95rem',
          letterSpacing: '0.02em',
        }}>
          Live queue metrics, throughput charts, and dead letter management.
        </p>
      </div>

      {error && (
        <div className="card-enter" style={{
          marginBottom: '1rem', padding: '0.75rem 1rem', borderRadius: 8,
          background: '#1a0505', border: '1px solid #dc262655', color: '#fca5a5', fontWeight: 500,
        }}>
          ❌ {error}
        </div>
      )}

      {/* Metric cards */}
      <section style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: '1rem', marginBottom: '1.25rem' }}>
        <MetricCard label="Queue Depth"  value={metrics?.queue_depth ?? '-'}              emoji="📋" color="#dc2626" />
        <MetricCard label="In-Flight"    value={metrics?.pending_count ?? '-'}             emoji="⏳" color="#fbbf24" />
        <MetricCard label="Produced"     value={metrics?.messages_produced_total ?? '-'}   emoji="📨" color="#22c55e" />
        <MetricCard label="Consumed"     value={metrics?.messages_consumed_total ?? '-'}   emoji="✅" color="#60a5fa" />
        <MetricCard label="Dead Letter"  value={dlqCount ?? metrics?.dead_letter_count ?? '-'} emoji="💀" color={hasDlq ? '#f87171' : '#6b7280'} alert={hasDlq} />
      </section>

      {/* Charts */}
      <section style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1.25rem' }}>
        <ChartCard label="📋 Queue depth"         data={metricsHistory.map(h => h.queueDepth)}   color="#dc2626" fillColor="rgba(220,38,38,0.15)" />
        <ChartCard label="⏳ In-flight (pending)"  data={metricsHistory.map(h => h.pending)}      color="#fbbf24" fillColor="rgba(251,191,36,0.12)" />
      </section>

      {/* Efficiency + throughput */}
      <section style={{ ...card, display: 'flex', justifyContent: 'center' }}>
        <EfficiencyGauge
          produced={metrics?.messages_produced_total ?? 0}
          consumed={metrics?.messages_consumed_total ?? 0}
        />
      </section>


      {/* Dead letter queue */}
      <section style={{
        ...card,
        border: hasDlq ? '2px solid #dc262688' : '1px solid rgba(220,38,38,0.15)',
        boxShadow: hasDlq
          ? '0 0 60px rgba(220,38,38,0.55), 0 0 120px rgba(220,38,38,0.25)'
          : card.boxShadow,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
            <span style={{ ...sectionLabel, marginBottom: 0, color: hasDlq ? '#f87171' : 'rgba(255,255,255,0.7)' }}>
              💀 Dead Letter Queue
            </span>
            {hasDlq && (
              <span style={{
                background: '#dc2626', color: '#fff', borderRadius: 99,
                padding: '0.1rem 0.55rem', fontSize: '0.75rem', fontWeight: 700,
              }}>
                {dlqEntries.length}
              </span>
            )}
          </div>
          {hasDlq && (
            <button
              onClick={handleRetry}
              disabled={retrying}
              style={{
                padding: '0.55rem 1.1rem',
                fontWeight: 800,
                fontFamily: "'Bebas Neue', sans-serif",
                fontSize: '1rem',
                letterSpacing: '0.04em',
                border: '1px solid #22c55e55',
                borderRadius: 8,
                background: retrying
                  ? '#22c55e'
                  : `linear-gradient(180deg, rgba(255,255,255,0.85) 0%, rgba(255,255,255,0.25) 100%), #22c55e`,
                color: retrying ? '#fff' : '#1e293b',
                cursor: retrying ? 'not-allowed' : 'pointer',
                boxShadow: retrying ? '0 4px 18px #22c55e88' : '0 2px 10px #22c55e55',
                transition: 'all 0.15s',
              }}
            >
              {retrying ? 'Retrying…' : '🔁 Retry All'}
            </button>
          )}
        </div>

        {dlqEntries.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '1.5rem 0', color: '#6b7280' }}>
            <div style={{ fontSize: '1.8rem', marginBottom: '0.4rem' }}>✅</div>
            <div style={{ fontWeight: 600, color: 'rgba(255,255,255,0.5)' }}>Dead letter queue is empty</div>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.65rem' }}>
            {dlqEntries.map((e) => {
              const patient = decodeBody(typeof e.body === 'string' ? e.body : null)
              const isDiscarding = discarding === e.message_id
              return (
                <div key={e.message_id} className="card-enter" style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '0.85rem 1rem',
                  borderRadius: 10,
                  background: '#1a0505',
                  border: '1px solid #dc262644',
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                    <span style={{ fontSize: '1.4rem' }}>⚠️</span>
                    <div>
                      <div style={{ fontWeight: 700, color: '#fff', fontFamily: "'Bebas Neue', sans-serif", fontSize: '1.1rem', letterSpacing: '0.03em' }}>
                        {patient?.patientId ?? e.message_id.slice(0, 8) + '…'}
                      </div>
                      <div style={{ display: 'flex', gap: '0.6rem', marginTop: '0.2rem', fontSize: '0.78rem', flexWrap: 'wrap' }}>
                        {patient?.urgency && <span style={{ color: '#9ca3af' }}>Urgency {patient.urgency}</span>}
                        <span style={{ color: '#f87171', fontWeight: 600 }}>Failed {e.retry_count}x</span>
                        <span style={{ color: '#6b7280' }}>{formatTime(e.failed_at)}</span>
                        <span style={{ fontFamily: 'monospace', color: '#4b5563' }}>{e.message_id.slice(0, 8)}…</span>
                      </div>
                    </div>
                  </div>
                  <button
                    onClick={() => handleDiscard(e.message_id)}
                    disabled={isDiscarding}
                    title="Permanently discard this poison message"
                    style={{
                      padding: '0.5rem 1rem',
                      fontWeight: 800,
                      fontFamily: "'Bebas Neue', sans-serif",
                      fontSize: '0.95rem',
                      letterSpacing: '0.04em',
                      border: '1px solid #dc262655',
                      borderRadius: 8,
                      background: isDiscarding
                        ? '#dc2626'
                        : `linear-gradient(180deg, rgba(255,255,255,0.85) 0%, rgba(255,255,255,0.25) 100%), #dc2626`,
                      color: isDiscarding ? '#fff' : '#1e293b',
                      cursor: isDiscarding ? 'not-allowed' : 'pointer',
                      boxShadow: isDiscarding ? '0 4px 18px #dc262688' : '0 2px 10px #dc262655',
                      transition: 'all 0.15s',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {isDiscarding ? 'Discarding…' : '🗑️ Discard'}
                  </button>
                </div>
              )
            })}
          </div>
        )}
      </section>

      {/* Activity log */}
      <section style={card}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
          <span style={sectionLabel}>📜 Activity Log</span>
          <select
            value={eventFilter}
            onChange={(e) => setEventFilter(e.target.value)}
            style={{
              padding: '0.35rem 0.6rem',
              background: '#1f2937',
              border: '1px solid #374151',
              borderRadius: 6,
              color: '#d1d5db',
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
            <p style={{ color: '#6b7280', textAlign: 'center', padding: '2rem 0' }}>No activity yet.</p>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid #1f2937' }}>
                  {['Time', 'Event', 'Message ID', 'Topic', 'Producer', 'Consumer'].map(h => (
                    <th key={h} style={{
                      textAlign: 'left', padding: '0.5rem 0.75rem',
                      color: '#6b7280', fontWeight: 600, fontSize: '0.78rem',
                      textTransform: 'uppercase', letterSpacing: '0.05em',
                    }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {activityLog.map((e, i) => {
                  const em = EVENT_META[e.event_type] ?? { emoji: '•', color: '#6b7280', bg: '#1e293b' }
                  return (
                    <tr key={i} style={{ borderBottom: '1px solid #1f2937' }}>
                      <td style={{ padding: '0.5rem 0.75rem', color: '#6b7280', whiteSpace: 'nowrap' }}>{formatTime(e.created_at)}</td>
                      <td style={{ padding: '0.5rem 0.75rem' }}>
                        <span style={{
                          display: 'inline-flex', alignItems: 'center', gap: '0.3rem',
                          padding: '0.15rem 0.5rem', borderRadius: 99,
                          background: em.bg, color: em.color, fontWeight: 600, fontSize: '0.78rem',
                          border: `1px solid ${em.color}33`,
                        }}>
                          {em.emoji} {e.event_type}
                        </span>
                      </td>
                      <td style={{ padding: '0.5rem 0.75rem', fontFamily: 'monospace', color: '#4b5563' }}>
                        {e.message_id ? e.message_id.slice(0, 8) + '…' : '-'}
                      </td>
                      <td style={{ padding: '0.5rem 0.75rem', color: '#9ca3af' }}>{e.topic || '-'}</td>
                      <td style={{ padding: '0.5rem 0.75rem', color: '#9ca3af' }}>{e.producer_id || '-'}</td>
                      <td style={{ padding: '0.5rem 0.75rem', color: '#9ca3af' }}>{e.consumer_id || '-'}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </div>
      </section>

      <p style={{ marginTop: '0.5rem', fontSize: '0.8rem', color: '#6b7280', textAlign: 'center' }}>
        🔄 Auto-refreshes every {POLL_INTERVAL_MS / 1000}s
      </p>
    </div>
  )
}

function MetricCard({ label, value, emoji, color, alert }) {
  return (
    <div style={{
      padding: '1rem 1.1rem',
      borderRadius: 12,
      background: '#111827',
      border: `1px solid ${alert ? '#dc262655' : 'rgba(220,38,38,0.15)'}`,
      boxShadow: alert
        ? '0 0 60px rgba(220,38,38,0.35), 0 0 120px rgba(220,38,38,0.15)'
        : '0 0 40px rgba(220,38,38,0.2)',
    }}>
      <div style={{
        fontFamily: "'Bebas Neue', sans-serif",
        fontSize: '0.9rem',
        letterSpacing: '0.08em',
        color: 'rgba(255,255,255,0.5)',
        marginBottom: '0.4rem',
      }}>
        {emoji} {label}
      </div>
      <div style={{ fontSize: '1.9rem', fontWeight: 800, color, fontFamily: "'Bebas Neue', sans-serif" }}>{value}</div>
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
      borderRadius: 12,
      background: '#111827',
      border: '1px solid rgba(220,38,38,0.15)',
      boxShadow: '0 0 40px rgba(220,38,38,0.2)',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.5rem' }}>
        <span style={{ fontSize: '0.8rem', color: 'rgba(255,255,255,0.5)', fontWeight: 600 }}>{label}</span>
        <span style={{ fontSize: '0.95rem', fontWeight: 800, color, fontFamily: "'Bebas Neue', sans-serif" }}>{latest}</span>
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
        <p style={{ margin: 0, fontSize: '0.75rem', color: '#6b7280' }}>Collecting data…</p>
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
  const color = pct >= 80 ? '#22c55e' : pct >= 50 ? '#fbbf24' : '#dc2626'
  const label = pct >= 80 ? 'Healthy' : pct >= 50 ? 'Moderate' : 'Low'

  return (
    <div style={{ textAlign: 'center' }}>
      <div style={{ fontFamily: "'Bebas Neue', sans-serif", fontSize: '1rem', letterSpacing: '0.06em', color: 'rgba(255,255,255,0.5)', marginBottom: '0.25rem' }}>
        ⚡ Processing Efficiency
      </div>
      <svg width={140} height={105} viewBox="0 0 140 105">
        <path d={trackPath} fill="none" stroke="#1f2937" strokeWidth="10" strokeLinecap="round" />
        {fillPath && <path d={fillPath} fill="none" stroke={color} strokeWidth="10" strokeLinecap="round" />}
        <text x={cx} y={cy + 6} textAnchor="middle" fontSize="20" fontWeight="800" fill={color}>{pct}%</text>
        <text x={cx} y={cy + 22} textAnchor="middle" fontSize="9" fill="#6b7280">{label}</text>
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
      <div style={{ fontFamily: "'Bebas Neue', sans-serif", fontSize: '1rem', letterSpacing: '0.06em', color: 'rgba(255,255,255,0.5)', marginBottom: '0.75rem' }}>
        📈 Throughput (30s avg)
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
        <ThroughputBar label="📨 Produced" value={avgProduced} color="#22c55e" perMin={perMin(avgProduced)} />
        <ThroughputBar label="✅ Consumed" value={avgConsumed} color="#60a5fa" perMin={perMin(avgConsumed)} />
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
        <span style={{ color: 'rgba(255,255,255,0.6)', fontWeight: 500 }}>{label}</span>
        <span style={{ color, fontWeight: 700 }}>{perMin}/min</span>
      </div>
      <div style={{ height: 8, borderRadius: 4, background: '#1f2937', overflow: 'hidden' }}>
        <div style={{ height: '100%', width: `${pct}%`, borderRadius: 4, background: color, transition: 'width 0.5s ease' }} />
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
