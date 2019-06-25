import numpy as np
regions = np.array(range(128)).reshape(8,-1)
region_names = ['Virginia','California','Oregon','Ireland','Sao Paulo','Singapore','Sydney','Tokyo']

driver.add_pipeline('', [
    Filter(Event, [SyncStarted, SyncCompleted]),
    SyncPlots(regions, region_names),
])