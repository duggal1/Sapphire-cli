# SKILL: Google Cloud Deployment

This skill defines how to deploy to Google Cloud. When performing any deployment to Google Cloud Run, GKE, or Artifact Registry, follow these docs exactly. Do not guess commands — use the patterns and snippets provided here. This is your authoritative reference for all GCP deployment actions.

**Covers:** Artifact Registry (Docker) · Cloud Run · Google Kubernetes Engine (GKE) · Cloud Build CI/CD
**Last verified:** March 2026

---

## PART 1 — PREREQUISITES & SETUP

### 1.1 Install & Authenticate gcloud CLI

```bash
# Install (Linux)
curl https://sdk.cloud.google.com | bash
exec -l $SHELL

# Install (macOS)
brew install --cask google-cloud-sdk

# Authenticate
gcloud auth login
gcloud config set project PROJECT_ID
gcloud config set compute/region us-central1   # change as needed

# Verify
gcloud info
gcloud version
gcloud components update
```

### 1.2 Enable Required APIs

```bash
# Cloud Run
gcloud services enable run.googleapis.com

# GKE
gcloud services enable container.googleapis.com

# Artifact Registry (replaces old Container Registry gcr.io)
gcloud services enable artifactregistry.googleapis.com

# Cloud Build (for source-based deploys)
gcloud services enable cloudbuild.googleapis.com
```

---

## PART 2 — DOCKER & ARTIFACT REGISTRY

> **Critical:** Google's image registry is now **Artifact Registry** (`*.pkg.dev`), NOT the deprecated Container Registry (`gcr.io`). Always use Artifact Registry.

### 2.1 Create a Docker Repository

```bash
gcloud artifacts repositories create REPO_NAME \
  --repository-format=docker \
  --location=REGION \
  --description="Docker images" \
  --project=PROJECT_ID

# Verify
gcloud artifacts repositories list --location=REGION
```

**Available regions:** `us-central1`, `us-east1`, `europe-west1`, `asia-east1`, etc.
**Multi-region options:** `us`, `europe`, `asia` (higher availability, slightly higher latency)

### 2.2 Authenticate Docker to Artifact Registry

```bash
# One-time setup per region
gcloud auth configure-docker REGION-docker.pkg.dev

# Multiple regions at once
gcloud auth configure-docker \
  us-central1-docker.pkg.dev,us-east1-docker.pkg.dev,europe-west1-docker.pkg.dev

# CI/CD: short-lived token auth (no static keys)
gcloud auth print-access-token | docker login \
  -u oauth2accesstoken \
  --password-stdin \
  https://REGION-docker.pkg.dev
```

### 2.3 Build, Tag & Push Images

**Image naming format:** `REGION-docker.pkg.dev/PROJECT_ID/REPO_NAME/IMAGE_NAME:TAG`

```bash
# Build with full Artifact Registry path
docker build -t us-central1-docker.pkg.dev/my-project/my-repo/myapp:v1.0.0 .

# Tag existing image
docker tag myapp:latest us-central1-docker.pkg.dev/my-project/my-repo/myapp:latest

# Push
docker push us-central1-docker.pkg.dev/my-project/my-repo/myapp:v1.0.0
docker push us-central1-docker.pkg.dev/my-project/my-repo/myapp:latest

# Pull
docker pull us-central1-docker.pkg.dev/my-project/my-repo/myapp:v1.0.0
```

### 2.4 Manage Images

```bash
# List all images in repo
gcloud artifacts docker images list \
  us-central1-docker.pkg.dev/PROJECT_ID/REPO_NAME

# List tags for a specific image
gcloud artifacts docker tags list \
  us-central1-docker.pkg.dev/PROJECT_ID/REPO_NAME/IMAGE_NAME

# Delete an image
gcloud artifacts docker images delete \
  us-central1-docker.pkg.dev/PROJECT_ID/REPO_NAME/IMAGE_NAME:TAG

# Enable vulnerability scanning
gcloud services enable containerscanning.googleapis.com
gcloud artifacts docker images describe \
  us-central1-docker.pkg.dev/PROJECT_ID/REPO_NAME/IMAGE_NAME:TAG
```

### 2.5 IAM for Artifact Registry

