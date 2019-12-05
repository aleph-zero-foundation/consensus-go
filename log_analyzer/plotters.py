from plugins import *

from matplotlib.patches import Patch
import matplotlib.pyplot as plt

class Plotter(Plugin):
    """Subclass for plugins that produce plots."""
    def saveplot(self, name):
        return ''

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
        if len(self.inc) == 0 and len(self.out) == 0:
            return sadpanda
        try:
            pid = int(name)
        except:
            pid = None

        names = []
        names.append(self.fancyplot(self.inc + self.out, name, pid))
        if self.divide:
            if self.out:
                names.append(self.fancyplot(self.out, name+'_o', pid))
            if self.inc:
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
        if recv == 0:
            return sadpanda
        ratio = 100*dupl/recv
        return f'Global average of received duplicates: {ratio:5.2f}%'

    def saveplot(self, name):
        if len(self.inc) == 0 and len(self.out) == 0:
            return sadpanda
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
