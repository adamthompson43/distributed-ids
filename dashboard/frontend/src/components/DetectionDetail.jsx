import { useState, useEffect } from 'react'
import { getDetection } from '../api'

export default function DetectionDetail({ id, onBack }) {
  const [data, setData] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    getDetection(id)
      .then(setData)
      .finally(() => setLoading(false))
  }, [id])

  if (loading) return <p>Loading...</p>
  if (!data) return <p>Not found.</p>

  return (
    <div>
      <button onClick={onBack} style={{ marginBottom: '1rem' }}>← Back</button>

      <h2>Detection #{data.id}</h2>

      <table>
        <tbody>
          <tr><td>Type</td><td>{data.detection_type}</td></tr>
          <tr><td>Node</td><td>{data.node_id}</td></tr>
          <tr><td>Time</td><td>{new Date(data.detected_at).toLocaleString()}</td></tr>
          <tr><td>Src IP</td><td>{data.src_ip}</td></tr>
          <tr><td>Dst IP</td><td>{data.dst_ip}:{data.dst_port}</td></tr>
          <tr><td>Protocol</td><td>{data.protocol}</td></tr>
          <tr><td>Local Score</td><td>{data.local_score > 0 ? data.local_score.toFixed(4) : '—'}</td></tr>
          <tr><td>Votes</td><td>{data.total_votes > 0 ? `${data.yes_votes}/${data.total_votes}` : '—'}</td></tr>
          <tr><td>Packets</td><td>fwd {data.fwd_packets} / bwd {data.bwd_packets}</td></tr>
          <tr><td>Duration</td><td>{data.duration_us > 0 ? `${(data.duration_us / 1000000).toFixed(2)}s` : '—'}</td></tr>
        </tbody>
      </table>

      {data.votes && data.votes.length > 0 && (
        <>
          <h3 style={{ marginTop: '1.5rem' }}>Per-node votes</h3>
          <table>
            <thead>
              <tr>
                <th>Node</th>
                <th>Vote</th>
                <th>Score</th>
              </tr>
            </thead>
            <tbody>
              {data.votes.map(v => (
                <tr key={v.voter_node_id}>
                  <td>{v.voter_node_id}</td>
                  <td style={{ color: v.vote ? '#e53e3e' : '#38a169', fontWeight: 600 }}>
                    {v.vote ? 'Attack' : 'Benign'}
                  </td>
                  <td>{v.score.toFixed(4)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  )
}
