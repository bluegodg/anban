export function settleOptionalLoads(loaders = []) {
  return Promise.allSettled(loaders.map((load) => Promise.resolve().then(load)));
}
