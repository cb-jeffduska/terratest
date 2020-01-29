terraform {
  source = "..//bar"
  arguments = [
    "-var-file=terraform.tfvars"
  ]
}
