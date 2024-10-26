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
        )
        if err != nil {
                log.Fatal(err)
        }
        defer h.Close()

        printHostAddress(h)
        fmt.Println("Libp2p Host created. ID:", h.ID().String())

        // 커스텀 검증기 설정
        validator := CustomValidator{}
        validatorMap := make(map[string]record.Validator)
        validatorMap["myapp"] = validator

        // DHT 설정
        dhtOpts := []dht.Option{
                dht.Mode(dht.ModeServer),
                dht.Validator(validator),
                dht.ProtocolPrefix("/myapp"),
                dht.RoutingTableRefreshPeriod(10 * time.Second),
                dht.BootstrapPeers(), // 빈 부트스트랩 피어 리스트로 시작
        }

        kademliaDHT, err := dht.New(ctx, h, dhtOpts...)
        if err != nil {
                log.Fatal(err)
        }

        // DHT 부트스트랩
        if err = kademliaDHT.Bootstrap(ctx); err != nil {
                log.Fatal(err)
        }

        var wg sync.WaitGroup

        if *isBootstrap {
                fmt.Println("Running as a bootstrap node...")
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

                // 부트스트랩 노드에 연결
                wg.Add(1)
                go func() {
                        defer wg.Done()
                        if err := h.Connect(ctx, *peerInfo); err != nil {
                                log.Printf("Failed to connect to bootstrap node: %v", err)
                                return
                        }
                        fmt.Printf("Connected to bootstrap node: %s\n", peerInfo.ID)
                }()

                // 연결이 완료될 때까지 대기
                wg.Wait()
                time.Sleep(5 * time.Second) // 네트워크 안정화를 위한 대기
        }

        // DHT 라우팅 테이블이 채워질 때까지 대기
        fmt.Println("Waiting for peers...")
        for i := 0; i < 10; i++ {
                peers := kademliaDHT.RoutingTable().ListPeers()
                if len(peers) > 0 {
                        fmt.Printf("Found %d peers\n", len(peers))
                        break
                }
                time.Sleep(1 * time.Second)
        }

        // 데이터 저장 및 검색
        key := "/myapp/test"
        value := []byte("Hello, DHT!")

        // 값 저장 시도 (여러 번 재시도)
        for attempts := 0; attempts < 3; attempts++ {
                fmt.Printf("Attempting to store value for key %s (attempt %d)...\n", key, attempts+1)
                err = kademliaDHT.PutValue(ctx, key, value)
                if err != nil {
                        log.Printf("Error storing value (attempt %d): %v\n", attempts+1, err)
                        time.Sleep(2 * time.Second)
                        continue
                }
                fmt.Println("Successfully stored value")
                break
        }

        // 값이 네트워크에 전파되도록 대기
        time.Sleep(5 * time.Second)

        // 값 검색
        fmt.Printf("Attempting to retrieve value for key %s...\n", key)
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
                        // 각 피어에 대한 상세 정보 출력
                        conns := h.Network().ConnsToPeer(p)
                        for _, conn := range conns {
                                fmt.Printf("- Peer: %s (Protocol: %s)\n", p.String(), conn.RemoteMultiaddr())
                        }
                }
                time.Sleep(10 * time.Second)
        }
}
