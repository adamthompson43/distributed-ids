package main

import (
	"math"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type FlowKey struct {
	SrcIP, DstIP     string
	SrcPort, DstPort uint16
	Proto            string
}

func (k FlowKey) Reversed() FlowKey {
	return FlowKey{k.DstIP, k.SrcIP, k.DstPort, k.SrcPort, k.Proto}
}

type Flow struct {
	Key       FlowKey
	StartTime time.Time
	LastSeen  time.Time

	FwdPackets int
	FwdLengths []float64

	BwdPackets int
	BwdLengths []float64

	PacketTimes []time.Time

	SYNCount, ACKCount, RSTCount int
}

func (f *Flow) Duration() time.Duration {
	return f.LastSeen.Sub(f.StartTime)
}

func (f *Flow) addPacket(length float64, fwd, syn, ack, rst bool, ts time.Time) {
	f.LastSeen = ts
	f.PacketTimes = append(f.PacketTimes, ts)
	if fwd {
		f.FwdPackets++
		f.FwdLengths = append(f.FwdLengths, length)
	} else {
		f.BwdPackets++
		f.BwdLengths = append(f.BwdLengths, length)
	}
	if syn {
		f.SYNCount++
	}
	if ack {
		f.ACKCount++
	}
	if rst {
		f.RSTCount++
	}
}

func (f *Flow) Features() [21]float64 {
	dur := f.Duration()
	durUs := float64(dur.Microseconds())
	durSec := dur.Seconds()
	// guard against divide-by-zero for single-packet or same-timestamp flows
	if durSec < 1e-9 {
		durSec = 1e-9
	}

	totalPkts := f.FwdPackets + f.BwdPackets
	totalFwdLen := fsum(f.FwdLengths)
	totalBwdLen := fsum(f.BwdLengths)
	allLengths := append(append([]float64{}, f.FwdLengths...), f.BwdLengths...)

	fwdMean, fwdStd := fmeanstd(f.FwdLengths)
	bwdMean, bwdStd := fmeanstd(f.BwdLengths)
	pktMean, pktStd := fmeanstd(allLengths)
	iatMean, iatStd := fmeanstd(interArrivalTimes(f.PacketTimes))

	flowRate := float64(totalPkts) / durSec
	fwdRate := float64(f.FwdPackets) / durSec
	bwdRate := float64(f.BwdPackets) / durSec

	avgPkt := 0.0
	if totalPkts > 0 {
		avgPkt = (totalFwdLen + totalBwdLen) / float64(totalPkts)
	}

	downUp := 0.0
	if f.FwdPackets > 0 {
		downUp = float64(f.BwdPackets) / float64(f.FwdPackets)
	}

	return [21]float64{
		durUs,
		float64(f.FwdPackets),
		float64(f.BwdPackets),
		totalBwdLen,
		flowRate,
		float64(f.SYNCount),
		float64(f.ACKCount),
		float64(f.RSTCount),
		float64(f.Key.DstPort),
		fwdMean,
		fwdStd,
		bwdMean,
		bwdStd,
		pktMean,
		pktStd,
		iatMean,
		iatStd,
		fwdRate,
		bwdRate,
		avgPkt,
		downUp,
	}
}

func fsum(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s
}

func fmeanstd(xs []float64) (mean, std float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	mean = fsum(xs) / float64(len(xs))
	v := 0.0
	for _, x := range xs {
		d := x - mean
		v += d * d
	}
	v /= float64(len(xs))
	return mean, math.Sqrt(v)
}

func interArrivalTimes(ts []time.Time) []float64 {
	if len(ts) < 2 {
		return nil
	}
	out := make([]float64, len(ts)-1)
	for i := 1; i < len(ts); i++ {
		out[i-1] = float64(ts[i].Sub(ts[i-1]).Microseconds())
	}
	return out
}

type OnExpiry func(*Flow)

type FlowTracker struct {
	mu       sync.Mutex
	flows    map[FlowKey]*Flow
	timeout  time.Duration
	onExpiry OnExpiry
}

func NewFlowTracker(timeout time.Duration, onExpiry OnExpiry) *FlowTracker {
	return &FlowTracker{
		flows:    make(map[FlowKey]*Flow),
		timeout:  timeout,
		onExpiry: onExpiry,
	}
}

func (t *FlowTracker) HandlePacket(pkt gopacket.Packet) {
	nl := pkt.NetworkLayer()
	tl := pkt.TransportLayer()
	if nl == nil || tl == nil {
		return
	}

	ts := pkt.Metadata().Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	src := nl.NetworkFlow().Src().String()
	dst := nl.NetworkFlow().Dst().String()
	pktLen := float64(len(pkt.Data()))

	var srcPort, dstPort uint16
	var proto string
	var syn, ack, rst, fin bool

	switch tr := tl.(type) {
	case *layers.TCP:
		srcPort, dstPort = uint16(tr.SrcPort), uint16(tr.DstPort)
		proto = "tcp"
		syn, ack, rst, fin = tr.SYN, tr.ACK, tr.RST, tr.FIN
	case *layers.UDP:
		srcPort, dstPort = uint16(tr.SrcPort), uint16(tr.DstPort)
		proto = "udp"
	default:
		return
	}

	key := FlowKey{src, dst, srcPort, dstPort, proto}
	rev := key.Reversed()

	t.mu.Lock()
	defer t.mu.Unlock()

	// expire immediately on teardown rather than waiting for the idle timeout
	if fin || rst {
		if f, ok := t.flows[key]; ok {
			delete(t.flows, key)
			go t.onExpiry(f)
		} else if f, ok := t.flows[rev]; ok {
			delete(t.flows, rev)
			go t.onExpiry(f)
		}
		return
	}

	if f, ok := t.flows[key]; ok {
		f.addPacket(pktLen, true, syn, ack, rst, ts)
		return
	}

	// checking the reverse direction, response packets arrive under the flipped key
	if f, ok := t.flows[rev]; ok {
		f.addPacket(pktLen, false, syn, ack, rst, ts)
		return
	}

	f := &Flow{Key: key, StartTime: ts, LastSeen: ts}
	f.addPacket(pktLen, true, syn, ack, rst, ts)
	t.flows[key] = f
}

func (t *FlowTracker) ExpireIdleFlows() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		t.mu.Lock()
		for key, f := range t.flows {
			if now.Sub(f.LastSeen) > t.timeout {
				delete(t.flows, key)
				go t.onExpiry(f)
			}
		}
		t.mu.Unlock()
	}
}

func (t *FlowTracker) FlushAll() {
	t.mu.Lock()
	flows := make([]*Flow, 0, len(t.flows))
	for key, f := range t.flows {
		delete(t.flows, key)
		flows = append(flows, f)
	}
	t.mu.Unlock()
	for _, f := range flows {
		t.onExpiry(f)
	}
}
