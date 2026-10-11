---
title: redis
description: Redis-backed pub/sub backend.
---

# Package redis

```go
import redisps "github.com/grx-gql/grx/pubsub/redis"
```

The Redis implementation is a separate Go module so applications that do not
need Redis do not add its dependency to the root module.

See the [pub/sub concept guide](/concepts/pubsub) for configuration and
multi-replica deployment guidance.
