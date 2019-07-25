SKIP = 5

driver.add_pipeline('Create service', [
    Filter(Service, CreateService),
    CreateCounter(),
    Filter(Event, [UnitCreated, PrimeUnitCreated]),
    Histogram('parents', [UnitCreated, PrimeUnitCreated], lambda entry: entry[NParents], SKIP),
    Timer('unit creation intervals', SKIP)
])


driver.add_pipeline('Timing units', [
    Filter(Event, [NewTimingUnit, LinearOrderExtended]),
    Histogram('timing unit choice delay', NewTimingUnit, lambda entry: (-1 if entry[Height] < 0 else entry[Height]-entry[Round]), SKIP),
    Counter('units ordered per level', LinearOrderExtended, lambda entry: entry[Size], SKIP),
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
    NetworkTraffic(SKIP)
])

driver.add_pipeline('Memory', MemoryStats(unit = 'kB'))