SKIP = 5

driver.add_pipeline('Create service', [
    Filter(Service, CreateService),
    CreateCounter(),
    Filter(Event, [UnitCreated, PrimeUnitCreated]),
    Histogram('parents', [UnitCreated, PrimeUnitCreated], lambda entry: entry[NParents]),
    Timer('unit creation intervals', SKIP)
])

driver.add_pipeline('Timing units', [
    Filter(Event, [NewTimingUnit, LinearOrderExtended]),
    Histogram('timing unit decision level', NewTimingUnit, lambda entry: (entry[Height] - entry[Round])),
    Histogram('dag levels above deciding prime unit', NewTimingUnit, lambda entry: (entry[Size] - entry[Height])),
    Counter('units ordered per level', LinearOrderExtended, lambda entry: entry[Size]),
    Filter(Event, NewTimingUnit),
    Timer('timing unit decision intervals', SKIP),
])

driver.add_pipeline('Latency', [
    Filter(Event, [UnitCreated, PrimeUnitCreated, OwnUnitOrdered]),
    Delay('Latency', [UnitCreated, PrimeUnitCreated], OwnUnitOrdered, lambda entry: entry[Height], SKIP),
])

driver.add_pipeline('Gossip stats', [
    Filter(Service, GossipService),
    GossipStats(),
])

driver.add_pipeline('Multicast stats', [
    Filter(Service, MCService),
    MulticastStats(),
    Histogram('number of missing parents', UnknownParents, lambda entry: entry[Size]),
])

driver.add_pipeline('Network traffic', NetworkTraffic())
driver.add_pipeline('Memory', MemoryStats(unit = 'kB'))