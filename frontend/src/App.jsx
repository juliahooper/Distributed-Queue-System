import { Routes, Route, NavLink } from 'react-router-dom'
import Reception from './views/Reception'
import Dashboard from './views/Dashboard'

function App() {
  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
      <nav style={{
        padding: '1rem 1.5rem',
        borderBottom: '1px solid #334155',
        display: 'flex',
        gap: '1.5rem',
        alignItems: 'center',
      }}>
        <NavLink
          to="/"
          end
          style={({ isActive }) => ({
            color: isActive ? '#f8fafc' : '#94a3b8',
            fontWeight: isActive ? 600 : 400,
          })}
        >
          Register patient
        </NavLink>
        <NavLink
          to="/dashboard"
          style={({ isActive }) => ({
            color: isActive ? '#f8fafc' : '#94a3b8',
            fontWeight: isActive ? 600 : 400,
          })}
        >
          View queue
        </NavLink>
      </nav>
      <main style={{ flex: 1, padding: '1.5rem' }}>
        <Routes>
          <Route path="/" element={<Reception />} />
          <Route path="/dashboard" element={<Dashboard />} />
        </Routes>
      </main>
    </div>
  )
}

export default App
