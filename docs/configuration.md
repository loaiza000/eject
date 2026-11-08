# Offline AWS export manifest

`eject inspect --manifest examples/aws-basic/manifest.json` reads a JSON manifest with `schema_version: "1"` and an `exports` array. Each entry requires `operation`, `file`, `resource`, and `region`. `file` is relative to the manifest directory. Paths that escape it or traverse symlinks are rejected. Each file is limited to 1 MiB and 64 JSON nesting levels; a manifest can name at most 1,000 exports. Unknown manifest versions and fields are rejected.

Supported `operation` values and expected AWS CLI response roots:

| Operation | Response root | Manifest resource |
| --- | --- | --- |
| `ecs.describe-task-definition` | `taskDefinition` | `taskDefinitionArn` |
| `ecs.describe-services` | `services` | matching `serviceArn` |
| `rds.describe-db-instances` | `DBInstances` | matching `DBInstanceArn` |
| `s3.get-bucket-versioning` | `Status`, optionally `MFADelete` | bucket name |
| `s3.get-bucket-encryption` | `ServerSideEncryptionConfiguration` | bucket name |

These are actual AWS CLI JSON response shapes, not an EJECT-specific export format. The example files use fictitious account IDs, hosts, and resources. AWS CLI commands to obtain similar snapshots are shown for reference; EJECT never runs them:

```sh
aws ecs describe-task-definition --task-definition demo --region us-east-1 > task-definition.json
aws ecs describe-services --cluster demo --services app --region us-east-1 > services.json
aws rds describe-db-instances --db-instance-identifier demo-db --region us-east-1 > db-instances.json
aws s3api get-bucket-versioning --bucket demo-bucket --region us-east-1 > bucket-versioning.json
aws s3api get-bucket-encryption --bucket demo-bucket --region us-east-1 > bucket-encryption.json
```

Review exports before sharing them. They may contain credentials, endpoint addresses, or other sensitive infrastructure information even though EJECT only emits a subset of fields. A missing S3 encryption response or absent metadata does not prove a bucket is unencrypted. RDS metadata does not contain database contents. ECS environment values and secret references are not written to the inventory; only variable names are recorded. A port mapping without `protocol` is recorded as `tcp`, the ECS default; protocols other than `tcp` and `udp` are rejected.

The output `schema_version: "1"` contains sorted `resources`, `dependencies`, and `findings`. Dependencies have `CONFIRMED` status only when a source field explicitly names the target resource. References without supplied target snapshots produce a `MISSING_REFERENCE` finding. Other relationships are not inferred. Changes to this schema will be versioned.
