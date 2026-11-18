import 'react-native-gesture-handler';
import { useCallback, useEffect, useState } from 'react';
import { Provider } from 'react-redux';
import { DarkTheme, DefaultTheme, NavigationContainer } from '@react-navigation/native';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { StatusBar } from 'expo-status-bar';
import { ChartPie, House, ReceiptText, Wallet } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from './app/hooks';
import { store } from './app/store';
import { LoadingStateView } from './components/LoadingStateView';
import { colors, scheme } from './theme';
import { FloatingTabBar } from './components/FloatingTabBar';
import { useReloadOnSchemeChange } from './utils/schemeReload';
import { apiClient } from './utils/api';
import type { AppTabParamList, AuthStackParamList, RootStackParamList } from './navigation/types';
import { hydrateAuth } from './features/auth/store/authSlice';
import { LoginScreen } from './features/auth/screens/LoginScreen';
import { fetchAllBudgets } from './features/budget/store/budgetSlice';
import { BudgetOnboardingScreen } from './features/budget/screens/BudgetOnboardingScreen';
import { DashboardScreen } from './features/dashboard/screens/DashboardScreen';
import { BudgetScreen } from './features/budget/screens/BudgetScreen';
import { CategoryScreen } from './features/category/screens/CategoryScreen';
import { TransactionsScreen } from './features/transactions/screens/TransactionsScreen';
import { PayeesScreen } from './features/payees/screens/PayeesScreen';
import { LoansScreen } from './features/loans/screens/LoansScreen';
import { ReportsScreen } from './features/reports/screens/ReportsScreen';
import { SettingsScreen } from './features/settings/screens/SettingsScreen';
import { DocumentsScreen } from './features/documents/screens/DocumentsScreen';
import { BootSplash, holdNativeSplash } from './components/BootSplash';
import { ReconcileScreen } from './features/reconcile/screens/ReconcileScreen';
import { StatementScreen } from './features/reconcile/screens/StatementScreen';
import { TagsScreen } from './features/tags/screens/TagsScreen';
import { JobsScreen } from './features/jobs/screens/JobsScreen';
import { PipelinesScreen } from './features/jobs/screens/PipelinesScreen';
import { AIScreen } from './features/ai/screens/AIScreen';
import { WebSocketProvider } from './features/websocket/WebSocketProvider';
import { AgentChat } from './features/agent/components/AgentChat';
import { registerDevicePushToken } from './features/notifications/push';
import { setupLocationSnap } from './features/notifications/locationSnapTask';
import { flushSmsOutbox } from './features/sms/smsForwarder';
import { ensureSmsPermission } from './features/sms/smsPermission';

// defines the background notification task; must happen at module scope so the
// task exists when Android wakes the app headlessly
setupLocationSnap();

const AuthStack = createNativeStackNavigator<AuthStackParamList>();
const RootStack = createNativeStackNavigator<RootStackParamList>();
const Tab = createBottomTabNavigator<AppTabParamList>();

const baseNavigationTheme = scheme === 'light' ? DefaultTheme : DarkTheme;
const navigationTheme = {
  ...baseNavigationTheme,
  colors: {
    ...baseNavigationTheme.colors,
    primary: colors.primary,
    background: colors.background,
    card: colors.surface,
    text: colors.text,
    border: colors.border,
    notification: colors.danger
  }
};

function AuthNavigator() {
  return (
    <AuthStack.Navigator screenOptions={{ headerShown: false }}>
      <AuthStack.Screen name="Login" component={LoginScreen} />
    </AuthStack.Navigator>
  );
}

const TAB_ICON_SIZE = 20;

const TABS = {
  Dashboard: { label: 'Home', icon: (color: string) => <House color={color} size={TAB_ICON_SIZE} /> },
  Budget: { label: 'Budget', icon: (color: string) => <Wallet color={color} size={TAB_ICON_SIZE} /> },
  Transactions: { label: 'Transactions', icon: (color: string) => <ReceiptText color={color} size={TAB_ICON_SIZE} /> },
  Reports: { label: 'Reports', icon: (color: string) => <ChartPie color={color} size={TAB_ICON_SIZE} /> }
} satisfies Record<keyof AppTabParamList, unknown>;

