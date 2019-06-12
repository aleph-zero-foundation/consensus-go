#!/bin/env python

import argparse
import json
import os
import sys

from driver import Driver
from const import *
from plugins import *

def lasttime(path, seek=128):
    with open(path, 'rb') as f:
        f.seek(-seek, os.SEEK_END)
        return json.loads(f.readlines()[-1])[Time]

parser = argparse.ArgumentParser()
parser.add_argument('filename', metavar='logfile', help='file with JSON log')
parser.add_argument('-p', '--pipe', metavar='file', help='file with pipelines definitions')
args = parser.parse_args()

if not os.path.isfile(args.filename):
    print(f'{args.filename}: invalid file')
    sys.exit(1)

pipelines = args.pipe if args.pipe else os.path.join(os.path.dirname(__file__), 'pipelines.py')

if not os.path.isfile(pipelines):
    print(f'{pipelines}: invalid file')
    sys.exit(1)

FULLTIME = lasttime(args.filename)

driver = Driver()
exec(compile(open(pipelines).read(), 'pipelines.py', 'exec'))

with open(args.filename) as f:
    for line in f:
        driver.handle(json.loads(line))

driver.finalize()
print(driver.report())
