import bech32 from 'bech32'

export function lnurlencode(url) {
  return bech32.encode(
    'lnurl',
    bech32.toWords(url.split('').map(c => c.charCodeAt(0))),
    1500
  )
}
