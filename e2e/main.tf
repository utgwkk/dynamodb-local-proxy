terraform {
  required_providers {
    aws = "~> 6.0"
  }
  backend "local" {
    path = "terraform.tfstate"
  }
}

provider "aws" {
  region = "ap-northeast-1"

  access_key = "DUMMY"
  secret_key = "DUMMY"

  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true

  endpoints {
    dynamodb = "http://localhost:8888"
  }
}

resource "aws_dynamodb_table" "no_gsi" {
  name         = "test"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "pk"
    type = "S"
  }
  attribute {
    name = "sk"
    type = "S"
  }

  hash_key  = "pk"
  range_key = "sk"
}

resource "aws_dynamodb_table" "with_gsi" {
  name         = "test"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "pk"
    type = "S"
  }
  attribute {
    name = "sk"
    type = "S"
  }
  attribute {
    name = "field1"
    type = "S"
  }
  attribute {
    name = "field2"
    type = "S"
  }

  global_secondary_index {
    name = "gsi1"

    key_schema {
      attribute_name = "field1"
      key_type       = "HASH"
    }

    projection_type = "ALL"
  }

  global_secondary_index {
    name = "gsi2"

    key_schema {
      attribute_name = "field2"
      key_type       = "HASH"
    }
    key_schema {
      attribute_name = "field1"
      key_type       = "RANGE"
    }

    projection_type = "ALL"
  }

  hash_key  = "pk"
  range_key = "sk"
}
