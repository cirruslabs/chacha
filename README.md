# Chacha

Chacha is caching proxy aimed to speed-up cache retrieval operations for Cirrus Runners. It tries to use the local disk when possible and falls back to a slower backend (such as S3) when no cache hit occurs.
