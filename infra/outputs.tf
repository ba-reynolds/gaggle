output "public_ip" {
  description = "Public IP of the gaggle box (set as the DEPLOY_HOST secret)"
  value       = aws_instance.gaggle.public_ip
}

output "instance_id" {
  value = aws_instance.gaggle.id
}

output "data_volume_id" {
  value = aws_ebs_volume.data.id
}