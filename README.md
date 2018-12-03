# AKMS - A distributed key management system

Implements the [tendermint local file kms](https://github.com/tendermint/tendermint/tree/master/privval) with a 
[Raft store](https://github.com/otoolep/hraftd/tree/master/store).

This is based on my initial version that I had used with "Validatorich" during Game of Stake for
an active active validator setup in a k8s cluster.
A 3 node akms cluster was used to sync the validators and prevent double signing.
A k8s ingress with health checks on `/sign/ready` routed requests to the RAFT master only.
 

## Components

* Akms
Server side KMS

* Akms proxy
Local proxy process as bridge between Tendermint and akms cluster


## To do
- [ ] Add load balancer to compose setup to route to RAFT master only]
- [ ] Add k8s example
- [ ] Better docs
- [ ] Metrics
- [ ] Address security concerns with keys on the file system  
- ...

## Build Artifacts
```bash
make dist
```

## Development

* Integration test
```bash
docker-compose up
go test --count=1 -v ./cmd/akms -tags=integration
# run benchmarks
go test  -bench=. ./cmd/akms  -tags=integration

```


## Local runtime environment
* Start kms
```bash
docker-compose up
```
* Find leader
```
curl -v  127.0.0.1:808{1,2,3}/sign/ready
```
* Start proxy
```
go run ./cmd/akmsproxy/main.go  --chain-id=akms-example -socket-addr=unix://./tmp/akms-proxy.sock -server-addr=http://127.0.0.1:8082
```
* Start tendermint
```bash


# start tendermint
tendermint init --home=$(pwd)/tmp
# replace chain_id in genesis
cat ./tmp/config/genesis.json | jq '.chain_id = "akms-example" | .validators[0].address="6AC7BA59D2B177FE1B73AD21E5EE1DE446A4DE21" | .validators[0].pub_key.value = "smAHzSepeo7g5jAFt5GudpW7fHxBdkZTRmZ/K+54xx0="' > ./tmp/config/genesis.json_new
rm -f ./tmp/config/genesis.json
mv ./tmp/config/genesis.json_new ./tmp/config/genesis.json

# set socket in config.toml (OSX)
 
sed -i '' "/^\s*priv_validator_laddr /s/=.*$/= \"unix:\/\/.\/tmp\/akms-proxy.sock\"/" ./tmp/config/config.toml

# start tendermit
tendermint node --proxy_app=kvstore --home=$(pwd)/tmp

# start in new window akms local proxy process

go run ./cmd/akmsproxy/main.go  --chain-id=akms-example -socket-addr=unix://./tmp/akms-proxy.sock -server-addr=http://127.0.0.1:8083

# send some data
curl -s 'localhost:26657/broadcast_tx_commit?tx="abcd"'

# check
curl -s 'localhost:26657/abci_query?data="abcd"'

```
See https://tendermint.com/docs/introduction/quick-start.html#local-node
