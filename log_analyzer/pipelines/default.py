SKIP = 2

driver.add_pipeline('Create', [
    Filter(Service, CreateService),
    CreateCounter(),
    Filter(Message, [UnitCreated]),
    Timer('unit creation intervals', SKIP)
])

units_per_level = Counter('units ordered per level', LinearOrderExtended, lambda entry: entry[Size])
timing_freq = Timer('timing unit decision intervals', SKIP)
driver.add_pipeline('Ordering', [
    Filter(Message, [NewTimingUnit, LinearOrderExtended]),
    Histogram('timing unit decision level', NewTimingUnit, lambda entry: (entry[Height] - entry[Round])),
    Histogram('dag levels above deciding prime unit', NewTimingUnit, lambda entry: (entry[Size] - entry[Height])),
    units_per_level,
    Filter(Message, NewTimingUnit),
    timing_freq,
    TXPS(units_per_level, timing_freq, config)
])

driver.add_pipeline('Latency', [
    Filter(Message, [UnitCreated, OwnUnitOrdered]),
    Delay('Ordering latency', [UnitCreated], OwnUnitOrdered, lambda entry: entry[Height], SKIP),
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
    Filter(Service, MCService),
    MulticastStats(),
    Histogram('number of missing parents', UnknownParents, lambda entry: entry[Size]),
])

driver.add_pipeline('Network traffic', NetworkTraffic())
driver.add_pipeline('Memory', MemoryStats(unit = 'MB'))
