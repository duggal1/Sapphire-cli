# SKILL: AWS Deployment

This skill defines how to deploy to AWS. When performing any deployment to ECR, ECS Fargate, S3, or Lambda, follow these docs exactly. Do not guess commands — use the patterns and snippets provided here. This is your authoritative reference for all AWS deployment actions.

**Covers:** ECR (Docker Registry) · ECS Fargate · S3 · Lambda · CloudWatch Logs · GitHub Actions CI/CD
**Last verified:** March 2026

---

## PART 1 — PREREQUISITES & SETUP

### 1.1 Install & Configure AWS CLI v2

```bash
# Install (Linux)
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install

# Install (macOS)
brew install awscli

# Verify (always use v2 — v1 is deprecated)
aws --version   # expect: aws-cli/2.x.x

# Configure credentials
aws configure
# Prompts: AWS Access Key ID, Secret Access Key, Default region, Output format (json)

# Use named profiles for multiple accounts
aws configure --profile prod
aws configure --profile staging

# Use a profile in any command
aws s3 ls --profile prod
export AWS_PROFILE=prod   # or set globally for session
```

### 1.2 Key Environment Variables

```bash
export AWS_REGION=us-east-1
export AWS_PROFILE=my-profile
export AWS_ACCESS_KEY_ID=AKIA...
export AWS_SECRET_ACCESS_KEY=...
# AWS_REGION overrides config; use it in CI/CD environments
```

### 1.3 Verify Identity

```bash
aws sts get-caller-identity
# Returns: Account ID, UserID, ARN — confirms auth is working
```

---

## PART 2 — ECR (ELASTIC CONTAINER REGISTRY)

> ECR = AWS's managed Docker image registry. Use this instead of Docker Hub for any image that gets deployed to AWS services.

### 2.1 Create a Repository

```bash
aws ecr create-repository \
  --repository-name my-app \
  --region us-east-1

# Output includes repositoryUri — save this:
# ACCOUNT_ID.dkr.ecr.REGION.amazonaws.com/my-app

# List existing repositories
aws ecr describe-repositories --region us-east-1
```

### 2.2 Authenticate Docker to ECR

```bash
# Standard auth (token valid 12 hours)
aws ecr get-login-password --region us-east-1 | \
  docker login \
  --username AWS \
  --password-stdin \
  ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com

# Note: aws ecr get-login (v1 command) is REMOVED in AWS CLI v2. Always use get-login-password.
```

### 2.3 Build, Tag & Push Images

**ECR image format:** `ACCOUNT_ID.dkr.ecr.REGION.amazonaws.com/REPO_NAME:TAG`

```bash
# Build with full ECR path
docker build -t ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/my-app:latest .

# Or tag existing image
docker tag my-app:latest ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/my-app:v1.0.0

# Push
docker push ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/my-app:latest
docker push ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/my-app:v1.0.0

# Pull
docker pull ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/my-app:latest
```

### 2.4 Manage Images

```bash
# List images in a repo
aws ecr list-images \
  --repository-name my-app \
  --region us-east-1

# Describe images (includes digest, size, pushed date)
aws ecr describe-images \
  --repository-name my-app \
  --region us-east-1

# Delete a specific image tag
aws ecr batch-delete-image \
  --repository-name my-app \
  --image-ids imageTag=v0.9.0 \
  --region us-east-1

# Enable image scanning on push
aws ecr put-image-scanning-configuration \
  --repository-name my-app \
  --image-scanning-configuration scanOnPush=true \
  --region us-east-1
```

### 2.5 Lifecycle Policies (Auto-clean old images)

```bash
# Create policy to keep only last 10 images
aws ecr put-lifecycle-policy \
  --repository-name my-app \
  --lifecycle-policy-text '{
    "rules": [{
      "rulePriority": 1,
      "description": "Keep last 10 images",
      "selection": {
        "tagStatus": "any",
        "countType": "imageCountMoreThan",
        "countNumber": 10
      },
      "action": { "type": "expire" }
    }]
  }' \
  --region us-east-1
```

---

## PART 3 — ECS WITH FARGATE

> ECS = AWS's container orchestrator. Fargate = serverless compute for ECS — no EC2 instances to manage. This is the AWS equivalent of Cloud Run but more powerful.

