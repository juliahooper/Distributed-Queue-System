import { Routes, Route, NavLink } from 'react-router-dom'
import Reception from './views/Reception'
import Dashboard from './views/Dashboard'
import Monitor from './views/Monitor'

function App() {
  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', background: '#111827' }}>

      {/* Top bar */}
      <header style={{
        background: 'linear-gradient(135deg, #7f1d1d 0%, #991b1b 40%, #dc2626 100%)',
        padding: '0 2rem',
        boxShadow: '0 4px 30px rgba(220,38,38,0.5)',
        display: 'flex',
        alignItems: 'center',
        gap: '1rem',
        minHeight: 68,
        borderBottom: '1px solid rgba(255,255,255,0.08)',
      }}>
        <span style={{ fontSize: '1.8rem', filter: 'drop-shadow(0 0 8px rgba(255,255,255,0.3))' }}>🏥</span>
        <div>
          <div style={{
            color: '#fff',
            fontFamily: "'Bebas Neue', sans-serif",
            fontSize: '1.7rem',
            letterSpacing: '0.08em',
            textShadow: '0 0 20px rgba(255,255,255,0.3)',
          }}>
            City ER — Patient Queue
          </div>
          <div style={{ color: 'rgba(255,255,255,0.6)', fontSize: '0.72rem', letterSpacing: '0.05em', textTransform: 'uppercase' }}>
            Emergency Department Management System
          </div>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <span style={{
            width: 9, height: 9, borderRadius: '50%',
            background: '#4ade80',
            display: 'inline-block',
            boxShadow: '0 0 8px #4ade80, 0 0 16px #4ade8066',
          }} />
          <span style={{
            color: 'rgba(255,255,255,0.85)',
            fontSize: '0.8rem',
            fontFamily: "'Bebas Neue', sans-serif",
            letterSpacing: '0.06em',
          }}>System Online</span>
        </div>
      </header>

      {/* Nav */}
      <nav style={{
        background: '#0d1117',
        padding: '0 2rem',
        display: 'flex',
        alignItems: 'stretch',
        borderBottom: '1px solid #1f2937',
        boxShadow: '0 4px 20px rgba(220,38,38,0.2)',
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
              padding: '0.85rem 1.5rem',
              color: isActive ? '#fff' : '#6b7280',
              fontFamily: "'Bebas Neue', sans-serif",
              fontSize: '1.1rem',
              letterSpacing: '0.06em',
              borderBottom: isActive ? '3px solid #dc2626' : '3px solid transparent',
              boxShadow: isActive ? 'inset 0 -3px 12px rgba(220,38,38,0.3)' : 'none',
              textDecoration: 'none',
              transition: 'all 0.15s',
              whiteSpace: 'nowrap',
              background: isActive ? 'rgba(220,38,38,0.07)' : 'transparent',
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
        background: '#0d1117',
        borderTop: '1px solid #1f2937',
        padding: '0.75rem 2rem',
        textAlign: 'center',
        fontFamily: "'Bebas Neue', sans-serif",
        fontSize: '0.85rem',
        letterSpacing: '0.06em',
        color: '#374151',
      }}>
        City ER Queue System — Distributed Message Broker
      </footer>
    </div>
  )
}

export default App
