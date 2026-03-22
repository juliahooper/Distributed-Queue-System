import { Routes, Route, NavLink } from 'react-router-dom'
import Reception from './views/Reception'
import Dashboard from './views/Dashboard'
import Monitor from './views/Monitor'

function App() {
  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', background: '#f1f5f9' }}>
      {/* Top bar */}
      <header style={{
        background: 'linear-gradient(135deg, #dc2626 0%, #991b1b 100%)',
        padding: '0 1.5rem',
        boxShadow: '0 4px 20px rgba(220,38,38,0.35)',
        display: 'flex',
        alignItems: 'center',
        gap: '1rem',
        minHeight: 64,
      }}>
        <span style={{ fontSize: '1.6rem' }}>🏥</span>
        <div>
          <div style={{ color: '#fff', fontWeight: 700, fontSize: '1.1rem', letterSpacing: '0.02em' }}>
            City ER — Patient Queue
          </div>
          <div style={{ color: 'rgba(255,255,255,0.7)', fontSize: '0.75rem' }}>
            Emergency Department Management System
          </div>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
          <span style={{
            width: 8, height: 8, borderRadius: '50%',
            background: '#4ade80',
            display: 'inline-block',
            boxShadow: '0 0 6px #4ade80',
          }} />
          <span style={{ color: 'rgba(255,255,255,0.8)', fontSize: '0.8rem' }}>System online</span>
        </div>
      </header>

      {/* Nav */}
      <nav style={{
        background: '#fff',
        padding: '0 1.5rem',
        borderBottom: '1px solid #e2e8f0',
        display: 'flex',
        gap: '0',
        alignItems: 'stretch',
        boxShadow: '0 1px 4px rgba(0,0,0,0.06)',
      }}>
        {[
          { to: '/', label: '📋 Reception', end: true },
          { to: '/dashboard', label: '👩‍⚕️ Nurse Station' },
          { to: '/monitor', label: '📊 Monitor' },
        ].map(({ to, label, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            style={({ isActive }) => ({
              padding: '0.9rem 1.25rem',
              color: isActive ? '#dc2626' : '#64748b',
              fontWeight: isActive ? 700 : 400,
              fontSize: '0.9rem',
              borderBottom: isActive ? '3px solid #dc2626' : '3px solid transparent',
              textDecoration: 'none',
              transition: 'all 0.15s',
              whiteSpace: 'nowrap',
            })}
          >
            {label}
          </NavLink>
        ))}
      </nav>

      <main style={{ flex: 1, padding: '2rem 1.5rem', maxWidth: 1100, width: '100%', margin: '0 auto' }}>
        <Routes>
          <Route path="/" element={<Reception />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/monitor" element={<Monitor />} />
        </Routes>
      </main>

      <footer style={{
        background: '#fff',
        borderTop: '1px solid #e2e8f0',
        padding: '0.75rem 1.5rem',
        textAlign: 'center',
        fontSize: '0.75rem',
        color: '#94a3b8',
      }}>
        City ER Queue System — Distributed Message Broker
      </footer>
    </div>
  )
}

export default App
