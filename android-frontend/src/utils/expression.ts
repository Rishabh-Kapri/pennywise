/**
 * Evaluates the arithmetic a user can type into an amount field ("1200+300",
 * "3×450", "(1000-120)/2"), so a budget or a split can be entered without
 * mental math. The web app does the same with expr-eval
 * (react-frontend `useAssignedEditor`); this is a dependency-free subset:
 * numbers, + − × ÷ (or + - * /), parentheses and unary minus. Commas and
 * spaces are ignored.
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

/** True when the text is more than a plain number, i.e. worth previewing. */
export function isExpression(input: string): boolean {
  return /\d\s*[-+*/×÷−]\s*[\d(.]/.test(input.replace(/,/g, '')) || input.includes('(');
}

export const OPERATORS = new Set(['+', '−', '×', '÷']);

/** Applies one key press to an expression, keeping it something the evaluator can read. */
export function pressKey(expression: string, key: string): string {
  if (key === 'back') return expression.slice(0, -1);
  if (key === 'clear') return '';
  if (key === '=') {
    const value = evaluateAmount(expression);
    return value === null ? expression : String(value);
  }
  const last = expression.slice(-1);
  if (OPERATORS.has(key)) {
    if (!expression) return key === '−' ? '−' : '';
    // a second operator replaces the first instead of stacking
    return OPERATORS.has(last) ? expression.slice(0, -1) + key : expression + key;
  }
  if (key === '.') {
    const operand = expression.split(/[+−×÷]/).pop() ?? '';
    if (operand.includes('.')) return expression;
    return expression + (operand ? '.' : '0.');
  }
  // two decimals at most, like the amounts themselves
  const operand = expression.split(/[+−×÷]/).pop() ?? '';
  if (/\.\d{2}$/.test(operand)) return expression;
  if (operand === '0') return expression.slice(0, -1) + key;
  return expression + key;
}
