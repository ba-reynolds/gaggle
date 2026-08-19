variable "region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "us-east-1"
}

variable "instance_type" {
  description = "EC2 instance type (2GiB RAM so the web docker build doesn't OOM)"
  type        = string
  default     = "t3.small"
}

variable "root_volume_size" {
  description = "Root EBS volume size in GiB"
  type        = number
  default     = 20
}

variable "data_volume_size" {
  description = "Attached /data EBS volume size in GiB (docker data-root + volumes)"
  type        = number
  default     = 30
}

variable "admin_public_key" {
  description = "Admin SSH public key (owner access, also the aws_key_pair)"
  type        = string
}

variable "deploy_public_key" {
  description = "Public half of the deploy keypair that GitHub Actions uses to SSH in"
  type        = string
}