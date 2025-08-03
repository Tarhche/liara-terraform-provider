terraform {
  required_providers {
    liara = {
      source = "tarhche/liara"
    }
  }
}

provider "liara" {
  token = "your-api-token"
}

data "liara_example" "example" {
  configurable_attribute = "some-value"
}