function AppTabs() {
  return (
    <Tab.Navigator
      initialRouteName="Dashboard"
      tabBar={(props) => <FloatingTabBar {...props} tabs={TABS} />}
      screenOptions={{ headerShown: false, animation: 'fade' }}
    >
      <Tab.Screen name="Dashboard" component={DashboardScreen} />
      <Tab.Screen name="Budget" component={BudgetScreen} />
      <Tab.Screen name="Transactions" component={TransactionsScreen} />
      <Tab.Screen name="Reports" component={ReportsScreen} />
    </Tab.Navigator>
  );
}

function AppNavigator() {
  return (
    <>
      <WebSocketProvider />
      <RootStack.Navigator screenOptions={{ headerShown: false, animation: 'slide_from_right' }}>
        <RootStack.Screen name="Main" component={AppTabs} />
        <RootStack.Screen name="Settings" component={SettingsScreen} />
        <RootStack.Screen name="Documents" component={DocumentsScreen} />
        <RootStack.Screen name="Reconcile" component={ReconcileScreen} />
        <RootStack.Screen name="Statement" component={StatementScreen} />
        <RootStack.Screen name="Tags" component={TagsScreen} />
        <RootStack.Screen name="Jobs" component={JobsScreen} />
        <RootStack.Screen name="Pipelines" component={PipelinesScreen} />
        <RootStack.Screen name="AI" component={AIScreen} />
        <RootStack.Screen name="Loans" component={LoansScreen} />
        <RootStack.Screen name="Payees" component={PayeesScreen} />
        <RootStack.Screen name="Penny" component={AgentChat} />
        <RootStack.Screen name="Category" component={CategoryScreen} />
      </RootStack.Navigator>
    </>
  );
}

function RootContent() {
  const dispatch = useAppDispatch();
  const auth = useAppSelector((state) => state.auth);
  const budgets = useAppSelector((state) => state.budgets);

  useEffect(() => {
    dispatch(hydrateAuth());
  }, [dispatch]);

  useEffect(() => {
    if (auth.hydrated && auth.isAuthenticated) {
      dispatch(fetchAllBudgets());
      // Sequential: Android drops a permission request made while another
      // permission dialog is still showing.
      void registerDevicePushToken()
        .then(ensureSmsPermission)
        .catch((error) => console.log('[sms] permission request failed', error))
        .then(flushSmsOutbox);
    }
  }, [auth.hydrated, auth.isAuthenticated, dispatch]);

  if (!auth.hydrated) {
    return <LoadingStateView label="Restoring session" />;
  }

  if (!auth.isAuthenticated) {
    return <AuthNavigator />;
  }

  if (budgets.loading === 'pending' && !budgets.selectedBudget) {
    return <LoadingStateView label="Loading budget" />;
  }

  if (!budgets.selectedBudget) {
    return <BudgetOnboardingScreen />;
  }

  return <AppNavigator />;
}

// Keep the native splash up until BootSplash has drawn its copy of the mark.
holdNativeSplash();

function AppShell() {
  const [splashDone, setSplashDone] = useState(false);
  // ready = RootContent is past its loading views
  const ready = useAppSelector(
    (state) =>
      state.auth.hydrated &&
      (!state.auth.isAuthenticated ||
        Boolean(state.budgets.selectedBudget) ||
        state.budgets.loading === 'success' ||
        state.budgets.loading === 'error')
  );
  const finishSplash = useCallback(() => setSplashDone(true), []);
  useReloadOnSchemeChange();

  useEffect(() => {
    apiClient
      .probeRoot()
      .then(({ status, body, url }) => {
        console.log(`[api-probe] GET ${url} -> ${status}: ${body}`);
      })
      .catch((error: unknown) => {
        console.log('[api-probe] GET /api failed', error);
      });
  }, []);

  return (
    <SafeAreaProvider>
      <StatusBar style={scheme === 'light' ? 'dark' : 'light'} />
      <NavigationContainer theme={navigationTheme}>
        <RootContent />
      </NavigationContainer>
      {splashDone ? null : <BootSplash ready={ready} onDone={finishSplash} />}
    </SafeAreaProvider>
  );
}

export default function App() {
  return (
    <Provider store={store}>
      <AppShell />
    </Provider>
  );
}
