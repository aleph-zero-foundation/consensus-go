from const import *

from statistics import mean, median
import numpy as np

from matplotlib.patches import Patch
import matplotlib.pyplot as plt

sadpanda = 'NO ENTRIES'


class Plugin:
    """Parent class definition for all plugins."""
    name = ''
    def process(self, entry):
        return entry
    def finalize(self):
        pass
    def get_data(self):
        return []
    def report(self):
        return ''
    @staticmethod
    def multistats(datasets):
        return ''


def multimean(datasets):
    full = []
    stats = []
    for name, data in datasets.items():
        if data:
            full += data
            stats.append((mean(data), name))
    stats.sort()
    if not full:
        return sadpanda
    glob = mean(full)
    ret =  '    Global Average: %13.2f\n' % glob
    ret += '    Min Average: %13.2f (%s)\n' % stats[0]
    ret += '    Max Average: %13.2f (%s)\n' % stats[-1]
    return ret


class Filter(Plugin):
    """
    Plugin filtering out entries. Only entries that have field *key* equal to
    one of *values* pass through. *values* can be a single item (int or str), list of
    items or None (in that case every value is accepted, as long as *key* is present.
    """
    def __init__(self, key, values=None):
        self.key = key
        self.values = values if (values is None or isinstance(values, list)) else [values]

    def process(self, entry):
        if self.key in entry and (self.values is None or entry[self.key] in self.values):
            return entry
        return None


class After(Plugin):
    """Plugin filtering out entries based on time."""
    def __init__(self, time):
        self.time = time
        self.name = 'Entries after '+str(self.time)

    def process(self, entry):
        return entry if entry[Time] > self.time else None


class Before(Plugin):
    """Plugin filtering out entries based on time."""
    def __init__(self, time):
        self.time = time
        self.name = 'Entries before '+str(self.time)

    def process(self, entry):
        return entry if entry[Time] < self.time else None


class Timer(Plugin):
    """Plugin gathering basic timing statistics of all incoming events."""
    multistats = multimean
    def __init__(self, name, skip_first=0):
        self.name = 'Timer: '+name
        self.skip = skip_first
        self.times = []

    def process(self, entry):
        self.times.append(entry[Time])
        return entry

    def finalize(self):
        self.times.sort()
        for i in range(len(self.times)-1, 1, -1):
            self.times[i] -= self.times[i-1]

    def get_data(self):
        return self.times[self.skip:]

    def report(self):
        t = self.get_data()
        if not t:
            return sadpanda
        ret =  '  (skipped first %d entries)\n'%self.skip if self.skip else ''
        ret += '    Min: %10d    ms\n' % min(t)
        ret += '    Max: %10d    ms\n' % max(t)
        ret += '    Avg: %13.2f ms\n' % mean(t)
        ret += '    Med: %13.2f ms\n' % median(t)
        return ret


class Counter(Plugin):
    """
    Plugin gathering basis statistic (min, max, med, avg) of some numeric value.
    'value' should be a function taking log entry (dict) and returning a value.
    'finalize()' method can be overriden. After 'finalize()' self.data should be a list of numbers.
    """
    multistats = multimean
    def __init__(self, name, events, value, skip_first=0):
        self.name = 'Counter: '+name
        self.skip = skip_first
        self.data = []
        self.val = value
        self.events = events if isinstance(events, list) else [events]

    def process(self, entry):
        if entry[Event] in self.events:
            self.data.append(self.val(entry))
        return entry

    def get_data(self):
        return self.data[self.skip:]

    def report(self):
        d = self.get_data()
        if not d:
            return sadpanda
        ret =  '  (skipped first %d entries)\n'%self.skip if self.skip else ''
        ret += '    Min: %10d\n' % min(d)
        ret += '    Max: %10d\n' % max(d)
        ret += '    Avg: %13.2f\n' % mean(d)
        ret += '    Med: %13.2f\n' % median(d)
        return ret


class Histogram(Plugin):
    """
    Plugin gathering a distribution of some numeric value.
    'value' should be a function taking log entry (dict) and returning a value.
    'finalize()' method can be overriden. After it self.data should be a list of ints.
    """
    multistats = multimean
    def __init__(self, name, events, value, skip_first=0):
        self.name = 'Histogram: '+name
        self.skip = skip_first
        self.data = []
        self.val = value
        self.events = events if isinstance(events, list) else [events]

    def process(self, entry):
        if entry[Event] in self.events:
            self.data.append(self.val(entry))
        return entry

    def get_data(self):
        return self.data[self.skip:]

    def report(self):
        d = self.get_data()
        if not d:
            return sadpanda
        h = {}
        for i in d:
            if i not in h:
                h[i] = 0
            h[i] += 1
        num, den = 0,0
        for k,v in h.items():
            num += k*v
            den += v

        ret =  '  (skipped first %d entries)\n'%self.skip if self.skip else ''
        ret += '    Value        Number of entries\n'
        for k in sorted(h.keys()):
            ret += '     %3d             %5d\n' % (k, h[k])
        ret += '    Average:  %5.2f\n' % (num/den)
        return ret


