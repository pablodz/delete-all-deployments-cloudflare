# delete-all-deployments-cloudflare

Delete Cloudflare Pages deployments from the command line while preserving the live production deployment.

## Setup

Required environment variables:

- `CF_API_TOKEN`: Cloudflare API token.
- `CF_ACCOUNT_ID`: Cloudflare account ID.
- `CF_PAGES_PROJECT_NAME`: Cloudflare Pages project name.

Optional:

- `CF_DELETE_ALIASED_DEPLOYMENTS=true`: Force deletion of aliased deployments.

Create the token at https://dash.cloudflare.com/profile/api-tokens.

Use `Account` permissions, not `Zone` permissions.

Minimum recommended permissions:

- `Account | Cloudflare Pages | Edit`
- `Account | Worker Scripts | Edit` if you are also working with Workers.
- `Account | Worker Routes | Edit` if your workflow touches routes.

## Usage

This project is intended to be used from GitHub Releases.

### Option 1: Install with the helper script

Download and run the installer:

```bash
curl -fsSL https://raw.githubusercontent.com/pablodz/delete-all-deployments-cloudflare/main/install.sh | sh
```

The script:

- detects the current OS and CPU architecture
- downloads the matching release binary when available
- falls back to building from the source release tarball if no matching binary exists

Install to a custom directory:

```bash
curl -fsSL https://raw.githubusercontent.com/pablodz/delete-all-deployments-cloudflare/main/install.sh | INSTALL_DIR="$HOME/.local/bin" sh
```

Install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/pablodz/delete-all-deployments-cloudflare/main/install.sh | VERSION="v1.0.0" sh
```

### Option 2: Download a release asset manually

Example for Linux AMD64:

```bash
curl -fL -o delete-all-deployments-cloudflare.tar.gz \
	https://github.com/pablodz/delete-all-deployments-cloudflare/releases/latest/download/delete-all-deployments-cloudflare_linux_amd64.tar.gz

tar -xzf delete-all-deployments-cloudflare.tar.gz
chmod +x delete-all-deployments-cloudflare
```

Run the binary:

```bash
export CF_API_TOKEN="your_token"
export CF_ACCOUNT_ID="your_account_id"
export CF_PAGES_PROJECT_NAME="your_project_name"
export CF_DELETE_ALIASED_DEPLOYMENTS="false"

./delete-all-deployments-cloudflare
```

## Behavior

- Production deployment is never deleted if it can be identified.
- Deployments are listed in pages of `10`, up to `30` per batch.
- List requests are retried with exponential backoff.
- Failures during single deployment deletion are logged and execution continues.
- API list failures are retried before returning a fatal error.
- If the canonical deployment cannot be fetched, the tool logs a warning and continues.
- A `500ms` delay is applied between API calls.
- The process repeats until no more deployments can be deleted.

## Notes

- There is no single generic Go binary that runs on every operating system.
- If a prebuilt release for your platform does not exist, `install.sh` falls back to downloading the source release and building it locally.

## References

- Cloudflare API token page: https://dash.cloudflare.com/profile/api-tokens
- Cloudflare Pages API reference: https://cfl.re/3CXesln
- Cloudflare note about deleting projects with many deployments: https://developers.cloudflare.com/pages/platform/known-issues/#delete-a-project-with-a-high-amount-of-deployments

## Automated Binary Releases (GitHub Actions)

This repository includes a GitHub Actions release workflow and GoReleaser config for tagged releases.

Publish a release with:

```bash
git tag v1.0.0
git push origin v1.0.0
```
