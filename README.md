# gitlab-workspaces-proxy

This proxy is responsible for authentication and authorization of the workspaces running in the cluster.
The proxy uses a central proxy design and automatically discovers backends based on annotations on the service.


## Installation Instructions

1. Generate TLS certificates

    TLS certificates have to be generated for 2 domains
    - The domain on which `gitlab-workspaces-proxy` will listen on. We'll call this `GITLAB_WORKSPACES_PROXY_DOMAIN`.
    - The domain on which all workspaces will be available. We'll call this `GITLAB_WORKSPACES_WILDCARD_DOMAIN`

    You can generate certificates from any certificate authority. Here's an example using Let's Encrypt.
    ```sh
    brew install certbot

    export EMAIL="YOUR_EMAIL@example.dev"
    export GITLAB_WORKSPACES_PROXY_DOMAIN="workspaces.example.dev"
    export GITLAB_WORKSPACES_WILDCARD_DOMAIN="*.workspaces.example.dev"

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

    export WORKSPACES_DOMAIN_CERT="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_PROXY_DOMAIN}/fullchain.pem"
    export WORKSPACES_DOMAIN_KEY="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_PROXY_DOMAIN}/privkey.pem"
    export WILDCARD_DOMAIN_CERT="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_WILDCARD_DOMAIN}/fullchain.pem"
    export WILDCARD_DOMAIN_KEY="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_WILDCARD_DOMAIN}/privkey.pem"
    ```

1. Register an app on your GitLab instance

    - Follow the instructions [here](https://docs.gitlab.com/ee/integration/oauth_provider.html) to register an OAuth application.
    - Set the redirect URI to `https://${GITLAB_WORKSPACES_PROXY_DOMAIN}/auth/callback` .
    - Set the scopes to `api`, `read_user`, `openid`, `profile` .
    - Make a note of the client id and secret generated.

    ```sh
    export CLIENT_ID="your_application_id"
    export CLIENT_SECRET="your_application_secret"
    export REDIRECT_URI="https://${GITLAB_WORKSPACES_PROXY_DOMAIN}/auth/callback"
    ```

1. Create configuration secret for the proxy and deploy the helm chart (**Ensure that you're using helm version v3.11.0 and above**)

    ```sh
    export GITLAB_URL="https://gitlab.com"
    export SIGNING_KEY="a_random_key_consisting_of_letters_numbers_and_special_chars"

    helm repo add gitlab-workspaces-proxy \
      https://gitlab.com/api/v4/projects/gitlab-org%2fremote-development%2fgitlab-workspaces-proxy/packages/helm/devel

    helm repo update

    helm upgrade --install gitlab-workspaces-proxy \
      gitlab-workspaces-proxy/gitlab-workspaces-proxy \
      --version 0.1.5 \
      --namespace=gitlab-workspaces \
      --create-namespace \
      --set="auth.client_id=$CLIENT_ID" \
      --set="auth.client_secret=$CLIENT_SECRET" \
      --set="auth.host=$GITLAB_URL" \
      --set="auth.redirect_uri=$REDIRECT_URI" \
      --set="auth.signing_key=$SIGNING_KEY" \
      --set="ingress.tls.workspaceDomainCert=$(cat $WORKSPACES_DOMAIN_CERT)" \
      --set="ingress.tls.workspaceDomainKey=$(cat $WORKSPACES_DOMAIN_KEY)" \
      --set="ingress.tls.wildcardDomainCert=$(cat $WILDCARD_DOMAIN_CERT)" \
      --set="ingress.tls.wildcardDomainKey=$(cat $WILDCARD_DOMAIN_KEY)" \
      --set="ingress.className=nginx"
    ```

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

## Local Installation Instructions

Follow the [Installation Instructions](#installation-instructions) outlined above

1. Generate TLS certificates

    For local development, you can use [mkcert](https://github.com/FiloSottile/mkcert)
    ```sh
    brew install mkcert
    brew install nss

    export GITLAB_WORKSPACES_PROXY_DOMAIN="workspaces.localdev.me"
    export GITLAB_WORKSPACES_WILDCARD_DOMAIN="*.workspaces.localdev.me"
    mkcert -install
    mkcert "${GITLAB_WORKSPACES_PROXY_DOMAIN}" "${GITLAB_WORKSPACES_WILDCARD_DOMAIN}"

    export WORKSPACES_DOMAIN_CERT="${PWD}/workspaces.localdev.me+1.pem"
    export WORKSPACES_DOMAIN_KEY="${PWD}/workspaces.localdev.me+1-key.pem"
    export WILDCARD_DOMAIN_CERT="${PWD}/workspaces.localdev.me+1.pem"
    export WILDCARD_DOMAIN_KEY="${PWD}/workspaces.localdev.me+1-key.pem"
    ```

1. Register an app on your GitLab instance -  - Follow the [Installation Instructions](#installation-instructions) outlined above.

1. Create configuration secret for the proxy and deploy the helm chart - Follow the [Installation Instructions](#installation-instructions) outlined above with the following overrides.

    ```sh
    export GITLAB_URL="http://gdk.test:3000"
    ```
