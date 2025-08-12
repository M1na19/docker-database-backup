# docker-database-backup

Containerized, one-shot backup tool for MySQL/MariaDB, PostgreSQL, and MongoDB with upload to S3-compatible storage.

Image: `ghcr.io/m1na19/docker-database-backup:latest`

It runs the appropriate dump tool(s), gzips the result, stores it locally, and uploads the file(s) to your S3 bucket.

## Quick start (all three DBs)

```bash
docker run --rm \
  --network database_network
  -e MYSQL_URI='mysql://user:pass@db.example.com:3306' \
  -e POSTGRES_URI='postgres://user:pass@db.example.com:5432/postgres' \
  -e MONGO_URI='mongodb://user:pass@db.example.com:27017' \
  -e S3_BUCKET='my-backups' \
  -e AWS_ACCESS_KEY_ID='AKIA...' \
  -e AWS_SECRET_ACCESS_KEY='...' \
  -e AWS_REGION='eu-central-1' \
  -e EXPORT_DIR='/backups' \
  ghcr.io/m1na19/docker-database-backup:latest
```

## What the container does

* **MySQL/MariaDB:** `mysqldump --all-databases --single-transaction --quick --routines --events` → `mysql-YYYY-MM-DD.sql.gz`
* **PostgreSQL:** `pg_dumpall` → `postgres-YYYY-MM-DD.sql.gz`
* **MongoDB:** `mongodump --uri ... --archive --gzip` → `mongo-YYYY-MM-DD.archive.gz` (archive format)


## Environment variables

| Variable                | Required         | Example                                                          | Notes                                                                                                                                                  |
| ----------------------- | ---------------- | ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `MYSQL_URI`             | No\*             | `mysql://user:pass@host:3306`                                    | Set to enable MySQL backup. If unset, MySQL backup is skipped.                                                                                         |
| `POSTGRES_URI`          | No\*             | `postgres://user:pass@host:5432/postgres`                        |Set to enable PostgreSQL backup. If unset, PostgreSQL backup is skipped.                                                                                                    |
| `MONGO_URI`             | No\*             | `mongodb://user:pass@host:27017`                                 | Set to enable MongoDB backup. If unset, MongoDB backup is skipped.                                                                                                                          |
| `EXPORT_DIR`            | No               | `/backups`                                                       | Local directory inside the container where dumps are written. Default: `./backups` (relative to container’s CWD). If folder does not exist one is created |
| `S3_BUCKET`             | Yes               | `my-backups`                                                     | Uploads each dump to this bucket.                                                                                                              |
| `S3_ENDPOINT`           | Yes               | `https://s3.eu-central-1.amazonaws.com` or `https://minio.local` | Custom endpoint (for MinIO, etc.).                                                                                                            |
| `S3_FORCE_PATH_STYLE`   | No               | `true`/`false`                                                   | Use path-style URLs (often required by MinIO).                                                                                                         |
| `AWS_ACCESS_KEY_ID`     | Yes |                                                                  | Standard AWS credentials (or use an instance/role).                                                                                                    |
| `AWS_SECRET_ACCESS_KEY` | Yes |                                                                  |                                                                                                                                                        |
| `AWS_REGION`            | Yes | `eu-central-1`                                                   | Needed by AWS SDK when using AWS S3.                                                                                                                   |
## docker-compose
[View docker-compose.yml](./docker-compose.yml)

## Notes & gotchas
* **Networking:** The container must be able to reach your databases (consider Docker networks, VPNs, or security groups).
* **Mongo output format:** It’s a compressed `mongodump` **archive** (one file). Restore with `mongorestore --gzip --archive=FILE`.
* **S3 credentials/region:** If uploading to AWS S3, set `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_REGION`. For MinIO/custom S3, use `S3_ENDPOINT` and (usually) `S3_FORCE_PATH_STYLE=true`.

## Restore

* **MySQL:**

  ```bash
  gzip -dc mysql-YYYY-MM-DD.sql.gz | mysql -h host -P 3306 -u user -p
  ```

* **PostgreSQL:**

  ```bash
  gzip -dc postgres-YYYY-MM-DD.sql.gz | psql -h host -p 5432 -U user postgres
  ```

* **MongoDB:**

  ```bash
  mongorestore --gzip --archive=mongo-YYYY-MM-DD.archive.gz --uri="mongodb://user:pass@host:27017"
  ```
