#!/usr/bin/env python2
import sys
sys.path.append("pyswarming/client")
import isolate
import isolate_format
import json
from code import interact


def parse_archive_command_line(args):
    opts = isolate.parse_archive_command_line(args[1:], args[0])
    d = eval(str(opts))
    # interact(local=locals())
    return d


def isolate_format_eval_variables(args):
    s = json.load(sys.stdin)
    return isolate_format.eval_variables(args[0], s)


def test_sum(args):
    return 0 if not args else sum(map(int, args))


if __name__ == "__main__":
    try:
        run = sys.argv[1]
        func = globals()[sys.argv[1]]
    except Exception:
        print "bad arguments: %s" % sys.argv[1:]
        sys.exit(250)
    try:
        d = func(sys.argv[2:])
        json.dump(d, sys.stdout)
        sys.exit(0)
    except Exception, e:
        print "raised %s", e
        raise
