resource "mongodbatlas_project" "this" {
  name   = var.name
  org_id = var.atlas_org_id
}

# Free M0 cluster. Atlas allows one per project.
resource "mongodbatlas_advanced_cluster" "this" {
  project_id   = mongodbatlas_project.this.id
  name         = var.name
  cluster_type = "REPLICASET"

  replication_specs = [{
    region_configs = [{
      provider_name         = "TENANT" # shared tier
      backing_provider_name = "AWS"
      region_name           = var.atlas_region
      priority              = 7

      electable_specs = {
        instance_size = "M0"
      }
    }]
  }]
}

# No special characters, so the password needs no URL-encoding in the connection string.
resource "random_password" "mongodb" {
  length  = 32
  special = false
}

resource "mongodbatlas_database_user" "app" {
  project_id         = mongodbatlas_project.this.id
  username           = "callprocessor"
  password           = random_password.mongodb.result
  auth_database_name = "admin"

  # Read and write (including creating indexes) on the app's database only.
  roles {
    role_name     = "readWrite"
    database_name = var.mongodb_database
  }

  scopes {
    name = mongodbatlas_advanced_cluster.this.name
    type = "CLUSTER"
  }
}

# M0 is reachable only over the public internet, from listed IPs. The pods
# leave the VPC through the NAT gateway, so that is the address Atlas sees.
resource "mongodbatlas_project_ip_access_list" "nat_gateway" {
  project_id = mongodbatlas_project.this.id
  ip_address = module.vpc.nat_public_ips[0]
  comment    = "EKS NAT gateway (${var.name})"
}

locals {
  # Atlas returns "mongodb+srv://host"; insert the credentials after the scheme.
  mongodb_uri = replace(
    mongodbatlas_advanced_cluster.this.connection_strings.standard_srv,
    "mongodb+srv://",
    "mongodb+srv://${mongodbatlas_database_user.app.username}:${random_password.mongodb.result}@",
  )
}
