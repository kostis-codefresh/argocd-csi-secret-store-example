# Argo CD with Vault and the CSI Secrets Store Driver demo

Example of using the CSI Secrets Store Driver with Vault and Argo CD. The demo application is a Go web server that reads database credentials from a CSI-mounted volume and automatically reloads them when they change—no pod restart required.

---

## How it works

```
HashiCorp Vault  →  CSI Secrets Store Driver  →  Volume mount  →  Application
```

The CSI driver fetches the secret directly from Vault and mounts it as a file at `/secrets/credentials`. Secret rotation is enabled so the file is refreshed periodically without restarting the pod.

---

## 1. Prerequisites

- A Kubernetes cluster with `cluster-admin` permissions
- `kubectl` and [Argo CD installed](https://argo-cd.readthedocs.io/en/stable/getting_started/) in the `argocd` namespace

```shell
kubectl create namespace argocd
kubectl apply -n argocd --server-side --force-conflicts -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl patch svc argocd-server -n argocd -p '{"spec": {"type": "LoadBalancer"}}'
kubectl get all -n argocd
argocd admin initial-password -n argocd
argocd login localhost:80
```

---

## 2. Set up Vault with CSI support

Install Vault in dev mode with the CSI provider enabled via Argo CD:

```bash
argocd app create vault \
--project default \
--repo https://helm.releases.hashicorp.com \
--helm-chart vault \
--revision 0.28.0 \
--sync-policy auto \
--sync-option CreateNamespace=true \
--parameter server.dev.enabled=true \
--parameter injector.enabled=false \
--parameter csi.enabled=true \
--dest-namespace vault \
--dest-server https://kubernetes.default.svc
```

Exec into the pod and configure it:

```bash
kubectl exec -it vault-0 -n vault -- /bin/sh
```

```sh
vault login root

# Write the demo credentials as a single .properties file value
vault kv put secret/mysql_credentials \
  credentials='db_con="mysql.example.com:3306"
db_user="my_demo_user"
db_password="my_demo_password"'

# Enable Kubernetes auth
vault auth enable kubernetes
vault write auth/kubernetes/config \
  kubernetes_host="https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT"

# Policy granting read access to the secret
vault policy write csi-read-policy - <<EOF
path "secret/data/mysql_credentials" {
  capabilities = ["read"]
}
EOF

vault write auth/kubernetes/role/demo \
  bound_service_account_names=secret-sa \
  bound_service_account_namespaces=default \
  policies=csi-read-policy \
  ttl=24h

exit
```

---

## 3. Install the CSI Secrets Store Driver

```bash
helm repo add secrets-store-csi-driver https://kubernetes-sigs.github.io/secrets-store-csi-driver/charts
helm install csi-secrets-store secrets-store-csi-driver/secrets-store-csi-driver \
  --namespace kube-system \
  --set enableSecretRotation=true \
  --set rotationPollInterval=15s
```

---

## 4. Apply the SecretProviderClass via Argo CD

The `SecretProviderClass` in `manifests/vault-integration` tells the CSI driver how to connect to Vault and which secret to fetch.

```bash
argocd app create vault-secret-store \
--project default \
--repo https://github.com/kostis-codefresh/argocd-csi-secret-store-example.git \
--path "./manifests/vault-integration" \
--sync-policy auto \
--dest-namespace default \
--dest-server https://kubernetes.default.svc

kubectl get secretproviderclass vault-db-credentials  # should exist in default namespace
```

---

## 5. Deploy the app with Argo CD

Point an Argo CD application at `manifests/app` in this repo. That directory contains the Deployment, Service, and ServiceAccount.

```bash
argocd app create my-secret-app \
--project default \
--repo https://github.com/kostis-codefresh/argocd-csi-secret-store-example.git \
--path "./manifests/app" \
--sync-policy auto \
--dest-namespace default \
--dest-server https://kubernetes.default.svc
```

Once synced, verify and access the app:

```bash
kubectl get pods -l app=gitops-secrets-app
kubectl port-forward svc/gitops-secrets-service 8080:8080
```

Open http://localhost:8080 to see the current credentials.

---

## 6. Rotate a secret and watch the app update itself

Update the secret in Vault:

```bash
kubectl exec -it vault-0 -n vault -- vault kv put secret/mysql_credentials \
  credentials='db_con="mysql.example.com:3306"
db_user="rotated_user"
db_password="new_super_secret_password"'
```

Within ~15 seconds the CSI driver refreshes the mounted file and the app reloads automatically—no restart, no Argo CD sync:

```bash
kubectl logs -f -l app=gitops-secrets-app
# Config file changed: /secrets/credentials
# Username is rotated_user
# Password is new_super_secret_password
```

Refresh http://localhost:8080 to confirm.
