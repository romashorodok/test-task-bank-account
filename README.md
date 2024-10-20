### Architecture

![](./docs/architecture.png)


### Improvements
- Separate query and command into own service
- Implement event sourcing

### TODO
- [x] Refactor `EventStoreEntity` to be a query builder api
- [x] `EventStoreEntity` must accept a transaction as arg
- [x] `Repository` must accept a transaction as arg

- [ ] Add ulid in snapshot version instead of int
- [ ] Keep 5 last version of snapshots
- [ ] How to rebuild snapshot state from all events. From earliest and which must be a latest which must be the same then integrity must observed
 
### Notes
Is ScyllaDB good as event store ? (Do I lose transactions?)
