import axios from 'axios'

const BASE = import.meta.env.VITE_API_BASE ?? ''

export function getDetections(params) {
  return axios.get(`${BASE}/api/detections`, { params }).then(r => r.data)
}

export function getDetection(id) {
  return axios.get(`${BASE}/api/detections/${id}`).then(r => r.data)
}

export function getStats() {
  return axios.get(`${BASE}/api/stats`).then(r => r.data)
}

export function getNodesHealth() {
  return axios.get(`${BASE}/api/nodes/health`).then(r => r.data)
}
