data "aws_availability_zones" "available" {
  state = "available"
}

# A VPC across two availability zones (EKS requires at least two).
#   public subnets:  the ALB and the NAT gateway
#   private subnets: the worker nodes, which reach the internet through the NAT gateway
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 6.0"

  name = var.name
  cidr = "10.0.0.0/16"

  azs             = slice(data.aws_availability_zones.available.names, 0, 2)
  private_subnets = ["10.0.0.0/20", "10.0.16.0/20"]
  public_subnets  = ["10.0.128.0/24", "10.0.129.0/24"]

  # One NAT gateway instead of one per zone: cheaper, and fine for a sandbox.
  enable_nat_gateway = true
  single_nat_gateway = true

  # These tags tell the AWS Load Balancer Controller where to put load balancers.
  public_subnet_tags = {
    "kubernetes.io/role/elb" = 1
  }
  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = 1
  }
}
