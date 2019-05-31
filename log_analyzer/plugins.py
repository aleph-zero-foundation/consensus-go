class Plugin:
    """Parent class definition for all plugins."""
    def process(self, entry):
        return entry
    def finalize(self):
        pass
    def report(self):
        return None, ''
