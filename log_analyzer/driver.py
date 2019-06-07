from collections import OrderedDict
from copy import deepcopy

class Driver:
    """
    Driver stores a list of pipelines that analyze log entries. Each pipeline consists of name and list of plugins.
    Every plugin has a process() method that takes a log entry and returns either a log entry (the same, or changed) or None.
    Each log entry is pushed through each pipeline by executing, in order, each plugin's process() method
    (next plugin's process() is fed with what the previous plugin returned).
    """
    def __init__(self):
        self.pipelines = OrderedDict()

    def add_pipeline(self, name, plugins):
        if not isinstance(plugins, list):
            plugins = [plugins]
        self.pipelines[name] = plugins

    def handle(self, entry):
        for pipeline in self.pipelines.values():
            e = deepcopy(entry)
            for plugin in pipeline:
                e = plugin.process(e)
                if e is None:
                    break

    def finalize(self):
        for pipeline in self.pipelines.values():
            for plugin in pipeline:
                plugin.finalize()

    def report(self):
        ret = ''
        for title, pipeline in self.pipelines.items():
            ret += maketitle(title, 80, '=') + '\n'
            for plugin in pipeline:
                name, rep = plugin.report()
                if name:
                    ret += maketitle(name, 60, '-') + '\n'
                    ret += rep + '\n'
            ret += '\n'
        return ret


def maketitle(string, length, pad):
    h = length - len(string) - 2
    return pad*(h // 2) + f' {string} ' + pad*((h+1) // 2)
