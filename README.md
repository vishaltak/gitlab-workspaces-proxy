# gitlab-workspaces-proxy

This proxy is responsible for authentication and authorization of the [workspaces](https://docs.gitlab.com/ee/user/workspace/) running in the cluster.
The proxy uses a central proxy design and automatically discovers backends based on annotations on the service.


## Installation Instructions

Ensure that your Kubernetes cluster is running, and an Ingress controller is installed. `kubectl` and `helm` are required on your local machine for the installation steps. 

1. Generate TLS certificates

    TLS certificates have to be generated for 2 domains
    - The domain on which `gitlab-workspaces-proxy` will listen on. We'll call this `GITLAB_WORKSPACES_PROXY_DOMAIN`.
    - The domain on which all workspaces will be available. We'll call this `GITLAB_WORKSPACES_WILDCARD_DOMAIN`.

    You can generate certificates from any certificate authority.
    
    Here's an example using Let's Encrypt using [certbot](https://certbot.eff.org/). The CLI wizard will ask you for the ACME DNS challenge, and requires you to create TXT records at your DNS provider. 

    ```sh
    brew install certbot
    ```

    ```
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
    ```

    Note the certificate directories from the output, and update the environment variables below. 

    ```
    export WORKSPACES_DOMAIN_CERT="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_PROXY_DOMAIN}/fullchain.pem"
    export WORKSPACES_DOMAIN_KEY="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_PROXY_DOMAIN}/privkey.pem"
    export WILDCARD_DOMAIN_CERT="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_WILDCARD_DOMAIN}/fullchain.pem"
    export WILDCARD_DOMAIN_KEY="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_WILDCARD_DOMAIN}/privkey.pem"
    ```

    Optional: The `certbot` command sometimes creates a different path for the wildcard domain, using the proxy domain and a `-0001` prefix. 

    ```
    export WORKSPACES_DOMAIN_CERT="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_PROXY_DOMAIN}/fullchain.pem"
    export WORKSPACES_DOMAIN_KEY="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_PROXY_DOMAIN}/privkey.pem"
    export WILDCARD_DOMAIN_CERT="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_PROXY_DOMAIN}-0001/fullchain.pem"
    export WILDCARD_DOMAIN_KEY="${HOME}/.certbot/config/live/${GITLAB_WORKSPACES_PROXY_DOMAIN}-0001/privkey.pem"
    ```    

1. Register an app on your GitLab instance

    - Follow the instructions [here](https://docs.gitlab.com/ee/integration/oauth_provider.html) to register an OAuth application.
    - Set the redirect URI to `https://${GITLAB_WORKSPACES_PROXY_DOMAIN}/auth/callback` .
    - Set the scopes to `api`, `read_user`, `openid`, `profile` .
    - Make a note of the client id and secret generated (document them in a secrets vault, e.g. 1Password).

    ```sh
    export CLIENT_ID="your_application_id"
    export CLIENT_SECRET="your_application_secret"
    export REDIRECT_URI="https://${GITLAB_WORKSPACES_PROXY_DOMAIN}/auth/callback"
    ```

1. Generate SSH Host Keys. In this example we are generating an RSA key, however you can generate a ECDSA variant as well.

  ```sh
  ssh-keygen -f ssh-host-key -N '' -t rsa 
  export SSH_HOST_KEY=$(pwd)/ssh-host-key
  ```

1. Create configuration secret for the proxy and deploy the helm chart (**Ensure that you're using helm version v3.11.0 and above**)

    Create the signing key, and store it in safe place (e.g. use a secrets vault like 1Password to create and store the key). 

    ```sh
    export GITLAB_URL="https://gitlab.com"
    export SIGNING_KEY="a_random_key_consisting_of_letters_numbers_and_special_chars"
    ```

    ```sh
    helm repo add gitlab-workspaces-proxy \
      https://gitlab.com/api/v4/projects/gitlab-org%2fremote-development%2fgitlab-workspaces-proxy/packages/helm/devel
    ```

    The default ingress class name being used is `nginx`. Please modify the `ingress.className` parameter if you are using a different ingress or ingress class.

    ```sh
    helm repo update

    helm upgrade --install gitlab-workspaces-proxy \
      gitlab-workspaces-proxy/gitlab-workspaces-proxy \
      --version 0.1.8 \
      --namespace=gitlab-workspaces \
      --create-namespace \
      --set="auth.client_id=${CLIENT_ID}" \
      --set="auth.client_secret=${CLIENT_SECRET}" \
      --set="auth.host=${GITLAB_URL}" \
      --set="auth.redirect_uri=${REDIRECT_URI}" \
      --set="auth.signing_key=${SIGNING_KEY}" \
      --set="ingress.host.workspaceDomain=${GITLAB_WORKSPACES_PROXY_DOMAIN}" \
      --set="ingress.host.wildcardDomain=${GITLAB_WORKSPACES_WILDCARD_DOMAIN}" \
      --set="ingress.tls.workspaceDomainCert=$(cat ${WORKSPACES_DOMAIN_CERT})" \
      --set="ingress.tls.workspaceDomainKey=$(cat ${WORKSPACES_DOMAIN_KEY})" \
      --set="ingress.tls.wildcardDomainCert=$(cat ${WILDCARD_DOMAIN_CERT})" \
      --set="ingress.tls.wildcardDomainKey=$(cat ${WILDCARD_DOMAIN_KEY})" \
      --set="ssh.host_key=$(cat ${SSH_HOST_KEY})" \
      --set="ingress.className=nginx"
    ```

    Verify the created `Ingress` resource for the `gitlab-workspace` namespace:

    ```sh 
    kubectl get ingress -n gitlab-workspaces
    ```

    **Note**: Depending on which certificates you are using, they might require renewal. For example, Let's Encrypt certificates are valid for 3 months by default. After obtaining new certificates, re-run the `helm` command above to update the TLS certificates. 

1. Update your DNS records

   Point the `${GITLAB_WORKSPACES_PROXY_DOMAIN}` and `${GITLAB_WORKSPACES_WILDCARD_DOMAIN}` to load balancer where your ingress controller is listening.

   To test if the traffic is correctly reaching gitlab-workspaces-proxy, run an external curl command to inspect the proxy pod logs.

    Terminal 1:
    ```sh
    curl -vL ${GITLAB_WORKSPACES_PROXY_DOMAIN} 
    ```
    An authorization HTTP 400 error is expected here. The workspace creation in GitLab will take care of authorization handling.

    Terminal 2:
    ```sh
    kubectl logs -f -l app.kubernetes.io/name=gitlab-workspaces-proxy -n gitlab-workspaces
    ```
    In the logs, the error `could not find upstream workspace upstream not found` is expected in this case.

### Troubleshooting 

#### TLS certificate errors 

Troublshoot TLS certificates errors by connecting to the proxy domain and inspecting the certificate issue. You can use `openssl`, `sslscan`, etc. 

```sh 
openssl s_client -connect ${GITLAB_WORKSPACES_PROXY_DOMAIN}:443
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

If you want to update the container image version, change the configuration in the following places
- `Makefile` - `CONTAINER_IMAGE_VERSION` variable
- `helm/values.yaml` - `image.tag` variable

If you want to update the helm chart version, change the configuration in the following places
- `Makefile` - `CHART_VERSION` variable
- `helm/Chart.yaml` - `version` variable
- `helm/Chart.yaml` - `appVersion` variable if any changes have been made to the code

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

### Local Installation Instructions

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

1. Register an app on your GitLab instance - Follow the [Installation Instructions](#installation-instructions) outlined above.

1. Create configuration secret for the proxy and deploy the helm chart - Follow the [Installation Instructions](#installation-instructions) outlined above with the following overrides.

    ```sh
    export GITLAB_URL="http://gdk.test:3000"
    ```
