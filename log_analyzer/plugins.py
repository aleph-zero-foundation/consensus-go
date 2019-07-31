from const import *

from statistics import mean, median
import numpy as np

from matplotlib.patches import Patch
import matplotlib.pyplot as plt


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

class Plotter(Plugin):
    """Subclass for plugins that produce plots."""
    def saveplot(self, name):
        return ''


def multimean(datasets):
    full = []
    stats = []
    for name, data in datasets.items():
        if data:
            full += data
            stats.append((mean(data), name))
    stats.sort()
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
            return 'NO ENTRIES'
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
            return 'NO ENTRIES'
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
            return 'NO ENTRIES'
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
            return 'NO SUCH EVENTS'
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


class MulticastStats(Plugin):
    """Plugin gathering statistics of units received via multicast."""
    name = 'Incoming multicast events'
    def __init__(self):
        self.succ = 0
        self.miss = 0
        self.dupl = 0
        self.err = 0

    def process(self, entry):
        if entry[Event] == AddedBCUnit:
            self.succ += 1
        elif entry[Event] == UnknownParents:
            self.miss += 1
        elif entry[Event] == DuplicatedUnit:
            self.dupl += 1
        elif entry[Level] == 3 and 'multicast.In' in entry['where']:
            self.err += 1
        return entry

    def get_data(self):
        return self.succ, self.miss, self.dupl

    @staticmethod
    def multistats(datasets):
        ret = '    PID     Success    Failed    Duplicated\n'
        for name in sorted(datasets.keys()):
            s,m,d = datasets[name]
            ret += f'     {name} {s:10} {m:10} {d:10}\n'
        return ret

    def report(self):
        ret  =  '    Units added:               %5d\n'%self.succ
        ret +=  '    Units with missing parents:%5d\n'%self.miss
        ret +=  '    Duplicates received:       %5d\n'%self.dupl
        ret +=  '    Interrupted by error:      %5d\n'%self.err
        ret +=  '    Total:                     %5d\n'%(self.succ+self.miss+self.dupl+self.err)
        return ret


