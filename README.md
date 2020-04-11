# stok

**s**upercharged **t**erraform **o**n **k**ubernetes

## install

```
kubectl create secret generic stok --from-file=google-credentials.json=[path to service account key]
```

```
helm repo add goalspike https://goalspike-charts.storage.googleapis.com
helm install stok goalspike/stok
```
