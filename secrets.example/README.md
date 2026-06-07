# secrets.example/

Templates for the file-mounted secrets that `docker-compose.yml` expects
under `./secrets/`. **`./secrets/` is gitignored — never commit real values.**

## First run

```sh
cp -r secrets.example secrets
# edit each file under ./secrets/ — see notes below
docker compose up -d --wait
```

## Files

| File | Used by | What goes in it |
|---|---|---|
| `postgres_password` | `postgres` (via `POSTGRES_PASSWORD_FILE`) | A single line: the postgres password. No trailing newline needed; the loader trims trailing whitespace. |
| `database_url` | `migrate`, `ggscale-server` (via `DATABASE_URL_FILE`) | Full connection string: `postgres://USER:PASSWORD@postgres:5432/DB?sslmode=disable`. The password must match `postgres_password`. |
| `ggscale_secret_key` | `doomerang-server` (via `GGSCALE_SECRET_KEY_FILE`) | The project-pinned **secret-tier** API key (`key_type='secret'`) minted from ggscale's dashboard setup. Authorises fleet writes and leaderboard submissions. Empty until you bootstrap ggscale; the server runs unregistered while it's empty. The game client uses a separate **publishable** key (`GGSCALE_PUBLISHABLE_KEY`) that lives in the shipped binary — never in this directory. |

## Rotation

Edit any file under `./secrets/`, then `docker compose up -d <service>` to
restart that service. The compose file mounts files (not values), so a
new file content appears at the next container start.

## Production / shared environments

Replace `./secrets/*` with values rendered from your secrets manager —
HashiCorp Vault Agent, SOPS-decrypted files, k8s `Secret` volumes,
AWS/GCP secret-mount sidecars, etc. The compose contract is just the
file path; how the file gets there doesn't matter to the services.
