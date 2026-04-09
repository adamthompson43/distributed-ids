package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func registerRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/detections", handleDetections(db))
	mux.HandleFunc("/api/detections/", handleDetection(db))
	mux.HandleFunc("/api/stats", handleStats(db))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

type Detection struct {
	ID            int64   `json:"id"`
	NodeID        string  `json:"node_id"`
	DetectedAt    string  `json:"detected_at"`
	DetectionType string  `json:"detection_type"`
	SrcIP         string  `json:"src_ip"`
	DstIP         string  `json:"dst_ip"`
	DstPort       int     `json:"dst_port"`
	Protocol      string  `json:"protocol"`
	LocalScore    float64 `json:"local_score"`
	YesVotes      int     `json:"yes_votes"`
	TotalVotes    int     `json:"total_votes"`
	FwdPackets    int     `json:"fwd_packets"`
	BwdPackets    int     `json:"bwd_packets"`
	DurationUs    int64   `json:"duration_us"`
}

type Vote struct {
	VoterNodeID string  `json:"voter_node_id"`
	Vote        bool    `json:"vote"`
	Score       float64 `json:"score"`
}

func handleDetections(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit <= 0 || limit > 200 {
			limit = 50
		}
		offset, _ := strconv.Atoi(q.Get("offset"))

		typeFilter := q.Get("type")
		nodeFilter := q.Get("node")

		args := []any{}
		where := "WHERE 1=1"
		if typeFilter != "" {
			args = append(args, typeFilter)
			where += " AND detection_type = $" + strconv.Itoa(len(args))
		}
		if nodeFilter != "" {
			args = append(args, nodeFilter)
			where += " AND node_id = $" + strconv.Itoa(len(args))
		}

		// total count
		var total int
		if err := db.QueryRow("SELECT COUNT(*) FROM detections "+where, args...).Scan(&total); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		// paginated rows
		args = append(args, limit, offset)
		rows, err := db.Query(`
				SELECT id, node_id, detected_at, detection_type,
						src_ip, dst_ip, dst_port, protocol,
						COALESCE(local_score, 0),
						COALESCE(yes_votes, 0), COALESCE(total_votes, 0),
						COALESCE(fwd_packets, 0), COALESCE(bwd_packets, 0),
						COALESCE(duration_us, 0)
				FROM detections
				`+where+`
				ORDER BY detected_at DESC
				LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)),
			args...,
		)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		detections := []Detection{}
		for rows.Next() {
			var d Detection
			if err := rows.Scan(
				&d.ID, &d.NodeID, &d.DetectedAt, &d.DetectionType,
				&d.SrcIP, &d.DstIP, &d.DstPort, &d.Protocol,
				&d.LocalScore, &d.YesVotes, &d.TotalVotes,
				&d.FwdPackets, &d.BwdPackets, &d.DurationUs,
			); err != nil {
				http.Error(w, "scan error", http.StatusInternalServerError)
				return
			}
			detections = append(detections, d)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"detections": detections,
			"total":      total,
		})
	}
}

func handleDetection(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/detections/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		var d Detection
		err = db.QueryRow(`
				SELECT id, node_id, detected_at, detection_type,
						src_ip, dst_ip, dst_port, protocol,
						COALESCE(local_score, 0),
						COALESCE(yes_votes, 0), COALESCE(total_votes, 0),
						COALESCE(fwd_packets, 0), COALESCE(bwd_packets, 0),
						COALESCE(duration_us, 0)
				FROM detections
				WHERE id = $1`, id,
		).Scan(
			&d.ID, &d.NodeID, &d.DetectedAt, &d.DetectionType,
			&d.SrcIP, &d.DstIP, &d.DstPort, &d.Protocol,
			&d.LocalScore, &d.YesVotes, &d.TotalVotes,
			&d.FwdPackets, &d.BwdPackets, &d.DurationUs,
		)
		if err == sql.ErrNoRows {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		rows, err := db.Query(`
				SELECT voter_node_id, vote, score
				FROM consensus_votes
				WHERE detection_id = $1
				ORDER BY voter_node_id`, id,
		)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		votes := []Vote{}
		for rows.Next() {
			var v Vote
			if err := rows.Scan(&v.VoterNodeID, &v.Vote, &v.Score); err != nil {
				http.Error(w, "scan error", http.StatusInternalServerError)
				return
			}
			votes = append(votes, v)
		}

		type detectionDetail struct {
			Detection
			Votes []Vote `json:"votes"`
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detectionDetail{d, votes})
	}
}

func handleStats(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// totals by detection type
		rows, err := db.Query(`
			SELECT detection_type, COUNT(*)
			FROM detections
			GROUP BY detection_type`)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		totals := map[string]int{}
		for rows.Next() {
			var dtype string
			var count int
			if err := rows.Scan(&dtype, &count); err != nil {
				http.Error(w, "scan error", http.StatusInternalServerError)
				return
			}
			totals[dtype] = count
		}

		// top 10 source IPs
		type ipCount struct {
			SrcIP string `json:"src_ip"`
			Count int    `json:"count"`
		}
		ipRows, err := db.Query(`
			SELECT src_ip::text, COUNT(*) AS cnt
			FROM detections
			GROUP BY src_ip
			ORDER BY cnt DESC
			LIMIT 10`)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer ipRows.Close()

		topIPs := []ipCount{}
		for ipRows.Next() {
			var ip ipCount
			if err := ipRows.Scan(&ip.SrcIP, &ip.Count); err != nil {
				http.Error(w, "scan error", http.StatusInternalServerError)
				return
			}
			topIPs = append(topIPs, ip)
		}

		// per-node counts
		type nodeCount struct {
			NodeID string `json:"node_id"`
			Count  int    `json:"count"`
		}
		nodeRows, err := db.Query(`
			SELECT node_id, COUNT(*)
			FROM detections
			GROUP BY node_id
			ORDER BY node_id`)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer nodeRows.Close()

		perNode := []nodeCount{}
		for nodeRows.Next() {
			var n nodeCount
			if err := nodeRows.Scan(&n.NodeID, &n.Count); err != nil {
				http.Error(w, "scan error", http.StatusInternalServerError)
				return
			}
			perNode = append(perNode, n)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"totals":      totals,
			"top_src_ips": topIPs,
			"per_node":    perNode,
		})
	}
}