### 3.1 When to Use ECS Fargate vs EC2 vs Lambda

| Use Fargate if...            | Use EC2 Launch Type if...                | Use Lambda if...          |
| ---------------------------- | ---------------------------------------- | ------------------------- |
| Stateless HTTP containers    | Need GPU or custom AMIs                  | Event-driven, short-lived |
| Don't want to manage servers | Very high workloads needing cost control | < 15 min execution time   |
| Standard web apps / APIs     | Need specific instance types             | Irregular traffic spikes  |

### 3.2 Core Concepts

- **Cluster** — Logical grouping of tasks/services
- **Task Definition** — Blueprint: Docker image, CPU, memory, env vars, ports
- **Task** — Running instance of a task definition (one-off run)
- **Service** — Keeps N tasks running, handles rolling deploys

### 3.3 Create a Cluster

```bash
aws ecs create-cluster \
  --cluster-name my-cluster \
  --region us-east-1

# List clusters
aws ecs list-clusters --region us-east-1
```

### 3.4 Register a Task Definition

```json
// task-definition.json
{
  "family": "my-app",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "512",
  "memory": "1024",
  "executionRoleArn": "arn:aws:iam::ACCOUNT_ID:role/ecsTaskExecutionRole",
  "taskRoleArn": "arn:aws:iam::ACCOUNT_ID:role/ecsTaskRole",
  "containerDefinitions": [
    {
      "name": "my-app",
      "image": "ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/my-app:latest",
      "portMappings": [{ "containerPort": 8080, "protocol": "tcp" }],
      "environment": [{ "name": "ENV_VAR", "value": "value" }],
      "secrets": [
        {
          "name": "DB_PASSWORD",
          "valueFrom": "arn:aws:secretsmanager:REGION:ACCOUNT_ID:secret:my-secret"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/my-app",
          "awslogs-region": "us-east-1",
          "awslogs-stream-prefix": "ecs"
        }
      },
      "essential": true
    }
  ]
}
```

```bash
# Register task definition
aws ecs register-task-definition \
  --cli-input-json file://task-definition.json \
  --region us-east-1

# Note: As of June 2025, default log driver mode is non-blocking.
# To keep blocking mode, add "mode": "blocking" in logConfiguration options.

# List task definitions
aws ecs list-task-definitions --family-prefix my-app --region us-east-1
```

### 3.5 Create a Service (Long-running, load-balanced)

```bash
aws ecs create-service \
  --cluster my-cluster \
  --service-name my-service \
  --task-definition my-app:1 \
  --desired-count 2 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={
    subnets=[subnet-abc123,subnet-def456],
    securityGroups=[sg-xyz789],
    assignPublicIp=ENABLED
  }" \
  --load-balancers "targetGroupArn=arn:aws:elasticloadbalancing:...,containerName=my-app,containerPort=8080" \
  --region us-east-1
```

### 3.6 Update a Service (Deploy New Image)

```bash
# Step 1: Register new task definition revision with new image
# (edit task-definition.json image tag, then re-register)
aws ecs register-task-definition \
  --cli-input-json file://task-definition.json \
  --region us-east-1

# Step 2: Update service to use new task definition
aws ecs update-service \
  --cluster my-cluster \
  --service my-service \
  --task-definition my-app:2 \
  --region us-east-1

# Force new deployment (e.g. same task def, pull latest image)
aws ecs update-service \
  --cluster my-cluster \
  --service my-service \
  --force-new-deployment \
  --region us-east-1

# Scale service up/down
aws ecs update-service \
  --cluster my-cluster \
  --service my-service \
  --desired-count 4 \
  --region us-east-1
```

### 3.7 Monitor Deployments

```bash
# Watch service events
aws ecs describe-services \
  --cluster my-cluster \
  --services my-service \
  --region us-east-1

# List running tasks
aws ecs list-tasks \
  --cluster my-cluster \
  --service-name my-service \
  --region us-east-1

# Describe specific task
aws ecs describe-tasks \
  --cluster my-cluster \
  --tasks TASK_ARN \
  --region us-east-1
```

### 3.8 Run a One-Off Task (Non-Service)