class GossipStats(Plugin):
    """Plugin gathering detailed statistics of syncs during gossip."""
    name = 'Gossip stats'
    def __init__(self, ignore_empty=True):
        self.ig = ignore_empty
        self.inc = {}
        self.out = {}

    def process(self, entry):
        if PID not in entry:
            return entry
        if OSID in entry:
            d = self.out
            key = (entry[PID], entry[OSID])
        elif ISID in entry:
            d = self.inc
            key = (entry[PID], entry[ISID])
        else:
            return entry

        if key not in d:
            d[key] = {'addexc':False, 'fail':False, 'dupl':0}

        if entry[Event] == SyncStarted:
            d[key]['start'] = entry[Time]
        elif entry[Event] == SyncCompleted:
            sent = entry[Sent] + entry.get(FreshSent,0)
            recv = entry[Recv] + entry.get(FreshRecv,0)
            if self.ig and sent == 0 and recv == 0: #empty sync, remove
                del d[key]
                return entry
            d[key]['end'] = entry[Time]
            d[key]['sent'] = sent
            d[key]['recv'] = recv
            d[key]['fsent'] = entry.get(FreshSent,0)
            d[key]['frecv'] = entry.get(FreshRecv,0)
        elif entry[Event] == AdditionalExchange:
            d[key]['addexc'] = True
        elif entry[Event] == DuplicatedUnit:
            d[key]['dupl'] += 1
        elif entry[Level] == '3':
            d[key]['fail'] = True
        return entry

    def finalize(self):
        values = list(self.inc.values()) + list(self.out.values())
        self.times = []
        self.sent = []
        self.recv = []
        self.fsent = []
        self.frecv = []
        self.dupl = []
        self.totaldupl = 0
        self.addexc, self.failed, self.unfinished = 0,0,0
        for d in values:
            if d['fail']:
                self.failed += 1
            elif 'end' not in d:
                self.unfinished += 1
                continue
            elif 'start' in d:
                self.times.append(d['end']-d['start'])
                self.sent.append(d['sent'])
                self.recv.append(d['recv'])
                self.fsent.append(d['fsent'])
                self.frecv.append(d['frecv'])
                self.dupl.append((d['dupl']/d['recv'] if d['recv'] > 0 else 0, d['recv']))
                self.totaldupl += d['dupl']
            if d['addexc']:
                self.addexc += 1
        self.times.sort()
        self.sent.sort()
        self.recv.sort()
        self.fsent.sort()
        self.frecv.sort()
        self.dupl.sort()

    def get_data(self):
        return self.times, self.totaldupl, sum(self.recv) + sum(self.frecv)

    @staticmethod
    def multistats(datasets):
        ret = '    PID     Received    Duplicated\n'
        for name in sorted(datasets.keys()):
            _,d,r = datasets[name]
            ret += f'     {name} {r:10} {d:10}\n'
        ret += '\n  Duration [ms]\n'
        ret += multimean({k:datasets[k][0] for k in datasets})
        return ret

    def report(self):
        ret =   '  (ignoring syncs that exchanged nothing)\n' if self.ig else ''
        ret +=  '    Syncs in total:           %5d\n'%(len(self.inc)+len(self.out))
        ret +=  '    Incoming:                 %5d\n'%len(self.inc)
        ret +=  '    Outgoing:                 %5d\n'%len(self.out)
        ret +=  '    Failed:                   %5d\n'%self.failed
        ret +=  '    Additional exchange:      %5d\n\n'%self.addexc
        if not self.times:
            return ret + 'NO SUCCESSFUL SYNCS :(\n'
        ret +=  '    Max time:            %10d    ms\n'%self.times[-1]
        ret +=  '    Avg time:            %13.2f ms\n'%mean(self.times)
        ret +=  '    Avg time (>10ms):    %13.2f ms\n'%mean(filter(lambda x:x>10, self.times))
        ret +=  '    Med time:            %13.2f ms\n\n'%median(self.times)
        ret +=  '    Max units sent:      %10d\n'%self.sent[-1]
        ret +=  '    Avg units sent:      %13.2f\n'%mean(self.sent)
        ret +=  '    Med units sent:      %13.2f\n\n'%median(self.sent)
        ret +=  '    Max units received:  %10d\n'%self.recv[-1]
        ret +=  '    Avg units received:  %13.2f\n'%mean(self.recv)
        ret +=  '    Med units received:  %13.2f\n\n'%median(self.recv)
        ret +=  '    Max fresh units sent:%10d\n'%self.fsent[-1]
        ret +=  '    Avg fresh units sent:%13.2f\n'%mean(self.fsent)
        ret +=  '    Med fresh units sent:%13.2f\n\n'%median(self.fsent)
        ret +=  '    Max fresh units recv:%10d\n'%self.frecv[-1]
        ret +=  '    Avg fresh units recv:%13.2f\n'%mean(self.frecv)
        ret +=  '    Med fresh units recv:%13.2f\n\n'%median(self.frecv)
        ret +=  '    Max duplicated ratio:%13.2f (%d units)\n'%self.dupl[-1]
        ret +=  '    Avg duplicated ratio:%13.2f\n'%mean(i[0] for i in self.dupl)
        ret +=  '    Med duplicated ratio:%13.2f\n'%median(i[0] for i in self.dupl)
        ret +=  '    Largest recv duplicated ratio:\n'
        for i in sorted(self.dupl, key=lambda x: x[1], reverse=True)[:10]:
            ret +=  '       %13.2f (%d units)\n'%i
        ret += '\n'
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
            return ret + 'NO ENTRIES\n'
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


