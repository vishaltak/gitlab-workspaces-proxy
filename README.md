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
  protocol: http
port: 9876
metrics_path: "/metrics"
EOT

# run
make
```

## Building and Publishing Assets

```shell
# build the image
make docker-build

# publish the image
make docker-publish

# package helm chart
make helm-package

# publish helm chart
make helm-publish
```

If you want to update the image version, change the configuration in the following places
- `Makefile` - `CONTAINER_IMAGE_VERSION` variable
- `helm/values.yaml` - `image.tag` variable

## Installation Instructions

1. Create a namespace

    ```sh
    kubectl create ns gitlab-workspaces
    ```

1. Register an app on your GitLab instance

    - Follow the instructions [here](https://docs.gitlab.com/ee/integration/oauth_provider.html) to register an OAuth application.
    - Set the redirect URI to `https://workspaces.localdev.me/auth/callback` .
    - Set the scopes to `api`, `read_user`, `openid`, `profile` .
    - Make a note of the client id and secret generated.

1. Generate TLS certificates

    TLS certificates have to be generated for 2 domains
    - The domain on which `gitlab-workspaces-proxy` will listen on. We'll call this `GITLAB_WORKSPACES_PROXY_DOMAIN`.
    - The domain on which all workspaces will be available. We'll call this `GITLAB_WORKSPACES_WILDCARD_DOMAIN`

    For real domains, you can generate certificates from any certificate authority. Here's an example using Let's Encrypt.
    ```sh
    brew install certbot

    export EMAIL="YOUR_EMAIL@example.com"
    export DOMAIN="example.remote.gitlab.dev"

    certbot -d "${GITLAB_WORKSPACES_PROXY_DOMAIN}" \
      -m "${EMAIL}" \
      --config-dir ~/.certbot/config \
      --logs-dir ~/.certbot/logs \
      --work-dir ~/.certbot/work \
      --manual \
      --preferred-challenges dns certonly

    certbot -d "${GITLAB_WORKSPACES_WILDCARD_DOMAIN}" \
      -m "${EMAIL}" \
      --config-dir ~/.certbot/config \
      --logs-dir ~/.certbot/logs \
      --work-dir ~/.certbot/work \
      --manual \
      --preferred-challenges dns certonly
    
    kubectl create secret tls gitlab-workspaces-proxy-tls -n gitlab-workspaces \
      --cert="~/.certbot/config/live/${GITLAB_WORKSPACES_PROXY_DOMAIN}/fullchain.pem" \
      --key="~/.certbot/config/live/${GITLAB_WORKSPACES_PROXY_DOMAIN}/privkey.pem"
    
    kubectl create secret tls gitlab-workspaces-wildcard-tls -n gitlab-workspaces \
      --cert="~/.certbot/config/live/${GITLAB_WORKSPACES_WILDCARD_DOMAIN}/fullchain.pem" \
      --key="~/.certbot/config/live/${GITLAB_WORKSPACES_WILDCARD_DOMAIN}/privkey.pem"
    ```

    For local development, you can use [mkcert](https://github.com/FiloSottile/mkcert)
    ```sh
    brew install mkcert
    brew install nss

    export GITLAB_WORKSPACES_PROXY_DOMAIN="workspaces.localdev.me"
    export GITLAB_WORKSPACES_WILDCARD_DOMAIN="*.workspaces.localdev.me"
    mkcert -install
    mkcert "${GITLAB_WORKSPACES_PROXY_DOMAIN}" "${GITLAB_WORKSPACES_WILDCARD_DOMAIN}"

    kubectl create secret tls gitlab-workspaces-proxy-tls -n gitlab-workspaces \
      --cert="./workspaces.localdev.me+1.pem" \
      --key="./workspaces.localdev.me+1-key.pem"
    
    kubectl create secret tls gitlab-workspaces-wildcard-tls -n gitlab-workspaces \
      --cert="./workspaces.localdev.me+1.pem" \
      --key="./workspaces.localdev.me+1-key.pem"
    ```

1. Create configuration secret for the proxy and deploy the helm chart. (**Ensure that you're using helm version v3.11.0 and above**)

    ```sh
    export CLIENT_ID="your_application_id"
    export CLIENT_SECRET="your_application_secret"
    export GITLAB_URL="http://gdk.test:3000"
    export REDIRECT_URI=https://workspaces.localdev.me/auth/callback
    export SIGNING_KEY="a_random_key_consisting_of_letters_numbers_and_special_chars"

    helm repo add gitlab-workspaces-proxy \
      https://gitlab.com/api/v4/projects/gitlab-org%2fremote-development%2fgitlab-workspaces-proxy/packages/helm/devel

    helm repo update

    helm upgrade --install gitlab-workspaces-proxy \
      gitlab-workspaces-proxy/gitlab-workspaces-proxy \
      --version 0.1.4 \
      --namespace=gitlab-workspaces \
      --set="auth.client_id=$CLIENT_ID" \
      --set="auth.client_secret=$CLIENT_SECRET" \
      --set="auth.host=$GITLAB_URL" \
      --set="auth.redirect_uri=$REDIRECT_URI" \
      --set="auth.signing_key=$SIGNING_KEY" \
      --set="ingress.className=nginx"
    ```

1. Create a DNS entry in core dns to enable the auth proxy to reach gdk from your cluster

    ```sh
    export GITLAB_HOST_WITHOUT_PORT=$(echo $GITLAB_URL | cut -d":" -f2 | cut -d "/" -f3)
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
