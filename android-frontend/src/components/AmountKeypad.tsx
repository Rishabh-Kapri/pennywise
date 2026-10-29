import { useState } from 'react';
import { Pressable, StyleSheet, View } from 'react-native';
import { Delete } from 'lucide-react-native';
import { AppText } from './AppText';
import { Button } from './Button';
import { Sheet } from './Sheet';
import { colors, radii, spacing } from '../theme';
import { OPERATORS, evaluateAmount, isExpression, pressKey } from '../utils/expression';

export { pressKey };
import { formatCurrency } from '../utils/date';

const KEYS = [
  ['7', '8', '9', '÷'],
  ['4', '5', '6', '×'],
  ['1', '2', '3', '−'],
  ['.', '0', 'back', '+']
] as const;

/** Calculator keys for amount entry; no system keyboard, so nothing gets covered. */
export function AmountKeypad({ onKey, onDone }: { onKey: (key: string) => void; onDone?: () => void }) {
  return (
    <View style={styles.pad}>
      {KEYS.map((row, index) => (
        <View key={index} style={styles.row}>
          {row.map((key) => {
            const operator = OPERATORS.has(key);
            return (
              <Pressable
                key={key}
                accessibilityRole="button"
                accessibilityLabel={key === 'back' ? 'Delete' : key}
                onPress={() => onKey(key)}
                onLongPress={key === 'back' ? () => onKey('clear') : undefined}
                style={({ pressed }) => [styles.key, operator && styles.operatorKey, pressed && styles.keyPressed]}
              >
                {key === 'back' ? (
                  <Delete size={20} color={colors.text} />
                ) : (
                  <AppText style={[styles.keyText, operator && styles.operatorText]}>{key}</AppText>
                )}
              </Pressable>
            );
          })}
        </View>
      ))}
      {onDone ? (
        <View style={styles.row}>
          <Pressable
            accessibilityRole="button"
            onPress={() => onKey('clear')}
            style={({ pressed }) => [styles.key, pressed && styles.keyPressed]}
          >
            <AppText variant="caption" weight="semibold" muted>
              Clear
            </AppText>
          </Pressable>
          <Pressable
            accessibilityRole="button"
            onPress={() => onKey('=')}
            style={({ pressed }) => [styles.key, styles.operatorKey, pressed && styles.keyPressed]}
          >
            <AppText style={[styles.keyText, styles.operatorText]}>=</AppText>
          </Pressable>
          <Pressable
            accessibilityRole="button"
            onPress={onDone}
            style={({ pressed }) => [styles.key, styles.doneKey, pressed && styles.keyPressed]}
          >
            <AppText weight="semibold" style={styles.doneText}>
              Done
            </AppText>
          </Pressable>
        </View>
      ) : null}
    </View>
  );
}

/** The typed expression with its running result underneath once it's a sum. */
export function ExpressionPreview({ expression, placeholder = '0' }: { expression: string; placeholder?: string }) {
  const value = evaluateAmount(expression);
  return (
    <View style={styles.preview}>
      <AppText variant="title" tabular numberOfLines={1} adjustsFontSizeToFit style={!expression && styles.placeholder}>
        {expression || placeholder}
      </AppText>
      {isExpression(expression) ? (
        <AppText variant="caption" tabular tone={value === null ? 'danger' : 'primary'}>
          {value === null ? 'Not a valid sum' : `= ${formatCurrency(value)}`}
        </AppText>
      ) : null}
    </View>
  );
}

/**
 * Sheet for entering one amount with arithmetic, e.g. a category's assigned
 * amount ("1200+300" tops a budget up without mental math).
 */
export function AmountSheet({
  visible,
  title,
  subtitle,
  initialValue,
  onSave,
  onClose
}: {
  visible: boolean;
  title: string;
  subtitle?: string;
  initialValue: number;
  onSave: (value: number) => void;
  onClose: () => void;
}) {
  return (
    <Sheet visible={visible} title={title} subtitle={subtitle} onClose={onClose}>
      {visible ? <AmountSheetBody initialValue={initialValue} onSave={onSave} /> : null}
    </Sheet>
  );
}

function AmountSheetBody({ initialValue, onSave }: { initialValue: number; onSave: (value: number) => void }) {
  const [expression, setExpression] = useState(initialValue ? String(initialValue) : '');
  // the first digit replaces the prefilled amount; an operator extends it
  const [fresh, setFresh] = useState(true);

  const onKey = (key: string) => {
    setExpression((current) => {
      const base = fresh && /^[\d.]$/.test(key) ? '' : current;
      return pressKey(base, key);
    });
    setFresh(false);
  };

  const save = () => {
    const value = evaluateAmount(expression || '0');
    if (value !== null) onSave(value);
  };

  return (
    <View style={styles.sheetBody}>
      <View style={styles.display}>
        <ExpressionPreview expression={expression} />
      </View>
      <AmountKeypad onKey={onKey} />
      <Button onPress={save} disabled={evaluateAmount(expression || '0') === null}>
        Save
      </Button>
    </View>
  );
}

const styles = StyleSheet.create({
  pad: {
    gap: spacing.sm
  },
  row: {
    flexDirection: 'row',
    gap: spacing.sm
  },
  key: {
    flex: 1,
    height: 52,
    borderRadius: radii.md,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceStrong
  },
  operatorKey: {
    backgroundColor: colors.primaryMuted
  },
  doneKey: {
    flex: 2,
    backgroundColor: colors.primary
  },
  keyPressed: {
    opacity: 0.6
  },
  keyText: {
    fontSize: 22,
    lineHeight: 28,
    fontWeight: '500'
  },
  operatorText: {
    color: colors.primary
  },
  doneText: {
    color: colors.onPrimary
  },
  preview: {
    alignItems: 'flex-end',
    gap: 2
  },
  placeholder: {
    color: colors.faint
  },
  sheetBody: {
    gap: spacing.lg,
    paddingBottom: spacing.sm
  },
  display: {
    minHeight: 64,
    justifyContent: 'center',
    borderRadius: radii.md,
    backgroundColor: colors.surfaceStrong,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md
  }
});
