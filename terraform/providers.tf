# AWS credentials come from your environment (for example AWS_PROFILE), never
# from this repository.
provider "aws" {
  region = var.region

  # Every AWS resource gets these tags, so leftovers are easy to find.
  default_tags {
    tags = {
      project    = var.name
      managed-by = "terraform"
    }
  }
}

provider "mongodbatlas" {
  public_key  = var.atlas_public_key
  private_key = var.atlas_private_key
}

# The kubernetes and helm providers talk to the EKS cluster created in eks.tf.
# They fetch a short-lived token with the AWS CLI each time Terraform runs.
locals {
  eks_auth_args = ["eks", "get-token", "--cluster-name", module.eks.cluster_name, "--region", var.region]
}

provider "kubernetes" {
  host                   = module.eks.cluster_endpoint
  cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args        = local.eks_auth_args
  }
}

provider "helm" {
  kubernetes = {
    host                   = module.eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

    exec = {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "aws"
      args        = local.eks_auth_args
    }
  }
}
