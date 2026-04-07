package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
)

func registerRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("/api/health", handleHealth)
    mux.HandleFunc("/api/detections", handleDetections(db))
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