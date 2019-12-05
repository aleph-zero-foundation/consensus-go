from plugins import *


class MulticastStats(Plugin):
    """Plugin gathering statistics of units received via multicast."""
    name = 'Incoming multicast events'
    def __init__(self):
        self.succ = 0
        self.miss = 0
        self.dupl = 0
        self.err = 0

    def process(self, entry):
        if entry[Service] != MCService:
            return entry
        if entry[Event] == AddedBCUnit:
            self.succ += 1
        elif entry[Event] == UnknownParents:
            self.miss += 1
        elif entry[Event] in [DuplicateUnit, DuplicatePreunit]:
            self.dupl += 1
        elif entry[Level] == 3 and 'multicast.In' in entry['where']:
            self.err += 1
        return entry

    def get_data(self):
        if self.succ+self.miss+self.dupl+self.err > 0:
            return self.succ, self.miss, self.dupl

    @staticmethod
    def multistats(datasets):
        ret = '    PID     Success    Failed    Duplicates\n'
        for name in sorted(datasets.keys()):
            if datasets[name] is not None:
                s,m,d = datasets[name]
                ret += f'{name:>7} {s:10} {m:10} {d:10}\n'
        return ret

    def report(self):
        if self.get_data() is None:
            return sadpanda
        ret  =  '    Units with all parents:    %5d\n'%self.succ
        ret +=  '    Units with missing parents:%5d\n'%self.miss
        ret +=  '    Duplicates received:       %5d\n'%self.dupl
        ret +=  '    Interrupted by error:      %5d\n'%self.err
        ret +=  '    Total:                     %5d\n'%(self.succ+self.miss+self.dupl+self.err)
        return ret


class FetchStats(Plugin):
    """Plugin gathering statistics of outgoing fetches."""
    name = 'Fetch stats (out only)'
    def __init__(self):
        self.data = {}

    def process(self, entry):
        if entry[Service] != FetchService or PID not in entry or OSID not in entry:
            return entry
        key = (entry[PID], entry[OSID])

        if key not in self.data:
            self.data[key] = {'dupl':0, 'fail':False}

        if entry[Event] == SyncStarted:
            self.data[key]['start'] = entry[Time]
        elif entry[Event] == SyncCompleted:
            self.data[key]['end'] = entry[Time]
            self.data[key]['recv'] = entry[Recv]
        elif entry[Event] in [DuplicateUnit, DuplicatePreunit]:
            self.data[key]['dupl'] += 1
        elif entry[Level] == '3':
            self.data[key]['fail'] = True
        return entry

    def finalize(self):
        self.times = []
        self.recv = []
        self.dupl = []
        self.totaldupl, self.failed, self.unfinished = 0,0,0
        for d in self.data.values():
            if d['fail']:
                self.failed += 1
            elif 'end' not in d:
                self.unfinished += 1
                continue
            elif 'start' in d:
                self.times.append(d['end']-d['start'])
                self.recv.append(d['recv'])
                self.dupl.append((d['dupl']/d['recv'] if d['recv'] > 0 else 0, d['recv']))
                self.totaldupl += d['dupl']
        self.times.sort()
        self.recv.sort()
        self.dupl.sort()

    def get_data(self):
        totalrecv = sum(self.recv)
        if len(self.times) > 0 and totalrecv > 0:
            return self.times, self.totaldupl, totalrecv

    @staticmethod
    def multistats(datasets):
        ret = '    PID     Received    Duplicates\n'
        for name in sorted(datasets.keys()):
            if datasets[name] is not None:
                _,d,r = datasets[name]
                ret += f'{name:>7} {r:10} {d:10}\n'
        ret += '\n  Duration [ms]\n'
        ret += multimean({k:datasets[k][0] for k in datasets if datasets[k]})
        return ret

    def report(self):
        if len(self.data) == 0:
            return sadpanda
        ret =   '    Fetches in total:         %5d\n'%len(self.data)
        ret +=  '    Failed:                   %5d\n'%self.failed
        ret +=  '    Unfinished:               %5d\n'%self.unfinished
        if not self.times:
            return ret + sadpanda
        ret +=  '    Max time:            %10d    ms\n'%self.times[-1]
        ret +=  '    Avg time:            %13.2f ms\n'%mean(self.times)
        if self.times[-1] > 10:
            ret +=  '    Avg time (>10ms):    %13.2f ms\n'%mean(filter(lambda x:x>10, self.times))
        ret +=  '    Med time:            %13.2f ms\n\n'%median(self.times)
        ret +=  '    Max units received:  %10d\n'%self.recv[-1]
        ret +=  '    Avg units received:  %13.2f\n'%mean(self.recv)
        ret +=  '    Med units received:  %13.2f\n\n'%median(self.recv)
        ret +=  '    Max duplicates ratio:%13.2f (%d units)\n'%self.dupl[-1]
        ret +=  '    Avg duplicates ratio:%13.2f\n'%mean(i[0] for i in self.dupl)
        ret +=  '    Med duplicates ratio:%13.2f\n'%median(i[0] for i in self.dupl)
        ret +=  '    Largest duplicate ratio:\n'
        for i in sorted(self.dupl, reverse=True)[:10]:
            ret +=  '       %13.2f (%d units)\n'%i
        ret += '\n'
        return ret


