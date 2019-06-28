SKIP = 5

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
    SyncStats(),
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
    SyncStats(),
])

import numpy as np
regions = np.array(range(128)).reshape(8,-1)
region_names = ['Virginia','California','Oregon','Ireland','Sao Paulo','Singapore','Sydney','Tokyo']

driver.add_pipeline('', [
    Filter(Event, [SyncStarted, SyncCompleted]),
    SyncPlots(regions, region_names),
])