```bash
aws ecs run-task \
  --cluster my-cluster \
  --task-definition my-app:1 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={
    subnets=[subnet-abc123],
    securityGroups=[sg-xyz789],
    assignPublicIp=ENABLED
  }" \
  --region us-east-1
```

### 3.9 Required IAM Roles

```bash
# ecsTaskExecutionRole — allows ECS to pull images from ECR and write logs
# Attach: AmazonECSTaskExecutionRolePolicy

# Create it if missing:
aws iam create-role \
  --role-name ecsTaskExecutionRole \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": { "Service": "ecs-tasks.amazonaws.com" },
      "Action": "sts:AssumeRole"
    }]
  }'

aws iam attach-role-policy \
  --role-name ecsTaskExecutionRole \
  --policy-arn arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy
```

### 3.10 Full CI/CD Deploy Script (Build → Push → Deploy)

```bash
#!/bin/bash
set -e

REGION=us-east-1
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REPO=my-app
CLUSTER=my-cluster
SERVICE=my-service
IMAGE_TAG=$(git rev-parse --short HEAD)
IMAGE_URI="$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/$REPO:$IMAGE_TAG"

# 1. Auth Docker to ECR
aws ecr get-login-password --region $REGION | \
  docker login --username AWS --password-stdin \
  "$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com"

# 2. Build & push
docker build -t "$IMAGE_URI" .
docker push "$IMAGE_URI"

# 3. Update task definition with new image
TASK_DEF=$(aws ecs describe-task-definition \
  --task-definition $REPO \
  --region $REGION \
  --query taskDefinition)

NEW_TASK_DEF=$(echo $TASK_DEF | \
  jq --arg IMAGE "$IMAGE_URI" \
  '.containerDefinitions[0].image = $IMAGE | del(.taskDefinitionArn, .revision, .status, .requiresAttributes, .placementConstraints, .compatibilities, .registeredAt, .registeredBy)')

NEW_REVISION=$(aws ecs register-task-definition \
  --region $REGION \
  --cli-input-json "$NEW_TASK_DEF" \
  --query 'taskDefinition.taskDefinitionArn' \
  --output text)

# 4. Deploy service
aws ecs update-service \
  --cluster $CLUSTER \
  --service $SERVICE \
  --task-definition $NEW_REVISION \
  --region $REGION

echo "Deployed $IMAGE_URI to $SERVICE"
```

---

## PART 4 — S3

> S3 = AWS object storage. Use for static files, build artifacts, frontend hosting, backups, data pipelines.

### 4.1 Create & Configure Buckets

```bash
# Create bucket (us-east-1 requires no LocationConstraint)
aws s3 mb s3://my-bucket --region us-east-1

# Create bucket in other regions
aws s3api create-bucket \
  --bucket my-bucket \
  --region us-west-2 \
  --create-bucket-configuration LocationConstraint=us-west-2

# List buckets
aws s3 ls

# List objects in bucket
aws s3 ls s3://my-bucket/
aws s3 ls s3://my-bucket/prefix/ --recursive
```

### 4.2 Upload & Download Files

```bash
# Upload single file
aws s3 cp local-file.txt s3://my-bucket/path/file.txt

# Upload directory (recursive)
aws s3 cp ./dist/ s3://my-bucket/app/ --recursive

# Download single file
aws s3 cp s3://my-bucket/path/file.txt ./local-file.txt

# Download directory
aws s3 cp s3://my-bucket/app/ ./dist/ --recursive

# Sync (only uploads changed/new files — most efficient for deploys)
aws s3 sync ./dist/ s3://my-bucket/ --delete
# --delete removes files in S3 that no longer exist locally

# With cache control headers (for frontend deployments)
aws s3 sync ./dist/ s3://my-bucket/ \
  --cache-control "max-age=31536000" \
  --exclude "*.html" \
  --delete

aws s3 cp ./dist/index.html s3://my-bucket/index.html \
  --cache-control "no-cache"
```

### 4.3 Static Website Hosting