class GossipPlots(Plotter):
    """Plugin preparing plots about syncs statistics."""
    name = 'Sync plots'
    def __init__(self, regions=None, region_names=None, divide=True):
        self.inc = {}
        self.out = {}
        self.regions = {r:i for i,region in enumerate(regions) for r in region} if regions is not None else None
        self.region_names = region_names
        self.divide = divide
        self.colornames = plt.rcParams['axes.prop_cycle'].by_key()['color']

    def color(self, pid):
        if self.regions is None:
            return None
        if pid is None:
            return 'black'
        try:
            return [self.colornames[self.regions[i]] for i in pid]
        except TypeError:
            return self.colornames[self.regions[pid]]

    def process(self, entry):
        if PID not in entry:
            return entry
        if OSID in entry:
            d = self.out
            key = (entry[PID], entry[OSID])
        elif ISID in entry:
            d = self.inc
            key = (entry[PID], entry[ISID])
        else:
            return entry

        if key not in d:
            d[key] = {}
        if entry[Event] == SyncStarted:
            d[key]['start'] = entry[Time]
        elif entry[Event] == SyncCompleted:
            d[key]['end'] = entry[Time]
            d[key]['units'] = entry[Sent] + entry[Recv] + entry.get(FreshSent,0) + entry.get(FreshRecv,0)
        return entry

    def finalize(self):
        lst = []
        for (pid,sid),v in self.inc.items():
            if 'start' in v and 'end' in v and v['units'] > 0:
                lst.append((v['start'],v['end'],v['units'],pid))
        self.inc = lst
        lst = []
        for (pid,sid),v in self.out.items():
            if 'start' in v and 'end' in v and v['units'] > 0:
                lst.append((v['start'],v['end'],v['units'],pid))
        self.out = lst

    def corrplot(self, data, name, pid=None):
        """Plots the correlation between the number of exchanged units and sync duration."""
        d = np.array(sorted([(i[1]-i[0], i[2], i[3]) for i in data]))
        filename = f'corr_{name}.png' if name else 'corr.png'

        colors = self.color(d[:,2])
        mycol = self.color(pid)

        fig, ax = plt.subplots()
        fig.set_size_inches(25,15)

        ax.set_title(str(pid), fontsize=32, color=mycol)
        ax.set_xlabel('exchanged units', color=mycol)
        ax.set_ylabel('sync duration (ms)', color=mycol)
        ax.tick_params(colors=mycol)

        sc = ax.scatter(d[:,1], d[:,0], c=colors, alpha=0.5)

        plt.savefig(filename)
        plt.close()
        return filename


    def fancyplot(self, data, name, pid=None):
        """For each sync, plots the beginning time (X axis), length of the sync (Y axis), number of
            exchanged units (size of the marker) and area (color of the marker).
            The borders of the plot have the color corresponding to the area of the examined process.
            The number of concurrently happening syncs is shown as the gray line in the background.
        """
        d = np.array(sorted(data))
        filename = f'syncs_{name}.png' if name else 'syncs.png'
        colors = self.color(d[:,3])
        mycol = self.color(pid)

        conc = [(i,1) for i in d[:,0]] + [(i,-1) for i in d[:,1]]
        conc.sort()
        x,y = [],[]
        cur, last = 0,0
        for t,s in conc:
            if t != last:
                if last != 0:
                    x.append(last)
                    y.append(cur)
                last = t
            cur += s

        fig, ax1 = plt.subplots()
        fig.set_size_inches(25,15)

        ax1.set_title(str(pid), fontsize=32, color=mycol)
        ax1.set_xlabel('time (ms)', color=mycol)
        ax1.set_ylabel('sync duration (ms)', color=mycol)
        ax1.tick_params(colors=mycol)
        sc = ax1.scatter(d[:,0],d[:,1]-d[:,0], s=d[:,2], c=colors, alpha=0.5)
        if self.region_names:
            leg = plt.legend(handles=[Patch(color=c, label=r) for c,r in zip(self.colornames, self.region_names)], loc="upper right", framealpha=0.5)
            ax1.add_artist(leg)
        ax1.legend(*sc.legend_elements(prop='sizes', num=5), title="units exchanged", loc="upper left", framealpha=0.5)

        ax2 = ax1.twinx()
        ax2.set_ylabel('number of concurrent syncs', color=mycol)
        ax2.tick_params(colors=mycol)
        ax2.spines['top'].set_color(mycol)
        ax2.spines['bottom'].set_color(mycol)
        ax2.spines['left'].set_color(mycol)
        ax2.spines['right'].set_color(mycol)
        ax2.plot(x, y, color='black', linewidth=1, alpha=0.2)

        plt.savefig(filename)
        plt.close()
        return filename

    def saveplot(self, name):
        try:
            pid = int(name)
        except:
            pid = None

        names = []
        names.append(self.fancyplot(self.inc + self.out, name, pid))
        if self.divide:
            names.append(self.fancyplot(self.out, name+'_o', pid))
            names.append(self.fancyplot(self.inc, name+'_i', pid))
        names.append(self.corrplot(self.inc + self.out, name, pid))
        return f'Plots saved as ' + ', '.join(names)


