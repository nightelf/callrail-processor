# callrail-processor

A small Go HTTP service that receives [CallRail](https://www.callrail.com/) call webhooks and stores them in MongoDB.

Calls are upserted on CallRail's call `id`, so retried or repeated webhooks for the same call update one document instead of creating duplicates.

## API

| Method | Path         | Description                                                                 |
| ------ | ------------ | --------------------------------------------------------------------------- |
| `POST` | `/api/calls` | Save a call. Body is the CallRail JSON payload and must include a string `id`. Returns `201` for a new call, `200` when an existing call was updated. |
| `GET`  | `/api/calls` | List all stored calls.                                                      |
| `GET`  | `/healthz`   | Liveness check. Returns `200` whenever the process is serving HTTP.         |
| `GET`  | `/readyz`    | Readiness check. Returns `200` if MongoDB responds to a ping, `503` if not. |

Errors: `400` for invalid JSON or a missing `id`, `405` for unsupported methods, `500` if the database write or read fails.

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

curl http://localhost:8080/api/calls
```

## Kubernetes

Manifests are in `k8s/` and use [Kustomize](https://kustomize.io/), which is built into `kubectl`.

```
k8s/
├── base/              # Deployment and Service (port 80 → 8080), shared by all environments
└── overlays/
    ├── local/         # Docker Desktop Kubernetes, with an in-cluster MongoDB
    └── aws/           # EKS: ALB Ingress, ECR image, 2 replicas, MongoDB Atlas
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

1. Build for the cluster's architecture and push to ECR:
   ```bash
   docker buildx build --platform linux/amd64 -t <account>.dkr.ecr.<region>.amazonaws.com/callprocessor:<tag> --push .
   ```
2. Set the image in `k8s/overlays/aws/kustomization.yaml`.
3. Copy `k8s/overlays/aws/mongodb.env.example` to `mongodb.env` and add the Atlas connection string. `mongodb.env` is gitignored.
4. Allow the cluster's outbound IP (its NAT gateway) in the Atlas IP access list.
5. Deploy:
   ```bash
   kubectl apply -k k8s/overlays/aws
   ```

The Ingress requires the [AWS Load Balancer Controller](https://kubernetes-sigs.github.io/aws-load-balancer-controller/) in the cluster. It listens on HTTP by default; the annotations for HTTPS with an ACM certificate are in `k8s/overlays/aws/ingress.yaml`, commented out.

## Project layout

```
main.go          # config, routes, startup and graceful shutdown
handlers/        # HTTP handlers
persistence/     # MongoDB access
data/            # sample CallRail payload
k8s/             # Kubernetes manifests
Dockerfile       # multi-stage build, distroless non-root image
compose.yaml     # local app + MongoDB
```
