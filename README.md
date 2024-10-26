# mykademlia

## clone
``
curl -L https://git.io/vQhTU | bash -s -- --version 1.23.2
git clone https://github.com/xcosmos/exercise-kad-dht-go.git

``

## 최소 피어 요구사항 설정:

MIN_PEERS = 3: 최소 3개의 피어가 필요
MAX_WAIT = 2 * time.Minute: 최대 2분간 피어를 기다림
RETRY_DELAY = 5 * time.Second: 5초마다 피어 수 확인


## 피어 대기 기능 추가:

waitForPeers 함수로 최소 피어 수를 확보할 때까지 대기
현재 연결된 피어 수와 상세 정보 주기적으로 출력


## 연결성 개선:

EnableAutoRelay 추가
EnableHolePunching 추가로 NAT 통과 기능 향상


## 에러 처리 개선:

저장 시도 횟수를 5회로 증가
각 실패 시 현재 피어 상태 출력



## 실행 방법:

먼저 여러 부트스트랩 노드 실행:
```
bashCopygo run main.go --bootstrap
```

그 다음 일반 노드 실행:
```
go run main.go --bootstrap-addr <bootstrap-node-address>
```

DHT가 제대로 작동하려면:

최소 1개의 부트스트랩 노드
최소 2-3개의 일반 노드
가 필요합니다.

각 노드는 서로 다른 컴퓨터나 네트워크에서 실행하는 것이 좋습니다. 모두 같은 컴퓨터에서 실행하면 네트워크 연결성 테스트가 제한될 수 있습니다.