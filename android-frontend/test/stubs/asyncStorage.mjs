// In-memory stand-in for @react-native-async-storage/async-storage.
const mem = new Map();

export default {
  getItem: async (k) => (mem.has(k) ? mem.get(k) : null),
  setItem: async (k, v) => void mem.set(k, v),
  removeItem: async (k) => void mem.delete(k),
  multiRemove: async (ks) => ks.forEach((k) => mem.delete(k)),
  __mem: mem
};
