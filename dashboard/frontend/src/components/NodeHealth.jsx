import { useState, useEffect } from 'react'
import { getNodesHealth } from '../api'

export default function NodeHealth() {
  const [data, setData] = useState({ nodes: [] })
  const [loading, setLoading] = useState(true)

  function load() {
    getNodesHealth()
      .then(setData)
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
    const interval = setInterval(load, 10000)
    return () => clearInterval(interval)
  }, [])

  if (loading) return <p>Loading...</p>

  return (
    <div>
      <table>
        <thead>
          <tr>
            <th>Node</th>
            <th>Status</th>
            <th>Response Time</th>
            <th>Last Checked</th>
          </tr>
        </thead>
        <tbody>
          {data.nodes.map(n => (
            <tr key={n.node_id}>
              <td>{n.node_id}</td>
              <td style={{ color: n.reachable ? '#38a169' : '#e53e3e', fontWeight: 600 }}>
                {n.reachable ? 'Online' : 'Offline'}
              </td>
              <td>{n.response_time_ms != null ? `${n.response_time_ms}ms` : '—'}</td>
              <td>{new Date(n.checked_at).toLocaleString()}</td>
            </tr>
          ))}
          {data.nodes.length === 0 && (
            <tr><td colSpan={4}>No health data yet.</td></tr>
          )}
        </tbody>
      </table>
    </div>
  )
}
