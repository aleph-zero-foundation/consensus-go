from const import *

class Plugin:
    """Parent class definition for all plugins."""
    def process(self, entry):
        return entry
    def finalize(self):
        pass
    def report(self):
        return None, ''


class Filter(Plugin):
    """Plugin filtering out entries. Only entries that have field *key* equal to
    one of *values* pass through. *values* can be a single item (int or str), list of
    items or None (in that case every value is accepted, as long as *key* is present."""

    def __init__(self, key, values=None):
        self.key = key
        self.values = values if (values is None or isinstance(values, list)) else [values]

    def process(self, entry):
        if self.key in entry and (self.values is None or entry[self.key] in self.values):
            return entry
            print(entry)
        return None


class Timer(Plugin):
    """Plugin gathering basic timing statistics of all incoming events."""
    def __init__(self, name):
        self.name = name
        self.times = []

    def process(self, entry):
        self.times.append(entry[Time])
        return entry

    def finalize(self):
        self.times.sort()
        print(self.times)
        self.avg = self.times[-1] / len(self.times)
        for i in range(len(self.times)-1, 1, -1):
            self.times[i] -= self.times[i-1]

    def report(self):
        ret =  '    Min: %10d ms\n'%min(self.times)
        ret += '    Max: %10d ms\n'%max(self.times)
        ret += '    Avg: %10d ms\n'%self.avg
        return 'Timer: '+self.name, ret


class CreateCounter(Plugin):
    """Plugin counting basis statistics of create service:
    * number of prime units
    * number of non-prime units
    * number of times create unit failed due to not enough parents
    * longest streak of non-prime units
    """
    def __init__(self):
        self.nonprimes = 0
        self.primes = 0
        self.noparents = 0
        self.streak = 0
        self.max_streak = 0

    def process(self, entry):
        if entry[Event] == UnitCreated:
            self.nonprimes += 1
            self.streak += 1
        elif entry[Event] == PrimeUnitCreated:
            self.primes += 1
            self.max_streak = max(self.max_streak, self.streak)
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
        ret += '    Longest nonprime streak:  %5d\n'%max(self.max_streak, self.streak)
        return 'Create counter', ret


class TimingUnitCounter(Plugin):
    """Plugin measuring, at the moment of choosing new timing unit, the difference
    between the height of that timing unit and the highest prime unit in poset."""
    def __init__(self):
        self.data = []

    def process(self, entry):
        if entry[Event] == NewTimingUnit:
            self.data.append((entry[Height], entry[Round]))
        return entry

    def finalize(self):
        self.data.sort()
        self.dif = {}
        for i in self.data:
            d = i[0] - i[1]
            if d not in self.dif:
                self.dif[d] = 0
            self.dif[d] += 1
        num, den = 0,0
        for k,v in self.dif.items():
            num += k*v
            den += v
        print (num, den)
        self.avg = num/den


    def report(self):
        ret =  '    Difference        Units\n'
        for i in self.dif.items():
            ret += '     %3d             %5d\n'%i
        ret += '    Average:  %5f\n'%self.avg
        return 'Timing unit choice delay', ret



