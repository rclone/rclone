var rc;
var rcValidResolve
var rcValid = new Promise(resolve => {
  rcValidResolve = resolve
});

var script = document.createElement('script');
script.src = "wasm_exec.js";
script.onload = function () {
  if (!WebAssembly.instantiateStreaming) { // polyfill
    WebAssembly.instantiateStreaming = async (resp, importObject) => {
      const source = await (await resp).arrayBuffer();
      return await WebAssembly.instantiate(source, importObject);
    };
  }
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch("rclone.wasm"), go.importObject).then((result) => {
    go.run(result.instance);
  });
  
};
document.head.appendChild(script);

rcValid.then(() => {
  // Some examples of using the rc call
  //
  // The rc call takes two parameters, method and input object and
  // returns an output object.
  //
  // If the output object has an "error" and a "status" then it is an
  // error (it would be nice to signal this out of band).
  console.log("core/version", rc("core/version", null))
  console.log("rc/noop", rc("rc/noop", {"string":"one",number:2}))
  console.log("operations/mkdir", rc("operations/mkdir", {"fs":":memory:","remote":"bucket"}))
  console.log("operations/list", rc("operations/list", {"fs":":memory:","remote":"bucket"}))
})