```bash
# Grant read (pull) to a service account
gcloud artifacts repositories add-iam-policy-binding REPO_NAME \
  --location=REGION \
  --member="serviceAccount:SA_EMAIL" \
  --role="roles/artifactregistry.reader"

# Grant write (push) to CI/CD service account
gcloud artifacts repositories add-iam-policy-binding REPO_NAME \
  --location=REGION \
  --member="serviceAccount:SA_EMAIL" \
  --role="roles/artifactregistry.writer"

# Create a service account for CI/CD
gcloud iam service-accounts create github-actions \
  --display-name="GitHub Actions"
```

---

## PART 3 — CLOUD RUN

> Cloud Run = serverless containers. Best for stateless HTTP services, APIs, background workers. No cluster management. Pay per request. Auto-scales to zero.

### 3.1 When to Use Cloud Run vs GKE

| Use Cloud Run if... | Use GKE if... |
|---|---|
| Stateless HTTP service / API | Need persistent storage (StatefulSets) |
| Sporadic or unpredictable traffic | Need fine-grained resource control |
| Prefer zero infra management | Running microservices at scale |
| Simple background workers | Need GPU workloads |
| Fast iteration / low ops overhead | Need custom networking / sidecars |

### 3.2 Deploy from Source (No Dockerfile needed)

```bash
# Simplest path — Cloud Build + Buildpacks handle everything
gcloud run deploy SERVICE_NAME \
  --source . \
  --region REGION \
  --project PROJECT_ID \
  --allow-unauthenticated   # remove for private services
```

Supported source runtimes (auto-detected): Node.js, Python (FastAPI, Flask, Gradio, Streamlit), Go, Java, .NET, Ruby, PHP.

### 3.3 Deploy from Docker Image

```bash
gcloud run deploy SERVICE_NAME \
  --image=us-central1-docker.pkg.dev/PROJECT_ID/REPO/IMAGE:TAG \
  --platform=managed \
  --region=REGION \
  --project=PROJECT_ID \
  --allow-unauthenticated \
  --max-instances=10 \
  --min-instances=1 \
  --cpu=1 \
  --memory=512Mi \
  --timeout=300
```

### 3.4 Deploy a Cloud Run Function

```bash
gcloud run deploy FUNCTION_NAME \
  --source . \
  --function ENTRYPOINT_FUNCTION_NAME \
  --base-image BASE_IMAGE \     # e.g. nodejs24, python314, go125, java25
  --region REGION
```

### 3.5 Environment Variables

```bash
# Set individually
gcloud run deploy SERVICE_NAME \
  --set-env-vars="KEY1=VALUE1,KEY2=VALUE2"

# From .env file (GA as of 2026)
gcloud run deploy SERVICE_NAME \
  --env-vars-file=.env

# Update env vars on existing service
gcloud run services update SERVICE_NAME \
  --set-env-vars="KEY=VALUE" \
  --region=REGION
```

### 3.6 Traffic Management & Revisions

```bash
# Route 100% to latest revision
gcloud run services update-traffic SERVICE_NAME \
  --to-latest \
  --region=REGION

# Canary: split traffic 90/10
gcloud run services update-traffic SERVICE_NAME \
  --to-revisions=REVISION_V2=10,REVISION_V1=90 \
  --region=REGION

# Route 100% to specific tag
gcloud run services update-traffic SERVICE_NAME \
  --to-source=tag=v2=100% \
  --region=REGION

# List services
gcloud run services list --region=REGION \
  --format="table(name,status,url)"
```

### 3.7 Multi-Region Deploy (GA 2026)

```bash
# Deploy as multi-region service from a single command
gcloud run deploy SERVICE_NAME \
  --source . \
  --region=us-central1,europe-west1 \
  --allow-unauthenticated
```

### 3.8 Cloud Run Jobs (Batch / Non-HTTP)

```bash
# Create a job
gcloud run jobs create JOB_NAME \
  --image=IMAGE_URL \
  --tasks=5 \
  --max-retries=3 \
  --region=REGION

# Execute a job
gcloud run jobs execute JOB_NAME --region=REGION

# Replace (update) a job
gcloud run jobs replace JOB_YAML_FILE
```

### 3.9 IAM — Controlling Access

```bash
# Make service public
gcloud run services add-iam-policy-binding SERVICE_NAME \
  --region=REGION \
  --member="allUsers" \
  --role="roles/run.invoker"

# Allow specific service account to invoke
gcloud run services add-iam-policy-binding SERVICE_NAME \
  --region=REGION \
  --member="serviceAccount:SA_EMAIL" \
  --role="roles/run.invoker"
```

### 3.10 Rollback

