#!/bin/env python

import argparse
import json
import os
import sys

from driver import Driver
from const import *
from plugins import *

driver = Driver()

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
    Histogram('timing unit choice delay', NewTimingUnit, lambda entry: entry[Height]-entry[Round], SKIP),
    Counter('units ordered per level', LinearOrderExtended, lambda entry: entry[Size], SKIP),
    Filter(Event, NewTimingUnit),
    Timer('timing unit decision intervals', SKIP),
])

driver.add_pipeline('Latency', [
    Filter(Event, [UnitCreated, PrimeUnitCreated, OwnUnitOrdered]),
    LatencyMeter(SKIP)
])

driver.add_pipeline('Sync stats', [
    Filter(Service, SyncService),
    SyncStats()
])

driver.add_pipeline('Memory', MemoryStats(unit = 'kB'))


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
