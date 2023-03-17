# Workspace Proxy

This proxy is reponsible for authentication and authorization of the workspaces running in the cluster.
The proxy uses a central proxy design and automatically discovers backends based on annotations on the service.

# Installation Instructions

1. Create a namespace

```sh
kubectl create ns gitlab-workspaces
```

2. Register an app on your GitLab instance

Follow the instuctions [here](https://docs.gitlab.com/ee/integration/oauth_provider.html) to register an oauth application.
Set the redirect URI to http://hostname:port/auth/callback
Make a note of the client id and secret generated

3. Set environment variables for secret
```sh
export CLIENT_ID=[your client id]
export CLIENT_SECRET=[your client secret>
export HOST_NAME=[url for GDK]
export REDIRECT_URI=http://workspaces.localdev.me/auth/callback
export SIGNING_KEY=[a random key consisting of letters, numbers and special chars]
```

4. Create the secret
```sh
SECRET_DATA=$(cat <<EOF
auth:
  client_id: $CLIENT_ID
  client_secret: $CLIENT_SECRET
  host: $HOST_NAME
  redirect_uri: $REDIRECT_URI
  signing_key: $SIGNING_KEY
EOF
)

kubectl create secret generic workspace-proxy -n gitlab-workspaces \
	--from-literal=config.yaml=$SECRET_DATA
```

5. Apply the manifests

```sh
kubectl apply -k ./deploy/k8s -n gitlab-workspaces
```

6. Create a DNS entry in core dns to enable the auth proxy to reach gdk from your cluster

```sh
export HOST_NAME_ONLY=[Host name without port]

cat <<EOF | kubectl apply -f -
apiVersion: v1
data:
  Corefile: |
    .:53 {
        errors
        health
        ready
        kubernetes cluster.local in-addr.arpa ip6.arpa {
          pods insecure
          fallthrough in-addr.arpa ip6.arpa
        }
        hosts /etc/coredns/NodeHosts {
          ttl 60
          reload 15s
          fallthrough
        }
        rewrite name $HOST_NAME_ONLY host.docker.internal
        prometheus :9153
        forward . /etc/resolv.conf
        cache 30
        loop
        reload
        loadbalance
    }
    import /etc/coredns/custom/*.server
  NodeHosts: |
    192.168.1.107 lima-rancher-desktop
kind: ConfigMap
metadata:
  annotations:
    objectset.rio.cattle.io/id: ""
    objectset.rio.cattle.io/owner-gvk: k3s.cattle.io/v1, Kind=Addon
    objectset.rio.cattle.io/owner-name: coredns
    objectset.rio.cattle.io/owner-namespace: kube-system
  name: coredns
  namespace: kube-system
EOF

```