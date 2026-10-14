# callrail-processor

A small Go HTTP service that receives [CallRail](https://www.callrail.com/) call webhooks and stores them in MongoDB.

Calls are upserted on CallRail's call `id`, so retried or repeated webhooks for the same call update one document instead of creating duplicates.

## API

| Method | Path         | Description                                                                 |
| ------ | ------------ | --------------------------------------------------------------------------- |
| `POST` | `/api/calls` | Save a call. Body is the CallRail JSON payload and must include a string `id`. Returns `201` for a new call, `200` when an existing call was updated. |
| `GET`  | `/api/calls` | List stored calls, most recently received first. Paginated: `?page=1&limit=50` (`limit` 1–200, default 50). |
| `GET`  | `/healthz`   | Liveness check. Returns `200` whenever the process is serving HTTP.         |
| `GET`  | `/readyz`    | Readiness check. Returns `200` if MongoDB responds to a ping, `503` if not. |

`GET /api/calls` responds with:

```json
{ "data": [ ... ], "page": 1, "limit": 50, "total": 123, "total_pages": 3 }
```

Errors: `400` for invalid JSON, a missing `id`, or an invalid `page` or `limit`, `405` for unsupported methods, `500` if the database write or read fails.

## Configuration

All configuration comes from environment variables.

| Variable           | Default                     | Description                |
| ------------------ | --------------------------- | -------------------------- |
| `PORT`             | `8080`                      | Port the server listens on |
| `MONGODB_URI`      | `mongodb://localhost:27017` | MongoDB connection string  |
| `MONGODB_DATABASE` | `callrail`                  | Database name              |

Calls are stored in the `calls` collection. On startup the service creates a unique index on `id` and exits if it can't, so it never serves traffic without it.

## Running locally

Requires Docker Desktop with Docker Compose 2.22 or later.

### Docker Compose

```bash
docker compose up --build
```

- API: `http://localhost:8080`
- MongoDB: `mongodb://localhost:27018` (for mongosh or Compass). Set `MONGO_HOST_PORT` to use a different host port.

To rebuild and restart the app automatically whenever source files change:

```bash
docker compose watch
```

### Without a container

Run only MongoDB in Docker and the app directly with Go 1.27 or later:

```bash
docker compose up -d mongo
MONGODB_URI=mongodb://localhost:27018 go run .
```

### Try it

```bash
curl -X POST http://localhost:8080/api/calls \
  -H "Content-Type: application/json" \
  --data @data/callrail_sample.json

curl "http://localhost:8080/api/calls?page=1&limit=10"
```

### Seed sample calls

`cmd/seed` posts 20 varied calls based on `data/callrail_sample.json`, five at a time, then re-sends four of them to show the `unchanged` and `updated` results and lists the newest calls:

```bash
go run ./cmd/seed                              # against http://localhost:8080
go run ./cmd/seed -url http://<ALB hostname>   # against EKS
go run ./cmd/seed -h                           # all options (-n, -concurrency, -resend, -sample)
```

Call ids are fixed (`CALseed0001` to `CALseed0020`), so running it again updates the same calls instead of adding more.

## Kubernetes

Manifests are in `k8s/` and use [Kustomize](https://kustomize.io/), which is built into `kubectl`.

```
k8s/
├── base/              # Deployment and Service (port 80 → 8080), shared by all environments
└── overlays/
    ├── local/         # Docker Desktop Kubernetes, with an in-cluster MongoDB
    └── aws/           # EKS: ALB Ingress, ECR image, 2 replicas (infrastructure in terraform/)
```

### Local cluster (Docker Desktop)

```bash
docker compose build app

# Docker Desktop's Kubernetes node has its own image store, so load the image into it.
docker save callprocessor:local | docker exec -i desktop-control-plane ctr -n k8s.io images import --digests -

kubectl apply -k k8s/overlays/local
kubectl -n callprocessor port-forward svc/callprocessor 8080:80
```

Remove it, including MongoDB's data:

```bash
kubectl delete -k k8s/overlays/local
```

### AWS (EKS and MongoDB Atlas)

`terraform/` creates everything the app needs on AWS and Atlas:

- a VPC, an EKS cluster with two worker nodes, and the AWS Load Balancer Controller
- an ECR repository for the image
- a free MongoDB Atlas M0 cluster, a database user, and an IP access list entry for the cluster's outbound IP
- the `callprocessor` namespace and the `callprocessor-mongodb` Secret holding the connection string

**This costs money while it exists**: roughly $0.25 an hour, about $150 a month if left running. Destroy it when you're done.

#### Prerequisites

- Terraform 1.11 or later, the AWS CLI, `kubectl` and Docker
- AWS credentials allowed to create VPC, EKS, IAM, EC2 and ECR resources, active in your shell (for example `export AWS_PROFILE=...`)
- A MongoDB Atlas account, its organization ID, and an API key with the Organization Project Creator role

#### Create the infrastructure

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars   # then fill in the Atlas values
terraform init
terraform plan
terraform apply                                # takes about 15 to 20 minutes
```

#### Deploy the app

```bash
# Point kubectl at the new cluster
$(terraform output -raw configure_kubectl)

# Build for the nodes' architecture and push to ECR
ECR=$(terraform output -raw ecr_repository_url)
cd ..
aws ecr get-login-password --region us-west-2 | docker login --username AWS --password-stdin "${ECR%%/*}"
docker buildx build --platform linux/amd64 -t "$ECR:0.1.0" --push .
```

The overlay's image uses an `ACCOUNT_ID` placeholder so the AWS account ID isn't committed. Fill it in while applying, and set `newTag` in `k8s/overlays/aws/kustomization.yaml` if you pushed a different tag:

```bash
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
kubectl kustomize k8s/overlays/aws | sed "s/ACCOUNT_ID/$ACCOUNT_ID/" | kubectl apply -f -
kubectl -n callprocessor get ingress callprocessor   # ADDRESS is the ALB hostname, ready after 2 to 3 minutes
```

The Ingress listens on HTTP by default. The annotations for HTTPS with an ACM certificate are in `k8s/overlays/aws/ingress.yaml`, commented out.

#### Tear down

```bash
kubectl delete -k k8s/overlays/aws
kubectl -n callprocessor get ingress             # wait until this shows nothing: the ALB is gone
cd terraform && terraform destroy
```

Delete the Kubernetes resources first: the ALB is created by the load balancer controller, not by Terraform. `terraform destroy` also deletes the Atlas cluster and its data. Afterwards, check the AWS console (EKS, EC2 instances, load balancers, NAT gateways, Elastic IPs) to confirm nothing is left.

## Project layout

```
main.go          # config, routes, startup and graceful shutdown
cmd/seed/        # posts sample calls to a running instance
handlers/        # HTTP handlers
persistence/     # MongoDB access
data/            # sample CallRail payload
k8s/             # Kubernetes manifests
terraform/       # AWS (VPC, EKS, ECR) and MongoDB Atlas infrastructure
Dockerfile       # multi-stage build, distroless non-root image
compose.yaml     # local app + MongoDB
```