```bash
# Enable website hosting
aws s3 website s3://my-bucket \
  --index-document index.html \
  --error-document 404.html

# Make bucket publicly readable (required for website hosting)
aws s3api put-public-access-block \
  --bucket my-bucket \
  --public-access-block-configuration \
  "BlockPublicAcls=false,IgnorePublicAcls=false,BlockPublicPolicy=false,RestrictPublicBuckets=false"

aws s3api put-bucket-policy \
  --bucket my-bucket \
  --policy '{
    "Version": "2012-10-17",
    "Statement": [{
      "Sid": "PublicReadGetObject",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::my-bucket/*"
    }]
  }'

# Website URL: http://my-bucket.s3-website-us-east-1.amazonaws.com
# Use CloudFront in front for HTTPS + custom domain
```

### 4.4 Manage Bucket Settings

```bash
# Enable versioning
aws s3api put-bucket-versioning \
  --bucket my-bucket \
  --versioning-configuration Status=Enabled

# Enable server-side encryption (SSE-S3)
aws s3api put-bucket-encryption \
  --bucket my-bucket \
  --server-side-encryption-configuration '{
    "Rules": [{"ApplyServerSideEncryptionByDefault": {"SSEAlgorithm": "AES256"}}]
  }'

# Delete all objects (needed before deleting bucket)
aws s3 rm s3://my-bucket/ --recursive

# Delete bucket
aws s3 rb s3://my-bucket --force
```

### 4.5 Presigned URLs (Temporary access)

```bash
# Generate presigned URL valid for 1 hour (3600 seconds)
aws s3 presign s3://my-bucket/private-file.pdf --expires-in 3600
```

---

## PART 5 — LAMBDA

> Lambda = serverless functions. Event-driven, scales to zero, max 15-min execution. Best for lightweight event handlers, API backends, and scheduled tasks.

### 5.1 Deploy a Lambda Function (ZIP)

```bash
# Package code
zip -r function.zip .

# Create function
aws lambda create-function \
  --function-name my-function \
  --runtime nodejs22.x \
  --role arn:aws:iam::ACCOUNT_ID:role/lambda-execution-role \
  --handler index.handler \
  --zip-file fileb://function.zip \
  --region us-east-1

# Update function code
aws lambda update-function-code \
  --function-name my-function \
  --zip-file fileb://function.zip \
  --region us-east-1

# Update function config (memory, timeout, env)
aws lambda update-function-configuration \
  --function-name my-function \
  --memory-size 512 \
  --timeout 30 \
  --environment "Variables={KEY=VALUE}" \
  --region us-east-1
```

### 5.2 Deploy Lambda from Container Image

```bash
# Build and push image to ECR first (same as Part 2)
# Lambda requires ARM or x86_64 image

aws lambda create-function \
  --function-name my-function \
  --package-type Image \
  --code ImageUri=ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/my-lambda:latest \
  --role arn:aws:iam::ACCOUNT_ID:role/lambda-execution-role \
  --region us-east-1

# Update image
aws lambda update-function-code \
  --function-name my-function \
  --image-uri ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com/my-lambda:v2.0.0 \
  --region us-east-1
```

### 5.3 Invoke & Test

```bash
# Invoke synchronously
aws lambda invoke \
  --function-name my-function \
  --payload '{"key":"value"}' \
  --cli-binary-format raw-in-base64-out \
  response.json \
  --region us-east-1

cat response.json

# Invoke asynchronously
aws lambda invoke \
  --function-name my-function \
  --invocation-type Event \
  --payload '{}' \
  --cli-binary-format raw-in-base64-out \
  /dev/null \
  --region us-east-1
```

### 5.4 Lambda IAM Execution Role (Minimum)

```bash
aws iam create-role \
  --role-name lambda-execution-role \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": { "Service": "lambda.amazonaws.com" },
      "Action": "sts:AssumeRole"
    }]
  }'

# Attach basic execution policy (CloudWatch Logs access)
aws iam attach-role-policy \
  --role-name lambda-execution-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
```

---

## PART 6 — CLOUDWATCH LOGS

> Every AWS service should log to CloudWatch. ECS tasks use it via `awslogs` driver. Lambda logs automatically.

```bash
# Create log group
aws logs create-log-group \
  --log-group-name /ecs/my-app \
  --region us-east-1

# Tail logs in real-time
aws logs tail /ecs/my-app --follow --region us-east-1

# Filter logs
aws logs filter-log-events \
  --log-group-name /ecs/my-app \
  --filter-pattern "ERROR" \
  --region us-east-1
```

---

## PART 7 — DECISION MATRIX

