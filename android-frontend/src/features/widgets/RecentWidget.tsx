import { FlexWidget, ListWidget, TextWidget } from 'react-native-android-widget';
import { formatCurrency } from '../../utils/date';
import { Header, Shell, StatusCard } from './chrome';
import { widgetColors as c } from './widgetTheme';
import type { RecentTransaction, RecentWidgetState } from './widgetData';

export const RECENT_WIDGET_NAME = 'Recent';

function Row({ transaction }: { transaction: RecentTransaction }) {
  const inflow = transaction.amount > 0;
  return (
    <FlexWidget
      style={{
        width: 'match_parent',
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'center',
        paddingVertical: 5
      }}
    >
      <FlexWidget style={{ flex: 1, flexDirection: 'column' }}>
        <TextWidget
          text={transaction.payee}
          maxLines={1}
          truncate="END"
          style={{ fontSize: 13, fontWeight: '500', color: c.text }}
        />
        {transaction.detail ? (
          <TextWidget
            text={transaction.detail}
            maxLines={1}
            truncate="END"
            style={{ fontSize: 10, color: c.faint }}
          />
        ) : (
          <FlexWidget style={{ width: 0, height: 0 }} />
        )}
      </FlexWidget>

      <TextWidget
        text={formatCurrency(transaction.amount, { signed: true })}
        maxLines={1}
        style={{ fontSize: 13, fontWeight: '600', color: inflow ? c.success : c.text, marginLeft: 8 }}
      />
    </FlexWidget>
  );
}

export function RecentWidget({ state }: { state: RecentWidgetState }) {
  if (state.kind !== 'ready') return <StatusCard title="Recent" state={state} />;

  if (state.transactions.length === 0) {
    return (
      <Shell>
        <Header title="Recent" />
        <TextWidget text="No transactions yet" style={{ fontSize: 14, color: c.muted }} />
        <FlexWidget style={{ width: 0, height: 0 }} />
      </Shell>
    );
  }

  return (
    <Shell>
      <Header title="Recent" />
      {/* ListWidget scrolls if the widget is resized shorter than its rows. */}
      <ListWidget style={{ width: 'match_parent', height: 'match_parent', marginTop: 4 }}>
        {state.transactions.map((transaction) => (
          <Row key={transaction.id} transaction={transaction} />
        ))}
      </ListWidget>
    </Shell>
  );
}
