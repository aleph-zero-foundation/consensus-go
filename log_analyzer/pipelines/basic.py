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
    units_per_level,
    Filter(Message, NewTimingUnit),
    timing_freq,
    TXPS(units_per_level, timing_freq, config)
])

driver.add_pipeline('Latency', [
    Filter(Message, [UnitCreated, OwnUnitOrdered]),
    Delay('Ordering latency', [UnitCreated], OwnUnitOrdered, lambda entry: entry[Height], SKIP),
])