class DuplUnitPlots(Plotter):
    """Plugin preparing plots about the ratio of duplicated units amongst received units."""
    name = 'Duplicated units plots'
    def __init__(self):
        self.inc = {}
        self.out = {}

    def process(self, entry):
        if PID not in entry:
            return entry
        if OSID in entry:
            d = self.out
            key = (entry[PID], entry[OSID])
        elif ISID in entry:
            d = self.inc
            key = (entry[PID], entry[ISID])
        else:
            return entry

        if key not in d:
            d[key] = {'dupl':0}
        if entry[Event] == SyncCompleted:
            d[key]['recv'] = entry[Recv] + entry.get(FreshRecv,0)
        elif entry[Event] == DuplicatedUnit:
            d[key]['dupl'] += 1
        return entry

    def finalize(self):
        self.inc = sorted([(v['recv'], v['dupl']) for v in self.inc.values() if 'recv' in v], reverse=True, key = lambda x: (x[0], -x[1]))
        self.out = sorted([(v['recv'], v['dupl']) for v in self.out.values() if 'recv' in v], reverse=True, key = lambda x: (x[0], -x[1]))
        self.recv, self.dupl = 0,0
        for r,d in self.inc:
            self.recv += r
            self.dupl += d
        for r,d in self.out:
            self.recv += r
            self.dupl += d

    def get_data(self):
        return self.dupl, self.recv

    @staticmethod
    def multistats(datasets):
        dupl, recv = 0,0
        for d,r in datasets.values():
            dupl += d
            recv += r
        ratio = 100*dupl/recv
        return f'Global average of received duplicates: {ratio:5.2f}%'

    def saveplot(self, name):
        filename = f'dupl_{name}.png' if name else 'dupl.png'
        x_inc = list(range(len(self.inc)))
        x_out = list(range(len(self.out)))
        unique_inc = [i[0] - i[1] for i in self.inc]
        unique_out = [i[0] - i[1] for i in self.out]
        dupl_inc = [i[1] for i in self.inc]
        dupl_out = [i[1] for i in self.out]
        di, do = sum(dupl_inc), sum(dupl_out)
        r_inc = 100*di/(di + sum(unique_inc))
        r_out = 100*do/(do + sum(unique_out))

        fig, ax = plt.subplots(nrows=2, sharex=True)
        fig.set_size_inches(12,10)
        fig.subplots_adjust(hspace=0)

        un = ax[0].bar(x_out, unique_out, width=1.0, color='green')
        du = ax[0].bar(x_out, dupl_out, bottom=unique_out, width=1.0, color='orange')
        ax[0].legend((un, du), ('new', 'duplicated'))
        ax[0].set_ylabel('received units (outgoing syncs)')
        ax[0].set_title(f'Duplicated units: {r_inc:4.1f}% inc, {r_out:4.1f}% out', fontsize=16)
        ax[1].bar(x_inc, unique_inc, width=1.0, color='green')
        ax[1].bar(x_inc, dupl_inc, bottom=unique_inc, width=1.0, color='orange')
        ax[1].invert_yaxis()
        ax[1].set_ylabel('received units (incoming syncs)')

        plt.savefig(filename)
        plt.close()
        return f'Plot saved as {filename}'
