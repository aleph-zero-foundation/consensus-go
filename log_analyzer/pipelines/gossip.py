SKIP = 0

driver.add_pipeline('Incoming gossip', [
    Filter(Service, GossipService),
    Filter(ISID),
    Delay('GetDagInfo', GetDagInfo, SendDagInfo, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('SendDagInfo', SendDagInfo, SendUnits, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('SendUnits', SendUnits, SentUnits, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('SendRequests', SendRequests, GetPreunits, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('GetPreunits', GetPreunits, ReceivedPreunits, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('GetRequests', GetRequests, AddUnits, lambda entry: (entry[PID],entry[ISID]), SKIP),
    Delay('AddUnits', AddUnits, SyncCompleted, lambda entry: (entry[PID],entry[ISID]), SKIP),
    GossipStats(),
])

driver.add_pipeline('Outgoing gossip', [
    Filter(Service, GossipService),
    Filter(OSID),
    Delay('SendDagInfo', SendDagInfo, GetDagInfo, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('GetDagInfo', GetDagInfo, GetPreunits, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('GetPreunits', GetPreunits, ReceivedPreunits, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('GetRequests', GetRequests, SendUnits, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('SendUnits', SendUnits, SentUnits, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('SendRequests', SendRequests, AddUnits, lambda entry: (entry[PID],entry[OSID]), SKIP),
    Delay('AddUnits', AddUnits, SyncCompleted, lambda entry: (entry[PID],entry[OSID]), SKIP),
    GossipStats(),
])
