# Handling rate limited requests

A GitLab instance may be behind a reverse proxy that has rate-limiting on API requests
to prevent abuse. GitLab Runner sends multiple requests to the API and could go over these
rate limits. As a result, GitLab Runner handles rate limited scenarios with the following logic:

1. A response code of **429 - TooManyRequests** is received.

1. The response headers are checked for a `RateLimit-ResetTime` header. The `RateLimit-ResetTime` header should have a value which is a valid **HTTP Date (RFC1123)**, like `Wed, 21 Oct 2015 07:28:00 GMT`.

- If the header is present and has a valid value the Runner waits until the specified time and issues another request.
- If the header is present, but isn't a valid date, a fallback of **1 minute** is used.
- If the header is not present, no additional actions are taken, the response error is returned.

1. The process above is repeated for 5 times, then a `gave up due to rate limit` error is returned.

NOTE: **Note:**
The header `RateLimit-ResetTime` is case insensitive since all header keys are run
through the [http.CanonicalHeaderKey](https://golang.org/pkg/net/http/#CanonicalHeaderKey) function.
