import {readable} from 'svelte/store'

import sales from './salesStore'
import * as toast from './toast'
import {lnurlencode} from './helpers'

function getInitial() {
  return {
    id: null,
    withdraw: null,
    session: localStorage.getItem('auth-session') || null,
    balance: 0,
    files: []
  }
}

var current = getInitial()
var es
var storeSet = () => {}

const account = readable(current, set => {
  storeSet = set
  startEventSource()

  return () => {
    es.close()
  }
})

export default account

account.reset = function() {
  if (es) {
    es.close()
  }

  localStorage.removeItem('auth-session')
  current = getInitial()
  storeSet(current)

  startEventSource()
}

function startEventSource() {
  es = new window.EventSource(
    '/~~~/auth?src=store&session=' + (current.session ? current.session : '')
  )
  es.onerror = e => console.log('accountstore sse error', e.data)

  es.addEventListener('session', e => {
    let session = e.data
    localStorage.setItem('auth-session', session)
    current = {...current, session}
    storeSet(current)
  })
  es.addEventListener('message', e => {
    toast.info(e.data)
  })
  es.addEventListener('buy', e => {
    let [saleId, magnet, fileId, fileName] = JSON.parse(e.data)
    sales.update(saleId, sale => {
      sale.magnet = magnet
      sale.file_id = fileId
      sale.file_name = fileName
      return sale
    })
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
