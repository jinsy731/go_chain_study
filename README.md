# SimpleCoin: 실행 가이드

이 문서는 Go로 작성된 SimpleCoin 블록체인 코어의 실행 및 테스트 방법을 안내합니다.
현재 CLI(명령줄 인터페이스)를 통해 지갑 생성, 블록체인 생성, 잔액 조회가 가능합니다.

## 1\. 사전 준비 (Dependencies)

프로젝트를 실행하기 전에 필요한 Go 모듈을 설치해야 합니다.

```bash
# (최초 1회) 모듈 초기화
go mod init [your_module_name]

# 의존성 모듈 다운로드 (bbolt, btcec, base58 등)
go mod tidy
```

## 2\. 빌드 (Build)

Go 소스 코드를 컴파일하여 실행 파일을 생성합니다.

```bash
# 모든 *.go 파일을 컴파일하여 실행 파일을 생성합니다.
# (macOS/Linux):
go build -o simplecoin .

# (Windows):
# go build -o simplecoin.exe .
```

이후 모든 명령어는 생성된 `simplecoin` (또는 `simplecoin.exe`) 실행 파일을 기준으로 합니다.

## 3\. 실행 및 테스트 (Usage)

아래 단계를 순서대로 따라주세요.

### 1단계: 지갑 생성

가장 먼저 코인을 받고 보상을 저장할 지갑(주소)을 생성합니다.

```bash
./simplecoin createwallet
```

**[출력 예시]**

```
Your new address: 1A2bC... (이 주소를 복사합니다)
```

**중요:** 출력된 주소 문자열(`1A...`)을 복사하여 다음 단계에서 사용합니다.

### 2단계: 블록체인 생성 (최초 1회)

방금 생성한 지갑 주소로 제네시스 블록의 보상을 받는 **새 블록체인을 생성**합니다. 이 명령어는 `blockchain.db` 파일이 없을 때 **단 한 번만** 실행합니다.

```bash
# <YOUR_ADDRESS> 부분에 1단계에서 복사한 주소를 붙여넣습니다.
./simplecoin createblockchain -address <YOUR_ADDRESS>
```

**[출력 예시]**

```
Done! Blockchain created and UTXO set indexed.
```

이 명령이 성공하면 `blockchain.db` 파일이 생성됩니다.

### 3단계: 잔액 확인

제네시스 블록의 채굴 보상(`subsidy = 10`)이 1단계에서 만든 주소로 제대로 들어왔는지 확인합니다.

```bash
./simplecoin getbalance -address <YOUR_ADDRESS>
```

**[예상 출력]**

```
Balance of '1A2bC...': 10
```

### 4단계: (테스트) UTXO 세트 재인덱싱

`reindexutxo` 명령어는 `blocksBucket`의 모든 블록을 다시 스캔하여 `utxoBucket`을 새로 구축합니다. 데이터가 올바른지 검증할 때 유용합니다.

```bash
./simplecoin reindexutxo
```

**[출력 예시]**

```
Done! UTXO set has been reindexed.
```

### 5단계: (테스트) 잔액 재확인

재인덱싱 후에도 잔액이 동일하게 조회되는지 확인하여 `reindex` 기능이 올바르게 작동했는지 검증합니다.

```bash
./simplecoin getbalance -address <YOUR_ADDRESS>
```

**[예상 출력]**

```
Balance of '1A2bC...': 10
```

## 4\. 현재 사용 가능한 명령어

  * `createwallet`
      * 새 지갑(개인키/공개키 쌍)을 생성하고 Base58 주소를 출력합니다.
  * `createblockchain -address <ADDRESS>`
      * 새 블록체인 DB와 제네시스 블록을 생성합니다.
      * 제네시스 블록의 보상은 지정된 `<ADDRESS>`로 전송됩니다.
  * `getbalance -address <ADDRESS>`
      * 해당 `<ADDRESS>`의 UTXO Set을 스캔하여 총 잔액을 출력합니다.
  * `reindexutxo`
      * `utxoBucket`을 삭제하고, 전체 블록체인을 스캔하여 UTXO Set을 새로 구축합니다.