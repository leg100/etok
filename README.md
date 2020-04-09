# stok

**s**upercharged **t**erraform **o**n **k**ubernetes

## install

```
kubectl create secret generic stok --from-file=google-credentials.json=[path to service account key]
```

```
helm upgrade -i stok stok/charts
```
