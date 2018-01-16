import os
import shutil

from stone.backend import CodeBackend
from stone.ir import (
    is_boolean_type,
    is_list_type,
    is_nullable_type,
    is_primitive_type,
    is_struct_type,
    is_union_type,
    is_void_type,
)

from go_helpers import (
    HEADER,
    fmt_type,
    fmt_var,
    generate_doc,
    needs_base_type,
    _needs_base_type
)


class GoTypesBackend(CodeBackend):
    def generate(self, api):
        rsrc_folder = os.path.join(os.path.dirname(__file__), 'go_rsrc')
        shutil.copy(os.path.join(rsrc_folder, 'sdk.go'),
                    self.target_folder_path)
        for namespace in api.namespaces.values():
            self._generate_namespace(namespace)

    def _generate_namespace(self, namespace):
        file_name = os.path.join(self.target_folder_path, namespace.name,
                                 'types.go')
        with self.output_to_relative_path(file_name):
            self.emit_raw(HEADER)
            self.emit()
            generate_doc(self, namespace)
            self.emit('package %s' % namespace.name)
            self.emit()

            for data_type in namespace.linearize_data_types():
                self._generate_data_type(data_type)

    def _generate_data_type(self, data_type):
        generate_doc(self, data_type)
        if is_struct_type(data_type):
            self._generate_struct(data_type)
            if data_type.has_enumerated_subtypes():
                self._generate_base_type(data_type)
        elif is_union_type(data_type):
            self._generate_union(data_type)
        else:
            self.logger.info("Unhandled data type", data_type)

    def _generate_base_type(self, base):
        t = fmt_type(base).lstrip('*')
        self.emit('// Is{0} is the interface type for {0} and its subtypes'.format(t))
        with self.block('type Is%s interface' % t):
            self.emit('Is%s()' % t)
        self.emit()
        self.emit('// Is{0} implements the Is{0} interface'.format(t))
        self.emit("func (u *{0}) Is{0}() {{}}".format(t))
        self.emit()
        self._generate_union_helper(base)

        self.emit("// Is{0}FromJSON converts JSON to a concrete Is{0} instance".format(t))
        with self.block("func Is{0}FromJSON(data []byte) (Is{0}, error)".format(t)):
            name = fmt_var(t, export=False) + 'Union'
            self.emit("var t {0}".format(name))
            with self.block("if err := json.Unmarshal(data, &t); err != nil"):
                self.emit("return nil, err")
            with self.block("switch t.Tag"):
                fields = base.get_enumerated_subtypes()
                for field in fields:
                    with self.block('case "%s":' % field.name, delim=(None, None)):
                        self.emit("return t.{0}, nil".format(fmt_var(field.name)))
            # FIX THIS
            self.emit("return nil, nil")

    def _generate_struct(self, struct):
        with self.block('type %s struct' % struct.name):
            if struct.parent_type:
                self.emit(fmt_type(struct.parent_type, struct.namespace).lstrip('*'))
            for field in struct.fields:
                self._generate_field(field, namespace=struct.namespace)
            if struct.name in ('DownloadArg',):
                self.emit('// ExtraHeaders can be used to pass Range, If-None-Match headers')
                self.emit('ExtraHeaders map[string]string `json:"-"`')
        self._generate_struct_builder(struct)
        self.emit()
        if needs_base_type(struct):
            self.emit('// UnmarshalJSON deserializes into a %s instance' % struct.name)
            with self.block('func (u *%s) UnmarshalJSON(b []byte) error' % struct.name):
                with self.block('type wrap struct'):
                    for field in struct.all_fields:
                        self._generate_field(field, namespace=struct.namespace,
                                             raw=_needs_base_type(field.data_type))
                self.emit('var w wrap')
                with self.block('if err := json.Unmarshal(b, &w); err != nil'):
                    self.emit('return err')
                for field in struct.all_fields:
                    dt = field.data_type
                    fn = fmt_var(field.name)
                    tn = fmt_type(dt, namespace=struct.namespace, use_interface=True)
                    if _needs_base_type(dt):
                        if is_list_type(dt):
                            self.emit("u.{0} = make({1}, len(w.{0}))".format(fn, tn))
                            # Grab the underlying type to get the correct Is...FromJSON method
                            tn = fmt_type(dt.data_type, namespace=struct.namespace, use_interface=True)
                            with self.block("for i, e := range w.{0}".format(fn)):
                                self.emit("v, err := {1}FromJSON(e)".format(fn, tn))
                                with self.block('if err != nil'):
                                    self.emit('return err')
                                self.emit("u.{0}[i] = v".format(fn))
                        else:
                            self.emit("{0}, err := {1}FromJSON(w.{0})".format(fn, tn))
                            with self.block('if err != nil'):
                                self.emit('return err')
                            self.emit("u.{0} = {0}".format(fn))
                    else:
                        self.emit("u.{0} = w.{0}".format(fn))
                self.emit('return nil')

    def _generate_struct_builder(self, struct):
        fields = ["%s %s" % (fmt_var(field.name),
                             fmt_type(field.data_type, struct.namespace,
                                      use_interface=True))
                  for field in struct.all_required_fields]
        self.emit('// New{0} returns a new {0} instance'.format(struct.name))
        signature = "func New{0}({1}) *{0}".format(struct.name, ', '.join(fields))
        with self.block(signature):
            self.emit('s := new({0})'.format(struct.name))
            for field in struct.all_required_fields:
                field_name = fmt_var(field.name)
                self.emit("s.{0} = {0}".format(field_name))

            for field in struct.all_optional_fields:
                if field.has_default:
                    if is_primitive_type(field.data_type):
                        default = field.default
                        if is_boolean_type(field.data_type):
                            default = str(default).lower()
                        self.emit('s.{0} = {1}'.format(fmt_var(field.name), default))
                    elif is_union_type(field.data_type):
                        self.emit('s.%s = &%s{Tagged:dropbox.Tagged{"%s"}}' %
                                  (fmt_var(field.name),
                                   fmt_type(field.data_type, struct.namespace).lstrip('*'),
                                   field.default.tag_name))
            self.emit('return s')
        self.emit()

    def _generate_field(self, field, union_field=False, namespace=None, raw=False):
        generate_doc(self, field)
        field_name = fmt_var(field.name)
        type_name = fmt_type(field.data_type, namespace, use_interface=True, raw=raw)
        json_tag = '`json:"%s"`' % field.name
        if is_nullable_type(field.data_type) or union_field:
            json_tag = '`json:"%s,omitempty"`' % field.name
        self.emit('%s %s %s' % (field_name, type_name, json_tag))

    def _generate_union(self, union):
        self._generate_union_helper(union)

    def _generate_union_helper(self, u):
        name = u.name
        namespace = u.namespace
        # Unions can be inherited, but don't need to be polymorphic.
        # So let's flatten out all the inherited fields.
        fields = u.all_fields
        if is_struct_type(u) and u.has_enumerated_subtypes():
            name = fmt_var(name, export=False) + 'Union'
            fields = u.get_enumerated_subtypes()

        with self.block('type %s struct' % name):
            self.emit('dropbox.Tagged')
            for field in fields:
                if is_void_type(field.data_type):
                    continue
                self._generate_field(field, union_field=True,
                                     namespace=namespace)
        self.emit()
        self.emit('// Valid tag values for %s' % fmt_var(u.name))
        with self.block('const', delim=('(', ')')):
            for field in fields:
                self.emit('%s%s = "%s"' % (fmt_var(u.name), fmt_var(field.name), field.name))
        self.emit()

        num_void_fields = sum([is_void_type(f.data_type) for f in fields])
        # Simple structure, no need in UnmarshalJSON
        if len(fields) == num_void_fields:
            return

        self.emit('// UnmarshalJSON deserializes into a %s instance' % name)
        with self.block('func (u *%s) UnmarshalJSON(body []byte) error' % name):
            with self.block('type wrap struct'):
                self.emit('dropbox.Tagged')
                for field in fields:
                    if is_void_type(field.data_type) or \
                            is_primitive_type(field.data_type):
                        continue
                    self._generate_field(field, union_field=True,
                                         namespace=namespace, raw=True)
            self.emit('var w wrap')
            self.emit('var err error')
            with self.block('if err = json.Unmarshal(body, &w); err != nil'):
                self.emit('return err')
            self.emit('u.Tag = w.Tag')
            with self.block('switch u.Tag'):
                for field in fields:
                    if is_void_type(field.data_type):
                        continue
                    field_name = fmt_var(field.name)
                    with self.block('case "%s":' % field.name, delim=(None, None)):
                        if is_union_type(field.data_type):
                            self.emit('err = json.Unmarshal(w.{0}, &u.{0})'
                                            .format(field_name))
                        elif _needs_base_type(field.data_type):
                            self.emit("u.{0}, err = Is{1}FromJSON(body)"
                                      .format(field_name, field.data_type.name))
                        else:
                            self.emit('err = json.Unmarshal(body, &u.{0})'
                                            .format(field_name))
                    with self.block("if err != nil"):
                        self.emit("return err")
            self.emit('return nil')
        self.emit()
