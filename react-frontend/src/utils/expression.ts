/**
 * Evaluates the arithmetic a user can type into an amount field ("1200+300",
 * "3×450", "(1000-120)/2"), so a budget or a split can be entered without
 * mental math. Same parser as android-frontend `src/utils/expression.ts`:
 * numbers, + − × ÷ (or + - * /), parentheses and unary minus. Commas and
 * spaces are ignored. Replaces expr-eval, which is unmaintained and has
 * unfixed prototype-pollution advisories.
 *
 * Returns null for anything that doesn't parse or doesn't end up finite.
 */
export function evaluateAmount(input: string): number | null {
  const source = input
    .replace(/[,\s₹]/g, '')
    .replace(/[×x]/g, '*')
    .replace(/÷/g, '/')
    .replace(/[−–]/g, '-');
  if (!source) return null;

  let pos = 0;

  const peek = () => source[pos];

  function parseExpression(): number {
    let value = parseTerm();
    while (peek() === '+' || peek() === '-') {
      const op = source[pos++];
      const rhs = parseTerm();
      value = op === '+' ? value + rhs : value - rhs;
    }
    return value;
  }

  function parseTerm(): number {
    let value = parseFactor();
    while (peek() === '*' || peek() === '/') {
      const op = source[pos++];
      const rhs = parseFactor();
      value = op === '*' ? value * rhs : value / rhs;
    }
    return value;
  }

  function parseFactor(): number {
    if (peek() === '-') {
      pos++;
      return -parseFactor();
    }
    if (peek() === '+') {
      pos++;
      return parseFactor();
    }
    if (peek() === '(') {
      pos++;
      const value = parseExpression();
      if (peek() !== ')') throw new Error('unclosed parenthesis');
      pos++;
      return value;
    }
    const match = /^(\d+\.?\d*|\.\d+)/.exec(source.slice(pos));
    if (!match) throw new Error('expected a number');
    pos += match[0].length;
    return Number(match[0]);
  }

  try {
    const value = parseExpression();
    if (pos !== source.length || !Number.isFinite(value)) return null;
    // 0.1+0.2 should read 0.3, not 0.30000000000000004
    return Math.round(value * 100) / 100;
  } catch {
    return null;
  }
}

/** Like evaluateAmount, but throws when the input isn't a valid amount. */
export function parseAmount(input: string | number): number {
  const value = evaluateAmount(String(input));
  if (value === null) throw new Error(`not an amount: ${input}`);
  return value;
}
