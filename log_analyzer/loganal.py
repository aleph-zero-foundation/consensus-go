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

def extract(path):
    pass



parser = argparse.ArgumentParser()
parser.add_argument('path', metavar='path', help='single JSON log, whole folder or ZIP archive')
parser.add_argument('-p', '--pipe', metavar='file', help='file with pipelines definitions')
args = parser.parse_args()


if not args.pipe:
    pipelines = os.path.join(os.path.dirname(__file__), 'default.py')
elif os.path.isfile(args.pipe):
    pipelines = args.pipe
elif os.path.isfile(args.pipe+'.py'):
    pipelines = args.pipe+'.py'
elif os.path.isfile(os.path.join(os.path.dirname(__file__), args.pipe)):
    pipelines = os.path.isfile(os.path.join(os.path.dirname(__file__), args.pipe))
elif os.path.isfile(os.path.join(os.path.dirname(__file__), args.pipe+'.py')):
    pipelines = os.path.isfile(os.path.join(os.path.dirname(__file__), args.pipe+'.py'))
else:
    print(f'{args.pipe}: invalid file')
    sys.exit(1)

driver = Driver()
exec(compile(open(pipelines).read(), 'pipelines.py', 'exec'))

if not (os.path.isdir(args.path) or (os.path.isfile(args.path) and (args.path.endswith('.log') or args.path.endswith('.zip')))):
    print(f'{args.path}: invalid path')
    sys.exit(1)

if os.path.isfile(args.path) and args.path.endswith('.log'):
    name = args.path[:-4]
    driver.new_dataset(name)
    with open(args.path) as f:
        for line in f:
            driver.handle(json.loads(line))
    driver.finalize()
    print(driver.report(name))
else:
    path = args.path if os.path.isdir(args.path) else extract(args.path)
    for filename in filter(lambda x: x.endswith('.log'), os.listdir(path)):
        name = filename[:-4]
        driver.new_dataset(name)
        with open(os.path.join(path, filename)) as f:
            for line in f:
                driver.handle(json.loads(line))
        driver.finalize()
        print(driver.report(name))
