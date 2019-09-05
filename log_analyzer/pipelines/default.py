SKIP = 0

driver.add_pipeline('Create', [
    Filter(Service, CreateService),
    CreateCounter(),
    Filter(Event, [UnitCreated, PrimeUnitCreated]),
    Histogram('parents', [UnitCreated, PrimeUnitCreated], lambda entry: entry[NParents]),
    Timer('unit creation intervals', SKIP)
])

driver.add_pipeline('Ordering', [
    Filter(Event, [NewTimingUnit, LinearOrderExtended]),
    Histogram('timing unit decision level', NewTimingUnit, lambda entry: (entry[Height] - entry[Round])),
    Histogram('dag levels above deciding prime unit', NewTimingUnit, lambda entry: (entry[Size] - entry[Height])),
    Counter('units ordered per level', LinearOrderExtended, lambda entry: entry[Size]),
    Filter(Event, NewTimingUnit),
    Timer('timing unit decision intervals', SKIP),
])

driver.add_pipeline('Latency', [
    Filter(Event, [UnitCreated, PrimeUnitCreated, OwnUnitOrdered, UnitBroadcasted]),
    Delay('Ordering latency', [UnitCreated, PrimeUnitCreated], OwnUnitOrdered, lambda entry: entry[Height], SKIP),
    Delay('Broadcasting latency', [UnitCreated, PrimeUnitCreated], UnitBroadcasted, lambda entry: entry[Height], SKIP),
])

driver.add_pipeline('Gossip', [
    Filter(Service, GossipService),
    GossipStats(),
])

driver.add_pipeline('Fetch', [
    Filter(Service, FetchService),
    FetchStats(),
])

driver.add_pipeline('Multicast', [
    Filter(Service, [MCService, RetryingService]),
    MulticastStats(),
    Histogram('number of missing parents', UnknownParents, lambda entry: entry[Size]),
    Delay('Backlog stay', AddedToBacklog, RemovedFromBacklog, lambda entry: entry[Hash], SKIP),
])

driver.add_pipeline('Network traffic', NetworkTraffic())
driver.add_pipeline('Memory', MemoryStats(unit = 'kB'))