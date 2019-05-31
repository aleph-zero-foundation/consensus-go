#!/bin/env python

import argparse
import json
import os
import sys

from driver import Driver

driver = Driver()

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
