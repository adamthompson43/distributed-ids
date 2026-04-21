package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

func main() {
	iface     := flag.String("interface", "en0", "Network interface for live capture (ignored if -pcap is set)")
	pcapFile  := flag.String("pcap", "", "Path to a .pcap file to replay instead of live capture")
	modelPath := flag.String("model", "../model_params.json", "Path to model JSON")
	nodeID    := flag.String("node-id", "node1", "Unique identifier for this node")
	listenAddr := flag.String("listen", "", "Address to serve /vote on (e.g. :8081); omit for standalone mode")
	peersFlag  := flag.String("peers", "", "Comma-separated peer base URLs (e.g. http://localhost:8082,http://localhost:8083)")
	dbDSN      := flag.String("db", "", "PostgreSQL DSN (optional)")
	flag.Parse()

	const (
		timeout          = 30 * time.Second
		minPkts          = 2
		bfWindow         = 60 * time.Second
		bfThreshold      = 50
		bfCooldown       = 30 * time.Second
		consensusTimeout = 2 * time.Second
	)

	det, err := LoadDetector(*modelPath)
	if err != nil {
		log.Fatalf("Failed to load model: %v", err)
	}
	log.Printf("Model loaded - %d features, %d PCA components, threshold=%.4f",
		det.NFeatures, det.NComponents, det.LRThreshold)
	log.Printf("Brute-force tracker - window=%s threshold=%d cooldown=%s",
		bfWindow, bfThreshold, bfCooldown)

	var store *Store
	if *dbDSN != "" {
		var err2 error
		store, err2 = NewStore(*nodeID, *dbDSN)
		if err2 != nil {
			log.Fatalf("Failed to connect to database: %v", err2)
		}
		defer store.Close()
		log.Printf("Database connected")
	} else {
		log.Printf("Database: disabled (no -db flag)")
	}

	var peers []string
	if *peersFlag != "" {
		for _, p := range strings.Split(*peersFlag, ",") {
			if p = strings.TrimSpace(p); p != "" {
				peers = append(peers, p)
			}
		}
	}
	cm := NewConsensusManager(*nodeID, peers, det, consensusTimeout)
	if *listenAddr != "" {
		cm.Start(*listenAddr)
	} else {
		log.Printf("Consensus: standalone mode (no -listen address configured)")
	}

	go store.PollPeerHealth(peers, 30*time.Second)

	bf := NewBruteForceTracker(bfWindow, bfThreshold, bfCooldown,
		func(srcIP string, dstPort uint16, count int) {
			fmt.Printf("[BRUTE-FORCE] %s → port %d  %d connections in %s\n",
				srcIP, dstPort, count, bfWindow)
			go store.SaveBruteForce(srcIP, dstPort, count)
		},
	)
	go bf.EvictStaleEntries()

	var handle *pcap.Handle
	if *pcapFile != "" {
		handle, err = pcap.OpenOffline(*pcapFile)
		if err != nil {
			log.Fatalf("Failed to open pcap file %q: %v", *pcapFile, err)
		}
		log.Printf("Reading from file: %s", *pcapFile)
	} else {
		handle, err = pcap.OpenLive(*iface, 65536, true, pcap.BlockForever)
		if err != nil {
			log.Fatalf("Failed to open interface %q: %v\n(Hint: try running with sudo", *iface, err)
		}
		log.Printf("Capturing on %s - press Ctrl+C to stop", *iface)
	}
	defer handle.Close()

	if err := handle.SetBPFFilter("ip"); err != nil {
		log.Fatalf("BPF filter error: %v", err)
	}

	onExpiry := func(f *Flow) {
		if f.FwdPackets+f.BwdPackets < minPkts {
			return
		}

		feats := sanitise(f.Features())
		anomaly, score := det.IsAnomaly(feats)
		if anomaly {
			result := cm.RequestVotes(f.Key, feats, anomaly, score)
			if result.Anomaly {
				fmt.Printf("[CONSENSUS-ANOMALY] %s → %s:%d  score=%.4f  votes=%d/%d  pkts=%d/%d  dur=%.2fs\n",
					f.Key.SrcIP, f.Key.DstIP, f.Key.DstPort,
					result.LocalScore, result.YesVotes, result.TotalVotes,
					f.FwdPackets, f.BwdPackets, f.Duration().Seconds(),
				)
				go store.SaveAnomaly(f, feats, result, "consensus_anomaly")
			} else {
				fmt.Printf("[OVERRULED] %s → %s:%d  local_score=%.4f  votes=%d/%d  pkts=%d/%d  dur=%.2fs\n",
					f.Key.SrcIP, f.Key.DstIP, f.Key.DstPort,
					result.LocalScore, result.YesVotes, result.TotalVotes,
					f.FwdPackets, f.BwdPackets, f.Duration().Seconds(),
				)
				go store.SaveAnomaly(f, feats, result, "overruled")
			}
		}

		if f.Key.Proto == "tcp" {
			bf.Record(f.Key.SrcIP, f.Key.DstPort, f.LastSeen)
		}
	}

	tracker := NewFlowTracker(timeout, onExpiry)
	go tracker.ExpireIdleFlows()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("Shutting down - flushing remaining flows...")
		tracker.FlushAll()
		os.Exit(0)
	}()

	src := gopacket.NewPacketSource(handle, handle.LinkType())
	for pkt := range src.Packets() {
		tracker.HandlePacket(pkt)
	}

	if *pcapFile != "" {
		log.Println("EOF - flushing remaining flows...")
		tracker.FlushAll()
		log.Println("PCAP replay complete - /vote endpoint still active. Ctrl+C to exit.")
		select {}
	}
}
