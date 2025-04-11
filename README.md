# Chacha

Chacha is an HTTP caching proxy aimed to speed-up cache retrieval operations for [Cirrus Runners](https://cirrus-runners.app/) and VM image fetches through OCI for [Tart](https://tart.run/).

It can store cached entries [on local disk](#disk-cache-disk-optional) with bounded size and work in [cluster mode](#cluster-cache-cluster-optional) to store cache entries on other Chacha nodes (sharding) to increase the overall storage capacity.

Chacha supports [TLS interception](#tls-interceptor-tls-interceptor-optional) to handle HTTPS requests that normally utilize the `CONNECT` method to connect to the target HTTPS server.

Chacha tries to build on:

* [RFC 9110 "HTTP Semantics"](https://datatracker.ietf.org/doc/html/rfc9110)
* [RFC 9111 "HTTP Caching"](https://datatracker.ietf.org/doc/html/rfc9111)
* [RFC 9112 "HTTP/1.1"](https://datatracker.ietf.org/doc/html/rfc9112)

With the following exceptions:

* Chacha is always validating: latency is traded for simplicity and security
* Chacha will only consider caching of the URLs if it matches at least one entry in [`rules`](#rules-rules-optional)
* [`rules`](#rules-rules-optional) allow to override the default standards-like behavior, for example, you can:
  * ignore the existence of `Authorization` header in the request for caching purposes
  * skip certain URL parameters (e.g. `X-Amz-Date` in S3 pre-signed URLs) from the cache key for caching purposes
  * perform a redirect to the other server in the Chacha cluster and ask the client to disable the proxy, thus improving the speed

## Configuration

### Address to listen on (`addr`, required)

Can be implicit (`:8080`, non-cluster mode only) or explicit (`127.0.0.1:8080`, normal and cluster modes).

#### Structure

* `addr` (string, required)

#### Example

```yaml
addr: 127.0.0.1:8080
```

### Disk cache (`disk`, optional)

Enables caching of HTTP response bodies on local disk, with an optional limit after which evictions will occur.

#### Structure

* `disk` (mapping, optional)
  * `dir` (string, required) — directory in which cache entries will be stored
  * `limit` (string, required) — limit (e.g. `50GB`) after which Chacha will start dropping the least recently accessed entries to free up the space

#### Example

```yaml
disk:
  dir: /chacha
  limit: 50GB
```

### TLS interceptor (`tls-interceptor`, optional)

TLS interceptor functionality allows Chacha to support `CONNECT` method, which is usually what proxy clients use to establish the connection with an HTTPS server.

TLS interceptor configuration requires a CA certificate and a key, both of which can be generated using OpenSSL:

```shell
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -keyout key.pem -out cert.pem -sha256 -days 365 -nodes -subj "/CN=Chacha Proxy Server"
```

#### Structure

* `tls-interceptor` (mapping, optional)
  * `cert` (string, required) — path to a CA certificate in PEM format
  * `key` (string, required) — path to a CA private key in PEM format

#### Example

```yaml
tls-interceptor:
  cert: /etc/chacha/root-ca.pem
  key: /etc/chacha/root-key.pem
```

### Rules (`rules`, optional)

Defines URLs to consider caching and offers an escape hatch for violating RFC specifications for these URLs.

#### Structure

* `paths` (sequence, optional)
  * `pattern` (string, required) — regular expression that matches the URL to be proxied
  * `ignore-authorization-header` (boolean, optional) — whether to ignore the existence of `Authorization` header for a given URL request, thus enabling its caching
  * `ignore-parameters` (sequence of strings, optional) — names of URL parameters to not include in the final cache key
  * `direct-connect` (boolean, optional) — enables direct connect functionality, it works like this:
    * when we have an existing and non-stale cache entry for a given request, the client is issued an HTTP 307 redirect to the Chacha cluster server responsible for the requested URL
    * this HTTP 307 redirect contains a `X-Chacha-Direct-Connect` header set to `1` as a hint for the client to disable its proxy and get faster download speed

#### Example

```yaml
paths:
  - pattern: "https:\/\/ghcr.io\/v2\/.*\/blobs\/sha256:[^\/]+"
    ignore-authorization-header: true

  - pattern: "https:\/\/[^\/]+.r2.cloudflarestorage.com\/.*"
    ignore-parameters:
      - "X-Amz-Date"
      - "X-Amz-Signature"
```

### Cluster cache (`cluster`, optional)

Enabling cluster mode distributes Chacha's cache across multiple nodes.

When enabled, Chacha performs a cache operation by picking a single node from `nodes` using a [rendezvous hashing algorithm](https://en.wikipedia.org/wiki/Rendezvous_hashing) with the calculated cache key, and then:

* in case Chacha's own `addr` is identical to the selected node — it'll use a local disk cache, which will additionally be exposed to other nodes via Chacha's KV protocol
* otherwise — Chacha will use a remote disk cache (through Chacha's KV protocol) on the selected node

Structure:

* `cluster` (dict, optional)
  * `secret` (string, required) — secret token used for authentication and authorization between nodes
  * `nodes` (sequence of dicts, required) — a list of nodes responsible for storing the cache entries
    * `addr` (string, required) — address of the Chacha node

Example:

```yaml
addr: 192.168.0.1:8080

cluster:
  secret: "AV8B._W.@cr7-n3ZcnBkUtXy7natj.KN"

  nodes:
    - addr: 192.168.0.2:8080
    - addr: 192.168.0.8:8080
    - addr: 192.168.0.16:8080
```

## Running

```shell
chacha run -f config.yaml
```
