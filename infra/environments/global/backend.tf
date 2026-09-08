terraform {
  backend "gcs" {
    bucket = "tfstate-fastfso-com"
    prefix = "environments/global"
  }
}
