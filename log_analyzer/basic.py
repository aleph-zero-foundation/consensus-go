SKIP = 5

driver.add_pipeline('Create service', [
    Filter(Event, [UnitCreated, PrimeUnitCreated]),
    Histogram('parents', [UnitCreated, PrimeUnitCreated], lambda entry: entry[NParents], SKIP),
    Timer('unit creation intervals', SKIP)
])

driver.add_pipeline('Timing units', [
    Filter(Event, [NewTimingUnit, LinearOrderExtended]),
    Histogram('timing unit decision level', NewTimingUnit, lambda entry: (entry[Height] - entry[Round]), SKIP),
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
