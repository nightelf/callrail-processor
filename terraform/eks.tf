module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 21.0"

  name               = var.name
  kubernetes_version = var.kubernetes_version

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  # The Kubernetes API is reachable from the internet (still requires AWS
  # credentials) so kubectl works from your laptop.
  endpoint_public_access       = true
  endpoint_public_access_cidrs = var.cluster_endpoint_allowed_cidrs

  # Gives the identity that runs `terraform apply` admin access to the cluster.
  enable_cluster_creator_admin_permissions = true

  # Sandbox settings: skip the extras that outlive `terraform destroy` or add
  # cost (a KMS key for secret encryption, control plane logs in CloudWatch).
  create_kms_key              = false
  encryption_config           = null
  enabled_log_types           = []
  create_cloudwatch_log_group = false

  addons = {
    vpc-cni                = { before_compute = true } # pod networking
    eks-pod-identity-agent = { before_compute = true } # IAM roles for pods
    kube-proxy             = {}
    coredns                = {}
  }

  eks_managed_node_groups = {
    default = {
      ami_type       = "AL2023_x86_64_STANDARD"
      instance_types = [var.node_instance_type]

      min_size     = var.node_count
      max_size     = var.node_count
      desired_size = var.node_count
    }
  }
}
