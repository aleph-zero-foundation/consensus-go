SKIP = 0

driver.add_pipeline('Create service', [
    Filter(Event, [UnitCreated]),
    Histogram('parents', [UnitCreated], lambda entry: entry[NParents]),
    Timer('unit creation intervals', SKIP)
])

driver.add_pipeline('Timing units', [
    Filter(Event, [NewTimingUnit, LinearOrderExtended]),
    Histogram('timing unit decision level', NewTimingUnit, lambda entry: (entry[Height] - entry[Round])),
    Filter(Event, NewTimingUnit),
    Timer('timing unit decision intervals', SKIP),
])

driver.add_pipeline('Latency', [
    Filter(Event, [UnitCreated, OwnUnitOrdered]),
    Delay('Latency', [UnitCreated], OwnUnitOrdered, lambda entry: entry[Height], SKIP),
])

driver.add_pipeline('Gossip stats', [
    Filter(Service, GossipService),
    GossipStats(),
])

driver.add_pipeline('Fetch', [
    Filter(Service, FetchService),
    FetchStats(),
])

driver.add_pipeline('Multicast stats', [
    Filter(Service, MCService),
    MulticastStats(),
])
