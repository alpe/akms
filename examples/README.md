# Examples
Install [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)

## Apply
```bash
kubelct apply -f minikube/kms
kubelct apply -f minikube/validator
```
## Manual


* Open RPC port
```bash
kubectl port-forward validator-a-0 26657:26657

```
* Send test data
```bash
curl -s "$(minikube service validator-a-rpc --url)/broadcast_tx_commit?tx=\"abcd\""
```
