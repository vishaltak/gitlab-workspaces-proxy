# gitlab-workspaces-proxy

This proxy is responsible for authentication and authorization of the workspaces running in the cluster.
The proxy uses a central proxy design and automatically discovers backends based on annotations on the service.

## Local Development

```shell
# add sample_config with updated client_id and client_secret
cat <<EOT >> sample_config.yaml
auth:
  client_id: ""
  client_secret: ""
  host: http://gdk.test:3000
  redirect_uri: http://127.0.0.1:9876/auth/callback
  signing_key: passwordpassword
EOT

# run
make
```

## Installation Instructions

1. Create a namespace

    ```sh
    kubectl create ns gitlab-workspaces
    ```

2. Register an app on your GitLab instance

    Follow the instructions [here](https://docs.gitlab.com/ee/integration/oauth_provider.html) to register an oauth application.
    Set the redirect URI to http://hostname:port/auth/callback
    Make a note of the client id and secret generated

3. Set environment variables for secret

   ```sh
    export CLIENT_ID=[your client id]
    export CLIENT_SECRET=[your client secret]
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
    export RANCHER_NODE_IP=$(
      kubectl get nodes lima-rancher-desktop \
        --output jsonpath="{.status.addresses[?(@.type=='InternalIP')].address}"
    )
    # Set it to host.docker.internal if you are running GDK on 127.0.0.1
    # If you are running on any other private IP, use that IP
    export GDK_IP=172.16.123.1
    
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
            rewrite name $HOST_NAME_ONLY $GDK_IP
            prometheus :9153
            forward . /etc/resolv.conf
            cache 30
            loop
            reload
            loadbalance
        }
        import /etc/coredns/custom/*.server
      NodeHosts: |
        $RANCHER_NODE_IP lima-rancher-desktop
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
