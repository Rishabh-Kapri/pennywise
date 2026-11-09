import assert from 'node:assert';
import { finish, test } from './harness.mts';
import { evaluateAmount, isExpression, pressKey } from '../src/utils/expression.ts';
import { formatPayoffDuration, projectPayoff } from '../src/features/loans/utils/payoffCalculator.ts';

await test('plain amounts', async () => {
  assert.equal(evaluateAmount('1200'), 1200);
  assert.equal(evaluateAmount('1,23,456.5'), 123456.5);
  assert.equal(evaluateAmount('.5'), 0.5);
  assert.equal(evaluateAmount(''), null);
});

await test('arithmetic with precedence and parentheses', async () => {
  assert.equal(evaluateAmount('1200+300'), 1500);
  assert.equal(evaluateAmount('1000-120/2'), 940);
  assert.equal(evaluateAmount('(1000-120)/2'), 440);
  assert.equal(evaluateAmount('3×450'), 1350);
  assert.equal(evaluateAmount('900÷4'), 225);
  assert.equal(evaluateAmount('−50+20'), -30);
  assert.equal(evaluateAmount('0.1+0.2'), 0.3);
});

await test('rejects what does not parse', async () => {
  assert.equal(evaluateAmount('12+'), null);
  assert.equal(evaluateAmount('(12+3'), null);
  assert.equal(evaluateAmount('5/0'), null);
  assert.equal(evaluateAmount('abc'), null);
});

await test('isExpression only flags sums', async () => {
  assert.equal(isExpression('1200'), false);
  assert.equal(isExpression('1,200'), false);
  assert.equal(isExpression('1200+300'), true);
  assert.equal(isExpression('3×4'), true);
});

await test('keypad builds valid expressions', async () => {
  const type = (keys: string[]) => keys.reduce(pressKey, '');
  assert.equal(type(['1', '2', '+', '3']), '12+3');
  assert.equal(type(['1', '+', '×', '2']), '1×2');
  assert.equal(type(['+']), '');
  assert.equal(type(['.', '5', '.', '5']), '0.55');
  assert.equal(type(['1', '.', '2', '5', '9']), '1.25');
  assert.equal(type(['0', '7']), '7');
  assert.equal(type(['1', '2', '+', '3', '=']), '15');
  assert.equal(type(['1', '2', 'back']), '1');
});

await test('loan payoff projection', async () => {
  const noInterest = projectPayoff({ currentBalance: 1200, interestRate: 0, monthlyPayment: 100 });
  assert.equal(noInterest.months, 12);
  assert.equal(noInterest.totalInterest, 0);
  const loan = projectPayoff({ currentBalance: 100000, interestRate: 12, monthlyPayment: 5000 });
  assert.equal(Math.round(loan.nextInterest), 1000);
  assert.ok(loan.paysOff && loan.months === 23, `months ${loan.months}`);
  const faster = projectPayoff({ currentBalance: 100000, interestRate: 12, monthlyPayment: 5000, extraMonthlyPayment: 5000 });
  assert.ok(faster.months < loan.months && faster.totalInterest < loan.totalInterest);
  assert.equal(projectPayoff({ currentBalance: 100000, interestRate: 12, monthlyPayment: 900 }).paysOff, false);
  assert.equal(formatPayoffDuration(14), '1 year 2 months');
});

finish();
