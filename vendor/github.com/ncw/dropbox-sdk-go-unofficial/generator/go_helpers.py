from stone.api import ApiNamespace
from stone.data_type import (
    Boolean,
    Float32,
    Float64,
    Int32,
    Int64,
    String,
    Timestamp,
    UInt32,
    UInt64,
    unwrap_nullable,
    is_composite_type,
    is_list_type,
    is_struct_type,
    Void,
)
from stone.target import helpers

HEADER = """\
// Copyright (c) Dropbox, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
"""

_reserved_keywords = {
    'break', 'default', 'func', 'interface', 'select',
    'case', 'defer', 'go',   'map',  'struct',
    'chan', 'else', 'goto', 'package', 'switch',
    'const', 'fallthrough', 'if',   'range', 'type',
    'continue', 'for',  'import',  'return',  'var',
}

_type_table = {
    UInt64: 'uint64',
    Int64: 'int64',
    UInt32: 'uint32',
    Int32: 'int32',
    Float64: 'float64',
    Float32: 'float32',
    Boolean: 'bool',
    String: 'string',
    Timestamp: 'time.Time',
    Void: 'struct{}',
}


def _rename_if_reserved(s):
    if s in _reserved_keywords:
        return s + '_'
    else:
        return s


def fmt_type(data_type, namespace=None, use_interface=False):
    data_type, nullable = unwrap_nullable(data_type)
    if is_list_type(data_type):
        return '[]%s' % fmt_type(data_type.data_type, namespace, use_interface)
    type_name = data_type.name
    if use_interface and _needs_base_type(data_type):
        type_name = 'Is' + type_name
    if is_composite_type(data_type) and namespace is not None and \
            namespace.name != data_type.namespace.name:
        type_name = data_type.namespace.name + '.' + type_name
    if use_interface and _needs_base_type(data_type):
        return _type_table.get(data_type.__class__, type_name)
    else:
        return _type_table.get(data_type.__class__, '*' + type_name)


def fmt_var(name, export=True, check_reserved=False):
    s = helpers.fmt_pascal(name) if export else helpers.fmt_camel(name)
    return _rename_if_reserved(s) if check_reserved else s


def _doc_handler(tag, val):
    if tag == 'type':
        return '`{}`'.format(val)
    elif tag == 'route':
        return '`{}`'.format(helpers.fmt_camel(val))
    elif tag == 'link':
        anchor, link = val.rsplit(' ', 1)
        return '`{}` <{}>'.format(anchor, link)
    elif tag == 'val':
        if val == 'null':
            return 'nil'
        else:
            return val
    elif tag == 'field':
        return '`{}`'.format(val)
    else:
        raise RuntimeError('Unknown doc ref tag %r' % tag)


def generate_doc(code_generator, t):
    doc = t.doc
    if doc is None:
        doc = 'has no documentation (yet)'
    doc = code_generator.process_doc(doc, _doc_handler)
    d = '%s : %s' % (fmt_var(t.name), doc)
    if isinstance(t, ApiNamespace):
        d = 'Package %s : %s' % (t.name, doc)
    code_generator.emit_wrapped_text(d, prefix='// ')


def _needs_base_type(data_type):
    if is_struct_type(data_type) and data_type.has_enumerated_subtypes():
        return True
    if is_list_type(data_type):
        return _needs_base_type(data_type.data_type)
    return False


def needs_base_type(struct):
    for field in struct.fields:
        if _needs_base_type(field.data_type):
            return True
    return False
