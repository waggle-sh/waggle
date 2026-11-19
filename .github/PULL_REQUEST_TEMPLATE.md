<!-- SPDX-License-Identifier: Apache-2.0 -->
## What and why

<!-- One or two sentences. Link the issue or task, for example "Closes #123" or "T-123". -->

## Checklist

- [ ] Every commit is signed off (`git commit -s`); the `dco` check fails without `Signed-off-by`.
- [ ] `go build ./...`, `go vet ./...` and `go test -race ./...` pass, and `gofmt -l .` prints nothing.
- [ ] New files start with an SPDX license header.
- [ ] No em dashes or en dashes in code, docs or commits.
- [ ] Docs updated if behavior changed.

<!-- The repo's gate checks leases and claims. If it fails on a PR from a fork, a maintainer may need to write a claim for your branch; this is expected. See CONTRIBUTING.md. -->
