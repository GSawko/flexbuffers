#!/usr/bin/env python3

# Usage:
#
# export PYTHONPATH=/home/gs/flexbuffers/python:/home/gs/flexbuffers/go/flexbuffers/testdata
# python3 -m flexeval 'Op1() | Op2() | EOF()'
#
# Examples:
#
# String("Hello, world") | EOF()
# StartVector() | Int(1) | Int(2) | EndVector() | EOF()
# StartMap() | Key("name") | String("Joe Blow") | EndMap() | EOF()

from lib2to3.pgen2.parse import ParseError
import sys
import re
from flatbuffers import flexbuffers

class state:
    def __init__(self):
        self.b = flexbuffers.Builder(
            share_strings=False,
            share_keys=True,
            force_min_bit_width=flexbuffers.BitWidth.W8,
        )
        self.startmap = []
        self.startvec = []
        self.endargs = []


class op:
    def __init__(self, method, *args):
        self.method = method
        self.args = args
        self.state = None
        self.endargs = None

    def _lazyinit(self):
        if self.state is not None:
            return
        self.state = state()

    def __or__(self, next):
        self._lazyinit()

        args = self.args
        if self.method == '_EndMap':
            args = [self.state.startmap.pop()]
        if self.method == '_EndVector':
            args = [self.state.startvec.pop()] + self.state.endargs.pop()

        #print(f'D: {self.method} args {args}', file=sys.stderr)
        ret = getattr(self.state.b, self.method)(*args)

        if self.method == '_StartMap':
            self.state.startmap.append(ret)
        if self.method == '_StartVector':
            self.state.startvec.append(ret)
            self.state.endargs.append(self.endargs)

        if next.method == 'Finish':
            if len(self.state.startvec) + len(self.state.startmap) + len(self.state.endargs) > 0:
                raise Exception('StartFoo and EndFoo unbalanced')
            buf = self.state.b.Finish()
            #int_buf = [int(b) for b in buf]
            sys.stdout.buffer.write(buf)
            #print(int_buf)
            return
        next.state = self.state
        return next


def EOF():
    return op('Finish')

def Null():
    return op('Add', None)


def Bool(b):
    return op('Bool', b)


def Int(n):
    return op('Int', n, 0)


def Int8(n):
    return op('Int', n, 1)


def Int16(n):
    return op('Int', n, 2)


def Int32(n):
    return op('Int', n, 4)


def Int64(n):
    return op('Int', n, 8)


def Uint(n):
    return op('UInt', n, 0)


def Float(n):
    return op('Float', n)


def String(s):
    return op('String', s)


def Key(k):
    return op('Key', k)


def StartMap():
    return op('_StartMap')


def EndMap():
    return op('_EndMap')


def StartVector():
    ret = op('_StartVector')
    ret.endargs = [False, False]
    return ret


# The first element determines the type of the vector.
def StartVectorTyped():
    ret = op('_StartVector')
    ret.endargs = [True, False]
    return ret


def StartVectorTypedFixed():
    ret = op('_StartVector')
    ret.endargs = [True, True]
    return ret


def EndVector():
    return op('_EndVector')


def Parse(cmdstr):
    reg = re.compile('(?:<(?P<key>[^>]+)>)?(?P<method>[a-zA-Z]+)\((?P<args>[^)]+)?\)')
    cmdmap = {
        "INT" : "Int",
        "UINT" : "Uint",
        "FLOAT" : "Float",
        "STRING" : "String",
        "BOOL" : "Bool",
        "NULL" : "Null",
        "KEY" : "Key",
        "VEC" : "StartVector",
        "INTVEC" : "StartVectorTyped",
        "UINTVEC" : "StartVectorTyped",
        "FLOATVEC" : "StartVectorTyped",
        "BOOLVEC" : "StartVectorTyped",
        "STRINGVEC" : "StartVectorTyped",
        "KEYVEC" : "StartVectorTyped",
        "BLOB" : "StartVectorTyped",
        "MAP" : "StartMap",
    }
    context = []
    def foo(str):
        if str=="":
            return ""
        parsedstr = ""
        matches = re.match(reg,str)
        if not matches:
            raise ParseError
        match_dict = matches.groupdict()
        key = match_dict["key"] if match_dict["key"] else ""
        method = match_dict["method"] if match_dict["method"] else ""
        args = match_dict["args"] if match_dict["args"] else ""
        cmd = ""
        
        if key:
            return foo(f'KEY("{key}") {method}({args}) ' + str[len(matches[0])+1:])
            
        if method == "END":
            con = context.pop(-1)
            if con == "VEC":
                cmd = "EndVector"
            elif con == "MAP":
                cmd = "EndMap"
            else:
                raise SyntaxError
            
        elif method in cmdmap:
            cmd = cmdmap[method]
        else:
            raise ParseError

        if cmd in ["StartVector","StartVectorTyped","StartVectorTypedFixed"]:
            context.append("VEC")
        elif cmd == "StartMap":
            context.append("MAP")

        if method =="STRING" or method == "KEY":
            args=args.replace('"','\\"').replace("'","\\'")
            parsedstr += f'{cmd}("{args}") | '
        else:
            parsedstr += f"{cmd}({args}) | "
        return parsedstr + foo(str[len(matches[0])+1:])
    s = foo(cmdstr) + "EOF()"
    return s

        



if __name__ == '__main__':
    eval(Parse(sys.argv[1]))
