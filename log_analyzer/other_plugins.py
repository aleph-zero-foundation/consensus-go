from plugins import *


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
        self.primes = 0

    def process(self, entry):
        if entry[Message] == UnitCreated:
            self.primes += 1
        return entry

    def report(self):
        ret =  '    Units:              %5d\n'%self.primes
        return ret


class TXPS(Plugin):
    """Plugin calculating the average number of units ordered per second."""
    multistats = multimean

    def __init__(self, units_per_level, timing_freq, config):
        self.units_per_level = units_per_level
        self.timing_freq = timing_freq
        if config and 'Txpu' in config:
            self.txpu = int(config['Txpu'])
            self.name = 'Tx per second'
        else:
            self.txpu = 1
            self.name = 'Units per second'

    def process(self, entry):
        return entry

    def finalize(self):
        upl = mean(self.units_per_level.get_data())
        lps = 1000/mean(self.timing_freq.get_data()) #avg levels per second
        self.value = upl*lps*self.txpu

    def get_data(self):
        return [self.value]

    def report(self):
        return  '    Average: %10d\n'%self.value


class NetworkTraffic(Plugin):
    """Plugin measuring the size of data sent/received through the network."""
    name = 'Network traffic [kB/s]'
    def __init__(self, skip_first=0):
        self.data = {}
        self.skip = skip_first

    def process(self, entry):
        return entry

    def finalize(self):
        self.sent = [self.data[k][0]>>10 for k in sorted(self.data.keys())]
        self.recv = [self.data[k][1]>>10 for k in sorted(self.data.keys())]

    def report(self):
        s = self.sent[self.skip:]
        r = self.recv[self.skip:]
        ret =  '  (skipped first %d entries)\n'%self.skip if self.skip else ''
        if not s:
            return ret + sadpanda
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
        return entry

    def report(self):
        ret = '     Time[s]     Heap[%s]     Total[%s]\n' % (self.unit, self.unit)
        for i in self.data:
            ret += '%10d    %10d    %10d\n' % i
        return ret

