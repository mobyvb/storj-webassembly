'use strict';

window.initApp = function initApp(goApp) {
  const access = goApp.funcs.access('addr', 'api-key', 'pwd')
  console.log(access)
 }


const go = new Go()
go.argv.push('initApp')

WebAssembly.instantiateStreaming(fetch('main.wasm'), go.importObject).then((result) => {
  go.run(result.instance);
})
