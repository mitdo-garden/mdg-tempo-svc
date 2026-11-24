# Tempo service deployment

The GitHub Actions workflow builds a Linux image, pushes it to Amazon ECR, and asks EC2 to pull and run the exact commit tag through AWS SSM.

## GitHub environment: `prd`

Configure repository or environment variables:

- `AWS_REGION`
- `AWS_ROLE_ARN`
- `ECR_REPOSITORY` (for example `mdg-tempo-svc`)
- `EC2_INSTANCE_ID`

`AWS_ROLE_ARN` must be assumable through GitHub OIDC and allowed to call ECR and SSM.

## EC2 prerequisites

- Docker installed and running
- AWS CLI installed
- AWS Systems Manager Agent running
- EC2 IAM role allowed `ecr:GetAuthorizationToken`, `ecr:BatchGetImage`, `ecr:GetDownloadUrlForLayer`, and `ecr:BatchCheckLayerAvailability`
- EC2 allowed to receive SSM commands
- `/etc/mdg-tempo-svc.env` contains production environment values
- Port `8081` reachable by your API proxy/load balancer

Minimum env file:

```dotenv
APP_ENV=prd
SERVER_PORT=8081
FRONTEND_URL=http://mitdo.cloud
MYSQL_DSN=<production DSN>
JWT_JWKS_URL=<auth service JWKS URL>
JWT_ISSUER=mdg-auth-svc
MEDIA_PUBLIC_BASE_URL=<public media URL>
TELEGRAM_BOT_TOKEN=<optional>
```

The workflow creates `/var/lib/mdg-tempo-svc/uploads` on the host when Docker mounts it. The container is named `mdg-tempo-svc` and listens on `8081`.
