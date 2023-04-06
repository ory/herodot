// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

// StatusClientClosedRequest (reported as 499 Client Closed Request) is a faux
// but de-facto standard HTTP status code first used by nginx, indicating the
// client canceled the request. Because the client canceled, it is never
// actually reported back to them. 499 is useful purely in logging, tracing,
// etc.
//
// http://nginx.org/en/docs/dev/development_guide.html
const StatusClientClosedRequest int = 499
