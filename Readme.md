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
Set the redirect Uri to http://hostname:port/auth/callback
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
base_url: "workspaces.localdev.me"
EOF
)

kubectl create secret generic workspace-proxy -n gitlab-workspaces \
	--from-literal=config.yaml=$SECRET_DATA
```

5. Apply the manifests

```sh
kubectl apply -k ./deploy/k8s -n gitlab-workspaces
```

