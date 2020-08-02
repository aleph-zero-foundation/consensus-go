from const import *

from statistics import mean, median
import numpy as np

sadpanda = 'NO ENTRIES\n'


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
    avgs = []
    meds = []
    for name, data in datasets.items():
        if data:
            full += data
            avgs.append((mean(data), name))
            meds.append((median(data), name))
    avgs.sort()
    meds.sort()
    if not full:
        return sadpanda
    glob = mean(full)
    ret  = '    Average of medians: %13.2f\n' % mean(i[0] for i in meds)
    ret += '    Min Median:         %13.2f (%s)\n' % meds[0]
    ret += '    Max Median:         %13.2f (%s)\n' % meds[-1]
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
        if entry[Message] in self.events:
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
        if entry[Message] in self.events:
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
        if entry[Message] in self.start:
            try:
                key = self.startid(entry)
            except:
                return entry
            if key not in self.tmpdata:
                self.tmpdata[key] = [None, None]
            self.tmpdata[key][0] = entry[Time]
        if entry[Message] in self.end:
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
        if len(times) == 0:
            return sadpanda
        if max(times) <= self.thr:
            return f'NEGLIGIBLE (all entries below {self.thr} ms)'
        ret =  '    Complete:   %7d\n' % len(self.data)
        ret += '    Incomplete: %7d\n\n' % len(self.incomplete)
        ret += '  (skipped first %d entries)\n'%self.skip if self.skip else ''
        ret += '    Min: %10d    ms\n' % min(times)
        ret += '    Max: %10d    ms\n' % max(times)
        ret += '    Avg: %13.2f ms\n' % mean(times)
        ret += '    Med: %13.2f ms\n\n' % median(times)
        data = self.data[self.skip:]
        data.sort(reverse=True)
        return ret
