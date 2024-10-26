package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/multiformats/go-multiaddr"
)

const (
	MIN_PEERS    = 3              // 최소 필요 피어 수
	MAX_WAIT     = 2 * time.Minute // 최대 대기 시간
	RETRY_DELAY  = 5 * time.Second // 재시도 간격
)

type CustomValidator struct{}

func (v CustomValidator) Validate(key string, value []byte) error {
	return nil
}

func (v CustomValidator) Select(key string, values [][]byte) (int, error) {
	return 0, nil
}

func printHostAddress(h host.Host) {
	fmt.Println("Node address:")
	for _, addr := range h.Addrs() {
		fullAddr := addr.Encapsulate(multiaddr.StringCast("/p2p/" + h.ID().String()))
		fmt.Println(fullAddr)
	}
}

// waitForPeers waits until the DHT has enough peers
func waitForPeers(ctx context.Context, kadDHT *dht.IpfsDHT, minPeers int) error {
	fmt.Printf("Waiting for at least %d peers...\n", minPeers)
	
	deadline := time.Now().Add(MAX_WAIT)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			peers := kadDHT.RoutingTable().ListPeers()
			peerCount := len(peers)
			
			if peerCount >= minPeers {
				fmt.Printf("Found %d peers, proceeding with operations\n", peerCount)
				return nil
			}
			
			fmt.Printf("Currently connected to %d peers, waiting for more...\n", peerCount)
			if peerCount > 0 {
				fmt.Println("Connected peers:")
				for _, p := range peers {
					fmt.Printf("  - %s\n", p.String())
				}
			}
			
			time.Sleep(RETRY_DELAY)
		}
	}
	return fmt.Errorf("timeout waiting for minimum number of peers (%d)", minPeers)
}

func main() {
	isBootstrap := flag.Bool("bootstrap", false, "Set as bootstrap node")
	bootstrapAddr := flag.String("bootstrap-addr", "", "The multiaddress of the bootstrap node to connect to")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// TCP와 QUIC 모두 활성화
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
		),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelay(),
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer h.Close()

	printHostAddress(h)
	fmt.Println("Libp2p Host created. ID:", h.ID().String())

	validator := CustomValidator{}
	validatorMap := make(map[string]record.Validator)
	validatorMap["myapp"] = validator

	dhtOpts := []dht.Option{
		dht.Mode(dht.ModeServer),
		dht.Validator(validator),
		dht.ProtocolPrefix("/myapp"),
		dht.RoutingTableRefreshPeriod(10 * time.Second),
		dht.BootstrapPeers(),
	}

	kademliaDHT, err := dht.New(ctx, h, dhtOpts...)
	if err != nil {
		log.Fatal(err)
	}

	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup

	if *isBootstrap {
		fmt.Println("Running as a bootstrap node...")
		fmt.Println("Bootstrap node address for other peers:")
		printHostAddress(h)
	} else {
		if *bootstrapAddr == "" {
			log.Fatal("Bootstrap node address required")
		}

		maddr, err := multiaddr.NewMultiaddr(*bootstrapAddr)
		if err != nil {
			log.Fatalf("Invalid bootstrap address: %v", err)
		}

		peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Fatalf("Failed to parse peer address: %v", err)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.Connect(ctx, *peerInfo); err != nil {
				log.Printf("Failed to connect to bootstrap node: %v", err)
				return
			}
			fmt.Printf("Connected to bootstrap node: %s\n", peerInfo.ID)
		}()

		wg.Wait()
		time.Sleep(5 * time.Second)
	}

	// 최소 피어 수를 확보할 때까지 대기
	if err := waitForPeers(ctx, kademliaDHT, MIN_PEERS); err != nil {
		log.Printf("Warning: %v", err)
		fmt.Println("Continuing with available peers...")
	}

	// 데이터 저장 및 검색
	key := "/myapp/test"
	value := []byte("Hello, DHT!")

	maxAttempts := 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		fmt.Printf("\nAttempting to store value for key %s (attempt %d/%d)...\n", key, attempt, maxAttempts)
		
		err = kademliaDHT.PutValue(ctx, key, value)
		if err != nil {
			log.Printf("Error storing value (attempt %d/%d): %v\n", attempt, maxAttempts, err)
			peers := kademliaDHT.RoutingTable().ListPeers()
			fmt.Printf("Current peer count: %d\n", len(peers))
			if len(peers) > 0 {
				fmt.Println("Connected peers:")
				for _, p := range peers {
					fmt.Printf("  - %s\n", p.String())
				}
			}
			
			if attempt < maxAttempts {
				time.Sleep(5 * time.Second)
				continue
			}
		} else {
			fmt.Println("Successfully stored value!")
			break
		}
	}

	time.Sleep(5 * time.Second)

	fmt.Printf("\nAttempting to retrieve value for key %s...\n", key)
	retrievedValue, err := kademliaDHT.GetValue(ctx, key)
	if err != nil {
		log.Printf("Error retrieving value: %v\n", err)
	} else {
		fmt.Printf("Retrieved value: %s\n", string(retrievedValue))
	}

	// 피어 모니터링
	for {
		peers := kademliaDHT.RoutingTable().ListPeers()
		fmt.Printf("\nConnected to %d peers:\n", len(peers))
		for _, p := range peers {
			conns := h.Network().ConnsToPeer(p)
			for _, conn := range conns {
				fmt.Printf("- Peer: %s (Protocol: %s)\n", p.String(), conn.RemoteMultiaddr())
			}
		}
		time.Sleep(10 * time.Second)
	}
}