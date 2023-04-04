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

## Building and Publishing Container Image

```shell
# to build the image
make docker-build

# to publish the image
make docker-publish
```

If you want to update the image version, change the configuration in the following places
- `Makefile` - `CONTAINER_IMAGE_VERSION` variable
- `deploy/k8s/deploy.yaml` - `image` attribute for the `proxy` container

## Installation Instructions

1. Create a namespace

    ```sh
    kubectl create ns gitlab-workspaces
    ```

2. Register an app on your GitLab instance

    - Follow the instructions [here](https://docs.gitlab.com/ee/integration/oauth_provider.html) to register an OAuth application.
    - Set the redirect URI to `http://workspaces.localdev.me/auth/callback` .
    - Set the scopes to `openid`, `profile`, `email` .
    - Make a note of the client id and secret generated.

3. Create configuration secret for the proxy

    ```sh
    export CLIENT_ID="your_application_id"
    export CLIENT_SECRET="your_application_secret"
    export GITLAB_URL="http://gdk.test:3000"
    export REDIRECT_URI=http://workspaces.localdev.me/auth/callback
    export SIGNING_KEY="a_random_key_consisting_of_letters_numbers_and_special_chars"

    SECRET_DATA=$(cat <<EOF
    auth:
      client_id: $CLIENT_ID
      client_secret: $CLIENT_SECRET
      host: $GITLAB_URL
      redirect_uri: $REDIRECT_URI
      signing_key: $SIGNING_KEY
    EOF
    )

    kubectl create secret generic gitlab-workspaces-proxy -n gitlab-workspaces \
      --from-literal=config.yaml=$SECRET_DATA
    ```

4. Apply the manifests

    ```sh
    kubectl apply -k ./deploy/k8s -n gitlab-workspaces
    ```

5. Create a DNS entry in core dns to enable the auth proxy to reach gdk from your cluster

    ```sh
    export GITLAB_HOST_WITHOUT_PORT=$(echo $GITLAB_URL | cut -d":" -f1 | cut -d "/" -f3)
    export RANCHER_NODE_IP=$(
      kubectl get nodes lima-rancher-desktop \
        --output jsonpath="{.status.addresses[?(@.type=='InternalIP')].address}"
    )
    ```

    If you are running GDK on 127.0.0.1, use [`host.docker.internal`](https://github.com/rancher-sandbox/rancher-desktop/issues/3686#issuecomment-1379539298)

    ```sh
    export GDK_IP=host.docker.internal
    ```

    If you are running on any other private IP, use that IP. Assuming that IP is `172.16.123.1`

    ```sh
    export GDK_IP=172.16.123.1
    ```

    Update CodeDNS to route all traffic from host `$GITLAB_HOST_WITHOUT_PORT` to the IP `$GDK_IP`

    ```sh
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
            rewrite name $GITLAB_HOST_WITHOUT_PORT $GDK_IP
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
