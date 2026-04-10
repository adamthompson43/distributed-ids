import { useState, useEffect } from 'react'
import { getStats } from '../api'

export default function Stats() {
  const [data, setData] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    getStats()
      .then(setData)
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <p>Loading...</p>
  if (!data) return <p>No data.</p>

  return (
    <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap' }}>
      <section>
        <h3>Detections by type</h3>
        <table>
          <thead>
            <tr><th>Type</th><th>Count</th></tr>
          </thead>
          <tbody>
            {Object.entries(data.totals).map(([type, count]) => (
              <tr key={type}>
                <td>{type}</td>
                <td>{count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section>
        <h3>Top source IPs</h3>
        <table>
          <thead>
            <tr><th>IP</th><th>Count</th></tr>
          </thead>
          <tbody>
            {data.top_src_ips.map(ip => (
              <tr key={ip.src_ip}>
                <td>{ip.src_ip}</td>
                <td>{ip.count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section>
        <h3>Detections per node</h3>
        <table>
          <thead>
            <tr><th>Node</th><th>Count</th></tr>
          </thead>
          <tbody>
            {data.per_node.map(n => (
              <tr key={n.node_id}>
                <td>{n.node_id}</td>
                <td>{n.count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  )
}