class Delay(Plugin):
    """
    Plugin gathering basis statistic about time between two types of events.
    'func' should be a function taking log entry (dict) and returning a unique, hashable key.
    A particular key has to be returned exactly once for start-type entry and once for end-type entry.
    Alternatively, 'func' can be a pair of functions, if different logic is needed for start and end entries.
    """
    multistats = multimean
    def __init__(self, name, start, end, func, skip_first=0, threshold=5):
        self.name = 'Delay: '+name
        self.skip = skip_first
        self.thr = threshold
        self.tmpdata = {}
        self.start = start if isinstance(start, list) else [start]
        self.end = end if isinstance(end, list) else [end]
        self.startid, self.endid = func if isinstance(func, tuple) else (func, func)

    def process(self, entry):
        if entry[Event] in self.start:
            try:
                key = self.startid(entry)
            except:
                return entry
            if key not in self.tmpdata:
                self.tmpdata[key] = [None, None]
            self.tmpdata[key][0] = entry[Time]
        if entry[Event] in self.end:
            try:
                key = self.endid(entry)
            except:
                return entry
            if key not in self.tmpdata:
                self.tmpdata[key] = [None, None]
            self.tmpdata[key][1] = entry[Time]
        return entry

    def finalize(self):
        self.tmpdata = [(v[0], v[1], k) for k,v in self.tmpdata.items()]
        self.data = []
        self.incomplete = []
        for s,e,k in self.tmpdata:
            if s == None or e == None:
                self.incomplete.append((s,e,k))
            else:
                self.data.append((s, e-s, k))
        self.data.sort()
        self.data = [(d, k) for _,d,k in self.data]

    def get_data(self):
        return [i[0] for i in self.data[self.skip:]]

    def report(self):
        if len(self.data) == 0:
            return sadpanda
        times = self.get_data()
        if max(times) <= self.thr:
            return 'NEGLIGIBLE'
        ret =  '    Complete:   %7d\n' % len(self.data)
        ret += '    Incomplete: %7d\n\n' % len(self.incomplete)
        ret += '  (skipped first %d entries)\n'%self.skip if self.skip else ''
        ret += '    Min: %10d    ms\n' % min(times)
        ret += '    Max: %10d    ms\n' % max(times)
        ret += '    Avg: %13.2f ms\n' % mean(times)
        ret += '    Med: %13.2f ms\n\n' % median(times)
        data = self.data[self.skip:]
        data.sort(reverse=True)
        ret += '  5 largest:\n'
        for t,k in data[:5]:
            ret += '%15s: %10d ms\n' % (k,t)
        ret += '  Distribution:\n'
        size, brackets = np.histogram(times, bins=10)
        for i in zip(brackets[:-1], brackets[1:],size):
            ret += '   %10.1f-%-8.1f: %10d\n' % i
        return ret


class CreateCounter(Plugin):
    """
    Plugin counting basis statistics of create service:
    * number of prime units
    * number of non-prime units
    * number of times create unit failed due to not enough parents
    * longest streak of non-prime units
    """
    name = 'Create counter'
    def __init__(self):
        self.nonprimes = 0
        self.primes = 0
        self.noparents = 0
        self.streaks = []
        self.streak = 0

    def process(self, entry):
        if entry[Event] == UnitCreated:
            self.nonprimes += 1
            self.streak += 1
        elif entry[Event] == PrimeUnitCreated:
            self.primes += 1
            self.streaks.append(self.streak)
            self.streak = 0
        elif entry[Event] == NotEnoughParents:
            self.noparents += 1
        return entry

    def report(self):
        ret =  '    Prime units:              %5d\n'%self.primes
        ret += '    Regular units:            %5d\n'%self.nonprimes
        ret += '    All units:                %5d\n'%(self.primes+self.nonprimes)
        ret += '    Failed (no parents):      %5d\n'%self.noparents
        ret += '    Total calls:              %5d\n'%(self.noparents+self.nonprimes+self.primes)
        ret += '    Longest nonprime streak:  %5d\n'%max(self.streaks)
        ret += '    Avg nonprime streak:      %8.2f\n'%mean(self.streaks)
        return ret


class NetworkTraffic(Plugin):
    """Plugin measuring the size of data sent/received through the network."""
    name = 'Network traffic [kB/s]'
    def __init__(self, skip_first=0):
        self.data = {}
        self.skip = skip_first

    def process(self, entry):
        if entry[Event] == ConnectionClosed:
            bracket = entry[Time] // 1000
            if bracket not in self.data:
                self.data[bracket] = [0, 0]
            self.data[bracket][0] += entry[Sent]
            self.data[bracket][1] += entry[Recv]
        return entry

    def finalize(self):
        self.sent = [self.data[k][0]>>10 for k in sorted(self.data.keys())]
        self.recv = [self.data[k][1]>>10 for k in sorted(self.data.keys())]

    def report(self):
        s = self.sent[self.skip:]
        r = self.recv[self.skip:]
        ret =  '  (skipped first %d entries)\n'%self.skip if self.skip else ''
        if not s:
            return ret + sadpanda+ '\n'
        ret += '                Sent           Received\n'
        ret += '    Min: %10d       %10d\n' % (min(s), min(r))
        ret += '    Max: %10d       %10d\n' % (max(s), max(r))
        ret += '    Avg: %13.2f    %13.2f\n' % (mean(s), mean(r))
        ret += '    Med: %13.2f    %13.2f\n' % (median(s), median(r))
        return ret


class MemoryStats(Plugin):
    """Plugin gathering memory usage statistics."""
    shifts = {'B':0, 'kB':10, 'MB':20, 'GB':30}
    name = 'Memory usage'
    def __init__(self, unit='MB'):
        self.data = []
        if unit not in self.shifts:
            print('MemoryStats: incorrect unit')
            self.unit, self.sh = '??', 0
        else:
            self.unit, self.sh = unit, self.shifts[unit]

    def process(self, entry):
        if entry[Event] == MemoryUsage:
            self.data.append((entry[Time]/1000, entry[Size]>>self.sh , entry[Memory]>>self.sh))
        return entry

    def report(self):
        ret = '     Time[s]     Heap[%s]     Total[%s]\n' % (self.unit, self.unit)
        for i in self.data:
            ret += '%10d    %10d    %10d\n' % i
        return ret

