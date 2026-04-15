import { useState, useEffect } from 'react'
import { getDetections } from '../api'

const TYPE_LABELS = {
  consensus_anomaly: 'Anomaly',
  overruled: 'Overruled',
  brute_force: 'Brute Force',
}

const TYPE_COLORS = {
  consensus_anomaly: '#e53e3e',
  overruled: '#d69e2e',
  brute_force: '#805ad5',
}

export default function Alerts({ onSelect }) {
  const [data, setData] = useState({ detections: [], total: 0 })
  const [typeFilter, setTypeFilter] = useState('')
  const [loading, setLoading] = useState(true)

  function load() {
    const params = {}
    if (typeFilter) params.type = typeFilter
    getDetections(params)
      .then(setData)
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
    const interval = setInterval(load, 5000)
    return () => clearInterval(interval)
  }, [typeFilter])

  return (
    <div>
      <div style={{ marginBottom: '1rem', display: 'flex', gap: '0.5rem' }}>
        <select
          value={typeFilter}
          onChange={e => setTypeFilter(e.target.value)}
          style={{
            color: 'var(--text-h)',
            background: 'var(--bg)',
            border: '1px solid var(--border)',
            borderRadius: '4px',
            padding: '0.25rem 0.5rem',
          }}
        >
          <option value=''>All types</option>
          <option value='consensus_anomaly'>Anomaly</option>
          <option value='overruled'>Overruled</option>
          <option value='brute_force'>Brute Force</option>
        </select>
        <span style={{ color: '#666', alignSelf: 'center' }}>
          {data.total} total
        </span>
      </div>

      {loading ? (
        <p>Loading...</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Time</th>
              <th>Type</th>
              <th>Node</th>
              <th>Src IP</th>
              <th>Dst IP</th>
              <th>Port</th>
              <th>Score</th>
              <th>Votes</th>
            </tr>
          </thead>
          <tbody>
            {data.detections.map(d => (
              <tr key={d.id} onClick={() => onSelect(d.id)} style={{ cursor: 'pointer' }}>
                <td>{new Date(d.detected_at).toLocaleString()}</td>
                <td>
                  <span style={{
                    color: TYPE_COLORS[d.detection_type] ?? '#333',
                    fontWeight: 600,
                  }}>
                    {TYPE_LABELS[d.detection_type] ?? d.detection_type}
                  </span>
                </td>
                <td>{d.node_id}</td>
                <td>{d.src_ip}</td>
                <td>{d.dst_ip}</td>
                <td>{d.dst_port}</td>
                <td>{d.local_score > 0 ? d.local_score.toFixed(3) : '—'}</td>
                <td>{d.total_votes > 0 ? `${d.yes_votes}/${d.total_votes}` : '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