```bash
# Redirect traffic to previous revision
gcloud run services update-traffic SERVICE_NAME \
  --to-revisions=PREVIOUS_REVISION=100 \
  --region=REGION
```

---

## PART 4 — GOOGLE KUBERNETES ENGINE (GKE)

> GKE = fully managed Kubernetes. Best for complex microservice architectures, stateful apps, fine-grained resource control, and scale.

### 4.1 Install kubectl

```bash
gcloud components install kubectl
gcloud components install beta
gcloud components update
```

### 4.2 Create a Cluster

```bash
# Standard cluster (most control)
gcloud container clusters create CLUSTER_NAME \
  --zone=us-central1-a \
  --num-nodes=3 \
  --machine-type=e2-standard-2 \
  --project=PROJECT_ID

# Autopilot cluster (recommended — Google manages nodes)
gcloud container clusters create-auto CLUSTER_NAME \
  --region=us-central1 \
  --project=PROJECT_ID

# List clusters
gcloud container clusters list
```

**Cluster modes:**
- **Autopilot** — Google manages node provisioning, scaling, security. Recommended for most use cases.
- **Standard** — You control node pools, machine types, autoscaling settings.

### 4.3 Get Credentials (Connect kubectl)

```bash
gcloud container clusters get-credentials CLUSTER_NAME \
  --zone=us-central1-a \
  --project=PROJECT_ID

# Verify connection
kubectl cluster-info
kubectl get nodes
```

### 4.4 Deploy an Application

```bash
# Create deployment
kubectl create deployment APP_NAME \
  --image=us-central1-docker.pkg.dev/PROJECT_ID/REPO/IMAGE:TAG

# Scale deployment
kubectl scale deployment APP_NAME --replicas=3

# Check status
kubectl get pods
kubectl get deployments
kubectl describe deployment APP_NAME
```

### 4.5 Expose Service (Load Balancer)

```bash
# External HTTP load balancer
kubectl expose deployment APP_NAME \
  --type=LoadBalancer \
  --port=80 \
  --target-port=8080

# Get external IP (takes a few minutes)
kubectl get service APP_NAME

# Internal cluster-only service
kubectl expose deployment APP_NAME \
  --type=ClusterIP \
  --port=80 \
  --target-port=8080
```

### 4.6 Kubernetes YAML Manifest (Recommended for Production)

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: my-app
          image: us-central1-docker.pkg.dev/PROJECT_ID/REPO/IMAGE:TAG
          ports:
            - containerPort: 8080
          env:
            - name: ENV_VAR
              value: "value"
          resources:
            requests:
              cpu: "250m"
              memory: "256Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"
---
apiVersion: v1
kind: Service
metadata:
  name: my-app-service
spec:
  type: LoadBalancer
  selector:
    app: my-app
  ports:
    - port: 80
      targetPort: 8080
```

```bash
# Apply manifest
kubectl apply -f deployment.yaml

# Watch rollout
kubectl rollout status deployment/my-app

# Delete resources
kubectl delete -f deployment.yaml
```

### 4.7 Update Deployment (Rolling Update)

```bash
# Update image (triggers rolling update automatically)
kubectl set image deployment/my-app \
  my-app=us-central1-docker.pkg.dev/PROJECT_ID/REPO/IMAGE:NEW_TAG

# Monitor rollout
kubectl rollout status deployment/my-app --timeout=120s

# Rollback if needed
kubectl rollout undo deployment/my-app

# Rollback to specific revision
kubectl rollout undo deployment/my-app --to-revision=2
```

### 4.8 Cloud Build CI/CD for GKE

```yaml
# cloudbuild.yaml
steps:
  # Step 1: Build Docker image
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-t', 'us-central1-docker.pkg.dev/$PROJECT_ID/my-repo/my-app:$SHORT_SHA', '.']

  # Step 2: Push to Artifact Registry
  - name: 'gcr.io/cloud-builders/docker'
    args: ['push', 'us-central1-docker.pkg.dev/$PROJECT_ID/my-repo/my-app:$SHORT_SHA']

  # Step 3: Get GKE credentials
  - name: 'gcr.io/cloud-builders/gke-deploy'
    entrypoint: 'bash'
    args:
      - '-c'
      - |
        gcloud container clusters get-credentials my-cluster \
          --zone us-central1-a \
          --project $PROJECT_ID

  # Step 4: Update deployment
  - name: 'gcr.io/cloud-builders/kubectl'
    args:
      - 'set'
      - 'image'
      - 'deployment/my-app'
      - 'my-app=us-central1-docker.pkg.dev/$PROJECT_ID/my-repo/my-app:$SHORT_SHA'
    env:
      - 'CLOUDSDK_COMPUTE_ZONE=us-central1-a'
      - 'CLOUDSDK_CONTAINER_CLUSTER=my-cluster'

  # Step 5: Verify rollout — auto-rollback on failure
  - name: 'gcr.io/cloud-builders/kubectl'
    entrypoint: 'bash'
    args:
      - '-c'
      - |
        if ! kubectl rollout status deployment/my-app --timeout=120s; then
          echo "Deployment failed, rolling back..."
          kubectl rollout undo deployment/my-app
          exit 1
        fi

