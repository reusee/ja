import reqwest from 'reqwest'
import {api_server_address} from './config'

export function call(method, args, ok_cb = () => {}) {
  let retry = 10;
  function do_call() {
    reqwest({
      method: 'POST',
      data: JSON.stringify(args),
      crossOrigin: true,
      url: api_server_address + '/' + method,
      type: 'json',
      success: (resp) => {
        if (resp.status != "ok") {
          console.log('call not ok =>', resp.status);
        } else {
          ok_cb(resp.result);
        }
      },
      error: (e) => {
        if (retry > 0) {
          retry--;
          do_call();
        }
        console.log('call error =>', e);
      },
    });
  }
  do_call();
}
