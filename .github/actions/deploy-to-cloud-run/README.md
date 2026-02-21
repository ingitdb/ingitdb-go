# Deploy to Cloud Run Action

A GitHub Actions composite action that builds and deploys the inGitDB server to Google Cloud Run.

## What This Action Does

This action automates the complete deployment pipeline for the inGitDB server:

1. **Validates inputs** — Ensures all required configuration parameters are provided
2. **Downloads release binary** — Fetches the latest inGitDB release from GitHub Releases
3. **Authenticates with GCP** — Uses Workload Identity Federation for secure authentication
4. **Checks for existing image** — Skips build/push if the Docker image already exists
5. **Builds Docker image** — Compiles the Docker image with versioning tags
6. **Pushes to Artifact Registry** — Uploads the image to Google Cloud's Artifact Registry
7. **Deploys to Cloud Run** — Updates the Cloud Run service with new configuration and environment variables
8. **Reports service URL** — Displays the deployed service URL

## Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `gcloud_project_id` | Google Cloud project ID | Yes | — |
| `gcloud_region` | Google Cloud region (e.g., `europe-west3`) | Yes | — |
| `service_name` | Cloud Run service name | Yes | — |
| `ingitdb_github_oauth_client_id` | GitHub OAuth Client ID | Yes | — |
| `ingitdb_github_oauth_callback_url` | GitHub OAuth callback URL | Yes | — |
| `ingitdb_auth_cookie_domain` | Domain for authentication cookies | Yes | — |
| `ingitdb_auth_api_base_url` | Base URL for authentication API | Yes | — |
| `ingitdb_auth_cookie_name` | Name of the auth session cookie | No | `ingitdb_session` |
| `ingitdb_auth_cookie_secure` | Enable secure flag on auth cookies | No | `true` |
| `gcp_oauth_client_secret_name` | GCP Secret Manager secret name for OAuth client secret | Yes | — |
| `workload_identity_provider` | Workload Identity Pool provider URI | Yes | — |

## Example Usage

### From Release Workflow

```yaml
deploy:
  name: Deploy to Cloud Run
  needs:
    - release
  runs-on: ubuntu-latest

  env:
    GCLOUD_PROJECT_ID: ingitdb
    GCLOUD_REGION: europe-west3
    SERVICE_NAME: ingitdb-server
    INGITDB_GITHUB_OAUTH_CLIENT_ID: ${{ vars.INGITDB_GITHUB_OAUTH_CLIENT_ID }}
    INGITDB_GITHUB_OAUTH_CALLBACK_URL: ${{ vars.INGITDB_GITHUB_OAUTH_CALLBACK_URL }}
    INGITDB_AUTH_COOKIE_DOMAIN: ${{ vars.INGITDB_AUTH_COOKIE_DOMAIN }}
    INGITDB_AUTH_API_BASE_URL: ${{ vars.INGITDB_AUTH_API_BASE_URL }}
    INGITDB_AUTH_COOKIE_NAME: ${{ vars.INGITDB_AUTH_COOKIE_NAME }}
    INGITDB_AUTH_COOKIE_SECURE: ${{ vars.INGITDB_AUTH_COOKIE_SECURE }}
    GCP_OAUTH_CLIENT_SECRET_NAME: ${{ vars.GCP_OAUTH_CLIENT_SECRET_NAME }}

  permissions:
    contents: 'read'
    id-token: 'write'
    packages: 'read'

  steps:
    - name: Deploy to Cloud Run
      uses: ./.github/actions/deploy-to-cloud-run
      with:
        gcloud_project_id: ${{ env.GCLOUD_PROJECT_ID }}
        gcloud_region: ${{ env.GCLOUD_REGION }}
        service_name: ${{ env.SERVICE_NAME }}
        ingitdb_github_oauth_client_id: ${{ env.INGITDB_GITHUB_OAUTH_CLIENT_ID }}
        ingitdb_github_oauth_callback_url: ${{ env.INGITDB_GITHUB_OAUTH_CALLBACK_URL }}
        ingitdb_auth_cookie_domain: ${{ env.INGITDB_AUTH_COOKIE_DOMAIN }}
        ingitdb_auth_api_base_url: ${{ env.INGITDB_AUTH_API_BASE_URL }}
        ingitdb_auth_cookie_name: ${{ env.INGITDB_AUTH_COOKIE_NAME }}
        ingitdb_auth_cookie_secure: ${{ env.INGITDB_AUTH_COOKIE_SECURE }}
        gcp_oauth_client_secret_name: ${{ env.GCP_OAUTH_CLIENT_SECRET_NAME }}
        workload_identity_provider: projects/152447355531/locations/global/workloadIdentityPools/github/providers/ingitdb
```

## Prerequisites

1. **GCP Setup**
   - A Google Cloud project with Cloud Run enabled
   - Artifact Registry configured for Docker images
   - Service account with permissions to deploy to Cloud Run and manage Artifact Registry
   - Workload Identity Pool and provider configured for GitHub

2. **Repository Secrets**
   - `INGITDB_GORELEASER_GITHUB_TOKEN` — GitHub token for accessing releases

3. **Repository Variables**
   - `INGITDB_GITHUB_OAUTH_CLIENT_ID` — GitHub OAuth application client ID
   - `INGITDB_GITHUB_OAUTH_CALLBACK_URL` — GitHub OAuth callback URL
   - `INGITDB_AUTH_COOKIE_DOMAIN` — Cookie domain
   - `INGITDB_AUTH_API_BASE_URL` — Auth API base URL
   - `INGITDB_AUTH_COOKIE_NAME` — Auth cookie name (optional, defaults to `ingitdb_session`)
   - `INGITDB_AUTH_COOKIE_SECURE` — Secure cookie flag (optional, defaults to `true`)
   - `GCP_OAUTH_CLIENT_SECRET_NAME` — GCP Secret Manager secret name

4. **GitHub Releases**
   - A published release with `ingitdb_*_linux_amd64.tar.gz` binary

## Cloud Run Configuration

The action deploys with the following Cloud Run settings:

- **Platform**: Managed
- **Authentication**: Unauthenticated (public access)
- **Min instances**: 0 (auto-scale down)
- **Max instances**: 2 (auto-scale up)
- **Memory**: 512Mi
- **CPU**: 1
- **Timeout**: 300 seconds

## Permissions Required

The job must have these permissions:

```yaml
permissions:
  contents: 'read'       # Read repository content and releases
  id-token: 'write'      # Write OIDC tokens for Workload Identity
  packages: 'read'       # Read container registry
```

## Notes

- The action uses Workload Identity Federation for secure, keyless authentication to GCP
- If the Docker image already exists in Artifact Registry, the build/push/deploy steps are skipped
- The service URL is printed at the end of the deployment
- All environment variables are securely passed as secrets or variables
- The OAuth client secret is retrieved from GCP Secret Manager at deployment time

## Troubleshooting

**Missing required inputs error**
- Verify all required inputs are provided
- Check that environment variables are properly defined and accessible

**Docker image build fails**
- Ensure `server/Dockerfile` exists in the repository
- Check Docker build context and dependencies

**GCP authentication fails**
- Verify Workload Identity Pool is properly configured
- Check that the service account has the necessary permissions
- Ensure the `id-token` permission is set to `write`

**Cloud Run deployment fails**
- Verify the service account has `roles/run.admin` permission
- Check that the Cloud Run service exists or can be created
- Ensure environment variables and secrets are properly configured in GCP
