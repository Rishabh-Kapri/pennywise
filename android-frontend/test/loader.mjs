import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// Lets Node import the app's TypeScript sources directly, without a bundler:
//  - swaps the React Native AsyncStorage module for an in-memory stub
//  - resolves the bundler-style extensionless relative imports the app uses
const STUB = new URL('./stubs/asyncStorage.mjs', import.meta.url).href;

export async function resolve(specifier, context, next) {
  if (specifier === '@react-native-async-storage/async-storage') {
    return { url: STUB, shortCircuit: true };
  }
  if (specifier.startsWith('.') && !/\.(m|c)?(j|t)sx?$/.test(specifier)) {
    for (const ext of ['.ts', '.tsx', '.js']) {
      const candidate = new URL(specifier + ext, context.parentURL);
      if (existsSync(fileURLToPath(candidate))) return next(specifier + ext, context);
    }
  }
  return next(specifier, context);
}
