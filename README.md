# delete-all-deployments-cloudflare

Delete Cloudflare Pages deployments while preserving the live production deployment.

## Use Latest Release

Download the binary for your OS/arch from the latest release and place it in the current folder:

```bash
curl -fsSL https://raw.githubusercontent.com/pablodz/delete-all-deployments-cloudflare/main/install.sh | sh
```

Run the binary with your environment variables:

```bash
export CF_API_TOKEN="your_token"
export CF_ACCOUNT_ID="your_account_id"
export CF_PAGES_PROJECT_NAME="your_project_name"
export CF_DELETE_ALIASED_DEPLOYMENTS="false"

./delete-all-deployments-cloudflare
```

Required variables:

- `CF_API_TOKEN`
- `CF_ACCOUNT_ID`
- `CF_PAGES_PROJECT_NAME`

Optional:

- `CF_DELETE_ALIASED_DEPLOYMENTS=true`

Token permissions (Account level):

- `Cloudflare Pages | Edit`
- `Worker Scripts | Edit` (if needed)
- `Worker Routes | Edit` (optional)

Create token: https://dash.cloudflare.com/profile/api-tokens

## Build From Repository

```bash
git clone https://github.com/pablodz/delete-all-deployments-cloudflare.git
cd delete-all-deployments-cloudflare
go build -o delete-all-deployments-cloudflare .
./delete-all-deployments-cloudflare
```
