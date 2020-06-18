import bech32 from 'bech32'

export function lnurlencode(url) {
  bech32.encode(
    'lnurl',
    bech32.toWords(url.split('').map(c => c.charCodeAt(0)))
  )
}
