#!/bin/env python

import argparse
import json
import os
import shutil
import sys

from os.path import join, isfile, isdir, dirname, basename, splitext
from tqdm import tqdm
from zipfile import ZipFile

from driver import Driver
from const import *
from plugins import *

def lasttime(path, seek=128):
    with open(path, 'rb') as f:
        f.seek(-seek, os.SEEK_END)
        return json.loads(f.readlines()[-1])[Time]

def extract(path):
    with ZipFile(path, 'r') as f:
        ret = join(dirname(path), dirname(f.namelist()[0]))
        f.extractall()
    return ret


parser = argparse.ArgumentParser(description='Log analyzer for JSON logs of Gomel. Can be used in one of two modes: single file mode (extensive report based on the single log) or folder mode (general stats gathered from all the .log files in the given folder (also ZIP compressed). The file with pipelines (-p flag) can be a custom .py file or one of the predefined pipelines from the log analyzer source directory. Possible pipelines: default, basic, sync, plots.')
parser.add_argument('path', metavar='path', help='single JSON log, whole folder or ZIP archived folder')
parser.add_argument('-p', '--pipe', metavar='name', help='file with pipelines definitions')
parser.add_argument('-a', '--all', action='store_true', help='print full report for each file in "folder mode"')
args = parser.parse_args()


if not args.pipe:
    pipelines = join(dirname(__file__), 'default.py')
elif isfile(args.pipe):
    pipelines = args.pipe
elif isfile(args.pipe+'.py'):
    pipelines = args.pipe+'.py'
elif isfile(join(dirname(__file__), args.pipe)):
    pipelines = join(dirname(__file__), args.pipe)
elif isfile(join(dirname(__file__), args.pipe+'.py')):
    pipelines = join(dirname(__file__), args.pipe+'.py')
else:
    print(f'{args.pipe}: invalid file')
    sys.exit(1)

driver = Driver()
exec(compile(open(pipelines).read(), pipelines, 'exec'))

if not (isdir(args.path) or (isfile(args.path) and (args.path.endswith('.log') or args.path.endswith('.zip')))):
    print(f'{args.path}: invalid path')
    sys.exit(1)

if isfile(args.path) and args.path.endswith('.log'):
    name = basename(args.path)[:-4]
    driver.new_dataset(name)
    with open(args.path) as f:
        for line in f:
            driver.handle(json.loads(line))
    driver.finalize()
    print(driver.report(name))
else:
    path = args.path if isdir(args.path) else extract(args.path)
    os.chdir(path)
    for filename in tqdm(list(filter(lambda x: x.endswith('.log'), os.listdir('.')))):
        name = filename[:-4]
        driver.new_dataset(name)
        with open(filename) as f:
            for line in f:
                driver.handle(json.loads(line))
        driver.finalize()
        if args.all:
            print(driver.report(name))
    print(driver.summary())