```
START: What are you deploying?
│
├── Static frontend (React, Next.js static export, HTML/CSS)?
│     └── S3 + CloudFront (CDN + HTTPS + custom domain)
│
├── Containerized HTTP API / backend service?
│     └── ECS Fargate + ALB (Application Load Balancer)
│           ├── Simple API / low ops → ECS Fargate (Fargate launch type)
│           └── High scale / custom infra → ECS on EC2
│
├── Short-lived function or event handler (<15 min)?
│     └── Lambda
│           ├── Simple code → ZIP deployment
│           └── Complex deps → Container image from ECR
│
├── Need to store a Docker image?
│     └── ECR (always — for any image that touches AWS)
│
└── Need to store files, assets, build artifacts?
      └── S3
```

---

## PART 8 — COMMON ERRORS & FIXES

| Error                                      | Cause                                       | Fix                                                                     |
| ------------------------------------------ | ------------------------------------------- | ----------------------------------------------------------------------- |
| `no basic auth credentials` on docker push | Docker not auth'd to ECR                    | Run `aws ecr get-login-password ... \| docker login ...`                |
| `InvalidClientTokenId`                     | Wrong or expired AWS credentials            | `aws configure` or check `AWS_ACCESS_KEY_ID`                            |
| `AccessDenied` on ECR push                 | IAM policy missing ECR write                | Attach `AmazonEC2ContainerRegistryPowerUser` to user/role               |
| ECS task stuck in `PENDING`                | Insufficient cluster capacity or bad subnet | Check VPC/subnet config; use Fargate to avoid capacity issues           |
| ECS task `STOPPED` immediately             | Container crash on start                    | `aws ecs describe-tasks` → check `stoppedReason`; check CloudWatch logs |
| `CannotPullContainerError`                 | ECS can't reach ECR                         | Check subnet has NAT gateway or VPC endpoint for ECR                    |
| Lambda timeout                             | Function exceeds timeout limit (default 3s) | Increase with `--timeout`; max is 900s                                  |
| S3 403 on public bucket                    | Block Public Access enabled                 | Use `put-public-access-block` to disable + attach bucket policy         |
| `ResourceNotFoundException` on task def    | Task definition not registered yet          | Run `register-task-definition` first                                    |
| ECS service stuck in deployment            | Health check failing                        | Check target group health check path/port; check app logs               |

---

## PART 9 — CI/CD GITHUB ACTIONS SKELETON

```yaml
# .github/workflows/deploy.yml
name: Build and Deploy to ECS

on:
  push:
    branches: [main]

env:
  AWS_REGION: us-east-1
  ECR_REPOSITORY: my-app
  ECS_CLUSTER: my-cluster
  ECS_SERVICE: my-service
  TASK_DEFINITION_FAMILY: my-app

jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read

    steps:
      - uses: actions/checkout@v4

      - name: Configure AWS credentials (OIDC — no static keys)
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::ACCOUNT_ID:role/github-actions-role
          aws-region: ${{ env.AWS_REGION }}

      - name: Login to ECR
        id: login-ecr
        uses: aws-actions/amazon-ecr-login@v2

      - name: Build and push image
        id: build-image
        run: |
          IMAGE_URI=${{ steps.login-ecr.outputs.registry }}/${{ env.ECR_REPOSITORY }}:${{ github.sha }}
          docker build -t $IMAGE_URI .
          docker push $IMAGE_URI
          echo "image=$IMAGE_URI" >> $GITHUB_OUTPUT

      - name: Download task definition
        run: |
          aws ecs describe-task-definition \
            --task-definition ${{ env.TASK_DEFINITION_FAMILY }} \
            --query taskDefinition \
            > task-definition.json

      - name: Update image in task definition
        id: task-def
        uses: aws-actions/amazon-ecs-render-task-definition@v1
        with:
          task-definition: task-definition.json
          container-name: my-app
          image: ${{ steps.build-image.outputs.image }}

      - name: Deploy to ECS
        uses: aws-actions/amazon-ecs-deploy-task-definition@v1
        with:
          task-definition: ${{ steps.task-def.outputs.task-definition }}
          service: ${{ env.ECS_SERVICE }}
          cluster: ${{ env.ECS_CLUSTER }}
          wait-for-service-stability: true
```

---
