# Resources

Consul 1.16 introduced a set of [generic APIs] for managing resources, and a
[controller runtime] for building functionality on top of them.

[generic APIs]: ../../proto-public/pbresource/resource.proto
[controller runtime]: ../../internal/controller

Previously, adding features to Consul involved making changes at every layer of
the stack, including: HTTP handlers, RPC handlers, MemDB tables, Raft
operations, and CLI commands.

This architecture made sense when the product was maintained by a small core
group who could keep the entire system in their heads, but presented significant
collaboration, ownership, and onboarding challenges when our contributor base
expanded to many engineers, across several teams, and the product grew in
complexity.

In the new model, teams can work with much greater autonomy by building on top
of a shared platform and owning their resource types and controllers.

## Architecture

![architecture diagram](./architecture-overview.png)

Our resource-oriented architecture comprises the following components:

* **Resource Service** is a gRPC service that contains the shared logic for
  creating, reading, updating, deleting, and watching resources. It will be
  consumed by controllers, our Kubernetes integration, the CLI, and mapped to
  an HTTP+JSON API.
* **Type Registry** is where teams register their resource types, along with
  hooks for performing structural validation, authorization, etc.
* **Storage Backend** is an abstraction over low-level storage primitives.
  Today, there are two implementations (Raft and an in-memory backend for
  tests) but in the future, we envisage external storage systems such as the
  Kubernetes API or an RDBMS could be used.
* **Controllers** implement Consul's business logic using asynchronous control
  loops that "wake up" when a relevant resource changes.

### Raft Storage Backend

The following diagram illustrates the flow of a write operation when using the
[Raft Storage Backend](../../internal/storage/raft/backend.go).

![raft storage backend diagram](./raft-backend.png)

1. User calls the Resource Service's `Write` endpoint (on a Raft follower)
2. Resource Service calls the Storage Backend's `WriteCAS` method
3. Storage Backend determines that the current server is a follower and forwards
   the operation to the leader over the multiplexed RPC port (`ports.server`)
4. Leader's Storage Backend receives the forwarded operation via a gRPC-based
   [forwarding service](../../proto/private/pbstorage/raft.proto) and attempts
   to apply it
5. Storage Backend serializes the operation to protobuf and applies it to the
   Raft log through the [`raftHandle`](../../agent/consul/raft_handle.go) shim
   which wraps it in our type byte-prefix envelope
6. Raft consensus happens! Which results in the [FSM](../../agent/consul/fsm)
   being called to apply the committed log. The FSM spots the resource type byte
   and calls the Storage Backend's `Apply` method with the protobuf-encoded
   operation, which applies the changes to the `inmem.Store`
7. Leader forwarding service responds successfully
8. Follower's Storage Backend responds successfuly
9. User gets success response from the Resource Service
10. Asynchronously, the log is replicated to followers
11. Follower's FSM applies the committed log and updates its `inmem.Store`
