module mykademlia

go 1.23.2

package main

import (
        "context"
        "flag"
        "fmt"
        "log"
        "time"

        libp2p "github.com/libp2p/go-libp2p"
        dht "github.com/libp2p/go-libp2p-kad-dht"
        "github.com/libp2p/go-libp2p/core/host"
        "github.com/libp2p/go-libp2p/core/peer"
        "github.com/multiformats/go-multiaddr"
)

// Node 주소 출력 함수
func printHostAddress(h host.Host) {
        fmt.Println("Node address:")
        for _, addr := range h.Addrs() {
                fullAddr := addr.Encapsulate(multiaddr.StringCast("/p2p/" + h.ID().String()))
                fmt.Println(fullAddr)
        }
}

func main() {
        // Define a flag to set the node as a bootstrap node
        isBootstrap := flag.Bool("bootstrap", false, "Set as bootstrap node")

        // Define a flag to accept a bootstrap node address if not running as a bootstrap node
        bootstrapAddr := flag.String("bootstrap-addr", "", "The multiaddress of the bootstrap node to connect to")

        flag.Parse()

        // 컨텍스트 생성
        ctx := context.Background()

        // 기본 libp2p 호스트 생성
        h, err := libp2p.New()
        if err != nil {
                panic(err)
        }
        defer h.Close()

        // 호스트 주소 출력
        printHostAddress(h)

        fmt.Println("Libp2p Host created. ID:", h.ID().String())

        // DHT 인스턴스 생성
        kademliaDHT, err := dht.New(ctx, h)
        if err != nil {
                panic(err)
        }

        // 노드가 부트스트랩 노드로 설정된 경우
        if *isBootstrap {
                fmt.Println("Running as a bootstrap node...")
                err = kademliaDHT.Bootstrap(ctx)
                if err != nil {
                        panic(err)
                }
                fmt.Println("DHT Bootstrapped")
        } else {
                // 부트스트랩 노드가 아닌 경우, 부트스트랩 노드의 주소 입력 필요
                if *bootstrapAddr == "" {
                        log.Fatal("You must specify a bootstrap node address with --bootstrap-addr when not running as a bootstrap node")
                }

                // 입력된 부트스트랩 노드 주소로 연결 시도
                maddr, err := multiaddr.NewMultiaddr(*bootstrapAddr)
                if err != nil {
                        log.Fatalf("Invalid bootstrap address: %v", err)
                }
                peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
                if err != nil {
                        log.Fatalf("Failed to parse peer address: %v", err)
                }

                if err := h.Connect(ctx, *peerInfo); err != nil {
                        log.Fatalf("Failed to connect to bootstrap node: %v", err)
                }
                fmt.Printf("Connected to bootstrap node: %s\n", peerInfo.ID)

                // DHT 부트스트랩
                err = kademliaDHT.Bootstrap(ctx)
                if err != nil {
                        panic(err)
                }
                fmt.Println("DHT Bootstrapped with bootstrap node")
        }

        // 피어 찾기
        /*
        for {
                fmt.Println("Searching for peers in DHT...")
                peers := kademliaDHT.RoutingTable().ListPeers()
                for _, p := range peers {
                        fmt.Printf("Found peer: %s\n", p.String())
                }
                time.Sleep(10 * time.Second)
        }
        */
        time.Sleep(10 * time.Second)

        // 데이터 저장 및 검색 예시
        key := "/tmp/example-key"
        value := []byte("example-value")

        // 값 저장 (Put)
        err = kademliaDHT.PutValue(ctx, key, value)
        if err != nil {
                fmt.Printf("Error storing value for key %s: %s\n", key, err)
        } else {
                fmt.Printf("Stored value for key %s\n", key)
        }

        // 값 검색 (Get)
        retrievedValue, err := kademliaDHT.GetValue(ctx, key)
        if err != nil {
                fmt.Printf("Error retrieving value for key %s: %s\n", key, err)
        } else {
                fmt.Printf("Retrieved value for key %s: %s\n", key, string(retrievedValue))
        }
}

