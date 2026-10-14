variable "name" {
  description = "Name used for the cluster, the VPC, the ECR repository and the Atlas project."
  type        = string
  default     = "callrail-processor"
}

variable "region" {
  description = "AWS region."
  type        = string
  default     = "us-west-2"
}

# ---- EKS ----

variable "kubernetes_version" {
  description = "EKS Kubernetes version. Must be one EKS currently offers in standard support."
  type        = string
  default     = "1.36"
}

variable "node_instance_type" {
  description = "EC2 instance type for the worker nodes (x86_64)."
  type        = string
  default     = "t3.small"
}

variable "node_count" {
  description = "Number of worker nodes."
  type        = number
  default     = 2
}

variable "cluster_endpoint_allowed_cidrs" {
  description = "IP ranges allowed to reach the Kubernetes API (kubectl). Narrow this to [\"<your-ip>/32\"] for a tighter setup."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

# ---- MongoDB Atlas ----

variable "atlas_org_id" {
  description = "MongoDB Atlas organization ID (Organization > Settings)."
  type        = string
}

variable "atlas_public_key" {
  description = "Atlas API public key. Needs the Organization Project Creator role."
  type        = string
  sensitive   = true
}

variable "atlas_private_key" {
  description = "Atlas API private key."
  type        = string
  sensitive   = true
}

variable "atlas_region" {
  description = "Atlas name for the AWS region the free cluster runs in. Keep it the same region as var.region."
  type        = string
  default     = "US_WEST_2"
}

variable "mongodb_database" {
  description = "Database the app uses. Must match MONGODB_DATABASE in k8s/base/deployment.yaml."
  type        = string
  default     = "callrail"
}

# ---- Cost safety ----

variable "budget_alert_email" {
  description = "Email for AWS budget alerts. Leave empty to skip creating a budget."
  type        = string
  default     = ""
}

variable "budget_limit_usd" {
  description = "Monthly budget for the whole AWS account, in USD. Alerts are sent at 50% and 100%."
  type        = number
  default     = 10
}
