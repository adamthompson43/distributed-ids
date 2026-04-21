package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/lib/pq"
)

type Store struct {
	db     *sql.DB
	nodeID string
}

func NewStore(nodeID, dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &Store{db: db, nodeID: nodeID}, nil
}

func (s *Store) Close() {
	if s != nil {
		s.db.Close()
	}
}

func (s *Store) SaveAnomaly(f *Flow, features [21]float64, result ConsensusResult, detType string) {
	if s == nil {
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("store: begin tx: %v", err)
		return
	}
	defer tx.Rollback() // no-op if commit succeeds, cleans up on any early return

	featSlice := features[:]

	var detectionID int64
	err = tx.QueryRow(`
		INSERT INTO detections
			(node_id, detection_type,
			 src_ip, dst_ip, src_port, dst_port, protocol,
			 flow_start, flow_end, fwd_packets, bwd_packets, duration_us,
			 local_score, features, yes_votes, total_votes, consensus_ms)
		VALUES ($1,$2, $3,$4,$5,$6,$7, $8,$9,$10,$11,$12, $13,$14,$15,$16,$17)
		RETURNING id`,
		s.nodeID, detType,
		f.Key.SrcIP, f.Key.DstIP, f.Key.SrcPort, f.Key.DstPort, f.Key.Proto,
		f.StartTime, f.LastSeen, f.FwdPackets, f.BwdPackets, f.Duration().Microseconds(),
		result.LocalScore, pq.Array(featSlice),
		result.YesVotes, result.TotalVotes, result.ConsensusMs,
	).Scan(&detectionID)
	if err != nil {
		log.Printf("store: insert detection: %v", err)
		return
	}

	// local node always voted yes, it's the one that triggered the consensus round
	localVote := detType == "consensus_anomaly" || detType == "overruled"
	if _, err := tx.Exec(`
		INSERT INTO consensus_votes (detection_id, voter_node_id, vote, score)
		VALUES ($1,$2,$3,$4)`,
		detectionID, s.nodeID, localVote, result.LocalScore,
	); err != nil {
		log.Printf("store: insert local vote: %v", err)
		return
	}

	for _, pv := range result.PeerVotes {
		if _, err := tx.Exec(`
			INSERT INTO consensus_votes (detection_id, voter_node_id, vote, score)
			VALUES ($1,$2,$3,$4)`,
			detectionID, pv.NodeID, pv.Vote, pv.Score,
		); err != nil {
			log.Printf("store: insert peer vote %s: %v", pv.NodeID, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("store: commit: %v", err)
	}
}

func (s *Store) SaveBruteForce(srcIP string, dstPort uint16, count int) {
	if s == nil {
		return
	}
	_, err := s.db.Exec(`
		INSERT INTO detections
			(node_id, detection_type, src_ip, dst_ip, src_port, dst_port, protocol, connection_count)
		VALUES ($1,'brute_force', $2,'0.0.0.0',0,$3,'tcp', $4)`,
		s.nodeID, srcIP, dstPort, count,
	)
	if err != nil {
		log.Printf("store: insert brute_force: %v", err)
	}
}

func (s *Store) RecordHealth(targetNodeID string, reachable bool, responseMs *int) {
	if s == nil {
		return
	}
	_, err := s.db.Exec(`
		INSERT INTO node_health (node_id, checked_by, reachable, response_time_ms)
		VALUES ($1,$2,$3,$4)`,
		targetNodeID, s.nodeID, reachable, responseMs,
	)
	if err != nil {
		log.Printf("store: insert node_health: %v", err)
	}
}

func (s *Store) PollPeerHealth(peers []string, interval time.Duration) {
	if s == nil || len(peers) == 0 {
		return
	}
	client := &http.Client{Timeout: 3 * time.Second}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		for _, peer := range peers {
			go func(peer string) {
				url := strings.TrimRight(peer, "/") + "/healthz"
				start := time.Now()
				resp, err := client.Get(url)
				elapsed := int(time.Since(start).Milliseconds())

				if err != nil {
					s.RecordHealth(peer, false, nil)
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					s.RecordHealth(peer, false, nil)
					return
				}

				// parse node_id from {"node_id":"nodeX","status":"ok"}
				var body struct {
					NodeID string `json:"node_id"`
				}
				nodeID := peer // fallback to URL if parse fails
				if err := json.NewDecoder(resp.Body).Decode(&body); err == nil && body.NodeID != "" {
					nodeID = body.NodeID
				}
				s.RecordHealth(nodeID, true, &elapsed)
			}(peer)
		}
	}
}
