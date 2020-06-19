import {readable} from 'svelte/store'

import * as toast from './toast'
import {lnurlencode} from './helpers'

const initial = {
  id: null,
  withdraw: null,
  session: window.localStorage.getItem('auth-session') || null,
  balance: 0,
  files: []
}

var current = {...initial}
var es
var storeSet = () => {}

const account = readable(initial, set => {
  storeSet = set
  startEventSource()

  return () => {
    es.close()
  }
})

export default account

account.reset = function() {
  window.localStorage.removeItem('auth-session')
  current = {...initial}
  storeSet(current)

  if (es) {
    es.close()
    window.localStorage.removeItem('auth-session')
    current = {...initial}
    storeSet(current)
  }

  startEventSource()
}

function startEventSource() {
  es = new window.EventSource(
    '/~~~/auth?src=store&session=' + (current.session ? current.session : '')
  )
  es.onerror = e => console.log('accountstore sse error', e.data)

  es.addEventListener('session', e => {
    let session = e.data
    window.localStorage.setItem('auth-session', session)
    current = {...current, session}
    storeSet(current)
  })
  es.addEventListener('id', e => {
    let id = e.data
    current = {...current, id}
    storeSet(current)
  })
  es.addEventListener('balance', e => {
    let balance = parseInt(e.data)
    current = {...current, balance}
    storeSet(current)
  })
  es.addEventListener('wallet-key', e => {
    let key = e.data
    let withdraw = lnurlencode(
      `https://lnpay.co/v1/wallet/${key}/lnurl-process?ott=ui-w`
    )
    current = {...current, withdraw}
    storeSet(current)
  })
  es.addEventListener('error', e => {
    toast.error(e.data)
  })
}
