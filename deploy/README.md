# Kubernetes Deployment – Torn OC History

This directory contains Kubernetes manifests and helper files for deploying the `torn-oc-history` application.

## 1. Prepare environment files

1. Copy `env.template` to `.env` and fill in real values.

   ```bash
   cp env.template .env
   # edit .env to add TORN_API_KEY and SPREADSHEET_ID   
   ```

2. Obtain Google service-account credentials JSON with Sheets API access and save as `credentials.json`.

## 2. Build & push image

```bash
# from repo root
cd torn_oc_history
# build
docker build -t localhost:32000/torn-oc-history:0.0.2 -f build/Dockerfile .
# push
docker push localhost:32000/torn-oc-history:0.0.2
```

Adjust the image tag/registry as appropriate and reflect the same tag inside `deployment.yaml`.

## 3. Create Kubernetes secret

```bash
# encode files
ENV_CONTENT=$(base64 -w 0 .env)
CREDS_CONTENT=$(base64 -w 0 credentials.json)

# insert into manifest
awk -v env="$ENV_CONTENT" -v creds="$CREDS_CONTENT" '{gsub("your_base64_encoded_env_file_content_here",env); gsub("your_base64_encoded_credentials_json_content_here",creds); print}' torn-history-secret.yaml > secret.yaml

kubectl apply -f /tmp/secret.yaml
```

## 4. Deploy

```bash
kubectl apply -f deployment.yaml
```

## 5. Rolling Restart

To perform a rolling restart of the deployment (e.g., after updating secrets or configuration):

```bash
kubectl rollout restart deployment torn-oc-history
```

This ensures zero downtime by creating new pods before terminating old ones.

## 6. Runtime configuration

The application is configured via environment variables. The deployment includes examples for:

* **TORN_OUTPUT=sheets** – Write to Google Sheets instead of stdout
* **TORN_INTERVAL=5m** – Run continuously every 5 minutes  
* **TORN_BOTH=true** – Generate both reports (all members and not-in-OC)
* **TORN_RANGE_NOC** and **TORN_RANGE_ALL** – Target sheet ranges

Edit the `env` section in `deployment.yaml` or update your `.env` file to customize behavior. All command line flags can be set as environment variables using the `TORN_` prefix (see main README for complete mapping).

## Security considerations

* Runs as non-root, drops all capabilities, seccomp profile runtime-default.
* .env and credentials mounted as read-only secrets.

## Files

* `env.template` – starter environment file.
* `torn-history-secret.yaml` – Kubernetes secret manifest.
* `deployment.yaml` – Deployment manifest.
