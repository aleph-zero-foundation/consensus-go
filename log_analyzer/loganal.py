#!/bin/env python

import argparse
import json
import os
import sys

from driver import Driver
from plugins import *

driver = Driver()

driver.add_pipeline('Create service', [Filter(Service, CreateService), CreateCounter(), Filter(Event, [UnitCreated, PrimeUnitCreated]), Timer('unit creation intervals')])
driver.add_pipeline('Timing units', [Filter(Event, NewTimingUnit), TimingUnitCounter(), Timer('timing unit decision intervals')])
driver.add_pipeline('Sync stats', [Filter(Service, SyncService), SyncStats()])
driver.add_pipeline('Latency', [Filter(Event, [UnitCreated, PrimeUnitCreated, OwnUnitOrdered]), LatencyMeter()])



parser = argparse.ArgumentParser()
parser.add_argument('filename')
args = parser.parse_args()

if not os.path.isfile(args.filename):
    print(f'{args.filename}: invalid file')
    sys.exit(1)

with open(args.filename) as f:
    for line in f:
        driver.handle(json.loads(line))

driver.finalize()
print(driver.report())
