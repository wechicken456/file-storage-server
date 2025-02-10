# Test out login api

```bash
curl -X POST --header "Content-Type: application/json" --data '{"email": "admin@tubely.com", "password": "password"}' http://localhost:8091/api/login
```

# Serverless architecture

Doesn't mean there isn't a server. It means the server is managed by someone else.

# AWS S3

## Unique bucket names

Buckets MUST have GLOBALLY UNIQUE names because they are part of the URL used to access them.

So I can't create a bucket called `bootdev` because someone else already created it.

![s3_architecture](./assets/s3_architecture.png)

## Use AWS CLI to upload files to S3 buckets:

```bash
aws s3 cp ./samples/boots-image-horizontal.png s3://tubely-283427619/
```
