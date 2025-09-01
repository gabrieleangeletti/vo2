terraform {
  backend "s3" {
    bucket = "" # to be configured during init
    key    = "vo2/terraform.tfstate"
    region = "eu-central-1"
  }
}
