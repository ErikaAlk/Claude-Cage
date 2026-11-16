import sys

import gi

gi.require_version('Gtk', '3.0')
gi.require_version('Gdk', '3.0')
from gi.repository import Gdk, Gtk

clipboard = Gtk.Clipboard.get(Gdk.SELECTION_CLIPBOARD)
selection = clipboard.wait_for_contents(Gdk.Atom.intern('image/png', False))
if selection is None:
    raise RuntimeError('客机剪贴板没有 PNG 图片')
sys.stdout.buffer.write(selection.get_data())
