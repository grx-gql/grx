---
title: pubsub
description: Backend-neutral byte-oriented pub/sub contract.
---

# Package pubsub

```go
import "github.com/grx-gql/grx/pubsub"
```

`pubsub.PubSub` is the contract used by GraphQL resolvers to publish and
subscribe to opaque byte payloads. It has no dependency on a concrete broker.

See the [pub/sub concept guide](/concepts/pubsub) for wiring examples and
payload ownership rules.