class GossipStats(Plugin):
    """Plugin gathering detailed statistics of syncs during gossip."""
    name = 'Gossip stats'
    def __init__(self, ignore_empty=True):
        self.ig = ignore_empty
        self.inc = {}
        self.out = {}

    def process(self, entry):
        if entry[Service] != GossipService:
            return entry
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
        elif entry[Event] in [DuplicateUnit, DuplicatePreunit]:
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
        self.totaldupl, self.addexc, self.failed, self.unfinished = 0,0,0,0
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
        totalrecv = sum(self.recv) + sum(self.frecv)
        if len(self.times) > 0 and totalrecv > 0:
            return self.times, self.totaldupl, totalrecv

    @staticmethod
    def multistats(datasets):
        ret = '    PID     Received    Duplicated\n'
        for name in sorted(datasets.keys()):
            if datasets[name] is not None:
                _,d,r = datasets[name]
                ret += f'{name:>7} {r:10} {d:10}\n'
        ret += '\n  Duration [ms]\n'
        ret += multimean({k:datasets[k][0] for k in datasets})
        return ret

    def report(self):
        if len(self.inc)+len(self.out) == 0:
            return sadpanda
        ret =   '  (ignoring syncs that exchanged nothing)\n' if self.ig else ''
        ret +=  '    Syncs in total:           %5d\n'%(len(self.inc)+len(self.out))
        ret +=  '    Incoming:                 %5d\n'%len(self.inc)
        ret +=  '    Outgoing:                 %5d\n'%len(self.out)
        ret +=  '    Failed:                   %5d\n'%self.failed
        ret +=  '    Additional exchange:      %5d\n\n'%self.addexc
        if not self.times:
            return ret + sadpanda
        ret +=  '    Max time:            %10d    ms\n'%self.times[-1]
        ret +=  '    Avg time:            %13.2f ms\n'%mean(self.times)
        if self.times[-1] > 10:
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
        ret +=  '    Max duplicates ratio:%13.2f (%d units)\n'%self.dupl[-1]
        ret +=  '    Avg duplicates ratio:%13.2f\n'%mean(i[0] for i in self.dupl)
        ret +=  '    Med duplicates ratio:%13.2f\n'%median(i[0] for i in self.dupl)
        ret +=  '    Largest recv duplicates ratio:\n'
        for i in sorted(self.dupl, reverse=True)[:10]:
            ret +=  '       %13.2f (%d units)\n'%i
        ret += '\n'
        return ret

