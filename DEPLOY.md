# Production deployment guide

This repository deploys `mdg-tempo-svc` from GitHub Actions to Amazon EC2 through Amazon ECR and AWS Systems Manager (SSM).

The workflow is `.github/workflows/deploy.yml`, named **deploy production ECR**.

## Deployment flow

1. Push to `main` or run the workflow manually.
2. GitHub validates the production environment variables.
3. GitHub assumes an AWS IAM role using OIDC.
4. GitHub creates or updates the ECR repository and pushes:
   - `<ecr-registry>/mdg-tempo-svc:<git-sha>`
   - `<ecr-registry>/mdg-tempo-svc:latest`
5. GitHub sends `AWS-RunShellScript` to the EC2 instance through SSM.
6. EC2 pulls the SHA-tagged image, replaces `mdg-tempo-svc`, and starts it on host port `8081`.
7. The workflow waits for `/healthz` to return successfully on `127.0.0.1:8081`.

## AWS resources

| Resource | Example | Purpose |
|---|---|---|
| AWS region | `ap-southeast-1` | Region for ECR, EC2, and SSM |
| ECR repository | `mdg-tempo-svc` | Stores service images |
| EC2 instance | configured instance ID | Runs Docker and the tempo container |
| IAM role for GitHub Actions | `mdg-tempo-github-actions-prd` | OIDC assume-role, ECR push, and SSM deploy |
| EC2 IAM instance role | `mdg-tempo-svc-ec2-prd` | ECR pull and SSM agent permissions |
| GitHub environment | `mitdo-cloud-tempo-svc-prd` | Holds AWS deployment variables |
| MySQL database | `mdg_tempo` | Tempo, media, tag, history, and Telegram data |

## Create the ECR repository

The workflow creates the repository automatically. To create it beforehand:

```bash
aws ecr create-repository \
  --region "$AWS_REGION" \
  --repository-name mdg-tempo-svc
```

Recommended repository settings:

- Image tag mutability: mutable, for the `latest` tag
- Private repository
- Lifecycle policy for old images
- Registry scanning enabled

## Configure GitHub OIDC and deployment IAM

Create or reuse a GitHub Actions IAM role and scope its OIDC trust policy to this repository and environment:

```text
repo:mitdo-garden/mdg-tempo-svc:environment:mitdo-cloud-tempo-svc-prd
```

The role needs:

- `sts:GetCallerIdentity`
- ECR authorization and repository push permissions
- SSM `SendCommand`, `GetCommandInvocation`, and `ListCommandInvocations`
- permission to wait for and inspect EC2 SSM command status

Example ECR authorization statement:

```json
{
  "Effect": "Allow",
  "Action": ["ecr:GetAuthorizationToken"],
  "Resource": "*"
}
```

Example repository statement:

```json
{
  "Effect": "Allow",
  "Action": [
    "ecr:BatchCheckLayerAvailability",
    "ecr:CompleteLayerUpload",
    "ecr:CreateRepository",
    "ecr:DescribeRepositories",
    "ecr:GetDownloadUrlForLayer",
    "ecr:InitiateLayerUpload",
    "ecr:PutImage",
    "ecr:UploadLayerPart",
    "ecr:BatchGetImage"
  ],
  "Resource": "arn:aws:ecr:<AWS_REGION>:<ACCOUNT_ID>:repository/mdg-tempo-svc"
}
```

Example SSM statement:

```json
{
  "Effect": "Allow",
  "Action": [
    "ssm:SendCommand",
    "ssm:GetCommandInvocation",
    "ssm:ListCommandInvocations"
  ],
  "Resource": "*"
}
```

## Configure the EC2 instance

The instance must have:

- Amazon Linux or a compatible Linux distribution
- Docker installed and running
- AWS CLI v2 installed
- SSM Agent installed, running, and online
- an IAM instance profile attached
- outbound access to ECR, AWS APIs, MySQL, and the auth JWKS endpoint
- TCP port `8081` reachable from the load balancer or reverse proxy
- `/var/lib/mdg-tempo-svc/uploads` available for persistent upload storage

