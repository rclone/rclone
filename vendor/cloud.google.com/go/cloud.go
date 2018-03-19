// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package cloud is the root of the packages used to access Google Cloud
Services. See https://godoc.org/cloud.google.com/go for a full list
of sub-packages.

Examples in this package show ways to authorize and authenticate the
sub packages.

Connection Pooling

Connection pooling differs in clients based on their transport. Cloud
clients either rely on HTTP or gRPC transports to communicate
with Google Cloud.

Cloud clients that use HTTP (bigquery, compute, storage, and translate) rely on the
underlying HTTP transport to cache connections for later re-use. These are cached to
the default http.MaxIdleConns and http.MaxIdleConnsPerHost settings in
http.DefaultTransport.

For gPRC clients (all others in this repo), connection pooling is configurable. Users
of cloud client libraries may specify option.WithGRPCConnectionPool(n) as a client
option to NewClient calls. This configures the underlying gRPC connections to be
pooled and addressed in a round robin fashion.

*/
package cloud // import "cloud.google.com/go"
