# Chacha

Chacha is caching proxy aimed to speed-up cache retrieval operations for Cirrus Runners.

It tries to use the local disk when possible and falls back to a slower backend (such as S3) when no cache hit occurs.

HTTP cache and GHA cache protocols are currently implemented.

Authentication can be done either by using `Authorization: Basic` (primarily intended for HTTP cache) or via `Authorization: Bearer` (primarily intended for GHA cache protocol). The password (in case of basic access authentication) and the token (in case of bearer authentication) excepted to be a JWT token provided by [GitHub](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect), [Gitlab](https://docs.gitlab.com/ee/ci/secrets/id_token_authentication.html) or any other OIDC provider.

## Configuration

### OIDC providers (`oidc-providers` section)

To achieve secure multi-tenancy and prevent cache poisoning by malicious PRs we need to namespace cache keys.

Since each cache operation will be authenticated using a JWT token, Chacha provides a way to dynamically generate cache prefixes using [Expr](https://expr-lang.org/) expressions.

These expressions can access the JWT token contents through `claims` field and should always return a `string`.

Here's an example configuration for GitHub and GitLab:

```yaml
oidc-providers:
  # https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect
  - url: https://token.actions.githubusercontent.com
    cache_key_exprs:
      - '"github/" + claims.repository + "/" + claims.ref'
      - '"github/" + claims.repository'

  # https://docs.gitlab.com/ee/ci/secrets/id_token_authentication.html
  - url: https://gitlab.com
    cache_key_exprs:
      - '"gitlab/" + claims.project_path + "/" + claims.ref_path'
      - '"gitlab/" + claims.project_path'
```

Here you can see that multiple cache key expressions are specified. This acts as a fallback for read-only operations, which has an effect of increasing the cache hit-rate for non-default branches.

### Local and remote caches (`disk` and `s3` sections)

These sections let you specify the local and remote cache configurations.

For example, with the following configuration:

```yaml
disk:
  dir: /cache
  limit: 50GB

s3:
  bucket: chacha
```

The on-disk cache will be consulted first on cache retrieval operation, and if no cache hit occurs, it will fall back to S3, additionally populating the on-disk cache.

## Running

```shell
chacha run -f config.yaml
```
