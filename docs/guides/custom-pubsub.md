---
title: Custom pub/sub
description: Build a pub/sub backend for grx subscriptions.
outline: deep
---

# Custom pub/sub

Use a custom pub/sub backend when subscriptions must share events across
processes or use infrastructure that is not provided by the built-in memory
backend.

Start with the `pubsub.PubSub` interface in the
[API reference](/reference/pubsub/). Then define delivery, cancellation,
buffering, and failure behavior for your backend.

## Choose the delivery model first

- An in-memory backend is process-local and best effort.
- A shared backend can deliver events to several service replicas.
- A durable broker can support replay or recovery, but the resolver contract
  still needs clear behavior for duplicates and disconnected clients.

Document whether your backend guarantees ordering, duplicate suppression,
durability, and delivery after a subscriber reconnects.

## Backend responsibilities

A backend must handle topic ownership, subscriber cancellation, cleanup after
disconnects, safe payload ownership, and errors from the underlying broker.
Bound channel buffers and backpressure deliberately. Do not let one slow
subscriber block unrelated publishers.

For multi-replica deployments, read [Deployment and scaling](/guides/deployment-and-scaling)
with [Realtime subscriptions](/guides/subscriptions).
