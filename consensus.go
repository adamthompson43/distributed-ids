package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type VoteRequest struct {
	NodeID   string      `json:"node_id"`
	FlowKey  FlowKey     `json:"flow_key"`
	Features [21]float64 `json:"features"`
	Score    float64     `json:"score"`
}

type VoteResponse struct {
	NodeID string  `json:"node_id"`
	Vote   bool    `json:"vote"`
	Score  float64 `json:"score"`
}

type PeerVote struct {
	NodeID string
	Vote   bool
	Score  float64
}

type ConsensusResult struct {
	Anomaly    bool
	YesVotes   int
	TotalVotes int
	LocalScore float64
	PeerVotes  []PeerVote
}

type ConsensusManager struct {
	nodeID  string
	peers   []string
	det     *Detector
	timeout time.Duration
	client  *http.Client
}

func NewConsensusManager(nodeID string, peers []string, det *Detector, timeout time.Duration) *ConsensusManager {
	return &ConsensusManager{
		nodeID:  nodeID,
		peers:   peers,
		det:     det,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

func (cm *ConsensusManager) Start(listenAddr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/vote", cm.HandleVote)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"node_id":%q,"status":"ok"}`, cm.nodeID)
	})
	log.Printf("Consensus server listening on %s (node_id=%s, peers=%d)", listenAddr, cm.nodeID, len(cm.peers))
	go func() {
		if err := http.ListenAndServe(listenAddr, mux); err != nil {
			log.Fatalf("Consensus server error: %v", err)
		}
	}()
}

func (cm *ConsensusManager) HandleVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	features := sanitise(req.Features)
	anomaly, score := cm.det.IsAnomaly(features)

	resp := VoteResponse{
		NodeID: cm.nodeID,
		Vote:   anomaly,
		Score:  score,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("consensus: encode vote response: %v", err)
	}
}

func (cm *ConsensusManager) RequestVotes(key FlowKey, features [21]float64, localAnomaly bool, localScore float64) ConsensusResult {
	// local node counts as vote #1
	yesVotes := boolToInt(localAnomaly)
	totalVotes := 1
	peerVotes := make([]PeerVote, 0, len(cm.peers))

	if len(cm.peers) == 0 {
		return ConsensusResult{
			Anomaly:    localAnomaly,
			YesVotes:   yesVotes,
			TotalVotes: totalVotes,
			LocalScore: localScore,
		}
	}

	body, err := json.Marshal(VoteRequest{
		NodeID:   cm.nodeID,
		FlowKey:  key,
		Features: features,
		Score:    localScore,
	})
	if err != nil {
		log.Printf("consensus: marshal error: %v", err)
		return ConsensusResult{Anomaly: localAnomaly, YesVotes: yesVotes, TotalVotes: totalVotes, LocalScore: localScore}
	}

	type peerResult struct {
		resp VoteResponse
		err  error
	}

	ch := make(chan peerResult, len(cm.peers))
	ctx, cancel := context.WithTimeout(context.Background(), cm.timeout)
	defer cancel()

	var wg sync.WaitGroup
	for _, peer := range cm.peers {
		wg.Add(1)
		go func(peer string) {
			defer wg.Done()
			url := strings.TrimRight(peer, "/") + "/vote"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				ch <- peerResult{err: err}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := cm.client.Do(req)
			if err != nil {
				ch <- peerResult{err: err}
				return
			}
			defer resp.Body.Close()
			var vr VoteResponse
			if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
				ch <- peerResult{err: fmt.Errorf("decode: %w", err)}
				return
			}
			ch <- peerResult{resp: vr}
		}(peer)
	}

	// close the channel once all peer goroutines finish so the range below terminates
	go func() {
		wg.Wait()
		close(ch)
	}()

	for res := range ch {
		if res.err != nil {
			log.Printf("consensus: peer vote error: %v", res.err)
			continue
		}
		totalVotes++
		peerVotes = append(peerVotes, PeerVote{
			NodeID: res.resp.NodeID,
			Vote:   res.resp.Vote,
			Score:  res.resp.Score,
		})
		if res.resp.Vote {
			yesVotes++
		}
	}

	// strict majority - ties go to benign - unreachable peers are excluded from totalVotes
	return ConsensusResult{
		Anomaly:    yesVotes > totalVotes/2,
		YesVotes:   yesVotes,
		TotalVotes: totalVotes,
		LocalScore: localScore,
		PeerVotes:  peerVotes,
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