images:
  - 'us-central1-docker.pkg.dev/$PROJECT_ID/my-repo/my-app:$SHORT_SHA'
```

```bash
# Create Cloud Build trigger (fires on push to main)
gcloud builds triggers create github \
  --repo-name="my-app" \
  --repo-owner="my-org" \
  --branch-pattern="^main$" \
  --build-config="cloudbuild.yaml" \
  --description="Deploy to GKE on push to main"
```

### 4.9 GKE IAM — Cloud Build Permissions

```bash
# Cloud Build service account needs GKE permission
PROJECT_NUMBER=$(gcloud projects describe PROJECT_ID --format='value(projectNumber)')
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com" \
  --role="roles/container.developer"
```

### 4.10 Useful kubectl Commands

```bash
# Inspect
kubectl get pods
kubectl get services
kubectl get deployments
kubectl get nodes
kubectl describe pod POD_NAME
kubectl logs POD_NAME
kubectl logs POD_NAME -f   # follow live logs
kubectl exec -it POD_NAME -- /bin/bash   # shell into pod

# Autoscale
kubectl autoscale deployment APP_NAME \
  --min=2 --max=10 --cpu-percent=70

# Delete
kubectl delete deployment APP_NAME
kubectl delete service SERVICE_NAME
kubectl delete pod POD_NAME
```

---

## PART 5 — IAM & SERVICE ACCOUNTS (CROSS-CUTTING)

```bash
# Create service account
gcloud iam service-accounts create SA_NAME \
  --display-name="My Service Account" \
  --project=PROJECT_ID

# Grant role to service account
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:SA_NAME@PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.admin"

# Key roles needed:
# roles/run.admin          — manage Cloud Run
# roles/container.admin    — manage GKE clusters
# roles/container.developer — deploy to GKE
# roles/artifactregistry.writer — push images
# roles/artifactregistry.reader — pull images
# roles/cloudbuild.builds.editor — trigger Cloud Build

# Download service account key (for CI/CD)
gcloud iam service-accounts keys create key.json \
  --iam-account=SA_NAME@PROJECT_ID.iam.gserviceaccount.com
```

---

## PART 6 — DECISION MATRIX

```
START: What are you deploying?
│
├── Stateless HTTP API / microservice / webhook?
│     └── Use CLOUD RUN (--source for speed, --image for control)
│
├── Background batch job (non-HTTP)?
│     └── Use CLOUD RUN JOBS
│
├── Complex multi-service app needing persistent storage,
│   sidecars, custom networking, or massive scale?
│     └── Use GKE
│           ├── Low ops overhead → Autopilot cluster
│           └── Full control → Standard cluster
│
└── Need to store/share container images?
      └── Artifact Registry (always — never old gcr.io)
```

---

## PART 7 — COMMON ERRORS & FIXES

| Error | Cause | Fix |
|---|---|---|
| `Permission denied` on push | Docker not auth'd to Artifact Registry | `gcloud auth configure-docker REGION-docker.pkg.dev` |
| `Image not found` on Cloud Run | Wrong image path format | Use `REGION-docker.pkg.dev/PROJECT/REPO/IMAGE:TAG` |
| `Error: no project set` | gcloud project not configured | `gcloud config set project PROJECT_ID` |
| `kubectl: no cluster` | Credentials not fetched | `gcloud container clusters get-credentials CLUSTER --zone ZONE` |
| Cloud Build timeout | Large image, slow build | Add `--timeout=1800` to build command |
| Cloud Run cold starts | Min instances = 0 | Set `--min-instances=1` for latency-sensitive services |
| GKE node auth to Artifact Registry | Different projects | Grant `roles/artifactregistry.reader` to GKE node service account |