# Test out login api

```bash
curl -X POST --header "Content-Type: application/json" --data '{"email": "admin@tubely.com", "password": "password"}' http://localhost:8091/api/login
```

# Serverless architecture

Doesn't mean there isn't a server. It means the server is managed by someone else.

# AWS S3

Uses **object store** instead of **file system**.

=> manges data as "blobs" or "objects" as oppposed to hierachies of directory. 
Think a giant hashmap of the keys being the file names.

The illusion of directories in the file names are just prefixes to the keys. Prefixes are used to group objects together based on certain features that you want to operate on (e.g. delete all images for a specific user, resize all the images belonging to a particular feature)
It just makes it easier to think about them as directories :) 

***Metadata*** is stored *separately from* the object, and you can have ***variable*** amount of metadata. This means that the data is **unstructured**.


## Unique bucket names

Buckets MUST have GLOBALLY UNIQUE names because they are part of the URL used to access them.

So I can't create a bucket called `bootdev` because someone else already created it.

![s3_architecture](./assets/s3_architecture.png)

## Use AWS CLI to upload files to S3 buckets:

```bash
aws s3 cp ./samples/boots-image-horizontal.png s3://tubely-283427619/
```
