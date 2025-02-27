# Test out login api

```bash
curl -X POST --header "Content-Type: application/json" --data '{"email": "admin@tubely.com", "password": "password"}' http://localhost:8091/api/login
```

# Go Functional Options Pattern

Read [this](https://golang.cafe/blog/golang-functional-options-pattern.html)

```Go
func New(options ...func(*Server)) *Server {
  svr := &Server{}
  for _, set_config_func := range options {
    set_config_func(svr)
  }
  return svr
}
```
Essentially, it allows us to define a fixed type signature for ANY possible configuration of a structure by creating functions that set the values for our structure.

# Presigned URLs
Creates URLs with an expiration time to access resources. Still serve content from the S3 bucket

Good for *truly* prviate content. A CDN like AWS CloudFront offers better and performance security than serving files directly from S3 buckets.

Does NOT require user to be logged in - it's just an URL that expires.

=> Temporary access to private S3 objects .

The most important thing about presigned URLs is that they **authenticate** you, but they do not **authorize** you. The distinction between these two concepts is important but it's pretty easy to gloss over it when dealing with most situations in AWS. 

Authenticate: Your ID is authentic, you are who you say you are

Authorize: You are authorized to perform this task (or access this resource, etc) 


# Serverless architecture

Doesn't mean there isn't a server. It means the server is managed by someone else.

# AWS S3

Uses **object store** instead of **file system**.

=> manges data as "blobs" or "objects" as oppposed to hierachies of directory. 
Think a giant hashmap of the keys being the file names.

The illusion of directories in the file names are just prefixes to the keys. Prefixes are used to group objects together based on certain features that you want to operate on (e.g. delete all images for a specific user, resize all the images belonging to a particular feature)
It just makes it easier to think about them as directories :) 

***Metadata*** is stored *separately from* the object, and you can have ***variable*** amount of metadata. This means that the data is **unstructured**.

A **region** has multiple **zones**, a zone has multiple **data centers**, and your **S3 bucket** is *replicated* across *multiple* **zones** in a *single* **region**.

![S3 regions](./assets/s3_regions.png)

# AWS CloudFront and CDNs
A Content Delivery Network (CDN) is a network of servers that serves content based on the requesting geographic location.

=> Lower latency for users far away from the **origin server**.

S3 bucket lives on the **origin** server (e.g. us-east-2), and **edge** servers are CloudFront servers (e.g. australia). When the origin server updates, the edge servers update their caches.

![CDNs](./assets/CDNs.png)


## Create pollicies to allow S3 object manipulation
Example:

```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Statement1",
            "Effect": "Allow",
            "Action": [
                "s3:PutObject",
                "s3:GetObject",
                "s3:DeleteObject",
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::tubely-283427619",
                "arn:aws:s3:::tubely-283427619/*"
            ]
        }
    ]
}
```


## Unique bucket names

Buckets MUST have GLOBALLY UNIQUE names because they are part of the URL used to access them.

So I can't create a bucket called `bootdev` because someone else already created it.

![s3_architecture](./assets/s3_architecture.png)

## Use AWS CLI to upload files to S3 buckets:

```bash
aws s3 cp ./samples/boots-image-horizontal.png s3://tubely-283427619/
```

## MP4 video fetch

The developer network tab shows that there are multiple `GET` requests to get the MP4 video.


1. 1st request: The `Range` header in the request is `bytes=0-`. This says "give me all the bytes". The response's `Content-Range` header tells the MP4 size, but the size of the response doesn't have that many bytes?? Strange...

2. 2nd request: The `Range` header in the request is `bytes=XXXXXX-`. This says "give me all the bytes from `XXXXXX` to the end". But `XXXXXX` is NOT contiguous with the # of bytes transferred in the 1st request. In other words, the browser is ONLY downloading the *end* of the MP4 file now. Interesting 

3. 3rd request: it's just getting a bit more from the start of the file.


So what's the deal? Well, "traditional" MP4 file, the `moov` (metadata) is at the **end** of the file. So the browswer needs to know how many bytes the video is (from the first request) in order to send a second request with `Range` equals to a reasonable offset from the end of the file to get its metadata. More [here](https://surma.dev/things/range-requests/#blobdef)

But video files CAN have its metadata at the front as well.

We can use `ffmpeg` to edit the video to put the metadata to the beginning

=> Only 1 request needed to start playing the video. However, as we play/skip the video, more requests will be sent.






