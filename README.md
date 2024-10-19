### Architecture

![](./docs/architecture.png)


### Improvements
- Separate query and command into own service
- Implement event sourcing

### TODO
- [ ] Refactor `EventStoreEntity` to be a query builder api
- [ ] `EventStoreEntity` must accept a transaction as arg
- [ ] `Repository` must accept a transaction as arg

### Notes
Is ScyllaDB good as event store ? (Do I lose transactions?)
