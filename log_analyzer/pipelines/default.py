SKIP = 2

driver.add_pipeline('Create', [
    Filter(Service, CreateService),
    CreateCounter(),
    Filter(Event, [UnitCreated, PrimeUnitCreated]),
    Timer('unit creation intervals', SKIP)
])

units_per_level = Counter('units ordered per level', LinearOrderExtended, lambda entry: entry[Size])
timing_freq = Timer('timing unit decision intervals', SKIP)
driver.add_pipeline('Ordering', [
    Filter(Event, [NewTimingUnit, LinearOrderExtended]),
    Histogram('timing unit decision level', NewTimingUnit, lambda entry: (entry[Height] - entry[Round])),
    Histogram('dag levels above deciding prime unit', NewTimingUnit, lambda entry: (entry[Size] - entry[Height])),
    units_per_level,
    Filter(Event, NewTimingUnit),
    timing_freq,
    TXPS(units_per_level, timing_freq, config)
])

driver.add_pipeline('Latency', [
    Filter(Event, [UnitCreated, PrimeUnitCreated, OwnUnitOrdered, UnitBroadcasted]),
    Delay('Ordering latency', [UnitCreated, PrimeUnitCreated], OwnUnitOrdered, lambda entry: entry[Height], SKIP),
    Delay('Broadcasting latency', [UnitCreated, PrimeUnitCreated], UnitBroadcasted, lambda entry: entry[Height], SKIP),
])

driver.add_pipeline('Adder', [
    Filter(Service, AdderService),
    Delay('Limbo (Incomplete = AddUnits calls)', AddUnitStarted, PreunitReady, lambda entry: (entry[Creator], entry[Height], entry[PID]), SKIP),
    Delay('Channels', PreunitReady, AddingStarted, lambda entry: (entry[Creator], entry[Height], entry[PID]), SKIP),
    Delay('Worker', AddingStarted, UnitAdded, lambda entry: (entry[Creator], entry[Height], entry[PID]), SKIP),
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
