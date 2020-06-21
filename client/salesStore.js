import {readable} from 'svelte/store'

const sales = JSON.parse(localStorage.getItem('sales') || '{}')

var globalSet

const store = readable(sales, set => {
  globalSet = set
  return () => {}
})

store.update = async (id, fn) => {
  sales[id] = fn(sales[id] || {})
  localStorage.setItem('sales', JSON.stringify(sales))
  globalSet(sales)
}

export default store
