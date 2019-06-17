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


driver.add_pipeline('Sync stats', [
    Filter(Service, SyncService),
    SyncStats(),
    NetworkTraffic(SKIP)
])


driver.add_pipeline('Incoming syncs', [
    Filter(ISID),
    Delay('ConnectionQueue', ConnectionReceived, SyncStarted, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('GetPosetInfo', GetPosetInfo, SendPosetInfo, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('SendPosetInfo', SendPosetInfo, SendUnits, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('SendUnits', SendUnits, SentUnits, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('SendRequests', SendRequests, GetPreunits, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('GetPreunits', GetPreunits, ReceivedPreunits, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('GetRequests', GetRequests, AddUnits, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('AddUnits', AddUnits, SyncCompleted, lambda entry: (entry[PID],entry[ISID]), SKIP),
])


driver.add_pipeline('Outgoing syncs', [
    Filter(OSID),
    Delay('ConnectionQueue', ConnectionEstablished, SyncStarted, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('SendPosetInfo', SendPosetInfo, GetPosetInfo, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('GetPosetInfo', GetPosetInfo, GetPreunits, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('GetPreunits', GetPreunits, ReceivedPreunits, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('GetRequests', GetRequests, SendUnits, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('SendUnits', SendUnits, SentUnits, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('SendRequests', SendRequests, AddUnits, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('AddUnits', AddUnits, SyncCompleted, lambda entry: (entry[PID],entry[OSID]), SKIP),
])


driver.add_pipeline('Memory', MemoryStats(unit = 'kB'))