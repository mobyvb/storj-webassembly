console.log("in run_wasm.js")

const go = new Go();
console.log("in run_wasm.js 2")
WebAssembly.instantiateStreaming(
  fetch("main.wasm").catch((error) => {
    console.log('Error:', error);
  }), go.importObject).then((result) => {
    console.log("calling go.run() WASM")
    go.run(result.instance);
  }).catch((error) => {
    console.log('Error:', error);
  });

