# dynamodb-local-proxy

## Motivation

`dynamodb-local-proxy` is a lightweight HTTP proxy for DynamoDB Local that automatically supplements the `DescribeTable` API response with the missing `WarmThroughput` field.

It solves the compatibility issue where **terraform-provider-aws v6.13+** (and potentially other modern clients) now expect `WarmThroughput` in `DescribeTable` output[^1]. DynamoDB Local (up to 3.3.0) does not return this field, causing Terraform to hang indefinitely waiting for table creation and eventually time out.

By sitting in front of DynamoDB Local and injecting a dummy `WarmThroughput` value only for `DescribeTable` calls, the proxy makes DynamoDB Local fully compatible with the latest Terraform AWS provider without changing any application code.

詳しくは [TerraformでDynamoDB localのテーブルを作成しようとするとタイムアウトする現象 - 私が歌川です](https://blog.utgw.net/entry/2026/03/29/220347) を読んでください。

[^1]: https://github.com/hashicorp/terraform-provider-aws/pull/41308

## Usage

### With Docker Compose

```yml
services:
  dynamodb-local:
    command: -jar DynamoDBLocal.jar -sharedDb -dbPath ./data
    image: amazon/dynamodb-local
    volumes:
      - "./docker/dynamodb:/home/dynamodblocal/data"
    working_dir: /home/dynamodblocal
  dynamodb-local-proxy:
    image: ghcr.io/utgwkk/dynamodb-local-proxy
    environment:
      - DYNAMODB_LOCAL_ADDR=dynamodb-local:8000
      - HOST=0.0.0.0
      - PORT=8888
    ports:
      - "8888:8888"
```

You can get patched response by executing `aws --endpoint-url=http://localhost:8888 dynamodb describe-table --table-name <table-name>`.
