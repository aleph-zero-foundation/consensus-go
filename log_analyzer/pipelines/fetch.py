SKIP = 0

driver.add_pipeline('Incoming fetch', [
    Filter(Service, FetchService),
    Filter(ISID),
    Delay('GetRequests', GetRequests, SendUnits, lambda entry: (entry[PID],entry[ISID]), SKIP, 1),
    Delay('SendUnits', SendUnits, SyncCompleted, lambda entry: (entry[PID],entry[ISID]), SKIP, 1),
])

driver.add_pipeline('Outgoing fetch', [
    Filter(Service, FetchService),
    Filter(OSID),
    Delay('SendRequests', SendRequests, GetPreunits, lambda entry: (entry[PID],entry[OSID]), SKIP, 1),
    Delay('GetPreunits', GetPreunits, ReceivedPreunits, lambda entry: (entry[PID],entry[OSID]), SKIP, 1),
    Delay('AddUnits', ReceivedPreunits, SyncCompleted, lambda entry: (entry[PID],entry[OSID]), SKIP, 1),
    FetchStats(),
])