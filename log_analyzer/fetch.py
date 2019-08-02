SKIP = 0

driver.add_pipeline('Incoming fetch', [
    Filter(Service, FetchService),
    Filter(ISID),
    Delay('GetRequests', GetRequests, SendUnits, lambda entry: (entry[PID],entry[ISID]), SKIP, 0),
    Delay('SendUnits', SendUnits, SyncCompleted, lambda entry: (entry[PID],entry[ISID]), SKIP, 0),
])

driver.add_pipeline('Outgoing fetch', [
    Filter(Service, FetchService),
    Filter(OSID),
    Delay('SendRequests', SendRequests, GetPreunits, lambda entry: (entry[PID],entry[OSID]), SKIP, 0),
    Delay('GetPreunits', GetPreunits, ReceivedPreunits, lambda entry: (entry[PID],entry[OSID]), SKIP, 0),
    Delay('AddUnits', ReceivedPreunits, SyncCompleted, lambda entry: (entry[PID],entry[OSID]), SKIP, 0),
])