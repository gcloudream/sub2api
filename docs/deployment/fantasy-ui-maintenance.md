# Fantasy UI maintenance

The production deployment uses an image with a frontend compiled into the application binary. The host deployment mounts runtime data, not frontend source.

## Upgrade workflow

1. Start from the upstream release tag in a local maintenance branch.
2. Reconstruct the Fantasy UI changes as source commits under `frontend/`.
3. Build a candidate image from the branch.
4. Run the candidate on the preview deployment and compare login, registration, dashboard, and account pages with the recorded production asset snapshot.
5. Back up the database and runtime configuration before switching production.
6. Switch the production image only after the preview checks pass.

Runtime data, database storage, and secrets stay outside this repository. Do not commit credentials, database dumps, or complete authentication files.
