import os

from stone.backend import CodeBackend
from stone.ir import (
    is_void_type,
    is_struct_type
)

from go_helpers import (
    HEADER,
    fmt_type,
    fmt_var,
    generate_doc,
)


class GoClientBackend(CodeBackend):
    def generate(self, api):
        for namespace in api.namespaces.values():
            if len(namespace.routes) > 0:
                self._generate_client(namespace)

    def _generate_client(self, namespace):
        file_name = os.path.join(self.target_folder_path, namespace.name,
                                 'client.go')
        with self.output_to_relative_path(file_name):
            self.emit_raw(HEADER)
            self.emit()
            self.emit('package %s' % namespace.name)
            self.emit()

            self.emit('// Client interface describes all routes in this namespace')
            with self.block('type Client interface'):
                for route in namespace.routes:
                    generate_doc(self, route)
                    self.emit(self._generate_route_signature(namespace, route))
            self.emit()

            self.emit('type apiImpl dropbox.Context')
            for route in namespace.routes:
                self._generate_route(namespace, route)
            self.emit('// New returns a Client implementation for this namespace')
            with self.block('func New(c dropbox.Config) Client'):
                self.emit('ctx := apiImpl(dropbox.NewContext(c))')
                self.emit('return &ctx')

    def _generate_route_signature(self, namespace, route):
        req = fmt_type(route.arg_data_type, namespace)
        res = fmt_type(route.result_data_type, namespace, use_interface=True)
        fn = fmt_var(route.name)
        style = route.attrs.get('style', 'rpc')

        arg = '' if is_void_type(route.arg_data_type) else 'arg {req}'
        ret = '(err error)' if is_void_type(route.result_data_type) else \
            '(res {res}, err error)'
        signature = '{fn}(' + arg + ') ' + ret
        if style == 'download':
            signature = '{fn}(' + arg + \
                ') (res {res}, content io.ReadCloser, err error)'
        elif style == 'upload':
            signature = '{fn}(' + arg + ', content io.Reader) ' + ret
            if is_void_type(route.arg_data_type):
                signature = '{fn}(content io.Reader) ' + ret
        return signature.format(fn=fn, req=req, res=res)


    def _generate_route(self, namespace, route):
        out = self.emit
        fn = fmt_var(route.name)
        err = fmt_type(route.error_data_type, namespace)
        out('//%sAPIError is an error-wrapper for the %s route' %
            (fn, route.name))
        with self.block('type {fn}APIError struct'.format(fn=fn)):
            out('dropbox.APIError')
            out('EndpointError {err} `json:"error"`'.format(err=err))
        out()

        signature = 'func (dbx *apiImpl) ' + self._generate_route_signature(
            namespace, route)
        with self.block(signature):
            if route.deprecated is not None:
                out('log.Printf("WARNING: API `%s` is deprecated")' % fn)
                if route.deprecated.by is not None:
                    out('log.Printf("Use API `%s` instead")' % fmt_var(route.deprecated.by.name))
                out()

            out('cli := dbx.Client')
            out()

            self._generate_request(namespace, route)
            self._generate_post()
            self._generate_response(route)
            ok_check = 'if resp.StatusCode == http.StatusOK'
            if fn == "Download":
                ok_check += ' || resp.StatusCode == http.StatusPartialContent'
            with self.block(ok_check):
                self._generate_result(route)
            self._generate_error_handling(route)

        out()

    def _generate_request(self, namespace, route):
        out = self.emit
        auth = route.attrs.get('auth', '')
        host = route.attrs.get('host', 'api')
        style = route.attrs.get('style', 'rpc')

        body = 'nil'
        if not is_void_type(route.arg_data_type):
            out('dbx.Config.LogDebug("arg: %v", arg)')

            out('b, err := json.Marshal(arg)')
            with self.block('if err != nil'):
                out('return')
            out()
            if host != 'content':
                body = 'bytes.NewReader(b)'
        if style == 'upload':
            body = 'content'

        headers = {}
        if not is_void_type(route.arg_data_type):
            if host == 'content' or style in ['upload', 'download']:
                headers["Dropbox-API-Arg"] = "string(b)"
            else:
                headers["Content-Type"] = '"application/json"'
        if style == 'upload':
            headers["Content-Type"] = '"application/octet-stream"'

        out('headers := map[string]string{')
        for k, v in sorted(headers.items()):
            out('\t"{}": {},'.format(k, v))
        out('}')
        if fmt_var(route.name) == "Download":
            out('for k, v := range arg.ExtraHeaders { headers[k] = v }')
        if auth != 'noauth' and auth != 'team':
            with self.block('if dbx.Config.AsMemberID != ""'):
                out('headers["Dropbox-API-Select-User"] = dbx.Config.AsMemberID')
        out()

        authed = 'false' if auth == 'noauth' else 'true'
        out('req, err := (*dropbox.Context)(dbx).NewRequest("{}", "{}", {}, "{}", "{}", headers, {})'.format(
            host, style, authed, namespace.name, route.name, body))
        with self.block('if err != nil'):
            out('return')

        out('dbx.Config.LogInfo("req: %v", req)')

        out()

    def _generate_post(self):
        out = self.emit

        out('resp, err := cli.Do(req)')

        with self.block('if err != nil'):
            out('return')
        out()

        out('dbx.Config.LogInfo("resp: %v", resp)')

    def _generate_response(self, route):
        out = self.emit
        style = route.attrs.get('style', 'rpc')
        if style == 'download':
            out('body := []byte(resp.Header.Get("Dropbox-API-Result"))')
            out('content = resp.Body')
        else:
            out('defer resp.Body.Close()')
            with self.block('body, err := ioutil.ReadAll(resp.Body);'
                            'if err != nil'):
                out('return')
            out()

        out('dbx.Config.LogDebug("body: %v", body)')

    def _generate_error_handling(self, route):
        out = self.emit
        style = route.attrs.get('style', 'rpc')
        with self.block('if resp.StatusCode == http.StatusConflict'):
            # If style was download, body was assigned to a header.
            # Need to re-read the response body to parse the error
            if style == 'download':
                out('defer resp.Body.Close()')
                with self.block('body, err = ioutil.ReadAll(resp.Body);'
                                'if err != nil'):
                    out('return')
            out('var apiError %sAPIError' % fmt_var(route.name))
            with self.block('err = json.Unmarshal(body, &apiError);'
                            'if err != nil'):
                out('return')
            out('err = apiError')
            out('return')
        out('var apiError dropbox.APIError')
        with self.block("if resp.StatusCode == http.StatusBadRequest || "
                        "resp.StatusCode == http.StatusInternalServerError"):
            out('apiError.ErrorSummary = string(body)')
            out('err = apiError')
            out('return')
        with self.block('err = json.Unmarshal(body, &apiError);'
                        'if err != nil'):
            out('return')
        out('err = apiError')
        out('return')

    def _generate_result(self, route):
        out = self.emit
        if is_struct_type(route.result_data_type) and \
                route.result_data_type.has_enumerated_subtypes():
            out('var tmp %sUnion' % fmt_var(route.result_data_type.name, export=False))
            with self.block('err = json.Unmarshal(body, &tmp);'
                            'if err != nil'):
                out('return')
            with self.block('switch tmp.Tag'):
                for t in route.result_data_type.get_enumerated_subtypes():
                    with self.block('case "%s":' % t.name, delim=(None, None)):
                        self.emit('res = tmp.%s' % fmt_var(t.name))
        elif not is_void_type(route.result_data_type):
            with self.block('err = json.Unmarshal(body, &res);'
                            'if err != nil'):
                out('return')
            out()
        out('return')
