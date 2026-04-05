package main

import (
	"sync"
	"time"
)

type BFKey struct {
	SrcIP   string
	DstPort uint16
}

type bfState struct {
	times       []time.Time
	lastAlerted time.Time
}

type BruteForceTracker struct {
	mu            sync.Mutex
	hosts         map[BFKey]*bfState
	window        time.Duration
	threshold     int
	alertCooldown time.Duration
	onAlert       func(srcIP string, dstPort uint16, count int)
}

func NewBruteForceTracker(
	window time.Duration,
	threshold int,
	alertCooldown time.Duration,
	onAlert func(srcIP string, dstPort uint16, count int),
) *BruteForceTracker {
	return &BruteForceTracker{
		hosts:         make(map[BFKey]*bfState),
		window:        window,
		threshold:     threshold,
		alertCooldown: alertCooldown,
		onAlert:       onAlert,
	}
}

func (t *BruteForceTracker) Record(srcIP string, dstPort uint16, now time.Time) {
	key := BFKey{srcIP, dstPort}
	t.mu.Lock()
	defer t.mu.Unlock()

	hs, ok := t.hosts[key]
	if !ok {
		hs = &bfState{}
		t.hosts[key] = hs
	}

	hs.times = append(hs.times, now)

	// drop timestamps that have fallen outside the sliding window
	cutoff := now.Add(-t.window)
	i := 0
	for i < len(hs.times) && hs.times[i].Before(cutoff) {
		i++
	}
	hs.times = hs.times[i:]

	count := len(hs.times)
	// cooldown stops repeated alerts for the same attacker during an ongoing burst
	if count >= t.threshold && now.Sub(hs.lastAlerted) >= t.alertCooldown {
		hs.lastAlerted = now
		go t.onAlert(srcIP, dstPort, count)
	}
}

func (t *BruteForceTracker) EvictStaleEntries() {
	ticker := time.NewTicker(t.window)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-t.window)
		t.mu.Lock()
		for key, hs := range t.hosts {
			i := 0
			for i < len(hs.times) && hs.times[i].Before(cutoff) {
				i++
			}
			hs.times = hs.times[i:]
			if len(hs.times) == 0 {
				delete(t.hosts, key)
			}
		}
		t.mu.Unlock()
	}
}
