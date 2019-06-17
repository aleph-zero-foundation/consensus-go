#!/bin/env python

import argparse
import json
import os
import sys
from os.path import join, isfile, isdir, dirname

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
    pipelines = join(dirname(__file__), 'default.py')
elif isfile(args.pipe):
    pipelines = args.pipe
elif isfile(args.pipe+'.py'):
    pipelines = args.pipe+'.py'
elif isfile(join(dirname(__file__), args.pipe)):
    pipelines = isfile(join(dirname(__file__), args.pipe))
elif isfile(join(dirname(__file__), args.pipe+'.py')):
    pipelines = isfile(join(dirname(__file__), args.pipe+'.py'))
else:
    print(f'{args.pipe}: invalid file')
    sys.exit(1)

driver = Driver()
exec(compile(open(pipelines).read(), 'pipelines.py', 'exec'))

if not (isdir(args.path) or (isfile(args.path) and (args.path.endswith('.log') or args.path.endswith('.zip')))):
    print(f'{args.path}: invalid path')
    sys.exit(1)

if isfile(args.path) and args.path.endswith('.log'):
    name = args.path[:-4]
    driver.new_dataset(name)
    with open(args.path) as f:
        for line in f:
            driver.handle(json.loads(line))
    driver.finalize()
    print(driver.report(name))
else:
    path = args.path if isdir(args.path) else extract(args.path)
    for filename in filter(lambda x: x.endswith('.log'), os.listdir(path)):
        name = filename[:-4]
        driver.new_dataset(name)
        with open(join(path, filename)) as f:
            for line in f:
                driver.handle(json.loads(line))
        driver.finalize()
        print(driver.report(name))
