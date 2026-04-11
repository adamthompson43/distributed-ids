# SentinelMesh

A distributed intrusion detection system that spreads traffic analysis across three independent nodes, each running a different machine learning model. When a node flags suspicious traffic, it consults its peers and a majority vote determines the final classification, reducing false positives without sacrificing recall.

**BSc. (Hons.) Computer Science — South East Technological University**
Adam Thompson (20103347) · Supervisor: Dr. John Sheppard

---

## Overview

Each node performs local packet inspection and anomaly detection using a different ML model architecture. When a flow is flagged, the detecting node broadcasts the raw feature vector to its peers via `POST /vote`. Each peer independently scores the same flow and returns a vote. A strict majority determines the final outcome.

- `[CONSENSUS-ANOMALY]` - majority of nodes agree the flow is malicious
- `[OVERRULED]` - local detection rejected by peers
- `[BRUTE-FORCE]` - sliding window TCP connection spike (not subject to consensus)

---

## Node Models

| Node  | Model              | 
|-------|--------------------|
| node1 | LR + PCA (9 components) |
| node2 | Decision Tree (max_depth=20) |
| node3 | MLP (64→32, ReLU) |

Trained on the [CICIDS2017](https://www.unb.ca/cic/datasets/ids-2017.html) dataset (all 8 days, 2.8M flows). All inference runs natively in Go from exported JSON, no Python dependency at runtime.

---

## Technologies

- **Go** — packet capture (gopacket), ML inference, consensus HTTP server, dashboard API
- **Python / scikit-learn** — model training (`cicids2017.ipynb`)
- **PostgreSQL** — detections, consensus votes, node health logs
- **React + Vite** — dashboard frontend
- **AWS** — EC2 (nodes + dashboard), RDS (PostgreSQL), ALB (dashboard)

---

## Running Locally

### Prerequisites
- Go 1.23+
- libpcap (`brew install libpcap`)
- PostgreSQL (or Docker)
- Node.js 18+ (for dashboard)

### Start a local database
```bash
docker run --name ids-db \
  -e POSTGRES_USER=ids -e POSTGRES_PASSWORD=pass -e POSTGRES_DB=distributed_ids \
  -p 5432:5432 -d postgres:16

psql "postgres://ids:pass@localhost/distributed_ids?sslmode=disable" \
  -f schema.sql
```

### Run the three nodes (separate terminals)
```bash
make node1 DB="postgres://ids:pass@localhost/distributed_ids?sslmode=disable"
make node2 DB="postgres://ids:pass@localhost/distributed_ids?sslmode=disable"
make node3 DB="postgres://ids:pass@localhost/distributed_ids?sslmode=disable"
```

Override default PCAP: `make node1 PCAP=/path/to/file.pcap DB=...`

### Run the dashboard
```bash
cd dashboard
go build -o sentineldash .
./sentineldash -db "postgres://ids:pass@localhost/distributed_ids?sslmode=disable"
```

```bash
cd dashboard/frontend
npm install
npm run dev   # http://localhost:5173
```

---

## Key Flags

| Flag | Default | Purpose |
|------|---------|---------|
| `-pcap` | - | Replay a pcap file instead of live capture |
| `-interface` | `en0` | Live capture interface (requires sudo) |
| `-model` | `../model_params.json` | Path to model JSON |
| `-node-id` | `node1` | Node identifier |
| `-listen` | - | Address to serve `/vote` on (e.g. `:8081`) |
| `-peers` | - | Comma-separated peer base URLs |
| `-db` | - | PostgreSQL DSN (omit for standalone mode) |
| `-consensus-timeout` | `2s` | Peer vote collection timeout |
| `-flow-timeout` | `30s` | Idle flow expiry |

---

## Dataset

CICIDS2017 Canadian Institute for Cybersecurity. Flows labelled across 8 days of captures covering DoS, port scan, brute force, botnet, and web attacks.