Attach `AmazonSSMManagedInstanceCore` and the following ECR pull permissions to the EC2 instance role:

```json
{
  "Effect": "Allow",
  "Action": ["ecr:GetAuthorizationToken"],
  "Resource": "*"
}
```

```json
{
  "Effect": "Allow",
  "Action": [
    "ecr:BatchCheckLayerAvailability",
    "ecr:BatchGetImage",
    "ecr:GetDownloadUrlForLayer"
  ],
  "Resource": "arn:aws:ecr:<AWS_REGION>:<ACCOUNT_ID>:repository/mdg-tempo-svc"
}
```

## Create the production environment file

Tempo loads a JSON secret from AWS Secrets Manager before MySQL and JWKS validation. Supported keys include `mysql_dsn`, or `mysql_host`, `mysql_port`, `mysql_user`, `mysql_password`, `mysql_database`, and `mysql_params`. Existing environment/YAML values take precedence over secret values. The EC2 instance role needs `secretsmanager:GetSecretValue` on the configured secret.

Create `/etc/mdg-tempo-svc.env` on EC2:

```dotenv
APP_ENV=prd
SERVER_PORT=8081
FRONTEND_URL=https://mitdo.cloud
MYSQL_DSN=<production MySQL DSN>
JWT_ISSUER=mdg-auth-svc
JWT_JWKS_URL=<auth service JWKS URL>
MEDIA_PUBLIC_BASE_URL=<public media URL>
TELEGRAM_BOT_TOKEN=<optional bot token>
TELEGRAM_BOT_USERNAME=mitdobot
TELEGRAM_BOT_NAME=Mitdo
TELEGRAM_CHAT_ID=<optional chat ID>
TELEGRAM_REMIND_INTERVAL=1m
PPROF_ADDR=
AWS_REGION=ap-southeast-1
AWS_SECRETS_MANAGER_SECRET_ID=mdg-tempo/prd
```

Example permissions:

```bash
sudo install -m 600 /dev/null /etc/mdg-tempo-svc.env
sudo vi /etc/mdg-tempo-svc.env
```

The deployment workflow changes this file's permissions to `600` before container startup.

Do not commit `/etc/mdg-tempo-svc.env` or production secrets.

## Configure GitHub environment variables

Create GitHub environment `mitdo-cloud-tempo-svc-prd` and set:

| Variable | Example |
|---|---|
| `AWS_REGION` | `ap-southeast-1` |
| `AWS_ROLE_ARN` | `arn:aws:iam::<ACCOUNT_ID>:role/mdg-tempo-github-actions-prd` |
| `ECR_REPOSITORY` | `mdg-tempo-svc` |
| `EC2_INSTANCE_ID` | `i-0123456789abcdef0` |

## Prepare MySQL

Create the production database and restricted application account, then apply migrations:

```bash
mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p "$MYSQL_DATABASE" \
  < migrations/001_init.sql
```

Apply later migrations in numeric order:

```text
001_init.sql
002_drop_position.sql
003_user_telegram.sql
004_telegram_link_code.sql
```

Verify migrations before deployment when upgrading an existing database.

## Deploy

Push a change to `main` or run **Deploy production ECR** manually from GitHub Actions.

The workflow:

- Builds the Linux Docker image
- Pushes the Git SHA and `latest` tags to ECR
- Sends an SSM command to the configured EC2 instance
- Pulls the SHA-tagged image
- Replaces the existing container
- Starts it with `/etc/mdg-tempo-svc.env`
- Mounts `/var/lib/mdg-tempo-svc/uploads` at `/app/uploads`
- Creates a systemd service
- Waits for `/healthz` on port `8081`

Useful EC2 checks:

```bash
docker ps --filter name=mdg-tempo-svc
docker logs --tail 100 mdg-tempo-svc
curl -fsS http://127.0.0.1:8081/healthz
systemctl status mdg-tempo-svc
```
