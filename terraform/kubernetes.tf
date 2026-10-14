# The namespace and the database Secret are created here rather than by
# Kustomize, so the connection string goes from Atlas straight into the
# cluster without passing through a file.

resource "kubernetes_namespace_v1" "app" {
  metadata {
    name = "callprocessor"
  }

  # Deleting a namespace deletes everything in it, including the Ingress. The
  # load balancer controller must still be running at that point to remove the
  # ALB. Depending on it makes `terraform destroy` delete the namespace first.
  depends_on = [helm_release.lb_controller]

  timeouts {
    delete = "15m"
  }
}

# The name and key match what k8s/base/deployment.yaml reads.
resource "kubernetes_secret_v1" "mongodb" {
  metadata {
    name      = "callprocessor-mongodb"
    namespace = kubernetes_namespace_v1.app.metadata[0].name
  }

  data = {
    MONGODB_URI = local.mongodb_uri
  }

  # Don't hand the app credentials before Atlas will accept them.
  depends_on = [
    mongodbatlas_database_user.app,
    mongodbatlas_project_ip_access_list.nat_gateway,
  ]
}
