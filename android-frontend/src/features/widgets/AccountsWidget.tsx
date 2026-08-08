import { FlexWidget, TextWidget } from 'react-native-android-widget';
import { formatCurrency } from '../../utils/date';
import { Header, Shell, StatusCard } from './chrome';
import { widgetColors as c } from './widgetTheme';
import type { AccountsWidgetState } from './widgetData';

export const ACCOUNTS_WIDGET_NAME = 'Accounts';

export function AccountsWidget({ state }: { state: AccountsWidgetState }) {
  if (state.kind !== 'ready') return <StatusCard title="Accounts" state={state} />;

  return (
    <Shell>
      <Header
        title="Net worth"
        trailing={state.accountCount === 1 ? '1 account' : `${state.accountCount} accounts`}
      />

      <TextWidget
        text={formatCurrency(state.total)}
        maxLines={1}
        style={{
          fontSize: 30,
          fontWeight: 'bold',
          color: state.total < 0 ? c.danger : c.text,
          adjustsFontSizeToFit: true
        }}
      />

      <FlexWidget style={{ flexDirection: 'row', width: 'match_parent', flexGap: 8 }}>
        <FlexWidget
          style={{
            flex: 1,
            flexDirection: 'column',
            backgroundColor: c.successMuted,
            borderRadius: 14,
            paddingVertical: 6,
            paddingHorizontal: 10
          }}
        >
          <TextWidget text="Cash" style={{ fontSize: 10, color: c.muted }} />
          <TextWidget
            text={formatCurrency(state.cash)}
            maxLines={1}
            truncate="END"
            style={{ fontSize: 13, fontWeight: '600', color: c.success }}
          />
        </FlexWidget>

        <FlexWidget
          style={{
            flex: 1,
            flexDirection: 'column',
            backgroundColor: c.warningMuted,
            borderRadius: 14,
            paddingVertical: 6,
            paddingHorizontal: 10
          }}
        >
          <TextWidget text="Debt" style={{ fontSize: 10, color: c.muted }} />
          <TextWidget
            text={formatCurrency(state.debt)}
            maxLines={1}
            truncate="END"
            style={{ fontSize: 13, fontWeight: '600', color: c.warning }}
          />
        </FlexWidget>
      </FlexWidget>
    </Shell>
  );
}
