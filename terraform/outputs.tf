output "cluster_name" {
  description = "EKS cluster name."
  value       = module.eks.cluster_name
}

output "configure_kubectl" {
  description = "Run this to point kubectl at the cluster."
  value       = "aws eks update-kubeconfig --region ${var.region} --name ${module.eks.cluster_name}"
}

output "ecr_repository_url" {
  description = "Where to push the callprocessor image."
  value       = aws_ecr_repository.app.repository_url
}

output "nat_public_ip" {
  description = "The cluster's outbound IP, allowed in the Atlas IP access list."
  value       = module.vpc.nat_public_ips[0]
}

output "atlas_project_id" {
  description = "MongoDB Atlas project ID."
  value       = mongodbatlas_project.this.id
}
