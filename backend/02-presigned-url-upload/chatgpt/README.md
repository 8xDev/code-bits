# Posts Presigned Upload Demo

This project demonstrates direct-to-object-storage (MinIO) presigned uploads (POST & PUT), multipart-init flow for larger files, SSE encryption (SSE-S3), MinIO webhook notifications processed by a worker, and a secure simple API.

## Key features
- Presigned POST and PUT upload flows (client uploads directly to MinIO).
- Multipart upload initiation for large/resumable uploads.
- Server-side encryption using SSE-S3 for stored objects.
- API key-based write authorization.
- MinIO webhook to a worker service for post-processing (thumbnailing/transcoding hooks).
- DB stores `object_key` (not raw credentials) and serves constructed public URLs in dev.
- Frontend demonstrates presigned flow for POST & PUT.

## Quickstart
1. Copy `.env.example` to `.env` and set values.
2. `docker compose up --build`
3. App: http://localhost:8080
4. Swagger: http://localhost:8080/swagger
5. MinIO console: http://localhost:9001 (use root creds from `.env`)

## Flow (recommended)
1. Client calls `POST /api/uploads/init` with `filename` and optionally `method=post|put`.
2. Server returns presigned POST form data or presigned PUT URL and `objectKey`.
   - **Important**: For presigned PUT URLs, the client **must include the `Content-Type` header** when uploading.  
   - SSE-S3 encryption is applied automatically by MinIO bucket policy.
3. Client uploads directly to MinIO using the returned info.
4. Client calls `POST /api/posts` with `title`, `description`, and `object_key` to create the DB record (server validates via HeadObject).
5. MinIO sends event notification to worker (for post-processing) — worker just logs for now.

## Security & Production notes
- **API Key**: simple `X-API-KEY` header for write endpoints. Replace with OAuth/OpenID Connect in production.
- **SSE**: uses SSE-S3 (server side encryption managed by MinIO) to keep objects encrypted at rest.
- **CORS**: configure allowed origins via `.env`.
- **Multipart**: implemented init + part presign in a demo-friendly manner — for production consider AWS multipart flow or multipart SDK helpers.
- **Switching to S3**: minimal code changes — swap MinIO client with AWS SDK or reuse S3-compatible endpoints and credentials. Keep storing `object_key` and generate signed GETs for private access if needed.

---

# Notes about limitations & assumptions (transparent)
- The `PresignMultipartPart` method returns a presigned URL built from a presigned object URL with appended `partNumber` and `uploadId` query params. In some S3/MinIO environments you may need a more precise signing approach; the current implementation is a demo to illustrate flow. For production multipart uploads you may prefer generating presigned URLs for each part using AWS S3 SDK `Presign` for `UploadPart` with exact query parameters included in the signature.
- The `minio-setup` service attempts to configure a webhook for MinIO in a best-effort manner. Depending on MinIO versions, admin API usage, and security, you may need to manually configure bucket notifications or expand the setup script.
- The sample worker simply logs events. Expand it to download objects, generate thumbnails, transcode videos, or enqueue work for background processing.
- SSE-S3 is used for server-side encryption at rest. If you require customer-provided encryption keys (SSE-C) or KMS integration, adapt the MinIO options and key management accordingly.
- Presigned PUT URLs **do not embed SSE headers**, so encryption is applied by MinIO automatically. The client only needs to set `Content-Type`.

---
