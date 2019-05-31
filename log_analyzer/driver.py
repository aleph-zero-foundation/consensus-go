from collections import OrderedDict
from copy import deepcopy

class Driver:
    """Driver is used to store all the plugins (for each service separately) and dispatch log events to them."""
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
