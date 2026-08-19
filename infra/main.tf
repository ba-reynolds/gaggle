terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-2023.*-x86_64"]
  }
  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

data "aws_vpc" "default" {
  default = true
}

resource "aws_key_pair" "admin" {
  key_name   = "gaggle-admin"
  public_key = var.admin_public_key
}

resource "aws_security_group" "gaggle" {
  name        = "gaggle-sg"
  description = "Gaggle web (80/443) + SSH (22) - key-only"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "gaggle" {
  ami                         = data.aws_ami.al2023.id
  instance_type               = var.instance_type
  vpc_security_group_ids      = [aws_security_group.gaggle.id]
  associate_public_ip_address = true
  key_name                    = aws_key_pair.admin.key_name

  user_data = templatefile("${path.module}/bootstrap.sh", {
    admin_public_key  = var.admin_public_key
    deploy_public_key = var.deploy_public_key
  })

  root_block_device {
    volume_type = "gp3"
    volume_size = var.root_volume_size
  }

  tags = {
    Name = "gaggle-web"
  }
}

resource "aws_ebs_volume" "data" {
  availability_zone = aws_instance.gaggle.availability_zone
  size              = var.data_volume_size
  type              = "gp3"

  tags = {
    Name = "gaggle-data"
  }
}

resource "aws_volume_attachment" "data" {
  device_name = "/dev/sdf"
  volume_id   = aws_ebs_volume.data.id
  instance_id = aws_instance.gaggle.id
}