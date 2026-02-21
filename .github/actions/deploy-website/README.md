# Deploy Website Action

A GitHub Actions composite action that deploys the inGitDB website to Firebase Hosting.

## What This Action Does

This action automates the complete website deployment pipeline:

1. **Validates inputs** — Ensures all required configuration parameters are provided
2. **Checks out repository** — Retrieves the latest code
3. **Authenticates with GCP** — Uses Workload Identity Federation for secure authentication
4. **Sets up Node.js** — Installs the specified Node.js version
5. **Detects Firebase CLI version** — Fetches the latest Firebase CLI version from npm
6. **Caches Firebase CLI** — Caches the Firebase CLI to speed up deployments
7. **Installs Firebase CLI** — Installs Firebase tools if not cached
8. **Configures PATH** — Adds Firebase CLI to the shell PATH
9. **Deploys to Firebase** — Deploys the website hosting target using Firebase CLI

## Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `gcloud_project_id` | Google Cloud project ID | Yes | — |
| `firebase_target` | Firebase hosting target name (e.g., `ingitdb-com`) | Yes | — |
| `firebase_config_path` | Path to firebase.json configuration file | No | `server/firebase.json` |
| `workload_identity_provider` | Workload Identity Pool provider URI | Yes | — |
| `node_version` | Node.js version to use | No | `20` |

## Example Usage

### From Release Workflow

```yaml
deploy-website:
  name: Deploy Website
  needs:
    - goreleaser
  runs-on: ubuntu-latest

  env:
    GCLOUD_PROJECT_ID: ingitdb
    FIREBASE_TARGET: ingitdb-com

  permissions:
    contents: 'read'
    id-token: 'write'

  steps:
    - name: Deploy website
      uses: ./.github/actions/deploy-website
      with:
        gcloud_project_id: ${{ env.GCLOUD_PROJECT_ID }}
        firebase_target: ${{ env.FIREBASE_TARGET }}
        firebase_config_path: server/firebase.json
        workload_identity_provider: projects/152447355531/locations/global/workloadIdentityPools/github/providers/ingitdb
        node_version: "20"
```

### From Standalone Workflow

```yaml
deploy:
  name: Deploy website
  runs-on: ubuntu-latest

  env:
    GCLOUD_PROJECT_ID: ingitdb
    FIREBASE_TARGET: ingitdb-com

  permissions:
    contents: 'read'
    id-token: 'write'

  steps:
    - name: Deploy website
      uses: ./.github/actions/deploy-website
      with:
        gcloud_project_id: ${{ env.GCLOUD_PROJECT_ID }}
        firebase_target: ${{ env.FIREBASE_TARGET }}
        workload_identity_provider: projects/152447355531/locations/global/workloadIdentityPools/github/providers/ingitdb
```

## Prerequisites

1. **GCP Setup**
   - A Google Cloud project with Firebase configured
   - Firebase Hosting enabled
   - Service account with permissions to deploy to Firebase
   - Workload Identity Pool and provider configured for GitHub

2. **Firebase Configuration**
   - `server/firebase.json` file with hosting target configuration
   - Firebase targets configured in `firebase.json`

3. **Node.js**
   - Node.js 18 or later

## Permissions Required

The job must have these permissions:

```yaml
permissions:
  contents: 'read'       # Read repository content
  id-token: 'write'      # Write OIDC tokens for Workload Identity
```

## Notes

- The action uses Workload Identity Federation for secure, keyless authentication to GCP
- Firebase CLI is cached to speed up deployments
- The Firebase CLI version is automatically detected from npm registry
- Deployment is non-interactive, suitable for automated workflows
- The service account must have Firebase Admin permissions

## Troubleshooting

**Missing required inputs error**
- Verify all required inputs are provided
- Check that environment variables are properly defined

**Firebase authentication fails**
- Verify Workload Identity Pool is properly configured
- Check that the service account has Firebase Admin permissions
- Ensure the `id-token` permission is set to `write`

**Firebase deploy fails**
- Verify `firebase.json` exists at the specified path
- Check that the Firebase target exists in `firebase.json`
- Ensure the service account has write access to Firebase Hosting
- Check Firebase project configuration and quotas

**Node.js version incompatibility**
- Ensure the specified Node.js version is available
- Check Firebase CLI compatibility with the selected Node.js version
